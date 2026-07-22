// Package workercmd runs the content outbox worker.
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

	"github.com/auraedu/content-service/internal/adapters/events"
	"github.com/auraedu/content-service/internal/adapters/postgres"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/nats-io/nats.go"
)

func Run() error {
	log := observ.DefaultLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	shutdownTelemetry, err := observ.InitTracing("content-service-worker", config.Getenv("GIT_SHA", "dev"))
	if err != nil {
		return fmt.Errorf("content worker telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush content worker telemetry", "err", err)
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
	natsURL := normalizeNATSURL(config.Getenv("NATS_URL", nats.DefaultURL))
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("content worker connect nats: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if _, err = eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}
	repo, publisher := postgres.NewRepository(database), events.New(eventbus.NewPublisher(js))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	metrics := observ.NewWorkerMetrics("content-service-worker", "outbox-batch", "outbox-publish")
	log.Info("content outbox worker started")
	for {
		started := time.Now()
		err := dispatch(ctx, repo, publisher, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("content outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo *postgres.Repository, publisher *events.Publisher, log *slog.Logger, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPending(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("content event publish deferred", "outbox_id", item.ID, "err", err)
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

func normalizeNATSURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nats.DefaultURL
	}
	if !strings.Contains(value, "://") {
		return "nats://" + value
	}
	return value
}
