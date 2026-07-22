// Package servercmd provides the payment-service server command.
package servercmd

import (
	"context"
	"errors"
	"fmt"
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

	svcevents "github.com/auraedu/payment-service/internal/adapters/events"
	feesadapter "github.com/auraedu/payment-service/internal/adapters/fees"
	svchttp "github.com/auraedu/payment-service/internal/adapters/http"
	"github.com/auraedu/payment-service/internal/adapters/postgres"
	provideradapter "github.com/auraedu/payment-service/internal/adapters/provider"
	"github.com/auraedu/payment-service/internal/application"
	"github.com/auraedu/payment-service/internal/ports"
)

const service = "payment-service"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if err := config.RequireProductionEnv("INTERNAL_SERVICE_TOKEN"); err != nil {
		return fmt.Errorf("invalid production runtime configuration: %w", err)
	}
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}
	shutdownTracing, err := observ.InitTracing(service, version)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Error("failed to flush tracing", "err", err)
		}
	}()

	// Validate the money-moving provider before opening the database or applying
	// migrations. Production must never become healthy with a deterministic mock,
	// test credentials, or a secret-bearing client pointed at an alternate host.
	prov, err := paymentProvider()
	if err != nil {
		return fmt.Errorf("configure payment provider: %w", err)
	}

	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	pub, closePublisher := publisher(ctx, log)
	defer closePublisher()
	gates := featureGates(log)

	paymentRepo := postgres.NewPaymentRepository(database)
	transactionRepo := postgres.NewTransactionRepository(database)
	webhookRepo := postgres.NewWebhookEventRepository(database)

	svc := application.NewService(paymentRepo, transactionRepo, webhookRepo,
		application.WithPublisher(pub),
		application.WithPaymentProvider(prov),
		application.WithFeatureGate(gates),
		application.WithWebhookSecrets(paystackWebhookSecret(), config.Getenv("FLUTTERWAVE_WEBHOOK_SECRET", "")),
		application.WithInvoiceAccessResolver(feesadapter.NewClient(config.Getenv("SERVICE_FEES_URL", ""), config.Getenv("INTERNAL_SERVICE_TOKEN", ""))),
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
		Handler:           observ.HTTPHandler(service, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Info(service+" listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("serve HTTP: %w", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
	case err := <-serverErrors:
		return err
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
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

func publisher(_ context.Context, log *slog.Logger) (*svcevents.Publisher, func()) {
	natsURL := config.Getenv("NATS_URL", "")
	if natsURL == "" {
		log.Info("NATS_URL not set; event publishing disabled")
		return svcevents.NewPublisher(nil), func() {}
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS; event publishing disabled", "err", err)
		return svcevents.NewPublisher(nil), func() {}
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Error("failed to create JetStream context; event publishing disabled", "err", err)
		nc.Close()
		return svcevents.NewPublisher(nil), func() {}
	}
	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		log.Error("failed to ensure NATS stream; event publishing disabled", "err", err)
		nc.Close()
		return svcevents.NewPublisher(nil), func() {}
	}
	log.Info("event publishing enabled", "nats_url", natsURL)
	return svcevents.NewPublisher(eventbus.NewPublisher(js)), nc.Close
}

func featureGates(log *slog.Logger) flags.Gate {
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

// paymentProvider selects the gateway adapter via PAYMENTS_PROVIDER (mock|paystack,
// default mock). Mock payments are strictly local/development-only. Production
// requires a live Paystack key and the canonical Paystack API origin so a
// configuration mistake cannot simulate a charge or exfiltrate the provider secret.
func paymentProvider() (ports.PaymentProvider, error) {
	environment := strings.ToLower(strings.TrimSpace(config.Getenv("ENVIRONMENT", "development")))
	providerName := strings.ToLower(strings.TrimSpace(config.Getenv("PAYMENTS_PROVIDER", "mock")))
	isProduction := environment == "production"

	switch providerName {
	case "mock":
		if isProduction {
			return nil, errors.New("mock payment provider is forbidden in production")
		}
		return provideradapter.NewMockProvider(), nil
	case "paystack":
		secretKey, err := config.MustGetenv("PAYSTACK_SECRET_KEY")
		if err != nil {
			return nil, err
		}
		baseURL := strings.TrimRight(
			strings.TrimSpace(config.Getenv("PAYSTACK_BASE_URL", provideradapter.DefaultPaystackBaseURL)),
			"/",
		)
		if isProduction {
			if !strings.HasPrefix(strings.TrimSpace(secretKey), "sk_live_") {
				return nil, errors.New("production Paystack configuration requires a live secret key")
			}
			if baseURL != provideradapter.DefaultPaystackBaseURL {
				return nil, errors.New("production Paystack API origin must be https://api.paystack.co")
			}
		}
		return provideradapter.NewPaystackProvider(provideradapter.PaystackConfig{
			SecretKey: secretKey,
			BaseURL:   baseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported PAYMENTS_PROVIDER %q (want mock|paystack)", providerName)
	}
}

// paystackWebhookSecret resolves the webhook HMAC secret. Paystack signs webhooks
// with the API secret key, so PAYSTACK_WEBHOOK_SECRET falls back to it.
func paystackWebhookSecret() string {
	if v := config.Getenv("PAYSTACK_WEBHOOK_SECRET", ""); v != "" {
		return v
	}
	return config.Getenv("PAYSTACK_SECRET_KEY", "")
}

// Run starts the payment-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
