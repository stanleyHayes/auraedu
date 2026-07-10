// Command server is the Tenant Service HTTP entrypoint — schools, branding, feature flags.
package main

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

	svchttp "github.com/auraedu/tenant-service/internal/adapters/http"
	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/application"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
)

const service = "tenant-service"

var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	// In-memory seeded store today; Postgres+RLS adapter is the next story.
	repo := memory.New()
	svc := application.NewService(repo)
	handler := svchttp.NewHandler(svc)

	mux := http.NewServeMux()
	httpx.NewHealth(service, version).WithLogger(log).Register(mux)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info(service + " stopped")
}
