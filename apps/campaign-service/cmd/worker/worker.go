// Package workercmd runs the durable Campaign outbox worker.
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

	"github.com/auraedu/campaign-service/internal/adapters/events"
	"github.com/auraedu/campaign-service/internal/adapters/postgres"
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
	shutdownTelemetry, err := observ.InitTracing("campaign-service-worker", config.Getenv("GIT_SHA", "dev"))
	if err != nil {
		return fmt.Errorf("campaign worker telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush campaign worker telemetry", "err", err)
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
	nc, err := nats.Connect(config.Getenv("NATS_URL", nats.DefaultURL), nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("campaign worker connect nats: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if _, err = eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}
	repo := postgres.NewRepository(database)
	pub := events.New(eventbus.NewPublisher(js))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	metrics := observ.NewWorkerMetrics("campaign-service-worker", "outbox-batch", "outbox-publish")
	log.Info("campaign outbox worker started")
	for {
		started := time.Now()
		err := dispatch(ctx, repo, pub, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("campaign outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo *postgres.Repository, pub *events.Publisher, log *slog.Logger, workerMetrics ...*observ.WorkerMetrics) error {
	var metrics *observ.WorkerMetrics
	if len(workerMetrics) > 0 {
		metrics = workerMetrics[0]
	}
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
		if err := pub.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			log.Warn("campaign event publish deferred", "outbox_id", item.ID, "err", err)
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
