// Package servercmd wires and runs the content HTTP service.
package servercmd

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/auraedu/content-service/internal/adapters/generator"
	contenthttp "github.com/auraedu/content-service/internal/adapters/http"
	"github.com/auraedu/content-service/internal/adapters/postgres"
	"github.com/auraedu/content-service/internal/application"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
)

const serviceName = "content-service"

func Run(version string) error {
	log := observ.DefaultLogger()
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return err
	}
	if err := config.RequireProductionEnv("OPENAI_API_KEY"); err != nil {
		return err
	}
	shutdownTracing, err := observ.InitTracing(serviceName, version)
	if err != nil {
		return err
	}
	defer func() {
		if shutdownErr := shutdownTracing(context.Background()); shutdownErr != nil {
			log.Error("flush content service telemetry", "err", shutdownErr)
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
	contentGenerator, err := configuredGenerator()
	if err != nil {
		return err
	}
	repo := postgres.NewRepository(database)
	service := application.NewService(repo, contentGenerator, application.WithFeatureGate(featureGate(log)))
	mux := http.NewServeMux()
	health := httpx.NewHealth(serviceName, version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })
	health.Register(mux)
	contenthttp.NewHandler(service).Register(mux)
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(config.Port(8116)),
		Handler:           observ.HTTPHandler(serviceName, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		log.Info(serviceName+" listening", "addr", server.Addr)
		errorsChannel <- server.ListenAndServe()
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
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func configuredGenerator() (ports.Generator, error) {
	apiKey := strings.TrimSpace(config.Getenv("OPENAI_API_KEY", ""))
	if apiKey == "" {
		return generator.Disabled{}, nil
	}
	return generator.NewOpenAI(
		config.Getenv("OPENAI_BASE_URL", "https://api.openai.com"),
		apiKey,
		config.Getenv("CONTENT_AI_MODEL", "gpt-5.6-sol"),
		&http.Client{Timeout: 45 * time.Second},
	)
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
