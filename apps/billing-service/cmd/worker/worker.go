// Package workercmd provides the billing-service worker command.
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

	svcevents "github.com/auraedu/billing-service/internal/adapters/events"
	"github.com/auraedu/billing-service/internal/adapters/postgres"
	"github.com/auraedu/billing-service/internal/application"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
)

const service = "billing-service-worker"

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

// Run consumes tenant onboarding and dispatches the transactional billing outbox.
func Run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	version := config.Getenv("GIT_SHA", "dev")
	shutdown, err := observ.InitTracing(service, version)
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
		return fmt.Errorf("billing worker: connect NATS: %w", err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}

	planRepo := postgres.NewPlanRepository(database)
	subRepo := postgres.NewSubscriptionRepository(database)
	invRepo := postgres.NewSaaSInvoiceRepository(database)
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))
	svc := application.NewService(planRepo, subRepo, invRepo, application.WithPublisher(pub))
	metrics := observ.NewWorkerMetrics(service, "tenant-created", "outbox-batch", "outbox-publish")

	subscription, err := eventbus.Subscribe(
		js, "AURA", "billing-worker-tenant-created", "tenant.created.v1",
		func(ctx context.Context, event tenancy.CloudEvent) error {
			started := time.Now()
			err := handleTenantCreated(ctx, log, svc, event)
			metrics.Observe(ctx, "tenant-created", started, err)
			return err
		}, nil,
	)
	if err != nil {
		return err
	}
	defer func() {
		if unsubscribeErr := subscription.Unsubscribe(); unsubscribeErr != nil {
			log.Error("failed to unsubscribe tenant consumer", "err", unsubscribeErr)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		started := time.Now()
		err := dispatch(ctx, subRepo, pub, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("billing outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatch(ctx context.Context, repo ports.OutboxRepository, pub outboxPublisher, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPendingBillingEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkBillingEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := pub.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkBillingEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := repo.MarkBillingEventPublished(ctx, item.ID); err != nil {
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}

func handleTenantCreated(ctx context.Context, log *slog.Logger, svc *application.Service, event tenancy.CloudEvent) error {
	var payload struct {
		TenantID   string `json:"tenant_id"`
		TenantCode string `json:"tenant_code"`
		Plan       string `json:"plan"`
	}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			log.Warn("tenant.created.v1 payload unmarshal failed; falling back to tenant_id from envelope", "err", err)
		}
	}
	tenantID := payload.TenantID
	if tenantID == "" {
		tenantID = payload.TenantCode
	}
	if tenantID == "" {
		tenantID = event.TenantID
	}
	if tenantID == "" {
		log.Warn("tenant.created.v1 missing tenant_id; skipping trial subscription")
		return nil
	}

	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var err error
	if payload.Plan != "" {
		_, err = svc.CreateSubscriptionForTenant(requestCtx, tenantID, payload.Plan)
	} else {
		_, err = svc.CreateTrialSubscriptionForTenant(requestCtx, tenantID)
	}
	if err != nil && err.Error() == "billing: no default active plan" {
		log.Warn("no default active plan found for new tenant; skipping trial subscription", "tenant_id", tenantID)
		return nil
	}
	return err
}
