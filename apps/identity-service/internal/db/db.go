// Package db is a minimal local stub for platform/db (AURA-2.5).
package db

import (
	"context"
	"embed"
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

func MigrateFS(ctx context.Context, pool *pgxpool.Pool, fsys embed.FS, dir string) error {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("exec %s: %w", e.Name(), err)
		}
	}
	return nil
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
