// Package workercmd provides the report-service worker command.
package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	"github.com/auraedu/report-service/internal/adapters/postgres"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
)

const service = "report-service-worker"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		log.Error("failed to open database", "err", err)
		os.Exit(1)
	}

	nc, js, err := connectNATS(log)
	if err != nil {
		log.Error("failed to connect to NATS", "err", err)
		database.Close()
		os.Exit(1)
	}

	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream", "err", err)
		nc.Close()
		database.Close()
		os.Exit(1)
	}

	repo := postgres.NewRepository(database)
	svc := application.NewService(repo, application.WithFeatureGate(featureGates(log)))

	consumer := newConsumer(js, svc, log)
	if err := consumer.Start(ctx); err != nil {
		log.Error("failed to start consumer", "err", err)
		nc.Close()
		database.Close()
		os.Exit(1)
	}
	defer func() {
		if err := consumer.Stop(); err != nil {
			log.Error("consumer stop error", "err", err)
		}
	}()
	defer database.Close()

	log.Info(service+" started", "version", version)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nc.Close()
	log.Info(service + " stopped")
}

func openDB(ctx context.Context) (*db.DB, error) {
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	return db.Open(ctx, db.Config{
		DSN:        dsn,
		Migrations: "migrations",
	})
}

func connectNATS(log *slog.Logger) (*nats.Conn, eventbus.JetStreamContext, error) {
	natsURL := config.Getenv("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("nats jetstream: %w", err)
	}
	log.Info("connected to NATS", "url", natsURL)
	return nc, js, nil
}

// featureGates loads the feature registry so the worker can skip tenants whose
// report_cards feature is disabled (same pattern as website-service).
func featureGates(log *slog.Logger) flags.Gate {
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	reg, err := flags.LoadYAML(path)
	if err != nil {
		log.Warn("failed to load feature registry; all features disabled", "path", path, "err", err)
		return flags.NewStaticSnapshot()
	}
	return reg.SnapshotFromRegistry()
}

type consumer struct {
	js   eventbus.JetStreamContext
	svc  *application.Service
	subs []*nats.Subscription
	log  *slog.Logger
}

func newConsumer(js eventbus.JetStreamContext, svc *application.Service, log *slog.Logger) *consumer {
	return &consumer{js: js, svc: svc, log: log}
}

func (c *consumer) Start(ctx context.Context) error {
	for _, eventType := range []string{"assessment.score_recorded.v1", "attendance.marked.v1"} {
		subject := eventbus.Subject("AURA", eventType)
		sub, err := c.js.Subscribe(subject, c.handleMsg,
			nats.Durable("report-worker-"+eventType),
			nats.ManualAck(),
			nats.AckWait(30*time.Second),
		)
		if err != nil {
			return fmt.Errorf("subscribe to %s: %w", subject, err)
		}
		c.subs = append(c.subs, sub)
		c.log.Info("subscribed", "subject", subject)
	}
	_ = ctx
	return nil
}

func (c *consumer) Stop() error {
	for _, sub := range c.subs {
		if sub != nil {
			if err := sub.Unsubscribe(); err != nil {
				return err
			}
		}
	}
	c.subs = nil
	return nil
}

func (c *consumer) handleMsg(msg *nats.Msg) {
	var event tenancy.CloudEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.log.Error("unmarshal message", "err", err)
		if nerr := msg.Nak(); nerr != nil {
			c.log.Error("failed to nak message", "err", nerr)
		}
		return
	}
	if err := event.Validate(); err != nil {
		c.log.Error("invalid cloudevent", "err", err)
		if nerr := msg.Nak(); nerr != nil {
			c.log.Error("failed to nak message", "err", nerr)
		}
		return
	}

	// Tenant context is required for Postgres RLS (app.tenant_id).
	ctx := tenancy.WithContext(context.Background(), event.TenantContext())
	err := c.materialize(ctx, event)
	switch {
	case err == nil:
		c.ack(msg)
	case errors.Is(err, domain.ErrValidation) || errors.Is(err, domain.ErrMissingTenant):
		// Poison message: retrying cannot fix a malformed event. Ack to drop it.
		c.log.Error("dropping invalid event", "type", event.Type, "id", event.ID, "err", err)
		c.ack(msg)
	default:
		c.log.Error("materialization failed; will redeliver", "type", event.Type, "id", event.ID, "err", err)
		if nerr := msg.Nak(); nerr != nil {
			c.log.Error("failed to nak message", "err", nerr)
		}
	}
}

func (c *consumer) ack(msg *nats.Msg) {
	if err := msg.Ack(); err != nil {
		c.log.Error("failed to ack message", "err", err)
	}
}

// materialize dispatches one event to the matching use case. The report_cards
// feature gate is enforced inside the service (disabled tenants are skipped).
func (c *consumer) materialize(ctx context.Context, event tenancy.CloudEvent) error {
	switch event.Type {
	case "assessment.score_recorded.v1":
		var data struct {
			StudentID    string   `json:"student_id"`
			SubjectID    string   `json:"subject_id"`
			AssessmentID string   `json:"assessment_id"`
			ScoreID      string   `json:"score_id"`
			TermID       string   `json:"term_id"`
			Score        float64  `json:"score"`
			MaxScore     *float64 `json:"max_score"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("%w: score_recorded data: %v", domain.ErrValidation, err)
		}
		return c.svc.MaterializeScore(ctx, application.ScoreRecordedInput{
			EventID:      event.ID,
			TenantID:     event.TenantID,
			StudentID:    data.StudentID,
			SubjectID:    data.SubjectID,
			AssessmentID: data.AssessmentID,
			ScoreID:      data.ScoreID,
			TermID:       data.TermID,
			Score:        data.Score,
			MaxScore:     data.MaxScore,
		})
	case "attendance.marked.v1":
		var data struct {
			StudentID string `json:"student_id"`
			Date      string `json:"date"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("%w: attendance.marked data: %v", domain.ErrValidation, err)
		}
		return c.svc.MaterializeAttendance(ctx, application.AttendanceMarkedInput{
			EventID:   event.ID,
			TenantID:  event.TenantID,
			StudentID: data.StudentID,
			Date:      data.Date,
			Status:    data.Status,
		})
	default:
		return fmt.Errorf("%w: unsupported event type %q", domain.ErrValidation, event.Type)
	}
}

// Run starts the report-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
