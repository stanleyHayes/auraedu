// Package workercmd provides the audit-service worker command.
// from the NATS JetStream event bus and persists them as immutable audit logs.
package workercmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auraedu/audit-service/internal/adapters/events"
	"github.com/auraedu/audit-service/internal/adapters/postgres"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/observ"

	// Register pgx SQL driver for database/sql based migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
)

const service = "audit-service-worker"

// version is injected via -ldflags "-X main.version=<sha>" (Dockerfile); falls back to env.
var version = ""

func run() error {
	log := observ.DefaultLogger()
	slog.SetDefault(log)
	if version == "" {
		version = config.Getenv("GIT_SHA", "dev")
	}

	ctx := context.Background()
	shutdownTelemetry, err := observ.InitTracing(service, version)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Error("flush audit worker telemetry", "err", err)
		}
	}()
	database, err := openDB(ctx)
	if err != nil {
		return err
	}
	defer database.Close()

	repo := postgres.NewRepository(database)
	sink := application.NewSink(repo)

	nc, js, err := connectNATS(log)
	if err != nil {
		return err
	}
	defer nc.Close()

	if _, err := eventbus.EnsureStream(js, "AURA"); err != nil {
		return err
	}

	metrics := observ.NewWorkerMetrics(service, "audit-sink")
	sub := events.NewSubscriber(js, log, metrics)
	if err := sub.Start(ctx, sink.Process); err != nil {
		return err
	}
	defer func() {
		if err := sub.Stop(); err != nil {
			log.Error("subscriber stop error", "err", err)
		}
	}()

	log.Info(service+" worker started", "version", version)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info(service + " worker stopped")
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

func connectNATS(log *slog.Logger) (*nats.Conn, eventbus.JetStreamContext, error) {
	natsURL := config.Getenv("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("nats jetstream: %w", err)
	}
	log.Info("connected to NATS", "url", natsURL)
	return nc, js, nil
}

// Run starts the audit-service background worker. It is invoked by the service CLI.
func Run() error {
	return run()
}
