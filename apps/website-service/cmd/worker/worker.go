// Package workercmd provides the website-service worker command.
package workercmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/platform/tenancy"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	svcevents "github.com/auraedu/website-service/internal/adapters/events"
	"github.com/auraedu/website-service/internal/adapters/postgres"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
)

const service = "website-service-worker"

func main() {
	log := observ.DefaultLogger()
	log.Info(service + " worker started")

	if err := run(log); err != nil {
		log.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx := context.Background()
	shutdownTelemetry, err := observ.InitTracing(service, config.Getenv("GIT_SHA", "dev"))
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush website worker telemetry", "err", err)
		}
	}()

	database, err := openDB(ctx)
	if err != nil {
		return err
	}
	defer database.Close()

	gates := featureGates(log)
	repo := postgres.NewRepository(database)

	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; no event subscriptions")
		waitForShutdown(log)
		return nil
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}

	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}

	metrics := observ.NewWorkerMetrics(service, "tenant-created")
	pub := svcevents.NewPublisher(eventbus.NewPublisher(js))
	outboxMetrics := observ.NewWorkerMetrics(service, "outbox-batch", "outbox-publish")
	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	defer cancelDispatch()
	go runOutboxDispatcher(dispatchCtx, repo, pub, outboxMetrics, log)
	sub, err := eventbus.Subscribe(js, "AURA", "website-service-tenant-created", "tenant.created.v1", func(ctx context.Context, event tenancy.CloudEvent) error {
		started := time.Now()
		log.Info("received tenant.created.v1", "tenant_id", event.TenantID)
		if !gates.IsEnabled(ctx, event.TenantID, "public_website") {
			log.Info("public_website disabled for tenant; skipping default page creation", "tenant_id", event.TenantID)
			metrics.Observe(ctx, "tenant-created", started, nil)
			return nil
		}
		err := createDefaultHomePage(ctx, repo, event.TenantID, log)
		metrics.Observe(ctx, "tenant-created", started, err)
		return err
	}, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			log.Error("failed to unsubscribe", "err", err)
		}
	}()

	log.Info("subscribed to tenant.created.v1")
	waitForShutdown(log)
	return nil
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

func featureGates(log *slog.Logger) flags.Gate {
	// Static registry snapshot: plan defaults baked into the deploy. It stays
	// the fallback when tenant-service is unreachable.
	fallback := flags.NewStaticSnapshot()
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	reg, err := flags.LoadYAML(path)
	if err != nil {
		log.Warn("failed to load feature registry; all features disabled", "path", path, "err", err)
	} else {
		fallback = reg.SnapshotFromRegistry()
	}

	return flags.NewRuntimeGate(config.Getenv("SERVICE_TENANT_URL", ""), fallback, log)
}

func createDefaultHomePage(ctx context.Context, repo *postgres.Repository, tenantID string, log *slog.Logger) error {
	if existing, err := repo.GetPageBySlug(ctx, tenantID, "home"); err == nil {
		log.Info("default home page already exists", "tenant_id", tenantID, "page_id", existing.ID)
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	page, err := domain.NewPage(tenantID, "home", "Home")
	if err != nil {
		return err
	}
	published := string(domain.PageStatusPublished)
	if _, err := page.ApplyUpdate(nil, nil, &published, nil, nil); err != nil {
		return err
	}
	section, err := domain.NewSection(tenantID, page.ID, domain.SectionTypeHero, domain.Content{
		"title":       "Welcome",
		"subtitle":    "Your school website is ready.",
		"button_text": "Learn more",
	}, 0)
	if err != nil {
		return err
	}
	publishedSection := string(domain.SectionStatusPublished)
	if _, err := section.ApplyUpdate(nil, nil, nil, &publishedSection); err != nil {
		return err
	}
	events := []ports.LifecycleEvent{
		{EventType: "website.page_created.v1", Payload: ports.PageEventData(page, nil)},
		{EventType: "website.page_published.v1", Payload: ports.PageEventData(page, nil)},
		{EventType: "website.section_created.v1", Payload: ports.SectionEventData(section, nil)},
	}
	if err := repo.ProvisionDefaultWebsite(ctx, tenantID, page, section, events); err != nil {
		log.Error("failed to create default hero section", "tenant_id", tenantID, "err", err)
		return err
	}

	log.Info("created default home page with hero section", "tenant_id", tenantID, "page_id", page.ID)
	return nil
}

type outboxPublisher interface {
	PublishWithID(context.Context, string, string, string, map[string]any) error
}

func runOutboxDispatcher(ctx context.Context, repo ports.OutboxRepository, pub outboxPublisher, metrics *observ.WorkerMetrics, log *slog.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		started := time.Now()
		err := dispatchOutbox(ctx, repo, pub, metrics)
		metrics.Observe(ctx, "outbox-batch", started, err)
		if err != nil && ctx.Err() == nil {
			log.Error("website outbox dispatch failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func dispatchOutbox(ctx context.Context, repo ports.OutboxRepository, pub outboxPublisher, metrics *observ.WorkerMetrics) error {
	items, err := repo.ClaimPendingWebsiteEvents(ctx, 25)
	if err != nil {
		return err
	}
	for _, item := range items {
		started := time.Now()
		var payload map[string]any
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkWebsiteEventFailed(ctx, item.ID, "invalid outbox payload"); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := pub.PublishWithID(ctx, item.ID, item.EventType, item.TenantID, payload); err != nil {
			metrics.Observe(ctx, "outbox-publish", started, err)
			if markErr := repo.MarkWebsiteEventFailed(ctx, item.ID, err.Error()); markErr != nil {
				return errors.Join(err, markErr)
			}
			continue
		}
		if err := repo.MarkWebsiteEventPublished(ctx, item.ID); err != nil {
			return err
		}
		metrics.Observe(ctx, "outbox-publish", started, nil)
	}
	return nil
}

func waitForShutdown(log *slog.Logger) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info(service + " worker stopped")
}

// Run starts the website-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
