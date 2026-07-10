package db

import (
	"context"
	"testing"

	"github.com/auraedu/platform/tenancy"
)

func TestDBPingAndTenantSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := newTestDB(ctx, t)

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "upshs"})
	tx, err := db.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := SetTenantID(ctx, tx); err != nil {
		t.Fatalf("set tenant id: %v", err)
	}

	var got string
	if err := tx.QueryRow(ctx, "SELECT current_setting('app.tenant_id', true)").Scan(&got); err != nil {
		t.Fatalf("query tenant id: %v", err)
	}
	if got != "upshs" {
		t.Fatalf("expected upshs, got %q", got)
	}
}

func TestSetTenantIDRequiresTenant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := newTestDB(ctx, t)
	tx, err := db.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := SetTenantID(ctx, tx); err == nil {
		t.Fatal("expected error when tenant id missing")
	}
}

func newTestDB(ctx context.Context, t *testing.T) *DB {
	t.Helper()
	dsn := startPostgresContainer(ctx, t)
	db, err := Open(ctx, Config{DSN: dsn})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}
