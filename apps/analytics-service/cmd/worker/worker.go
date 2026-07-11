// Package workercmd provides the analytics-service worker command.
package workercmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"

	// Register pgx SQL driver for database/sql based migrations.
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

	if err := run(log); err != nil {
		log.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx := context.Background()
	database, err := openDB(ctx)
	if err != nil {
		return err
	}
	defer database.Close()

	repo := postgres.NewRepository(database)
	projection := application.NewProjection(repo, log)

	natsURL, err := config.MustGetenv("NATS_URL")
	if err != nil {
		return err
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("create JetStream context: %w", err)
	}

	sub := svcevents.NewSubscriber(js, projection, log)
	if err := sub.Start(ctx); err != nil {
		return fmt.Errorf("start subscriber: %w", err)
	}
	defer func() {
		if err := sub.Stop(); err != nil {
			log.Error("subscriber stop error", "err", err)
		}
	}()

	log.Info(service + " worker started")
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

// Run starts the analytics-service background worker. It is invoked by the service CLI.
func Run() error {
	main()
	return nil
}
