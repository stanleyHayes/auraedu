// Package db provides shared PostgreSQL connection, transaction and tenant
// isolation helpers used by all AuraEDU Go services.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	// Registers the pgx database/sql driver used by Goose migrations.
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type DB struct {
	pool   *pgxpool.Pool
	dsn    string
	schema string
}

type Config struct {
	DSN        string
	MaxConns   int32
	MinConns   int32
	Migrations string
	// Schema pins this service's tables, migration history and every pooled
	// connection to one PostgreSQL schema so that many services can share a
	// single database instance without colliding. Empty (the default) keeps
	// the server's own search_path, which is the dedicated-database topology.
	// When empty, DATABASE_SCHEMA is consulted so the topology can be chosen
	// by deployment configuration without touching service code.
	Schema string
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
	schema, err := resolveSchema(cfg.Schema)
	if err != nil {
		return nil, err
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: parse DSN: %w", err)
	}
	maxConns, err := resolveMaxConns(cfg.MaxConns)
	if err != nil {
		return nil, err
	}
	if maxConns > 0 {
		poolCfg.MaxConns = maxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if schema != "" {
		// Every pooled connection starts in the service's own schema. public
		// stays on the path so shared extension functions remain reachable.
		poolCfg.ConnConfig.RuntimeParams["search_path"] = schema + ", public"
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	d := &DB{pool: pool, dsn: cfg.DSN, schema: schema}
	if schema != "" {
		if err := d.ensureSchema(ctx, schema); err != nil {
			pool.Close()
			return nil, err
		}
	}
	if cfg.Migrations != "" {
		if err := d.RunMigrations(ctx, cfg.Migrations); err != nil {
			pool.Close()
			return nil, fmt.Errorf("db: run migrations: %w", err)
		}
	}
	return d, nil
}

// schemaPattern is deliberately strict: the schema name is interpolated into
// DDL that cannot be parameterized, so only unquoted lower-case identifiers
// are accepted and anything else fails closed.
var schemaPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// resolveSchema prefers the explicit config value and otherwise falls back to
// DATABASE_SCHEMA, so the single-database topology is a deployment choice
// rather than a code change in each service.
func resolveSchema(configured string) (string, error) {
	schema := strings.TrimSpace(configured)
	if schema == "" {
		schema = strings.TrimSpace(os.Getenv("DATABASE_SCHEMA"))
	}
	if schema == "" {
		return "", nil
	}
	if !schemaPattern.MatchString(schema) {
		return "", fmt.Errorf("db: invalid schema name %q: expected an unquoted lower-case identifier", schema)
	}
	return schema, nil
}

// resolveMaxConns prefers the explicit config value and otherwise consults
// DATABASE_MAX_CONNS. pgx otherwise defaults each pool to max(4, NumCPU); when
// the whole fleet shares one PostgreSQL instance those defaults multiply by the
// number of running services and can exhaust the server's connection limit, so
// the ceiling has to be settable per deployment without a code change.
func resolveMaxConns(configured int32) (int32, error) {
	if configured > 0 {
		return configured, nil
	}
	raw := strings.TrimSpace(os.Getenv("DATABASE_MAX_CONNS"))
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("db: invalid DATABASE_MAX_CONNS %q: expected a positive integer", raw)
	}
	return int32(parsed), nil
}

func (d *DB) ensureSchema(ctx context.Context, schema string) error {
	// schema is validated against schemaPattern before reaching this point.
	if _, err := d.pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		return fmt.Errorf("db: create schema %s: %w", schema, err)
	}
	return nil
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

// gooseMu guards Goose's process-global dialect and version-table settings so
// that services migrating different schemas concurrently in one process cannot
// apply one another's migration history.
var gooseMu sync.Mutex

// RunMigrations applies Goose migration scripts from dir using a dedicated sql.DB.
// When the handle is pinned to a schema, both the migration statements and the
// recorded migration history are confined to that schema, and the advisory lock
// is scoped to it so sibling services migrate independently.
func (d *DB) RunMigrations(ctx context.Context, dir string) (returnErr error) {
	dsn := d.dsn
	if d.schema != "" {
		connCfg, err := pgx.ParseConfig(d.dsn)
		if err != nil {
			return fmt.Errorf("parse DSN for migrations: %w", err)
		}
		connCfg.RuntimeParams["search_path"] = d.schema + ", public"
		dsn = stdlib.RegisterConnConfig(connCfg)
		defer stdlib.UnregisterConnConfig(dsn)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql db for migrations: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, sqlDB.Close()) }()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sql db for migrations: %w", err)
	}
	lockConn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration lock connection: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, lockConn.Close()) }()
	lockKey := "auraedu.migrations"
	if d.schema != "" {
		lockKey = d.schema + ":" + lockKey
	}
	const migrationLock = "hashtextextended(current_database() || ':' || $1, 0)"
	if _, err := lockConn.ExecContext(ctx, "SELECT pg_advisory_lock("+migrationLock+")", lockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, unlockErr := lockConn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock("+migrationLock+")", lockKey)
		returnErr = errors.Join(returnErr, unlockErr)
	}()

	gooseMu.Lock()
	defer gooseMu.Unlock()
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	// Goose records history in a process-global table name; restore the default
	// so an unpinned handle in the same process is unaffected.
	if d.schema != "" {
		goose.SetTableName(d.schema + ".goose_db_version")
		defer goose.SetTableName("goose_db_version")
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
	return d.setPlatformAdmin(ctx)
}

func (d *DB) setPlatformAdmin(ctx context.Context) error {
	value := "false"
	if actor, ok := auth.ActorFromContext(ctx); ok && actor.PlatformAdmin {
		value = "true"
	}
	_, err := d.pool.Exec(ctx, "SELECT set_config('app.is_platform_admin', $1, false)", value)
	if err != nil {
		return fmt.Errorf("db: set is_platform_admin: %w", err)
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
	return SetPlatformAdmin(ctx, q)
}

// SetPlatformAdmin configures the PostgreSQL session variable app.is_platform_admin
// based on the actor in context. Platform super admins may bypass tenant isolation.
func SetPlatformAdmin(ctx context.Context, q Querier) error {
	value := "false"
	if actor, ok := auth.ActorFromContext(ctx); ok && actor.PlatformAdmin {
		value = "true"
	}
	_, err := q.Exec(ctx, "SELECT set_config('app.is_platform_admin', $1, false)", value)
	if err != nil {
		return fmt.Errorf("db: set is_platform_admin: %w", err)
	}
	return nil
}

// ResetTenantID clears the PostgreSQL session variable app.tenant_id.
func ResetTenantID(ctx context.Context, q Querier) error {
	_, err := q.Exec(ctx, "SELECT set_config('app.tenant_id', '', false)")
	return err
}
