// Package servercmd provides the identity-service server command.
package servercmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	svchttp "github.com/auraedu/identity-service/internal/adapters/http"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	notificationadapter "github.com/auraedu/identity-service/internal/adapters/notification"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	"github.com/auraedu/identity-service/internal/adapters/session"
	tenantadapter "github.com/auraedu/identity-service/internal/adapters/tenant"
	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/ports"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"
	"github.com/nats-io/nats.go"
)

const service = "identity-service"

var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
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
	if err := validateProductionRuntime(); err != nil {
		return err
	}

	signingKeyValue, err := config.MustGetenv("JWT_SIGNING_KEY")
	if err != nil {
		return err
	}
	signingKey := []byte(signingKeyValue)
	accessTTL := envDuration("JWT_ACCESS_TTL", 15*time.Minute)
	refreshTTL := envDuration("JWT_REFRESH_TTL", 7*24*time.Hour)

	repo, repoReady, closeRepo, err := initRepo(ctx, log)
	if err != nil {
		return err
	}
	if closeRepo != nil {
		defer closeRepo()
	}
	sessions, err := initSessions(ctx)
	if err != nil {
		return err
	}
	publisher, closePublisher, err := initPublisher(log)
	if err != nil {
		return err
	}
	if closePublisher != nil {
		defer closePublisher()
	}

	svc := application.NewService(repo, sessions, publisher, signingKey, accessTTL, refreshTTL,
		application.WithPrivilegedMFA(
			config.Getenv("MFA_ENCRYPTION_KEY", signingKeyValue),
			config.Getenv("ENVIRONMENT", "development") == "production",
		),
		application.WithTransactionalNotifier(notificationadapter.NewClient(
			config.Getenv("SERVICE_NOTIFICATION_URL", "http://notification-service:8099"),
			config.Getenv("INTERNAL_SERVICE_TOKEN", ""),
		)),
		application.WithTenantActivator(tenantadapter.NewClient(
			config.Getenv("SERVICE_TENANT_URL", "http://tenant-service:8082"),
			config.Getenv("INTERNAL_SERVICE_TOKEN", ""),
		)),
	)
	handler := svchttp.NewHandler(svc)

	health := httpx.NewHealth(service, version).WithLogger(log)
	if repoReady != nil {
		health.AddReadinessCheck("postgres", repoReady)
	}
	mux := http.NewServeMux()
	health.Register(mux)
	handler.Register(mux)

	port := config.Getenv("PORT", "8081")
	addr := ":" + port
	srv := &http.Server{
		Addr:              addr,
		Handler:           observ.HTTPHandler(service, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info(service + " stopped")
	return nil
}

func validateProductionRuntime() error {
	if config.Getenv("ENVIRONMENT", "development") != "production" {
		return nil
	}
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "NATS_URL", "INTERNAL_SERVICE_TOKEN", "MFA_ENCRYPTION_KEY"} {
		if config.Getenv(key, "") == "" {
			return errors.New(key + " is required in production")
		}
	}
	if len(config.Getenv("MFA_ENCRYPTION_KEY", "")) < 32 {
		return errors.New("MFA_ENCRYPTION_KEY must contain at least 32 characters in production")
	}
	return nil
}

func initRepo(ctx context.Context, log *slog.Logger) (ports.Repository, func() error, func(), error) {
	databaseURL := config.Getenv("DATABASE_URL", "")
	if databaseURL == "" {
		if config.Getenv("ENVIRONMENT", "development") == "production" {
			return nil, nil, nil, errors.New("DATABASE_URL is required in production")
		}
		log.Info("DATABASE_URL not set; using in-memory repository")
		repo, err := memory.New()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("initialize memory repository: %w", err)
		}
		return repo, nil, nil, nil
	}
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, nil, err
	}
	return postgres.NewRepository(pool), readinessCheck(pool), pool.Close, nil
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

func initSessions(ctx context.Context) (ports.SessionStore, error) {
	store, err := session.NewFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// mustInitPublisher wires the platform/eventbus JetStream publisher. When
// NATS_URL is unset or the connection fails, publishing is disabled (noop),
// mirroring the other Go services (see student-service).
func initPublisher(log *slog.Logger) (ports.EventPublisher, func(), error) {
	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		if config.Getenv("ENVIRONMENT", "development") == "production" {
			return nil, nil, errors.New("NATS_URL is required in production")
		}
		log.Info("NATS_URL not set; event publishing disabled")
		return events.NewPublisher(nil), nil, nil
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		if config.Getenv("ENVIRONMENT", "development") == "production" {
			return nil, nil, fmt.Errorf("connect production NATS: %w", err)
		}
		log.Error("failed to connect to NATS; event publishing disabled", "err", err)
		return events.NewPublisher(nil), nil, nil
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		if config.Getenv("ENVIRONMENT", "development") == "production" {
			return nil, nil, fmt.Errorf("initialize production JetStream: %w", err)
		}
		log.Error("failed to create JetStream context; event publishing disabled", "err", err)
		return events.NewPublisher(nil), nil, nil
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		nc.Close()
		if config.Getenv("ENVIRONMENT", "development") == "production" {
			return nil, nil, fmt.Errorf("ensure production event stream: %w", err)
		}
		log.Error("failed to ensure NATS stream; event publishing disabled", "err", err)
		return events.NewPublisher(nil), nil, nil
	}
	log.Info("event publishing enabled", "nats_url", natsURL)
	return events.NewPublisher(eventbus.NewPublisher(js)), nc.Close, nil
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := config.Getenv(key, "")
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// Run starts the identity-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
