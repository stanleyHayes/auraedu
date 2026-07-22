// Package workercmd provides the fees payment-reconciliation worker.
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

	feesevents "github.com/auraedu/fees-service/internal/adapters/events"
	"github.com/auraedu/fees-service/internal/adapters/postgres"
	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"
	_ "github.com/jackc/pgx/v5/stdlib" // Register the pgx database/sql driver for migrations.
	"github.com/nats-io/nats.go"
)

const service = "fees-service-worker"

var version = ""

func main() {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}
	ctx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	shutdownTelemetry, err := observ.InitTracing(service, version)
	if err != nil {
		log.Error("failed to initialize worker telemetry", "err", err)
		return
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush fees worker telemetry", "err", err)
		}
	}()

	database, err := openDB(ctx)
	if err != nil {
		log.Error("failed to open database", "err", err)
		return
	}
	defer database.Close()
	nc, js, err := connectNATS(log)
	if err != nil {
		log.Error("failed to connect to NATS", "err", err)
		return
	}
	defer nc.Close()
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream", "err", err)
		return
	}

	feeRepo := postgres.NewFeeStructureRepository(database)
	invoiceRepo := postgres.NewInvoiceRepository(database)
	publisher := feesevents.NewPublisher(eventbus.NewPublisher(js))
	svc := application.NewService(feeRepo, invoiceRepo,
		application.WithPublisher(publisher),
		application.WithFeatureGate(featureGates(log)),
		application.WithFinancialRepositories(invoiceRepo, invoiceRepo, invoiceRepo),
	)
	consumer := newConsumer(js, svc, log, observ.NewWorkerMetrics(service, "reconcile-payment"))
	if err := consumer.Start(); err != nil {
		log.Error("failed to start payment consumer", "err", err)
		return
	}
	defer consumer.Stop()
	outboxDone := make(chan struct{})
	go func() {
		defer close(outboxDone)
		runFeeOutbox(ctx, invoiceRepo, publisher, log, observ.NewWorkerMetrics(service, "outbox-batch", "outbox-publish"))
	}()

	log.Info(service+" started", "version", version)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelWorker()
	<-outboxDone
	log.Info(service + " stopped")
}

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

func runFeeOutbox(ctx context.Context, repo ports.OutboxRepository, publisher outboxPublisher, log *slog.Logger, metrics *observ.WorkerMetrics) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		started := time.Now()
		err := dispatchFeeOutbox(ctx, repo, publisher, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("fees outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func dispatchFeeOutbox(ctx context.Context, repo ports.OutboxRepository, publisher outboxPublisher, log *slog.Logger, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPendingFeeEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFeeEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFeeEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("fees event publish deferred", "outbox_id", item.ID, "event_type", item.EventType, "err", err)
			continue
		}
		if err := repo.MarkFeeEventPublished(ctx, item.ID); err != nil {
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
	return db.Open(ctx, db.Config{DSN: dsn, Migrations: "migrations"})
}

func connectNATS(log *slog.Logger) (*nats.Conn, eventbus.JetStreamContext, error) {
	nc, err := nats.Connect(config.Getenv("NATS_URL", nats.DefaultURL))
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("nats jetstream: %w", err)
	}
	log.Info("connected to NATS")
	return nc, js, nil
}

func featureGates(log *slog.Logger) flags.Gate {
	fallback := flags.NewStaticSnapshot()
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	if registry, err := flags.LoadYAML(path); err != nil {
		log.Warn("failed to load feature registry; fees disabled", "path", path, "err", err)
	} else {
		fallback = registry.SnapshotFromRegistry()
	}
	return flags.NewRuntimeGate(config.Getenv("SERVICE_TENANT_URL", ""), fallback, log)
}

type consumer struct {
	js      eventbus.JetStreamContext
	svc     *application.Service
	log     *slog.Logger
	metrics *observ.WorkerMetrics
	sub     *eventbus.Subscription
}

func newConsumer(js eventbus.JetStreamContext, svc *application.Service, log *slog.Logger, metrics *observ.WorkerMetrics) *consumer {
	return &consumer{js: js, svc: svc, log: log, metrics: metrics}
}

func (c *consumer) Start() error {
	const eventType = "payment.received.v1"
	sub, err := eventbus.Subscribe(
		c.js,
		eventbus.EventStreamName,
		"fees-payment-received-v1",
		eventType,
		c.handleEvent,
		nil,
	)
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", eventbus.Subject("AURA", eventType), err)
	}
	c.sub = sub
	c.log.Info("subscribed", "subject", eventbus.Subject("AURA", eventType))
	return nil
}

func (c *consumer) Stop() {
	if c.sub != nil {
		if err := c.sub.Unsubscribe(); err != nil {
			c.log.Error("unsubscribe payment consumer", "err", err)
		}
		c.sub = nil
	}
}

func (c *consumer) handleEvent(ctx context.Context, event tenancy.CloudEvent) error {
	started := time.Now()
	input, err := paymentInput(event)
	if err == nil {
		_, _, _, err = c.svc.ApplyPaymentReceived(ctx, input)
	}
	c.metrics.Observe(ctx, "reconcile-payment", started, err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrValidation),
		errors.Is(err, domain.ErrMissingTenant),
		errors.Is(err, domain.ErrForbidden),
		errors.Is(err, flags.ErrFeatureDisabled):
		c.log.Error("dropping non-retryable payment event", "err", err)
		return nil
	default:
		c.log.Error("payment reconciliation failed; will redeliver", "err", err)
		return err
	}
}

func paymentInput(event tenancy.CloudEvent) (application.PaymentReceivedInput, error) {
	if event.Type != "payment.received.v1" {
		return application.PaymentReceivedInput{}, fmt.Errorf("%w: unsupported event type %q", domain.ErrValidation, event.Type)
	}
	var data struct {
		PaymentID         string  `json:"payment_id"`
		InvoiceID         string  `json:"invoice_id"`
		Amount            int     `json:"amount"`
		AmountCents       int     `json:"amount_cents"`
		ProviderReference *string `json:"provider_reference"`
		CompletedAt       string  `json:"completed_at"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return application.PaymentReceivedInput{}, fmt.Errorf("%w: decode payment data: %w", domain.ErrValidation, err)
	}
	amount := data.AmountCents
	if amount == 0 {
		amount = data.Amount
	}
	receivedAt := time.Now().UTC()
	if value := strings.TrimSpace(data.CompletedAt); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return application.PaymentReceivedInput{}, fmt.Errorf("%w: completed_at must be RFC3339", domain.ErrValidation)
		}
		receivedAt = parsed
	} else if value := strings.TrimSpace(event.Time); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			receivedAt = parsed
		}
	}
	if data.PaymentID == "" || data.InvoiceID == "" || amount <= 0 {
		return application.PaymentReceivedInput{}, fmt.Errorf("%w: payment_id, invoice_id and positive amount are required", domain.ErrValidation)
	}
	return application.PaymentReceivedInput{
		TenantID: event.TenantID, InvoiceID: data.InvoiceID, PaymentID: data.PaymentID,
		AmountCents: amount, ProviderReference: data.ProviderReference, ReceivedAt: receivedAt,
	}, nil
}

// Run starts the fees reconciliation worker.
func Run(serviceVersion string) error {
	version = serviceVersion
	main()
	return nil
}
