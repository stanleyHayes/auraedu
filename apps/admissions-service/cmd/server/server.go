// Package servercmd provides the Admissions Service HTTP server command.
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

	fileadapter "github.com/auraedu/admissions-service/internal/adapters/file"
	admissionshttp "github.com/auraedu/admissions-service/internal/adapters/http"
	"github.com/auraedu/admissions-service/internal/adapters/postgres"
	"github.com/auraedu/admissions-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
)

func Run(version string) error {
	log := observ.DefaultLogger()
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return err
	}
	shutdownTracing, err := observ.InitTracing("admissions-service", version)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdownTracing(context.Background()); shutdownErr != nil {
			log.Error("flush admissions telemetry", "err", shutdownErr)
		}
	}()
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
	fileVerifier := fileadapter.NewClient(config.Getenv("SERVICE_FILE_URL", ""), config.Getenv("INTERNAL_SERVICE_TOKEN", ""))
	svc := application.NewService(postgres.NewRepository(database), application.WithFeatureGate(featureGate(log)), application.WithDocumentVerifier(fileVerifier))
	mux := http.NewServeMux()
	health := httpx.NewHealth("admissions-service", version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })
	health.Register(mux)
	admissionshttp.NewHandler(svc).Register(mux)
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(config.Port(8114)),
		Handler:           observ.HTTPHandler("admissions-service", httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errs := make(chan error, 1)
	go func() { log.Info("admissions-service listening", "addr", srv.Addr); errs <- srv.ListenAndServe() }()
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
	return srv.Shutdown(shutdown)
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
