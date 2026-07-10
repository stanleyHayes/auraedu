// Command server is the Identity Service HTTP entrypoint — login + token verification.
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

	svchttp "github.com/auraedu/identity-service/internal/adapters/http"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/application"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
)

const service = "identity-service"

var version = ""

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	// The gateway verifies tokens with the SAME key (JWT_SIGNING_KEY). Dev default is
	// insecure and must be overridden in every real environment (Render env group).
	signingKey := []byte(config.Getenv("JWT_SIGNING_KEY", "dev-insecure-signing-key-change-me"))

	repo, err := memory.New()
	if err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}
	svc := application.NewService(repo, signingKey, time.Hour)
	handler := svchttp.NewHandler(svc)

	mux := http.NewServeMux()
	httpx.NewHealth(service, version).WithLogger(log).Register(mux)
	handler.Register(mux)

	addr := ":" + strconv.Itoa(config.Port(8081))
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
