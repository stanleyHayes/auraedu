// Package db owns Identity's embedded, advisory-lock-protected migration runner
// and tenant-scoped PostgreSQL helpers.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return MigrateFS(ctx, pool, migrationsFS, "migrations")
}

func MigrationsFS() embed.FS {
	return migrationsFS
}

func MigrateFS(ctx context.Context, pool *pgxpool.Pool, fsys embed.FS, dir string) (returnErr error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('auraedu.identity.migrations'))`); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, unlockErr := conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('auraedu.identity.migrations'))`)
		if unlockErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("release migration lock: %w", unlockErr))
		}
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS identity_schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if err := applyMigration(ctx, conn, fsys, dir, e); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, fsys embed.FS, dir string, entry fs.DirEntry) error {
	var applied bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM identity_schema_migrations WHERE version = $1)`, entry.Name()).Scan(&applied); err != nil {
		return fmt.Errorf("check %s: %w", entry.Name(), err)
	}
	if applied {
		return nil
	}
	if entry.Name() == "0001_init.sql" {
		var initialized bool
		if err := conn.QueryRow(ctx, `SELECT to_regclass('public.users') IS NOT NULL`).Scan(&initialized); err != nil {
			return fmt.Errorf("detect identity baseline: %w", err)
		}
		if initialized {
			_, err := conn.Exec(ctx, `INSERT INTO identity_schema_migrations (version) VALUES ($1)`, entry.Name())
			return err
		}
	}
	body, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
	if err != nil {
		return fmt.Errorf("read %s: %w", entry.Name(), err)
	}
	upSQL, err := migrationUpSQL(string(body))
	if err != nil {
		return fmt.Errorf("parse %s: %w", entry.Name(), err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", entry.Name(), err)
	}
	if _, err := tx.Exec(ctx, upSQL); err != nil {
		return errors.Join(fmt.Errorf("exec %s: %w", entry.Name(), err), tx.Rollback(ctx))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO identity_schema_migrations (version) VALUES ($1)`, entry.Name()); err != nil {
		return errors.Join(fmt.Errorf("record %s: %w", entry.Name(), err), tx.Rollback(ctx))
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", entry.Name(), err)
	}
	return nil
}

// migrationUpSQL returns only the forward section of a Goose-compatible
// migration. Identity embeds migrations into its binary and applies them with
// pgx, so executing the whole file would otherwise run the rollback section
// immediately after the forward statements. Legacy migrations without Goose
// annotations remain supported as forward-only files.
func migrationUpSQL(body string) (string, error) {
	const (
		upMarker   = "-- +goose Up"
		downMarker = "-- +goose Down"
	)

	up := strings.Index(body, upMarker)
	if up < 0 {
		if strings.TrimSpace(body) == "" {
			return "", errors.New("empty migration")
		}
		return body, nil
	}

	forward := body[up+len(upMarker):]
	if down := strings.Index(forward, downMarker); down >= 0 {
		forward = forward[:down]
	}
	if strings.TrimSpace(forward) == "" {
		return "", errors.New("empty goose up section")
	}
	return forward, nil
}

func SetTenantContext(ctx context.Context, tx pgx.Tx, tenantID string, isPlatformAdmin bool) error {
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return err
	}
	admin := "false"
	if isPlatformAdmin {
		admin = "true"
	}
	_, err := tx.Exec(ctx, "SELECT set_config('app.is_platform_admin', $1, true)", admin)
	return err
}

func ParsePagination(limit, cursor string) (int, string, error) {
	l := 25
	if limit != "" {
		if n, err := strconv.Atoi(limit); err == nil && n > 0 && n <= 100 {
			l = n
		} else if err == nil && (n <= 0 || n > 100) {
			return 0, "", fmt.Errorf("limit out of range")
		}
	}
	return l, cursor, nil
}
