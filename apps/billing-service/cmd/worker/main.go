// Command worker is the Billing Service background consumer.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/billing-service/internal/adapters/postgres"
	"github.com/auraedu/billing-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
)

const service = "billing-service-worker"

var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		log.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	planRepo := postgres.NewPlanRepository(database)
	subRepo := postgres.NewSubscriptionRepository(database)
	invRepo := postgres.NewSaaSInvoiceRepository(database)
	svc := application.NewService(planRepo, subRepo, invRepo)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; worker running without event consumption")
		<-stop
		log.Info(service + " stopped")
		return
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Error("failed to create JetStream context", "err", err)
		os.Exit(1)
	}

	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream", "err", err)
		os.Exit(1)
	}

	sub, err := eventbus.Subscribe(js, "AURA", "billing-worker-tenant-created", "tenant.created.v1", func(ctx context.Context, event tenancy.CloudEvent) error {
		return handleTenantCreated(ctx, log, svc, event)
	}, nil)
	if err != nil {
		log.Error("failed to subscribe to tenant.created.v1", "err", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	log.Info(service+" started", "version", version)
	<-stop
	log.Info(service + " stopped")
}

func openDB(ctx context.Context) (*db.DB, error) {
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	return db.Open(ctx, db.Config{
		DSN:        dsn,
		Migrations: "migrations",
	})
}

func handleTenantCreated(ctx context.Context, log *slog.Logger, svc *application.Service, event tenancy.CloudEvent) error {
	var payload struct {
		TenantID string `json:"tenant_id"`
	}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			log.Warn("tenant.created.v1 payload unmarshal failed; falling back to tenant_id from envelope", "err", err)
		}
	}
	tenantID := payload.TenantID
	if tenantID == "" {
		tenantID = event.TenantID
	}
	if tenantID == "" {
		log.Warn("tenant.created.v1 missing tenant_id; skipping trial subscription")
		return nil
	}

	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
	subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := svc.CreateTrialSubscriptionForTenant(subCtx, tenantID)
	if err != nil {
		if isNoDefaultPlan(err) {
			log.Warn("no default active plan found for new tenant; skipping trial subscription", "tenant_id", tenantID)
			return nil
		}
		log.Error("failed to create trial subscription for tenant", "tenant_id", tenantID, "err", err)
		return err
	}
	log.Info("created trial subscription for tenant", "tenant_id", tenantID)
	return nil
}

func isNoDefaultPlan(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "billing: no default active plan"
}
