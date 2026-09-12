// Package servercmd provides the audit-service server command.
package servercmd

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	svchttp "github.com/auraedu/audit-service/internal/adapters/http"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/audit-service/internal/persistence"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/observ"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const service = "audit-service"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
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
	database, err := persistence.Open(ctx)
	if err != nil {
		return err
	}
	defer database.Close()

	health := httpx.NewHealth(service, version).WithLogger(log)
	health.AddReadinessCheck(database.Driver, func() error { return database.Ping(ctx) })

	repo := database.Repository
	query := application.NewQuery(repo)

	mux := http.NewServeMux()
	svchttp.NewHandler(health, query).Register(mux)

	addr := net.JoinHostPort(config.Getenv("BIND_HOST", ""), strconv.Itoa(config.Port(8080)))
	srv := &http.Server{
		Addr:              addr,
		Handler:           observ.HTTPHandler(service, httpx.RequestIDMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info(service+" listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
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

// Run starts the audit-service HTTP server. It is invoked by the service CLI.
func Run(serviceVersion string) error {
	version = serviceVersion
	return run()
}
