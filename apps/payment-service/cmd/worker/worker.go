// Package workercmd dispatches Payment Service's transactional reconciliation outbox.
package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	paymentevents "github.com/auraedu/payment-service/internal/adapters/events"
	"github.com/auraedu/payment-service/internal/adapters/postgres"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/nats-io/nats.go"
)

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

func Run(serviceVersion string) error {
	log := observ.DefaultLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if serviceVersion == "" {
		serviceVersion = config.Getenv("GIT_SHA", "dev")
	}
	shutdownTelemetry, err := observ.InitTracing("payment-service-worker", serviceVersion)
	if err != nil {
		return fmt.Errorf("payment worker telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush payment worker telemetry", "err", err)
		}
	}()

	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if err != nil {
		return fmt.Errorf("payment worker database: %w", err)
	}
	defer database.Close()

	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		return err
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("payment worker connect nats: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("payment worker jetstream: %w", err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return fmt.Errorf("payment worker ensure stream: %w", err)
	}

	repo := postgres.NewPaymentRepository(database)
	publisher := paymentevents.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics("payment-service-worker", "outbox-batch", "outbox-publish")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Info("payment outbox worker started")
	for {
		started := time.Now()
		err := dispatch(ctx, repo, publisher, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("payment outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo ports.OutboxRepository, publisher outboxPublisher, log *slog.Logger, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPendingPaymentEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkPaymentEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkPaymentEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("payment event publish deferred", "outbox_id", item.ID, "event_type", item.EventType, "err", err)
			continue
		}
		if err := repo.MarkPaymentEventPublished(ctx, item.ID); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}
