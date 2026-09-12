// Package persistence wires the configured driver to the service's ports.
package persistence

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	adapter "github.com/auraedu/analytics-service/internal/adapters/mongo"
	"github.com/auraedu/analytics-service/internal/adapters/postgres"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	pmongo "github.com/auraedu/platform/mongo"
	"github.com/auraedu/platform/store"
)

type Repositories struct {
	Repository ports.Repository
	Driver     store.Driver
	close      func()
	ping       func(context.Context) error
}

func (r *Repositories) Close() { r.close() }
func (r *Repositories) Ping(ctx context.Context) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.ping(bounded)
}
func Open(ctx context.Context) (*Repositories, error) {
	driver, err := store.Selected()
	if err != nil {
		return nil, err
	}
	r := &Repositories{Driver: driver}
	if driver.IsMongo() {
		return openMongo(ctx, r)
	}
	return openPostgres(ctx, r)
}
func openMongo(ctx context.Context, r *Repositories) (*Repositories, error) {
	uri, err := config.MustGetenv("MONGODB_URI")
	if err != nil {
		return nil, err
	}
	poolSize, err := strconv.ParseUint(config.Getenv("MONGODB_MAX_POOL_SIZE", "2"), 10, 64)
	if err != nil || poolSize == 0 {
		return nil, fmt.Errorf("MONGODB_MAX_POOL_SIZE must be a positive integer")
	}
	m, err := pmongo.Open(ctx, pmongo.Config{URI: uri, Database: config.Getenv("MONGODB_DATABASE", "auraedu_analytics"), MaxPoolSize: poolSize})
	if err != nil {
		return nil, err
	}
	r.close = func() {
		bounded, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if closeErr := m.Close(bounded); closeErr != nil {
			slog.Warn("close Mongo store", "error", closeErr)
		}
	}
	r.ping = m.Ping
	indexCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = m.RequireTransactions(indexCtx); err != nil {
		r.Close()
		return nil, err
	}
	if err = adapter.EnsureIndexes(indexCtx, m); err != nil {
		r.Close()
		return nil, err
	}
	r.Repository = adapter.NewRepository(m)
	return r, nil
}
func openPostgres(ctx context.Context, r *Repositories) (*Repositories, error) {
	dsn, err := config.MustGetenv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	p, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
	if err != nil {
		return nil, err
	}
	r.close = p.Close
	r.ping = p.Ping
	r.Repository = postgres.NewRepository(p)
	return r, nil
}
