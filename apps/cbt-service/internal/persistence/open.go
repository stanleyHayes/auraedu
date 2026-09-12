// Package persistence selects the service's persistence adapter at startup.
package persistence

import (
	"context"
	"log/slog"

	adapter "github.com/auraedu/cbt-service/internal/adapters/mongo"
	"github.com/auraedu/cbt-service/internal/adapters/postgres"
	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/store"
)

type Repository interface {
	ports.Repository
	ports.LifecycleRepository
	ports.OutboxRepository
}
type Connection struct {
	Repository Repository
	Driver     string
	Close      func()
	Ping       func(context.Context) error
}

func Open(ctx context.Context) (*Connection, error) {
	driver, err := store.Selected()
	if err != nil {
		return nil, err
	}
	if driver.IsMongo() {
		s, err := pmongo.Open(ctx, pmongo.Config{URI: config.Getenv("MONGODB_URI", ""), Database: config.Getenv("MONGODB_DATABASE", "cbt-service"), MaxPoolSize: 2})
		if err != nil {
			return nil, err
		}
		r := adapter.NewRepository(s)
		if err = r.EnsureIndexes(ctx); err != nil {
			closeMongo(ctx, s)
			return nil, err
		}
		return &Connection{r, string(driver), func() { closeMongo(context.Background(), s) }, s.Ping}, nil
	}
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	s, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: "migrations"})
	if err != nil {
		return nil, err
	}
	return &Connection{postgres.NewRepository(s), string(driver), s.Close, s.Ping}, nil
}

func closeMongo(ctx context.Context, s *pmongo.Store) {
	if err := s.Close(ctx); err != nil {
		slog.Warn("close MongoDB connection", "error", err)
	}
}
