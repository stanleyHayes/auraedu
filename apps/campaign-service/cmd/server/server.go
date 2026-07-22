// Package servercmd runs the Campaign Service HTTP server.
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

	campaignhttp "github.com/auraedu/campaign-service/internal/adapters/http"
	"github.com/auraedu/campaign-service/internal/adapters/postgres"
	"github.com/auraedu/campaign-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
)

func Run(version string) error {
	log := observ.DefaultLogger()
	shutdownTracing, err := observ.InitTracing("campaign-service", version)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdownTracing(context.Background()); shutdownErr != nil {
			log.Error("failed to shut down tracing", "err", shutdownErr)
		}
	}()
	slog.SetDefault(log)
	ctx := context.Background()
	dsn, e := config.MustGetenv("DATABASE_URL")
	if e != nil {
		return e
	}
	database, e := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if e != nil {
		return e
	}
	defer database.Close()
	repo := postgres.NewRepository(database)
	svc := application.NewService(repo, application.WithFeatureGate(featureGate(log)))
	mux := http.NewServeMux()
	health := httpx.NewHealth("campaign-service", version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })
	health.Register(mux)
	campaignhttp.NewHandler(svc).Register(mux)
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(config.Port(8113)),
		Handler:           observ.HTTPHandler("campaign-service", httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errs := make(chan error, 1)
	go func() { log.Info("campaign-service listening", "addr", server.Addr); errs <- server.ListenAndServe() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case e = <-errs:
		if !errors.Is(e, http.ErrServerClosed) {
			return e
		}
	case <-stop:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}
func featureGate(log *slog.Logger) flags.Gate {
	fallback := flags.NewStaticSnapshot()
	if registry, e := flags.LoadYAML(config.Getenv("FEATURES_REGISTRY", "/contracts/features.yaml")); e == nil {
		fallback = registry.SnapshotFromRegistry()
	} else {
		log.Warn("feature registry unavailable", "err", e)
	}
	return flags.NewRuntimeGate(config.Getenv("SERVICE_TENANT_URL", ""), fallback, log)
}
