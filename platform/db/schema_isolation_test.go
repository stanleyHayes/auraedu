package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Collapsing the per-service databases into one instance is only safe if a
// service pinned to its own schema cannot see, migrate or overwrite a sibling's
// tables — including when both services own a table of the same name.

func writeProbeMigration(t *testing.T, dir, table string) {
	t.Helper()
	migration := "-- +goose Up\nCREATE TABLE " + table + " (id integer PRIMARY KEY, owner text NOT NULL);\n\n-- +goose Down\nDROP TABLE " + table + ";\n"
	if err := os.WriteFile(filepath.Join(dir, "00001_probe.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
}

func TestSchemaPinnedServicesShareOneDatabaseWithoutColliding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL schema isolation test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	dsn := startPostgresContainer(ctx, t)

	// Two services, the same table name, one database.
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	writeProbeMigration(t, firstDir, "records")
	writeProbeMigration(t, secondDir, "records")

	first, err := Open(ctx, Config{DSN: dsn, Migrations: firstDir, Schema: "identity_svc"})
	if err != nil {
		t.Fatalf("open first service: %v", err)
	}
	defer first.Close()

	second, err := Open(ctx, Config{DSN: dsn, Migrations: secondDir, Schema: "student_svc"})
	if err != nil {
		t.Fatalf("open second service against the same database: %v", err)
	}
	defer second.Close()

	if _, err := first.Pool().Exec(ctx, `INSERT INTO records (id, owner) VALUES (1, 'identity')`); err != nil {
		t.Fatalf("first service insert: %v", err)
	}
	if _, err := second.Pool().Exec(ctx, `INSERT INTO records (id, owner) VALUES (1, 'student')`); err != nil {
		t.Fatalf("second service insert collided with the first: %v", err)
	}

	// Each service must read back only its own row through the unqualified name.
	var owner string
	if err := first.Pool().QueryRow(ctx, `SELECT owner FROM records WHERE id = 1`).Scan(&owner); err != nil {
		t.Fatalf("first service read: %v", err)
	}
	if owner != "identity" {
		t.Fatalf("first service read a sibling's row: got %q", owner)
	}
	if err := second.Pool().QueryRow(ctx, `SELECT owner FROM records WHERE id = 1`).Scan(&owner); err != nil {
		t.Fatalf("second service read: %v", err)
	}
	if owner != "student" {
		t.Fatalf("second service read a sibling's row: got %q", owner)
	}

	// Migration history must be per-schema, not shared.
	for _, probe := range []struct{ schema, table string }{
		{"identity_svc", "goose_db_version"},
		{"student_svc", "goose_db_version"},
	} {
		var exists bool
		if err := first.Pool().QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`,
			probe.schema, probe.table,
		).Scan(&exists); err != nil {
			t.Fatalf("inspect migration history: %v", err)
		}
		if !exists {
			t.Fatalf("%s.%s missing: migration history was not scoped to the schema", probe.schema, probe.table)
		}
	}
}

func TestSchemaIsResolvedFromEnvironmentAndFailsClosedOnInjection(t *testing.T) {
	for _, bad := range []string{"public; DROP SCHEMA public", "Identity", "1svc", "svc-name", "sv'c"} {
		if _, err := ResolveSchema(bad); err == nil {
			t.Fatalf("invalid schema name was accepted: %q", bad)
		}
	}
	for _, good := range []string{"identity_svc", "_x", "s1"} {
		got, err := ResolveSchema(good)
		if err != nil || got != good {
			t.Fatalf("valid schema rejected: %q got=%q err=%v", good, got, err)
		}
	}

	t.Setenv("DATABASE_SCHEMA", "billing_svc")
	got, err := ResolveSchema("")
	if err != nil || got != "billing_svc" {
		t.Fatalf("schema not resolved from environment: got=%q err=%v", got, err)
	}
	// An explicit config value must win over the environment.
	got, err = ResolveSchema("fees_svc")
	if err != nil || got != "fees_svc" {
		t.Fatalf("explicit schema did not take precedence: got=%q err=%v", got, err)
	}
}

func TestMaxConnsIsBoundedByDeploymentConfiguration(t *testing.T) {
	// Explicit configuration always wins.
	got, err := resolveMaxConns(7)
	if err != nil || got != 7 {
		t.Fatalf("explicit MaxConns ignored: got=%d err=%v", got, err)
	}

	// Unset means "leave pgx's own default alone".
	got, err = resolveMaxConns(0)
	if err != nil || got != 0 {
		t.Fatalf("unset MaxConns should not force a value: got=%d err=%v", got, err)
	}

	t.Setenv("DATABASE_MAX_CONNS", "3")
	got, err = resolveMaxConns(0)
	if err != nil || got != 3 {
		t.Fatalf("MaxConns not resolved from environment: got=%d err=%v", got, err)
	}

	// A shared instance must not be handed a nonsensical ceiling.
	for _, bad := range []string{"0", "-1", "many", "3.5"} {
		t.Setenv("DATABASE_MAX_CONNS", bad)
		if _, err := resolveMaxConns(0); err == nil {
			t.Fatalf("invalid DATABASE_MAX_CONNS accepted: %q", bad)
		}
	}
}

// Services own separate schemas but share one catalog. CREATE EXTENSION IF NOT EXISTS
// is not atomic against a concurrent creator, so schema-scoped migration locking let
// simultaneously booting services race and one died on pg_extension_name_index. This
// reproduces that boot: many schemas, all migrating an extension at once.
func TestConcurrentSchemaMigrationsShareTheCatalogSafely(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL shared-catalog migration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	dsn := startPostgresContainer(ctx, t)

	const services = 8
	migration := "-- +goose Up\n" +
		"CREATE EXTENSION IF NOT EXISTS \"pgcrypto\";\n" +
		"CREATE TABLE records (id UUID PRIMARY KEY DEFAULT gen_random_uuid());\n\n" +
		"-- +goose Down\nDROP TABLE records;\n"

	dirs := make([]string, services)
	for i := range dirs {
		dirs[i] = t.TempDir()
		if err := os.WriteFile(filepath.Join(dirs[i], "00001_probe.sql"), []byte(migration), 0o600); err != nil {
			t.Fatalf("write migration: %v", err)
		}
	}

	start := make(chan struct{})
	errs := make(chan error, services)
	handles := make(chan *DB, services)
	for i := range services {
		go func() {
			<-start
			database, err := Open(ctx, Config{
				DSN:        dsn,
				Migrations: dirs[i],
				Schema:     fmt.Sprintf("svc_%d", i),
			})
			if err == nil {
				handles <- database
			}
			errs <- err
		}()
	}
	close(start)

	var failures []error
	for range services {
		if err := <-errs; err != nil {
			failures = append(failures, err)
		}
	}
	close(handles)
	for h := range handles {
		defer h.Close()
	}
	if len(failures) > 0 {
		t.Fatalf("%d/%d services failed to migrate concurrently against the shared catalog: %v",
			len(failures), services, failures)
	}
}
