// Package workercmd runs CRM event projections.
package workercmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/auraedu/crm-service/internal/adapters/postgres"
	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"
	"github.com/nats-io/nats.go"
)

type payload struct {
	LeadID *string `json:"lead_id"`
}

func Run() error {
	log := observ.DefaultLogger()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	const service = "crm-service-worker"
	shutdownTelemetry, err := observ.InitTracing(service, config.Getenv("GIT_SHA", "dev"))
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush CRM worker telemetry", "err", err)
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
	nc, err := nats.Connect(config.Getenv("NATS_URL", nats.DefaultURL))
	if err != nil {
		return err
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
	metrics := observ.NewWorkerMetrics(service, "admissions-projection")
	mapping := map[string]domain.LeadStage{
		"application.started.v1":   domain.StageApplicationStarted,
		"application.submitted.v1": domain.StageApplicationCompleted,
		"application.admitted.v1":  domain.StageAdmitted,
		"offer.accepted.v1":        domain.StageOfferAccepted,
	}
	subs := []*eventbus.Subscription{}
	for eventType, stage := range mapping {
		consumer := "crm-admissions-" + strings.ReplaceAll(eventType, ".", "-")
		sub, err := eventbus.Subscribe(js, "AURA", consumer, eventType, func(ctx context.Context, event tenancy.CloudEvent) error {
			started := time.Now()
			var data payload
			if err := json.Unmarshal(event.Data, &data); err != nil {
				metrics.Observe(ctx, "admissions-projection", started, err)
				return err
			}
			if data.LeadID == nil || *data.LeadID == "" {
				metrics.Observe(ctx, "admissions-projection", started, nil)
				return nil
			}
			occurredAt, parseErr := time.Parse(time.RFC3339, event.Time)
			if parseErr != nil {
				occurredAt = time.Now().UTC()
			}
			err := repo.ProjectAdmissionsStage(ctx, event.TenantID, *data.LeadID, event.ID, event.Type, stage, occurredAt)
			metrics.Observe(ctx, "admissions-projection", started, err)
			return err
		}, nil)
		if err != nil {
			return fmt.Errorf("crm subscribe %s: %w", eventType, err)
		}
		subs = append(subs, sub)
	}
	defer func() {
		for _, sub := range subs {
			if err := sub.Unsubscribe(); err != nil {
				log.Error("unsubscribe CRM projection", "err", err)
			}
		}
	}()
	log.Info("crm admissions projection worker started")
	<-ctx.Done()
	return nil
}
