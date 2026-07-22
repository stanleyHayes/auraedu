// Package servercmd starts the CRM HTTP service.
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

	"github.com/auraedu/crm-service/internal/adapters/events"
	crmhttp "github.com/auraedu/crm-service/internal/adapters/http"
	"github.com/auraedu/crm-service/internal/adapters/postgres"
	"github.com/auraedu/crm-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
	"github.com/nats-io/nats.go"
)

const serviceName = "crm-service"

func Run(version string) error {
	log := observ.DefaultLogger()
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return err
	}
	shutdownTracing, err := observ.InitTracing(serviceName, version)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdownTracing(context.Background()); shutdownErr != nil {
			log.Error("flush CRM service telemetry", "err", shutdownErr)
		}
	}()
	slog.SetDefault(log)
	ctx := context.Background()
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: "migrations"})
	if err != nil {
		return err
	}
	defer database.Close()
	repo := postgres.NewRepository(database)
	svc := application.NewService(
		repo,
		application.WithFeedbackRepository(repo),
		application.WithCallbackRepository(repo),
		application.WithFeatureGate(featureGate(log)),
		application.WithPublisher(eventPublisher(log)),
	)
	mux := http.NewServeMux()
	health := httpx.NewHealth(serviceName, version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })
	health.Register(mux)
	handler := crmhttp.NewHandler(svc)
	handler.Register(mux)
	handler.RegisterInternal(mux, config.Getenv("INTERNAL_SERVICE_TOKEN", ""))
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(config.Port(8080)),
		Handler:           observ.HTTPHandler(serviceName, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { log.Info("crm-service listening", "addr", server.Addr); errCh <- server.ListenAndServe() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-stop:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func featureGate(log *slog.Logger) flags.Gate {
	fallback := flags.NewStaticSnapshot()
	path := config.Getenv("FEATURES_REGISTRY", "../../contracts/features/features.yaml")
	if registry, err := flags.LoadYAML(path); err == nil {
		fallback = registry.SnapshotFromRegistry()
	} else {
		log.Warn("feature registry unavailable", "err", err)
	}
	return flags.NewRuntimeGate(config.Getenv("SERVICE_TENANT_URL", ""), fallback, log)
}
func eventPublisher(log *slog.Logger) *events.Publisher {
	url := config.Getenv("NATS_URL", "")
	if url == "" {
		return events.NewPublisher(nil)
	}
	connection, err := nats.Connect(url)
	if err != nil {
		log.Warn("NATS unavailable", "err", err)
		return events.NewPublisher(nil)
	}
	js, err := connection.JetStream()
	if err != nil {
		return events.NewPublisher(nil)
	}
	if _, err = eventbus.EnsureStream(js, "AURA"); err != nil {
		return events.NewPublisher(nil)
	}
	return events.NewPublisher(eventbus.NewPublisher(js))
}
