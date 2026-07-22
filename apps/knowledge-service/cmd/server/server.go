// Package servercmd wires and runs the knowledge HTTP service.
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

	knowledgehttp "github.com/auraedu/knowledge-service/internal/adapters/http"
	"github.com/auraedu/knowledge-service/internal/adapters/postgres"
	"github.com/auraedu/knowledge-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
)

const serviceName = "knowledge-service"

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
			log.Error("flush knowledge service telemetry", "err", shutdownErr)
		}
	}()
	slog.SetDefault(log)
	ctx := context.Background()
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if err != nil {
		return err
	}
	defer database.Close()
	repo := postgres.NewRepository(database)
	svc := application.NewService(repo, application.WithFeatureGate(featureGate(log)))
	mux := http.NewServeMux()
	health := httpx.NewHealth(serviceName, version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })
	health.Register(mux)
	handler := knowledgehttp.NewHandler(svc)
	handler.Register(mux)
	handler.RegisterInternal(mux, config.Getenv("INTERNAL_SERVICE_TOKEN", ""))
	server := &http.Server{Addr: ":" + strconv.Itoa(config.Port(8110)), Handler: observ.HTTPHandler(serviceName, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Info(serviceName+" listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()
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
	if registry, err := flags.LoadYAML(config.Getenv("FEATURES_REGISTRY", "/contracts/features.yaml")); err == nil {
		fallback = registry.SnapshotFromRegistry()
	} else {
		log.Warn("feature registry unavailable", "err", err)
	}
	return flags.NewRuntimeGate(config.Getenv("SERVICE_TENANT_URL", ""), fallback, log)
}
