// Package servercmd provides the file-service server command.
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

	svcevents "github.com/auraedu/file-service/internal/adapters/events"
	svchttp "github.com/auraedu/file-service/internal/adapters/http"
	"github.com/auraedu/file-service/internal/adapters/postgres"
	"github.com/auraedu/file-service/internal/adapters/storage"
	"github.com/auraedu/file-service/internal/application"
	"github.com/auraedu/file-service/internal/ports"
)

const service = "file-service"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return err
	}
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}
	shutdownTracing, err := observ.InitTracing(service, version)
	if err != nil {
		return err
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Error("failed to flush tracing", "err", err)
		}
	}()

	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		return err
	}
	defer database.Close()

	store, err := initStorage(log)
	if err != nil {
		return err
	}
	pub := publisher(ctx, log)
	gates := featureGates(log)

	repo := postgres.NewRepository(database)
	opts := []application.Option{
		application.WithPublisher(pub),
		application.WithFeatureGate(gates),
	}
	if signer, ok := store.(ports.SignedUploadProvider); ok {
		opts = append(opts, application.WithSignedUploadProvider(signer))
	}
	if delivery, ok := store.(ports.DeliveryURLProvider); ok {
		opts = append(opts, application.WithDeliveryURLProvider(delivery))
	}
	if verifier, ok := store.(ports.WebhookVerifier); ok {
		opts = append(opts, application.WithWebhookVerifier(verifier))
	}
	svc := application.NewService(repo, store, opts...)
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
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		log.Info(service+" listening", "addr", addr)
		errorsChannel <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-stop:
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		return err
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

func initStorage(log *slog.Logger) (ports.Storage, error) {
	if cloudURL := config.Getenv("CLOUDINARY_URL", ""); cloudURL != "" {
		store, err := storage.NewCloudinaryStorage(cloudURL,
			storage.WithResourceType(config.Getenv("CLOUDINARY_RESOURCE_TYPE", "raw")),
		)
		if err != nil {
			return nil, fmt.Errorf("initialize cloudinary storage: %w", err)
		}
		log.Info("cloudinary storage initialized")
		return store, nil
	}
	if config.Getenv("ENVIRONMENT", "development") == "production" {
		return nil, errors.New("CLOUDINARY_URL is required in production")
	}

	dir := config.Getenv("FILE_STORAGE_DIR", "/tmp/auraedu-files")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create storage directory %s: %w", dir, err)
	}
	log.Info("local storage initialized", "dir", dir)
	return storage.NewLocalStorage(dir), nil
}

func publisher(_ context.Context, log *slog.Logger) *svcevents.Publisher {
	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; event publishing disabled")
		return svcevents.NewPublisher(nil)
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil)
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Error("failed to create JetStream context; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil)
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil)
	}
	log.Info("event publishing enabled", "nats_url", natsURL)
	return svcevents.NewPublisher(eventbus.NewPublisher(js))
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

// Run starts the file-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
