// Package workercmd dispatches durable file lifecycle work.
package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	svcevents "github.com/auraedu/file-service/internal/adapters/events"
	"github.com/auraedu/file-service/internal/adapters/postgres"
	"github.com/auraedu/file-service/internal/adapters/storage"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	_ "github.com/jackc/pgx/v5/stdlib" // Register the pgx database/sql driver for migrations.
	"github.com/nats-io/nats.go"
)

const service = "file-service-worker"

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

func Run(version string) error {
	log := observ.DefaultLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	shutdown, err := observ.InitTracing(service, version)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdown(context.Background()); shutdownErr != nil {
			log.Error("flush file worker telemetry", "err", shutdownErr)
		}
	}()
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: "migrations"})
	if err != nil {
		return err
	}
	defer database.Close()
	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		return err
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("file worker: connect NATS: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}
	store, err := workerStorage()
	if err != nil {
		return err
	}
	repo := postgres.NewRepository(database)
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics(service, "outbox-batch", "outbox-publish")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Info(service+" started", "version", version)
	for {
		started := time.Now()
		err := dispatch(ctx, repo, store, pub, log, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("file outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func workerStorage() (ports.Storage, error) {
	if cloudURL := config.Getenv("CLOUDINARY_URL", ""); cloudURL != "" {
		return storage.NewCloudinaryStorage(cloudURL, storage.WithResourceType(config.Getenv("CLOUDINARY_RESOURCE_TYPE", "raw")))
	}
	if config.Getenv("ENVIRONMENT", "development") == "production" {
		return nil, errors.New("file worker: CLOUDINARY_URL is required in production")
	}
	dir := config.Getenv("FILE_STORAGE_DIR", "/tmp/auraedu-files")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return storage.NewLocalStorage(dir), nil
}

func dispatch(
	ctx context.Context,
	repo ports.OutboxRepository,
	store ports.Storage,
	pub outboxPublisher,
	log *slog.Logger,
	metrics *observ.WorkerMetrics,
) error {
	items, err := repo.ClaimPendingFileEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFileEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if item.CleanupPath != "" {
			if err := store.Delete(ctx, item.TenantID, item.CleanupPath); err != nil {
				metrics.Observe(ctx, "outbox-publish", started, err)
				if markErr := repo.MarkFileEventFailed(ctx, item.ID, err.Error()); markErr != nil {
					return errors.Join(err, markErr)
				}
				log.Warn("file cleanup deferred", "outbox_id", item.ID, "err", err)
				continue
			}
		}
		if err := pub.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkFileEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := repo.MarkFileEventPublished(ctx, item.ID); err != nil {
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}
