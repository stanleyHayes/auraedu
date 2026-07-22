// Package workercmd dispatches Tenant Service's transactional integration-event outbox.
package workercmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/tenant-service/internal/adapters/events"
	"github.com/auraedu/tenant-service/internal/adapters/postgres"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/nats-io/nats.go"
)

type outboxRepository interface {
	ClaimPending(context.Context, int) ([]ports.OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

// Run starts the durable Tenant Service outbox worker.
func Run() error {
	log := observ.DefaultLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	shutdownTelemetry, err := observ.InitTracing("tenant-service-worker", config.Getenv("GIT_SHA", "dev"))
	if err != nil {
		return fmt.Errorf("tenant worker telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush tenant worker telemetry", "err", err)
		}
	}()

	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if err != nil {
		return fmt.Errorf("tenant worker database: %w", err)
	}
	defer database.Close()

	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		return err
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("tenant worker connect nats: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("tenant worker jetstream: %w", err)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return fmt.Errorf("tenant worker ensure stream: %w", err)
	}

	repo := postgres.NewRepository(database)
	publisher := events.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics("tenant-service-worker", "outbox-batch", "outbox-publish")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Info("tenant outbox worker started")
	for {
		started := time.Now()
		err := dispatch(ctx, repo, publisher, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("tenant outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo outboxRepository, publisher outboxPublisher, log *slog.Logger, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPending(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return markErr
			}
			continue
		}
		if err := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				return markErr
			}
			log.Warn("tenant event publish deferred", "outbox_id", item.ID, "event_type", item.EventType)
			continue
		}
		if err := repo.MarkPublished(ctx, item.ID); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}
