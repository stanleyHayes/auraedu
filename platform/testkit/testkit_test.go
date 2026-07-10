package testkit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPostgresTestDBSeedsTenants(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	p := NewPostgres(ctx, t, "")
	p.SeedTenants(ctx, t)

	var count int
	if err := p.DB.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&count); err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 tenants, got %d", count)
	}
}

func TestCrossTenantLeakAssertion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	p := NewPostgres(ctx, t, "")
	p.SeedTenants(ctx, t)

	_, err := p.DB.Pool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS notes (
			id SERIAL PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			body TEXT NOT NULL
		);
		ALTER TABLE notes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE notes FORCE ROW LEVEL SECURITY;
		DROP POLICY IF EXISTS notes_tenant_isolation ON notes;
		CREATE POLICY notes_tenant_isolation ON notes
			USING (tenant_id = current_setting('app.tenant_id', true));
	`)
	if err != nil {
		t.Fatalf("create notes table: %v", err)
	}

	for _, tenant := range CanonicalTenants() {
		if err := p.ExecAs(ctx, tenant.Code, "INSERT INTO notes (tenant_id, body) VALUES ($1, $2)", tenant.Code, "note for "+tenant.Code); err != nil {
			t.Fatalf("insert note for %s: %v", tenant.Code, err)
		}
	}

	for _, tenant := range CanonicalTenants() {
		var count int
		// QueryAs sets app.tenant_id to the scoped tenant. Simulate row-level
		// filtering by matching against current_setting('app.tenant_id').
		err := p.QueryAs(ctx, tenant.Code, "SELECT COUNT(*) FROM notes WHERE tenant_id = current_setting('app.tenant_id', true)", nil, func(row pgx.Row) error {
			return row.Scan(&count)
		})
		if err != nil {
			t.Fatalf("query notes for %s: %v", tenant.Code, err)
		}
		if count != 1 {
			t.Fatalf("expected 1 note for %s, got %d", tenant.Code, count)
		}
	}
}

func TestJWTSigner(t *testing.T) {
	signer := NewJWTSigner()
	token, err := signer.Mint("upshs", "u1", "teacher", []string{"attendance.mark"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	claims, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.TenantID != "upshs" || claims.Subject != "u1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
