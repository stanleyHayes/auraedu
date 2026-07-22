// Package servercmd provides the tenant-service server command.
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
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
	"github.com/auraedu/tenant-service/internal/adapters/events"
	svchttp "github.com/auraedu/tenant-service/internal/adapters/http"
	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/adapters/postgres"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/nats-io/nats.go"
)

const service = "tenant-service"

var version = ""

func main() {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if err := validateProductionRuntime(); err != nil {
		log.Error("invalid production runtime configuration", "err", err)
		os.Exit(1)
	}
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}
	shutdownTracing, err := observ.InitTracing(service, version)
	if err != nil {
		log.Error("failed to initialize tracing", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Error("failed to flush tracing", "err", err)
		}
	}()

	ctx := context.Background()

	repo, repoReady, closeDB := mustInitRepository(ctx, log)
	if closeDB != nil {
		defer closeDB()
	}

	pub := mustInitPublisher(ctx, log)
	svc := application.NewService(repo, pub)
	handler := svchttp.NewHandler(svc)

	health := httpx.NewHealth(service, version).WithLogger(log)
	if repoReady != nil {
		health.AddReadinessCheck("postgres", repoReady)
	}
	mux := http.NewServeMux()
	health.Register(mux)
	handler.Register(mux)
	handler.RegisterInternal(mux, config.Getenv("INTERNAL_SERVICE_TOKEN", ""))

	addr := ":" + strconv.Itoa(config.Port(8082))
	srv := &http.Server{
		Addr:              addr,
		Handler:           observ.HTTPHandler(service, httpx.RequestIDMiddleware(mux)),
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown server", "err", err)
	}
	log.Info(service + " stopped")
}

func validateProductionRuntime() error {
	if config.Getenv("ENVIRONMENT", "development") != "production" {
		return nil
	}
	for _, key := range []string{"DATABASE_URL", "INTERNAL_SERVICE_TOKEN"} {
		if config.Getenv(key, "") == "" {
			return errors.New(key + " is required in production")
		}
	}
	return nil
}

func mustInitRepository(ctx context.Context, log *slog.Logger) (ports.Repository, func() error, func()) {
	if dsn := config.Getenv("DATABASE_URL", ""); dsn != "" {
		database, err := db.Open(ctx, db.Config{
			DSN:        dsn,
			Migrations: config.Getenv("MIGRATIONS_PATH", "migrations"),
		})
		if err != nil {
			log.Error("database init failed", "err", err)
			os.Exit(1)
		}
		return postgres.NewRepository(database), readinessCheck(database), func() { database.Close() }
	}
	log.Info("DATABASE_URL not set; using in-memory development repository")
	return memory.New(), nil, nil
}

type databasePinger interface {
	Ping(context.Context) error
}

func readinessCheck(database databasePinger) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return database.Ping(ctx)
	}
}

// mustInitPublisher supports the non-durable development repository. The
// production PostgreSQL adapter commits lifecycle events to its transactional
// outbox, which the required tenant worker delivers independently; therefore a
// temporary broker outage must not prevent the authoritative API from starting.
func mustInitPublisher(_ context.Context, log *slog.Logger) application.Option {
	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; event publishing disabled")
		return application.WithPublisher(events.NewPublisher(nil))
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS; event publishing disabled", "err", err)
		return application.WithPublisher(events.NewPublisher(nil))
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Error("failed to create JetStream context; event publishing disabled", "err", err)
		return application.WithPublisher(events.NewPublisher(nil))
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream; event publishing disabled", "err", err)
		return application.WithPublisher(events.NewPublisher(nil))
	}
	log.Info("event publishing enabled", "nats_url", natsURL)
	return application.WithPublisher(events.NewPublisher(eventbus.NewPublisher(js)))
}

// Run starts the tenant-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	main()
	return nil
}
