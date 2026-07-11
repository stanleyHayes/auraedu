// Command worker is the website-service background event consumer.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	"github.com/auraedu/website-service/internal/adapters/postgres"
	"github.com/auraedu/website-service/internal/domain"
)

const service = "website-service"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Info(service + " worker started")

	if err := run(log); err != nil {
		log.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx := context.Background()

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

	sub, err := eventbus.Subscribe(js, "AURA", "website-service-tenant-created", "tenant.created.v1", func(ctx context.Context, event tenancy.CloudEvent) error {
		log.Info("received tenant.created.v1", "tenant_id", event.TenantID)
		if !gates.IsEnabled(ctx, event.TenantID, "public_website") {
			log.Info("public_website disabled for tenant; skipping default page creation", "tenant_id", event.TenantID)
			return nil
		}
		return createDefaultHomePage(ctx, repo, event.TenantID, log)
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
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	reg, err := flags.LoadYAML(path)
	if err != nil {
		log.Warn("failed to load feature registry; all features disabled", "path", path, "err", err)
		return flags.NewStaticSnapshot()
	}
	return reg.SnapshotFromRegistry()
}

func createDefaultHomePage(ctx context.Context, repo *postgres.Repository, tenantID string, log *slog.Logger) error {
	page, err := domain.NewPage(tenantID, "home", "Home")
	if err != nil {
		return err
	}
	published := string(domain.PageStatusPublished)
	if _, err := page.ApplyUpdate(nil, nil, &published, nil, nil); err != nil {
		return err
	}
	if err := repo.CreatePage(ctx, tenantID, page); err != nil {
		log.Error("failed to create default home page", "tenant_id", tenantID, "err", err)
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
	if err := repo.CreateSection(ctx, tenantID, section); err != nil {
		log.Error("failed to create default hero section", "tenant_id", tenantID, "err", err)
		return err
	}

	log.Info("created default home page with hero section", "tenant_id", tenantID, "page_id", page.ID)
	return nil
}

func waitForShutdown(log *slog.Logger) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info(service + " worker stopped")
}
