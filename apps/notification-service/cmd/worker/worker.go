// Package workercmd provides the notification-service worker command.
package workercmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
)

const service = "notification-service-worker"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	if err := run(log); err != nil {
		log.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		return err
	}
	defer database.Close()

	nc, js, err := connectNATS(log)
	if err != nil {
		return err
	}
	defer nc.Close()

	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}

	gates := featureGates(log)
	messageRepo := postgres.NewMessageRepository(database)
	templateRepo := postgres.NewTemplateRepository(database)
	subscriptionRepo := postgres.NewSubscriptionRepository(database)
	svc := application.NewService(messageRepo, templateRepo, subscriptionRepo,
		application.WithNotifiers(notifier.Registry()),
		application.WithFeatureGate(gates),
	)

	consumer := newConsumer(js, log, svc)
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("start consumer: %w", err)
	}
	defer func() {
		if err := consumer.Stop(); err != nil {
			log.Error("consumer stop error", "err", err)
		}
	}()

	log.Info(service+" started", "version", version)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

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
	subs []*nats.Subscription
	log  *slog.Logger
	svc  *application.Service
}

func newConsumer(js eventbus.JetStreamContext, log *slog.Logger, svc *application.Service) *consumer {
	return &consumer{js: js, log: log, svc: svc}
}

func (c *consumer) Start(ctx context.Context) error {
	for _, eventType := range []string{
		"payment.received.v1",
		"invoice.created.v1",
		"attendance.marked.v1",
		"assessment.score_recorded.v1",
		"report.published.v1",
	} {
		subject := eventbus.Subject("AURA", eventType)
		sub, err := c.js.Subscribe(subject, c.handleMsg,
			nats.Durable("notification-worker-"+eventType),
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
		_ = msg.Nak()
		return
	}
	if err := event.Validate(); err != nil {
		c.log.Error("invalid cloudevent", "err", err)
		_ = msg.Nak()
		return
	}
	ctx := tenancy.WithContext(context.Background(), event.TenantContext())
	c.log.Info("received event",
		"type", event.Type,
		"tenant_id", event.TenantID,
		"subject", event.Subject,
	)
	_ = c.svc
	_ = ctx
	_ = msg.Ack()
}

// Run starts the notification-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
