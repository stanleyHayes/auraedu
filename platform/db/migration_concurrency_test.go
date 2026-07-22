package db

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenSerializesConcurrentFirstRunMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL migration concurrency test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dsn := startPostgresContainer(ctx, t)
	migrations := t.TempDir()
	migration := `-- +goose Up
SELECT pg_sleep(0.1);
CREATE TABLE migration_serialization_probe (id integer PRIMARY KEY);

-- +goose Down
DROP TABLE migration_serialization_probe;
`
	if err := os.WriteFile(filepath.Join(migrations, "00001_create_probe.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	const starters = 8
	start := make(chan struct{})
	results := make(chan error, starters)
	dbs := make(chan *DB, starters)
	var wg sync.WaitGroup
	for range starters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			database, err := Open(ctx, Config{DSN: dsn, Migrations: migrations})
			if err == nil {
				dbs <- database
			}
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(dbs)

	for err := range results {
		if err != nil {
			t.Fatalf("concurrent migration startup: %v", err)
		}
	}
	var database *DB
	for opened := range dbs {
		t.Cleanup(opened.Close)
		if database == nil {
			database = opened
		}
	}
	if database == nil {
		t.Fatal("no database handle returned")
	}
	var applied int
	if err := database.Pool().QueryRow(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id = 1 AND is_applied`).Scan(&applied); err != nil {
		t.Fatalf("query migration ledger: %v", err)
	}
	if applied != 1 {
		t.Fatalf("applied migration count=%d, want 1", applied)
	}
}
