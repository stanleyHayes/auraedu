// Package servercmd provides the academic-service server command.
package servercmd

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

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	svcevents "github.com/auraedu/academic-service/internal/adapters/events"
	svchttp "github.com/auraedu/academic-service/internal/adapters/http"
	"github.com/auraedu/academic-service/internal/adapters/postgres"
	"github.com/auraedu/academic-service/internal/application"
)

const service = "academic-service"

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

	pub := publisher(ctx, log)
	gates := featureGates(log)

	yearRepo := postgres.NewRepository(database)
	termRepo := postgres.NewTermRepository(database)
	classRepo := postgres.NewClassRepository(database)
	subjectRepo := postgres.NewSubjectRepository(database)
	svc := application.NewService(yearRepo, termRepo, classRepo, subjectRepo,
		application.WithPublisher(pub),
		application.WithFeatureGate(gates),
	)
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
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
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
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Error("server shutdown error", "err", err)
	}
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
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	reg, err := flags.LoadYAML(path)
	if err != nil {
		log.Warn("failed to load feature registry; all features disabled", "path", path, "err", err)
		return flags.NewStaticSnapshot()
	}
	return reg.SnapshotFromRegistry()
}

// Run starts the academic-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	main()
	return nil
}
