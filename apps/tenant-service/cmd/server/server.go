// Package servercmd provides the tenant-service server command.
package servercmd

import (
	"context"
	"encoding/json"
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
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	ctx := context.Background()

	repo, closeDB := mustInitRepository(ctx, log)
	if closeDB != nil {
		defer closeDB()
	}

	pub := mustInitPublisher(ctx, log)
	svc := application.NewService(repo, pub)
	handler := svchttp.NewHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"service": service, "version": version, "status": "healthy"})
	})
	handler.Register(mux)

	addr := ":" + strconv.Itoa(config.Port(8082))
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
		log.Error("failed to shutdown server", "err", err)
	}
	log.Info(service + " stopped")
}

func mustInitRepository(ctx context.Context, log *slog.Logger) (ports.Repository, func()) {
	if dsn := config.Getenv("DATABASE_URL", ""); dsn != "" {
		database, err := db.New(ctx, dsn)
		if err != nil {
			log.Error("database init failed", "err", err)
			os.Exit(1)
		}
		if err := database.Migrate(ctx, "migrations"); err != nil {
			log.Error("migrations failed", "err", err)
			os.Exit(1)
		}
		return postgres.NewRepository(database), func() { database.Close() }
	}
	return memory.New(), nil
}

// mustInitPublisher wires the platform/eventbus JetStream publisher. When
// NATS_URL is unset or the connection fails, publishing is disabled (noop),
// mirroring the other Go services (see student-service).
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

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("failed to encode health response", "err", err)
	}
}

// Run starts the tenant-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	main()
	return nil
}
