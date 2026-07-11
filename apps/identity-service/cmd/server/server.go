// Package servercmd provides the identity-service server command.
package servercmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	svchttp "github.com/auraedu/identity-service/internal/adapters/http"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	"github.com/auraedu/identity-service/internal/adapters/session"
	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/ports"

	"github.com/auraedu/platform/config"
)

const service = "identity-service"

var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	ctx := context.Background()

	signingKey := []byte(config.Getenv("JWT_SIGNING_KEY", "dev-insecure-signing-key-change-me"))
	accessTTL := envDuration("JWT_ACCESS_TTL", 15*time.Minute)
	refreshTTL := envDuration("JWT_REFRESH_TTL", 7*24*time.Hour)

	repo := mustInitRepo(ctx, log)
	sessions := mustInitSessions(ctx, log)
	publisher := mustInitPublisher(ctx, log)

	svc := application.NewService(repo, sessions, publisher, signingKey, accessTTL, refreshTTL)
	handler := svchttp.NewHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"service": service, "version": version, "status": "healthy"})
	})
	handler.Register(mux)

	port := config.Getenv("PORT", "8081")
	addr := ":" + port
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
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
		log.Error("server shutdown error", "err", err)
	}
	log.Info(service + " stopped")
}

func mustInitRepo(ctx context.Context, log *slog.Logger) ports.Repository {
	databaseURL := config.Getenv("DATABASE_URL", "")
	if databaseURL == "" {
		log.Info("DATABASE_URL not set; using in-memory repository")
		repo, err := memory.New()
		if err != nil {
			log.Error("memory seed failed", "err", err)
			os.Exit(1)
		}
		return repo
	}
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		log.Error("open database failed", "err", err)
		os.Exit(1)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		log.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	return postgres.NewRepository(pool)
}

func mustInitSessions(ctx context.Context, log *slog.Logger) ports.SessionStore {
	store, err := session.NewFromEnv(ctx)
	if err != nil {
		log.Error("session store init failed", "err", err)
		os.Exit(1)
	}
	return store
}

func mustInitPublisher(ctx context.Context, log *slog.Logger) ports.EventPublisher {
	pub, err := events.NewPublisher(ctx, log)
	if err != nil {
		log.Error("event publisher init failed", "err", err)
		os.Exit(1)
	}
	return pub
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

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// Run starts the identity-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	main()
	return nil
}
