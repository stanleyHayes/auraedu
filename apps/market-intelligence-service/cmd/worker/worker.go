// Package workercmd dispatches durable market-intelligence events.
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

	"github.com/auraedu/market-intelligence-service/internal/adapters/events"
	"github.com/auraedu/market-intelligence-service/internal/adapters/postgres"
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
	shutdownTelemetry, e := observ.InitTracing("market-intelligence-service-worker", config.Getenv("GIT_SHA", "dev"))
	if e != nil {
		return fmt.Errorf("intelligence worker telemetry: %w", e)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if e := shutdownTelemetry(shutdownCtx); e != nil {
			log.Error("flush intelligence worker telemetry", "err", e)
		}
	}()
	dsn, e := config.MustGetenv("DATABASE_URL")
	if e != nil {
		return e
	}
	database, e := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if e != nil {
		return e
	}
	defer database.Close()
	nc, e := nats.Connect(config.Getenv("NATS_URL", nats.DefaultURL), nats.Timeout(5*time.Second))
	if e != nil {
		return fmt.Errorf("intelligence worker connect nats: %w", e)
	}
	defer nc.Close()
	js, e := nc.JetStream()
	if e != nil {
		return e
	}
	if _, e = eventbus.EnsureStream(js, "AURA"); e != nil {
		return e
	}
	repo := postgres.NewRepository(database)
	publisher := events.New(eventbus.NewPublisher(js))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	metrics := observ.NewWorkerMetrics("market-intelligence-service-worker", "outbox-batch", "outbox-publish")
	log.Info("market intelligence outbox worker started")
	for {
		started := time.Now()
		e := dispatch(ctx, repo, publisher, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, e)
		if e != nil && ctx.Err() == nil {
			log.Error("market intelligence outbox dispatch failed", "err", e)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func dispatch(ctx context.Context, repo *postgres.Repository, publisher *events.Publisher, log *slog.Logger, workerMetrics ...*observ.WorkerMetrics) error {
	var metrics *observ.WorkerMetrics
	if len(workerMetrics) > 0 {
		metrics = workerMetrics[0]
	}
	items, e := repo.ClaimPending(ctx, 25)
	if e != nil {
		return e
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if e := json.Unmarshal(item.Payload, &payload); e != nil {
			metrics.Observe(ctx, "outbox-publish", started, e)
			if markErr := repo.MarkFailed(ctx, item.ID, e.Error()); markErr != nil {
				return errors.Join(e, markErr)
			}
			continue
		}
		if e := publisher.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); e != nil {
			metrics.Observe(ctx, "outbox-publish", started, e)
			if markErr := repo.MarkFailed(ctx, item.ID, e.Error()); markErr != nil {
				return errors.Join(e, markErr)
			}
			log.Warn("intelligence event publish deferred", "outbox_id", item.ID, "err", e)
			continue
		}
		if e := repo.MarkPublished(ctx, item.ID); e != nil {
			metrics.Observe(ctx, "outbox-publish", started, e)
			return e
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}
