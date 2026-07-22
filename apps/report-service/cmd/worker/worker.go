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
	"strings"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	svcevents "github.com/auraedu/report-service/internal/adapters/events"
	"github.com/auraedu/report-service/internal/adapters/pdf"
	"github.com/auraedu/report-service/internal/adapters/postgres"
	"github.com/auraedu/report-service/internal/adapters/storage"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

const service = "report-service-worker"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	ctx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	shutdownTelemetry, err := observ.InitTracing(service, version)
	if err != nil {
		return fmt.Errorf("initialize worker telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush report worker telemetry", "err", err)
		}
	}()
	database, err := openDB(ctx)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	nc, js, err := connectNATS(log)
	if err != nil {
		return err
	}
	defer nc.Close()

	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return fmt.Errorf("ensure NATS stream: %w", err)
	}

	repo := postgres.NewRepository(database)
	reportStorage, err := initStorage()
	if err != nil {
		return fmt.Errorf("initialize report storage: %w", err)
	}
	publisher := svcevents.NewPublisher(eventbus.NewPublisher(js))
	svc := application.NewService(repo,
		application.WithFeatureGate(featureGates(log)),
		application.WithPDFGenerator(pdf.NewGenerator()),
		application.WithStorage(reportStorage),
		application.WithPublisher(publisher),
	)

	metrics := observ.NewWorkerMetrics(service, "materialize-report")
	consumer := newConsumer(js, svc, log, metrics)
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("start consumer: %w", err)
	}
	defer func() {
		if err := consumer.Stop(); err != nil {
			log.Error("consumer stop error", "err", err)
		}
	}()
	generationDone := make(chan struct{})
	go func() {
		defer close(generationDone)
		runGenerationQueue(ctx, svc, repo, publisher, log, observ.NewWorkerMetrics(service, "generate-report", "outbox-publish"))
	}()

	log.Info(service+" started", "version", version)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelWorker()
	<-generationDone

	log.Info(service + " stopped")
	return nil
}

func initStorage() (ports.ReportStorage, error) {
	backend := config.Getenv("REPORT_STORAGE_BACKEND", "local")
	if config.Getenv("ENVIRONMENT", "development") == "production" && backend != "cloudinary" {
		return nil, errors.New("production report storage must use cloudinary")
	}
	switch backend {
	case "local":
		return storage.NewLocal(config.Getenv("REPORT_OUTPUT_DIR", application.DefaultReportOutputDir))
	case "cloudinary":
		cloudURL, err := config.MustGetenv("CLOUDINARY_URL")
		if err != nil {
			return nil, err
		}
		return storage.NewCloudinary(cloudURL)
	default:
		return nil, errors.New("unsupported REPORT_STORAGE_BACKEND")
	}
}

func runGenerationQueue(
	ctx context.Context,
	svc *application.Service,
	repo ports.OutboxRepository,
	publisher *svcevents.Publisher,
	log *slog.Logger,
	metrics *observ.WorkerMetrics,
) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Bound generation work per tick so a continuously busy renderer cannot
			// starve publication events waiting in the transactional outbox.
			for range 25 {
				started := time.Now()
				processed, err := svc.ProcessNextGeneration(ctx, 2*time.Minute, 5)
				metrics.Observe(ctx, "generate-report", started, err)
				if err != nil {
					log.Error("report generation job failed", "err", err)
				}
				if !processed || err != nil {
					break
				}
			}
			if err := dispatchReportOutbox(ctx, repo, publisher, log, metrics); err != nil {
				log.Error("report outbox dispatch failed", "err", err)
			}
		}
	}
}

func dispatchReportOutbox(
	ctx context.Context,
	repo ports.OutboxRepository,
	publisher *svcevents.Publisher,
	log *slog.Logger,
	metrics *observ.WorkerMetrics,
) error {
	items, err := repo.ClaimPendingReportEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkReportEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkReportEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("report event publish deferred", "outbox_id", item.ID, "event_type", item.EventType, "err", err)
			continue
		}
		if err := repo.MarkReportEventPublished(ctx, item.ID); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
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
	// Static registry snapshot: plan defaults baked into the deploy. It stays
	// the fallback when tenant-service is unreachable.
	fallback := flags.NewStaticSnapshot()
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	reg, err := flags.LoadYAML(path)
	if err != nil {
		log.Warn("failed to load feature registry; all features disabled", "path", path, "err", err)
	} else {
		fallback = reg.SnapshotFromRegistry()
	}

	return flags.NewRuntimeGate(config.Getenv("SERVICE_TENANT_URL", ""), fallback, log)
}

type consumer struct {
	js      eventbus.JetStreamContext
	svc     *application.Service
	subs    []*eventbus.Subscription
	log     *slog.Logger
	metrics *observ.WorkerMetrics
}

func newConsumer(js eventbus.JetStreamContext, svc *application.Service, log *slog.Logger, metrics ...*observ.WorkerMetrics) *consumer {
	var workerMetrics *observ.WorkerMetrics
	if len(metrics) > 0 {
		workerMetrics = metrics[0]
	}
	return &consumer{js: js, svc: svc, log: log, metrics: workerMetrics}
}

func (c *consumer) Start(ctx context.Context) error {
	for _, eventType := range []string{"assessment.score_recorded.v1", "attendance.marked.v1"} {
		subject := eventbus.Subject("AURA", eventType)
		sub, err := eventbus.Subscribe(
			c.js,
			eventbus.EventStreamName,
			"report-worker-"+strings.ReplaceAll(eventType, ".", "-"),
			eventType,
			c.handleEvent,
			nil,
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

func (c *consumer) handleEvent(ctx context.Context, event tenancy.CloudEvent) error {
	started := time.Now()
	err := c.materialize(ctx, event)
	c.metrics.Observe(ctx, "materialize-report", started, err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrValidation) || errors.Is(err, domain.ErrMissingTenant):
		// Poison message: retrying cannot fix a malformed event.
		c.log.Error("dropping invalid event", "type", event.Type, "id", event.ID, "err", err)
		return nil
	default:
		c.log.Error("materialization failed; will redeliver", "type", event.Type, "id", event.ID, "err", err)
		return err
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
			return fmt.Errorf("%w: score_recorded data: %w", domain.ErrValidation, err)
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
			return fmt.Errorf("%w: attendance.marked data: %w", domain.ErrValidation, err)
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
	return run()
}
