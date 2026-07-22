package integration

import (
	"context"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/postgres"
	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestCleanupAuthArtifactsPreservesLiveFamiliesAndPendingDelivery(t *testing.T) {
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

	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	expiredFamily := uuid.New()
	liveFamily := uuid.New()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin seed: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := identitydb.SetTenantContext(ctx, tx, "", true); err != nil {
		t.Fatalf("set seed context: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO users (id, tenant_id, email, name, role) VALUES ($1, 'upshs', 'cleanup@upshs.example', 'Cleanup User', 'teacher')`, userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family_id, expires_at, revoked_at) VALUES
			($1, 'upshs', 'expired-old', $2, $4, $4),
			($1, 'upshs', 'expired-child', $2, $5, $5),
			($1, 'upshs', 'live-old', $3, $4, $4),
			($1, 'upshs', 'live-child', $3, $6, NULL)
	`, userID, expiredFamily, liveFamily,
		now.Add(-120*24*time.Hour),
		now.Add(-100*24*time.Hour),
		now.Add(24*time.Hour)); err != nil {
		t.Fatalf("seed refresh tokens: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO password_resets (user_id, tenant_id, token_hash, expires_at) VALUES
			($1, 'upshs', 'reset-old', $2),
			($1, 'upshs', 'reset-recent', $3)
	`, userID, now.Add(-120*24*time.Hour), now.Add(-5*24*time.Hour)); err != nil {
		t.Fatalf("seed resets: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO invites (tenant_id, email, role, token_hash, expires_at, used_at) VALUES
			('upshs', 'old@example.test', 'teacher', 'invite-old', $1, NULL),
			('upshs', 'used-recently@example.test', 'teacher', 'invite-used-recently', $1, $2),
			('upshs', 'unexpired@example.test', 'teacher', 'invite-unexpired', $3, NULL)
	`, now.Add(-120*24*time.Hour), now.Add(-5*24*time.Hour), now.Add(24*time.Hour)); err != nil {
		t.Fatalf("seed invites: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO identity_outbox (id, tenant_id, event_type, payload, created_at, published_at) VALUES
			($1, 'upshs', 'user.role_changed.v1', '{}', $4, $4),
			($2, 'upshs', 'user.role_changed.v1', '{}', $4, NULL),
			($3, 'upshs', 'user.role_changed.v1', '{}', $5, $5)
	`, uuid.New(), uuid.New(), uuid.New(), now.Add(-120*24*time.Hour), now.Add(-5*24*time.Hour)); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit seed: %v", err)
	}

	result, err := postgres.NewRepository(pool).CleanupAuthArtifacts(ctx, ports.AuthRetentionCutoffs{
		RefreshFamiliesBefore: now.Add(-24 * time.Hour),
		PasswordResetsBefore:  now.Add(-30 * 24 * time.Hour),
		InvitesBefore:         now.Add(-90 * 24 * time.Hour),
		PublishedOutboxBefore: now.Add(-30 * 24 * time.Hour),
		BatchSize:             1,
	})
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result != (ports.AuthCleanupResult{RefreshTokens: 2, PasswordResets: 1, Invites: 1, OutboxEvents: 1}) {
		t.Fatalf("cleanup result=%+v", result)
	}

	assertArtifactCount := func(query string, want int) {
		t.Helper()
		readTx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin assertion: %v", err)
		}
		if err := identitydb.SetTenantContext(ctx, readTx, "", true); err != nil {
			rollbackTx(ctx, t, readTx)
			t.Fatalf("set assertion context: %v", err)
		}
		var got int
		if err := readTx.QueryRow(ctx, query).Scan(&got); err != nil {
			rollbackTx(ctx, t, readTx)
			t.Fatalf("query artifact count: %v", err)
		}
		if err := readTx.Rollback(ctx); err != nil {
			t.Fatalf("close assertion transaction: %v", err)
		}
		if got != want {
			t.Fatalf("artifact count=%d want=%d for %s", got, want, query)
		}
	}
	assertArtifactCount(`SELECT count(*) FROM refresh_tokens WHERE token_hash LIKE 'expired-%'`, 0)
	assertArtifactCount(`SELECT count(*) FROM refresh_tokens WHERE token_hash LIKE 'live-%'`, 2)
	assertArtifactCount(`SELECT count(*) FROM password_resets WHERE token_hash = 'reset-old'`, 0)
	assertArtifactCount(`SELECT count(*) FROM password_resets WHERE token_hash = 'reset-recent'`, 1)
	assertArtifactCount(`SELECT count(*) FROM invites WHERE token_hash = 'invite-old'`, 0)
	assertArtifactCount(`SELECT count(*) FROM invites WHERE token_hash IN ('invite-used-recently', 'invite-unexpired')`, 2)
	assertArtifactCount(`SELECT count(*) FROM identity_outbox WHERE published_at IS NULL`, 1)
	assertArtifactCount(`SELECT count(*) FROM identity_outbox WHERE published_at IS NOT NULL`, 1)
}
