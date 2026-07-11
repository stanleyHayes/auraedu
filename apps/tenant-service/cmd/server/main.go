// Command server is the Tenant Service HTTP entrypoint — schools, branding, feature flags.
package main

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

	"github.com/auraedu/tenant-service/internal/adapters/events"
	svchttp "github.com/auraedu/tenant-service/internal/adapters/http"
	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/application"

	"github.com/auraedu/platform/config"
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

	// In-memory seeded store today; Postgres+RLS adapter is the next story.
	repo := memory.New()
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

func mustInitPublisher(_ context.Context, log *slog.Logger) application.Option {
	pub, err := events.NewPublisher(context.Background(), log)
	if err != nil {
		log.Error("event publisher init failed", "err", err)
		os.Exit(1)
	}
	return application.WithPublisher(pub)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("failed to encode health response", "err", err)
	}
}
