// Package workercmd runs the durable assistant event worker.
package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/adapters/events"
	"github.com/auraedu/ai-orchestrator-service/internal/adapters/postgres"
	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/nats-io/nats.go"
)

const serviceName = "ai-orchestrator-service-worker"

type outboxRepository interface {
	ClaimPending(context.Context, int) ([]ports.OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}
type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

func Run(version string) error {
	log := observ.DefaultLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	shutdown, err := observ.InitTracing(serviceName, version)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdown(context.Background()); shutdownErr != nil {
			log.Error("failed to shut down tracing", "err", shutdownErr)
		}
	}()
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if err != nil {
		return err
	}
	defer database.Close()
	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		return err
	}
	nc, err := nats.Connect(normalizeNATSURL(natsURL), nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("assistant worker connect NATS: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}
	repo := postgres.NewRepository(database)
	pub := events.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics(serviceName, "outbox-batch", "outbox-publish")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Info("assistant escalation outbox worker started", "version", version)
	for {
		started := time.Now()
		err := dispatch(ctx, repo, pub, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("assistant outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo outboxRepository, pub outboxPublisher, log *slog.Logger, metrics *observ.WorkerMetrics) error {
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
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := pub.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("assistant escalation publish deferred", "outbox_id", item.ID, "err", err)
			continue
		}
		if err := repo.MarkPublished(ctx, item.ID); err != nil {
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}

func normalizeNATSURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "://") {
		return value
	}
	return "nats://" + value
}
