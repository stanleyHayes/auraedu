// Package servercmd provides the notification-service server command.
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

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	svcevents "github.com/auraedu/notification-service/internal/adapters/events"
	svchttp "github.com/auraedu/notification-service/internal/adapters/http"
	"github.com/auraedu/notification-service/internal/adapters/notifier"
	"github.com/auraedu/notification-service/internal/adapters/postgres"
	providerwebhooks "github.com/auraedu/notification-service/internal/adapters/webhooks"
	"github.com/auraedu/notification-service/internal/application"
)

const service = "notification-service"

type serverRuntime struct {
	publicAppURL       string
	environment        string
	unsubscribeManager *application.UnsubscribeManager
}

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	runtime, err := loadServerRuntime()
	if err != nil {
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

	pub, closePublisher := publisher(log)
	if closePublisher != nil {
		defer closePublisher()
	}
	gates := featureGates(log)

	messageRepo := postgres.NewMessageRepository(database)
	templateRepo := postgres.NewTemplateRepository(database)
	subscriptionRepo := postgres.NewSubscriptionRepository(database)
	announcementRepo := postgres.NewAnnouncementRepository(database)
	deviceRepo := postgres.NewDeviceTokenRepository(database)
	journeyRepo := postgres.NewJourneyRepository(database)
	notifiers, err := notifier.RegistryFromEnvWithPush(deviceRepo)
	if err != nil {
		return err
	}

	svc := application.NewService(messageRepo, templateRepo, subscriptionRepo,
		application.WithPublisher(pub),
		application.WithNotifiers(notifiers),
		application.WithFeatureGate(gates),
		application.WithAnnouncementRepository(announcementRepo),
		application.WithDeviceTokenRepository(deviceRepo),
		application.WithJourneyRepository(journeyRepo),
		application.WithPublicAppURL(runtime.publicAppURL),
		application.WithUnsubscribeManager(runtime.unsubscribeManager),
	)
	handler := svchttp.NewHandler(svc).WithInternalToken(config.Getenv("INTERNAL_SERVICE_TOKEN", ""))
	if err := configureProviderWebhooks(handler, runtime.environment); err != nil {
		return err
	}

	return serve(ctx, log, database, handler)
}

func loadServerRuntime() (serverRuntime, error) {
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return serverRuntime{}, err
	}
	if err := config.RequireProductionEnv("PUBLIC_APP_URL"); err != nil {
		return serverRuntime{}, err
	}
	if err := config.RequireProductionEnv("NOTIFICATION_UNSUBSCRIBE_SIGNING_KEY"); err != nil {
		return serverRuntime{}, err
	}
	if strings.EqualFold(config.Getenv("NOTIFICATION_PROVIDER", "mock"), "resend") {
		if err := config.RequireProductionEnv("RESEND_WEBHOOK_SECRET"); err != nil {
			return serverRuntime{}, err
		}
	}
	environment := config.Getenv("ENVIRONMENT", "development")
	publicAppURL := config.Getenv("PUBLIC_APP_URL", "http://localhost:3000")
	if err := application.ValidatePublicAppURL(
		publicAppURL,
		strings.EqualFold(environment, "production"),
	); err != nil {
		return serverRuntime{}, err
	}
	var unsubscribeManager *application.UnsubscribeManager
	if key := config.Getenv("NOTIFICATION_UNSUBSCRIBE_SIGNING_KEY", ""); key != "" {
		manager, managerErr := application.NewUnsubscribeManager(key, publicAppURL)
		if managerErr != nil {
			return serverRuntime{}, managerErr
		}
		unsubscribeManager = manager
	}
	return serverRuntime{
		publicAppURL:       publicAppURL,
		environment:        environment,
		unsubscribeManager: unsubscribeManager,
	}, nil
}

func configureProviderWebhooks(handler *svchttp.Handler, environment string) error {
	if secret := config.Getenv("RESEND_WEBHOOK_SECRET", ""); secret != "" {
		verifier, err := providerwebhooks.NewResendVerifier(secret)
		if err != nil {
			return err
		}
		handler.WithResendWebhookVerifier(verifier)
	}
	if accountSID := config.Getenv("TWILIO_ACCOUNT_SID", ""); accountSID != "" {
		allowInsecure := !strings.EqualFold(environment, "production")
		verifier, err := providerwebhooks.NewTwilioVerifier(
			accountSID,
			config.Getenv("TWILIO_AUTH_TOKEN", ""),
			config.Getenv("TWILIO_STATUS_CALLBACK_URL", ""),
			allowInsecure,
		)
		if err != nil {
			return err
		}
		handler.WithTwilioWebhookVerifier(verifier)
	}
	return nil
}

func serve(ctx context.Context, log *slog.Logger, database *db.DB, handler *svchttp.Handler) error {
	health := httpx.NewHealth(service, version).WithLogger(log)
	health.AddReadinessCheck("postgres", func() error { return database.Ping(ctx) })

	mux := http.NewServeMux()
	health.Register(mux)
	handler.Register(mux)

	addr := ":" + strconv.Itoa(config.Port(8080))
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
	case listenErr := <-errorsChannel:
		if !errors.Is(listenErr, http.ErrServerClosed) {
			return listenErr
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

func publisher(log *slog.Logger) (*svcevents.Publisher, func()) {
	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; event publishing disabled")
		return svcevents.NewPublisher(nil), nil
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil), nil
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		log.Error("failed to create JetStream context; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil), nil
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		nc.Close()
		log.Error("failed to ensure NATS stream; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil), nil
	}
	log.Info("event publishing enabled", "nats_url", natsURL)
	return svcevents.NewPublisher(eventbus.NewPublisher(js)), nc.Close
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

// Run starts the notification-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
