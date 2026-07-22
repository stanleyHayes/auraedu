// Package workercmd dispatches durable Student Service outbox events.
package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	svcevents "github.com/auraedu/student-service/internal/adapters/events"
	"github.com/auraedu/student-service/internal/adapters/postgres"
	"github.com/auraedu/student-service/internal/ports"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
)

const service = "student-service-worker"

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
		if err := shutdown(context.Background()); err != nil {
			log.Error("flush student worker tracing", "err", err)
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
		return fmt.Errorf("student worker: connect NATS: %w", err)
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
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))
	metrics := observ.NewWorkerMetrics(service, "outbox-batch", "outbox-publish")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Info(service+" started", "version", version)
	for {
		started := time.Now()
		err := dispatch(ctx, repo, pub, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("student outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo ports.OutboxRepository, pub outboxPublisher, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPendingStudentEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkStudentEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := pub.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkStudentEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := repo.MarkStudentEventPublished(ctx, item.ID); err != nil {
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}
