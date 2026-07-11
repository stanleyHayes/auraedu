// Package db provides shared PostgreSQL connection, transaction and tenant
// isolation helpers used by all AuraEDU Go services.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/auraedu/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

type DB struct {
	pool *pgxpool.Pool
	dsn  string
}

type Config struct {
	DSN        string
	MaxConns   int32
	MinConns   int32
	Migrations string
}

// New opens a PostgreSQL pool from a DSN, runs migrations from the relative
// migrations directory and returns a shared DB handle. It is a convenience
// wrapper used by service main.go files.
func New(ctx context.Context, dsn string) (*DB, error) {
	return Open(ctx, Config{DSN: dsn, Migrations: "migrations"})
}

// Open opens a PostgreSQL pool, runs migrations when configured and returns a
// shared DB handle.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: parse DSN: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	d := &DB{pool: pool, dsn: cfg.DSN}
	if cfg.Migrations != "" {
		if err := d.RunMigrations(ctx, cfg.Migrations); err != nil {
			return nil, fmt.Errorf("db: run migrations: %w", err)
		}
	}
	return d, nil
}

func (d *DB) Close() {
	if d != nil && d.pool != nil {
		d.pool.Close()
	}
}

func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

func (d *DB) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

// Migrate applies Goose migration scripts from dir using a dedicated sql.DB.
// It is an alias for RunMigrations with a shorter name for service main.go files.
func (d *DB) Migrate(ctx context.Context, dir string) error {
	return d.RunMigrations(ctx, dir)
}

// RunMigrations applies Goose migration scripts from dir using a dedicated sql.DB.
func (d *DB) RunMigrations(ctx context.Context, dir string) error {
	sqlDB, err := sql.Open("pgx", d.dsn)
	if err != nil {
		return fmt.Errorf("open sql db for migrations: %w", err)
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sql db for migrations: %w", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	return goose.Up(sqlDB, dir)
}

type TxFn func(ctx context.Context, tx pgx.Tx) error

// WithTx runs fn inside a transaction and sets the tenant session variable.
func (d *DB) WithTx(ctx context.Context, fn TxFn) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := SetTenantID(ctx, tx); err != nil {
		return err
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := d.setTenantID(ctx); err != nil {
		return pgconn.CommandTag{}, err
	}
	return d.pool.Exec(ctx, sql, args...)
}

func (d *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := d.setTenantID(ctx); err != nil {
		return nil, err
	}
	return d.pool.Query(ctx, sql, args...)
}

func (d *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := d.setTenantID(ctx); err != nil {
		return &errRow{err: err}
	}
	return d.pool.QueryRow(ctx, sql, args...)
}

type errRow struct {
	err error
}

func (r *errRow) Scan(_ ...any) error {
	return r.err
}

func (d *DB) setTenantID(ctx context.Context) error {
	if id := tenancy.TenantID(ctx); id != "" {
		_, err := d.pool.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", id)
		if err != nil {
			return fmt.Errorf("db: set tenant_id: %w", err)
		}
	}
	return nil
}

type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// SetTenantID configures the PostgreSQL session variable app.tenant_id for the
// provided querier.
func SetTenantID(ctx context.Context, q Querier) error {
	id := tenancy.TenantID(ctx)
	if id == "" {
		return errors.New("db: cannot set app.tenant_id: tenant_id missing from context")
	}
	_, err := q.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", id)
	if err != nil {
		return fmt.Errorf("db: set app.tenant_id: %w", err)
	}
	return nil
}

// ResetTenantID clears the PostgreSQL session variable app.tenant_id.
func ResetTenantID(ctx context.Context, q Querier) error {
	_, err := q.Exec(ctx, "SELECT set_config('app.tenant_id', '', false)")
	return err
}
