// Command server is the file-service HTTP entrypoint.
package main

import (
	"context"
	"errors"
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

	store := initStorage(log)
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
	svc := application.NewService(repo, store, opts...)
	handler := svchttp.NewHandler(svc)

	health := httpx.NewHealth(service, version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })

	mux := http.NewServeMux()
	health.Register(mux)
	handler.Register(mux)

	addr := ":" + strconv.Itoa(config.Port(8080))
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpx.RequestIDMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Info(service+" listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShutdown)
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

func initStorage(log *slog.Logger) ports.Storage {
	if cloudURL := config.Getenv("CLOUDINARY_URL", ""); cloudURL != "" {
		store, err := storage.NewCloudinaryStorage(cloudURL,
			storage.WithResourceType(config.Getenv("CLOUDINARY_RESOURCE_TYPE", "raw")),
		)
		if err != nil {
			log.Error("failed to initialize cloudinary storage", "err", err)
			os.Exit(1)
		}
		log.Info("cloudinary storage initialized")
		return store
	}

	dir := config.Getenv("FILE_STORAGE_DIR", "/tmp/auraedu-files")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Error("failed to create storage directory", "dir", dir, "err", err)
		os.Exit(1)
	}
	log.Info("local storage initialized", "dir", dir)
	return storage.NewLocalStorage(dir)
}

func publisher(ctx context.Context, log *slog.Logger) *svcevents.Publisher {
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
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	reg, err := flags.LoadYAML(path)
	if err != nil {
		log.Warn("failed to load feature registry; all features disabled", "path", path, "err", err)
		return flags.NewStaticSnapshot()
	}
	return reg.SnapshotFromRegistry()
}
