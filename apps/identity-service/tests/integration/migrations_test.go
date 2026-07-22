package integration

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/platform/testkit"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationsRestartConcurrencyAndRLSIsolation(t *testing.T) {
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "")
	pool, err := identitydb.Open(ctx, tdb.DSN)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- identitydb.Migrate(ctx, pool) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent migration: %v", err)
		}
	}
	if err := identitydb.Migrate(ctx, pool); err != nil {
		t.Fatalf("restart migration: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM identity_schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("ledger: %v", err)
	}
	entries, err := identitydb.MigrationsFS().ReadDir("migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	expectedMigrations := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			expectedMigrations++
		}
	}
	if count != expectedMigrations {
		t.Fatalf("expected %d applied migrations, got %d", expectedMigrations, count)
	}
	for _, table := range []string{"users", "invites", "identity_processed_events", "identity_outbox", "user_mfa"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("expected table %s, exists=%v err=%v", table, exists, err)
		}
	}

	for _, table := range []string{"users", "credentials", "refresh_tokens", "password_resets", "invites", "identity_processed_events", "identity_outbox", "user_mfa"} {
		var enabled, forced bool
		if err := pool.QueryRow(ctx, `
			SELECT relrowsecurity, relforcerowsecurity
			FROM pg_class WHERE oid = $1::regclass
		`, table).Scan(&enabled, &forced); err != nil {
			t.Fatalf("inspect RLS for %s: %v", table, err)
		}
		if !enabled || !forced {
			t.Fatalf("expected enabled and forced RLS for %s, enabled=%v forced=%v", table, enabled, forced)
		}
	}

	assertRuntimeTenantIsolation(ctx, t, pool, tdb.DSN)
}

func TestPlatformIdentityEmailIsGloballyUnique(t *testing.T) {
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "")
	pool, err := identitydb.Open(ctx, tdb.DSN)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	if err := identitydb.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	seedTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin seed: %v", err)
	}
	if err := identitydb.SetTenantContext(ctx, seedTx, "", true); err != nil {
		rollbackTx(ctx, t, seedTx)
		t.Fatalf("set seed context: %v", err)
	}
	if _, err := seedTx.Exec(ctx, `
		INSERT INTO users (tenant_id, email, name, role) VALUES
			(NULL, 'platform@auraedu.example', 'Platform One', 'platform_super_admin'),
			('upshs', 'platform@auraedu.example', 'School One', 'school_admin'),
			('aboom-ame-zion-c', 'platform@auraedu.example', 'School Two', 'school_admin')
	`); err != nil {
		rollbackTx(ctx, t, seedTx)
		t.Fatalf("same email across distinct identity realms must remain valid: %v", err)
	}
	if err := seedTx.Commit(ctx); err != nil {
		t.Fatalf("commit seed: %v", err)
	}

	duplicateTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin duplicate probe: %v", err)
	}
	defer duplicateTx.Rollback(ctx) //nolint:errcheck // Cleanup is best effort and the primary operation determines the result.
	if err := identitydb.SetTenantContext(ctx, duplicateTx, "", true); err != nil {
		t.Fatalf("set duplicate context: %v", err)
	}
	_, err = duplicateTx.Exec(ctx, `
		INSERT INTO users (tenant_id, email, name, role)
		VALUES (NULL, '  PLATFORM@AURAEDU.EXAMPLE  ', 'Ambiguous Platform Identity', 'platform_super_admin')
	`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "users_platform_email_unique_idx" {
		t.Fatalf("expected platform email uniqueness violation, got %v", err)
	}
}

func assertRuntimeTenantIsolation(
	ctx context.Context,
	t *testing.T,
	owner *pgxpool.Pool,
	dsn string,
) {
	t.Helper()
	_, err := owner.Exec(ctx, `
		CREATE ROLE identity_runtime LOGIN PASSWORD 'runtime-test';
		GRANT USAGE ON SCHEMA public TO identity_runtime;
		GRANT SELECT, INSERT, DELETE ON identity_processed_events, identity_outbox TO identity_runtime;
		INSERT INTO identity_processed_events (event_id, event_type, tenant_id) VALUES
			('rls-upshs', 'tenant.onboarding_approved.v1', 'upshs'),
			('rls-aboom', 'tenant.onboarding_approved.v1', 'aboom-ame-zion-c');
		INSERT INTO identity_outbox (id, tenant_id, event_type, payload) VALUES
			('11111111-1111-4111-8111-111111111111', 'upshs', 'user.role_changed.v1', '{}'),
			('22222222-2222-4222-8222-222222222222', 'aboom-ame-zion-c', 'user.role_changed.v1', '{}');
	`)
	if err != nil {
		t.Fatalf("seed runtime RLS probe: %v", err)
	}

	runtimeDSN := strings.Replace(dsn, "test:test@", "identity_runtime:runtime-test@", 1)
	runtime, err := pgxpool.New(ctx, runtimeDSN)
	if err != nil {
		t.Fatalf("open runtime pool: %v", err)
	}
	defer runtime.Close()

	tx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatalf("begin runtime read: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := identitydb.SetTenantContext(ctx, tx, "upshs", false); err != nil {
		t.Fatalf("set runtime tenant: %v", err)
	}
	var count, outboxCount int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM identity_processed_events WHERE event_id LIKE 'rls-%'),
		       (SELECT count(*) FROM identity_outbox)
	`).Scan(&count, &outboxCount); err != nil {
		t.Fatalf("query runtime tenant: %v", err)
	}
	if count != 1 || outboxCount != 1 {
		t.Fatalf("expected runtime tenant to see one processed event and one outbox event, got %d and %d", count, outboxCount)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit runtime read: %v", err)
	}

	crossTx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatalf("begin cross-tenant write: %v", err)
	}
	defer crossTx.Rollback(ctx) //nolint:errcheck // Cleanup is best effort and the primary operation determines the result.
	if err := identitydb.SetTenantContext(ctx, crossTx, "upshs", false); err != nil {
		t.Fatalf("set cross-tenant context: %v", err)
	}
	if _, err := crossTx.Exec(ctx, `
		INSERT INTO identity_processed_events (event_id, event_type, tenant_id)
		VALUES ('rls-forbidden', 'tenant.onboarding_approved.v1', 'aboom-ame-zion-c')
	`); err == nil {
		t.Fatal("expected cross-tenant event claim to be denied by RLS")
	}
	rollbackTx(ctx, t, crossTx)

	outboxTx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatalf("begin cross-tenant outbox write: %v", err)
	}
	defer outboxTx.Rollback(ctx) //nolint:errcheck // Cleanup is best effort and the primary operation determines the result.
	if err := identitydb.SetTenantContext(ctx, outboxTx, "upshs", false); err != nil {
		t.Fatalf("set outbox tenant context: %v", err)
	}
	if _, err := outboxTx.Exec(ctx, `
		INSERT INTO identity_outbox (id, tenant_id, event_type, payload)
		VALUES ('33333333-3333-4333-8333-333333333333', 'aboom-ame-zion-c', 'user.role_changed.v1', '{}')
	`); err == nil {
		t.Fatal("expected cross-tenant outbox write to be denied by RLS")
	}
}
