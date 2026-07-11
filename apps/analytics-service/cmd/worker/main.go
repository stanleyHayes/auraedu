// Command worker is the analytics-service event-sink entrypoint.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"

	svcevents "github.com/auraedu/analytics-service/internal/adapters/events"
	"github.com/auraedu/analytics-service/internal/adapters/postgres"
	"github.com/auraedu/analytics-service/internal/application"
)

const service = "analytics-service"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	log.Info(service + " worker started")

	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		log.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	repo := postgres.NewRepository(database)
	projection := application.NewProjection(repo, log)

	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		log.Error("NATS_URL is required for worker", "err", err)
		os.Exit(1)
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Error("failed to connect to NATS", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		log.Error("failed to create JetStream context", "err", err)
		os.Exit(1)
	}

	sub := svcevents.NewSubscriber(js, projection, log)
	if err := sub.Start(ctx); err != nil {
		log.Error("failed to start subscriber", "err", err)
		os.Exit(1)
	}
	defer func() { _ = sub.Stop() }()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info(service + " worker stopped")
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
