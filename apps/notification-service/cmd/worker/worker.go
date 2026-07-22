// Package workercmd provides the notification-service worker command.
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

	crmadapter "github.com/auraedu/notification-service/internal/adapters/crm"
	svcevents "github.com/auraedu/notification-service/internal/adapters/events"
	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/adapters/postgres"
	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
)

const service = "notification-service-worker"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func main() {
	log := observ.DefaultLogger()
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
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return err
	}
	if err := config.RequireProductionEnv("PUBLIC_APP_URL"); err != nil {
		return err
	}
	if err := config.RequireProductionEnv("NOTIFICATION_UNSUBSCRIBE_SIGNING_KEY"); err != nil {
		return err
	}
	publicAppURL := config.Getenv("PUBLIC_APP_URL", "http://localhost:3000")
	if err := application.ValidatePublicAppURL(publicAppURL, strings.EqualFold(config.Getenv("ENVIRONMENT", "development"), "production")); err != nil {
		return err
	}
	var unsubscribeManager *application.UnsubscribeManager
	if key := config.Getenv("NOTIFICATION_UNSUBSCRIBE_SIGNING_KEY", ""); key != "" {
		manager, managerErr := application.NewUnsubscribeManager(key, publicAppURL)
		if managerErr != nil {
			return managerErr
		}
		unsubscribeManager = manager
	}
	ctx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	shutdownTelemetry, err := observ.InitTracing(service, version)
	if err != nil {
		return fmt.Errorf("initialize telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush worker telemetry", "err", err)
		}
	}()
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
	processedRepo := postgres.NewProcessedEventRepository(database)
	deviceRepo := postgres.NewDeviceTokenRepository(database)
	journeyRepo := postgres.NewJourneyRepository(database)
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))
	notifiers, err := notifier.RegistryFromEnvWithPush(deviceRepo)
	if err != nil {
		return err
	}
	svc := application.NewService(messageRepo, templateRepo, subscriptionRepo,
		application.WithPublisher(pub),
		application.WithNotifiers(notifiers),
		application.WithFeatureGate(gates),
		application.WithProcessedEventRepository(processedRepo),
		application.WithLeadResolver(crmadapter.NewClient(config.Getenv("SERVICE_CRM_URL", "http://crm-service:8080"), config.Getenv("INTERNAL_SERVICE_TOKEN", ""))),
		application.WithDeviceTokenRepository(deviceRepo),
		application.WithJourneyRepository(journeyRepo),
		application.WithPublicAppURL(publicAppURL),
		application.WithUnsubscribeManager(unsubscribeManager),
	)
	workerMetrics := observ.NewWorkerMetrics(service, "event-delivery", "journey-event", "scheduled-claim", "scheduled-delivery", "outbox-batch", "outbox-publish")

	consumer := newConsumer(js, log, svc, workerMetrics)
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("start consumer: %w", err)
	}
	defer func() {
		if err := consumer.Stop(); err != nil {
			log.Error("consumer stop error", "err", err)
		}
	}()
	go runScheduler(ctx, log, messageRepo, svc, workerMetrics)
	outboxDone := make(chan struct{})
	go func() {
		defer close(outboxDone)
		runNotificationOutbox(ctx, messageRepo, pub, log, workerMetrics)
	}()

	log.Info(service+" started", "version", version)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelWorker()
	<-outboxDone

	return nil
}

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

func runNotificationOutbox(ctx context.Context, repo ports.OutboxRepository, publisher outboxPublisher, log *slog.Logger, metrics *observ.WorkerMetrics) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		started := time.Now()
		err := dispatchNotificationOutbox(ctx, repo, publisher, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("notification outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func dispatchNotificationOutbox(
	ctx context.Context,
	repo ports.OutboxRepository,
	publisher outboxPublisher,
	log *slog.Logger,
	metrics *observ.WorkerMetrics,
) error {
	items, err := repo.ClaimPendingNotificationEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkNotificationEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkNotificationEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("notification event publish deferred", "outbox_id", item.ID, "event_type", item.EventType, "err", err)
			continue
		}
		if err := repo.MarkNotificationEventPublished(ctx, item.ID); err != nil {
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
	subs    []*eventbus.Subscription
	log     *slog.Logger
	svc     *application.Service
	metrics *observ.WorkerMetrics
}

func newConsumer(js eventbus.JetStreamContext, log *slog.Logger, svc *application.Service, metrics ...*observ.WorkerMetrics) *consumer {
	var workerMetrics *observ.WorkerMetrics
	if len(metrics) > 0 {
		workerMetrics = metrics[0]
	}
	return &consumer{js: js, log: log, svc: svc, metrics: workerMetrics}
}

func (c *consumer) Start(ctx context.Context) error {
	for _, eventType := range []string{
		"lead.created.v1",
		"offer.issued.v1",
		"offer.accepted.v1",
		"payment.received.v1",
		"invoice.created.v1",
		"attendance.marked.v1",
		"assessment.score_recorded.v1",
		"report.published.v1",
		"intelligence.alert.changed.v1",
	} {
		subject := eventbus.Subject("AURA", eventType)
		sub, err := eventbus.Subscribe(
			c.js,
			eventbus.EventStreamName,
			"notification-worker-"+strings.ReplaceAll(eventType, ".", "-"),
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
	for _, eventType := range domain.JourneyEventTypes() {
		subject := eventbus.Subject("AURA", eventType)
		sub, err := eventbus.Subscribe(
			c.js,
			eventbus.EventStreamName,
			"notification-journey-"+strings.ReplaceAll(eventType, ".", "-"),
			eventType,
			c.handleJourneyEvent,
			nil,
		)
		if err != nil {
			return fmt.Errorf("subscribe journey to %s: %w", subject, err)
		}
		c.subs = append(c.subs, sub)
		c.log.Info("subscribed journey", "subject", subject)
	}
	_ = ctx
	return nil
}

func runScheduler(ctx context.Context, log *slog.Logger, repo ports.ScheduledMessageRepository, svc *application.Service, metrics *observ.WorkerMetrics) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			claimStarted := time.Now()
			messages, err := repo.ClaimDue(ctx, 50, 5*time.Minute)
			metrics.Observe(ctx, "scheduled-claim", claimStarted, err)
			if err != nil {
				log.Error("claim scheduled notifications", "err", err)
				continue
			}
			for _, message := range messages {
				tenantCtx := tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: message.TenantID})
				deliveryStarted := time.Now()
				err := svc.DeliverScheduled(tenantCtx, message)
				metrics.Observe(tenantCtx, "scheduled-delivery", deliveryStarted, err)
				if err != nil {
					log.Error("deliver scheduled notification", "message_id", message.ID, "tenant_id", message.TenantID, "err", err)
				}
			}
		}
	}
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
	c.log.Info("received event",
		"type", event.Type,
		"tenant_id", event.TenantID,
		"subject", event.Subject,
	)
	if err := c.svc.HandleCloudEvent(ctx, event); err != nil {
		c.metrics.Observe(ctx, "event-delivery", started, err)
		// Nak so JetStream redelivers after AckWait; the processed-event claim
		// was released by the service so the retry can re-attempt the side effect.
		c.log.Error("event side effect failed", "type", event.Type, "event_id", event.ID, "err", err)
		return err
	}
	c.metrics.Observe(ctx, "event-delivery", started, nil)
	return nil
}

func (c *consumer) handleJourneyEvent(ctx context.Context, event tenancy.CloudEvent) error {
	started := time.Now()
	err := c.svc.HandleJourneyEvent(ctx, event)
	c.metrics.Observe(ctx, "journey-event", started, err)
	if err != nil {
		c.log.Error("journey event projection failed", "type", event.Type, "event_id", event.ID, "err", err)
	}
	return err
}

// Run starts the notification-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
