// Package servercmd provides the staff-service server command.
package servercmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	svcevents "github.com/auraedu/staff-service/internal/adapters/events"
	svchttp "github.com/auraedu/staff-service/internal/adapters/http"
	"github.com/auraedu/staff-service/internal/adapters/postgres"
	"github.com/auraedu/staff-service/internal/application"
)

const service = "staff-service"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return fmt.Errorf("invalid production runtime configuration: %w", err)
	}
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}
	shutdownTracing, err := observ.InitTracing(service, version)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Error("failed to flush tracing", "err", err)
		}
	}()

	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	pub, closePublisher := publisher(ctx, log)
	defer closePublisher()
	gates := featureGates(log)

	repo := postgres.NewRepository(database)
	svc := application.NewService(repo,
		application.WithPublisher(pub),
		application.WithFeatureGate(gates),
	)
	handler := svchttp.NewHandler(svc)

	health := httpx.NewHealth(service, version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })

	mux := http.NewServeMux()
	health.Register(mux)
	handler.Register(mux)
	handler.RegisterInternal(mux, config.Getenv("INTERNAL_SERVICE_TOKEN", ""))

	addr := ":" + strconv.Itoa(config.Port(8080))
	srv := &http.Server{
		Addr:              addr,
		Handler:           observ.HTTPHandler(service, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Info(service+" listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("serve HTTP: %w", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
	case err := <-serverErrors:
		return err
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	log.Info(service + " stopped")
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

func publisher(_ context.Context, log *slog.Logger) (*svcevents.Publisher, func()) {
	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; event publishing disabled")
		return svcevents.NewPublisher(nil), func() {}
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil), func() {}
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Error("failed to create JetStream context; event publishing disabled", "err", err)
		nc.Close()
		return svcevents.NewPublisher(nil), func() {}
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream; event publishing disabled", "err", err)
		nc.Close()
		return svcevents.NewPublisher(nil), func() {}
	}
	log.Info("event publishing enabled", "nats_url", natsURL)
	return svcevents.NewPublisher(eventbus.NewPublisher(js)), nc.Close
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

// Run starts the staff-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
