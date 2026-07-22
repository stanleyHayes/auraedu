package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	"github.com/auraedu/identity-service/internal/application"
	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/testkit"
)

func TestPasswordResetConsumptionCredentialAndRevocationAreAtomic(t *testing.T) {
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

	repo := postgres.NewRepository(pool)
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("reset-atomicity-signing-key"), time.Hour, 7*24*time.Hour)
	admin := auth.Actor{
		UserID: "platform-admin", PlatformAdmin: true, Role: auth.RolePlatformSuperAdmin,
		Permissions: []string{"users.create", "users.update", "roles.assign"},
	}
	user, err := svc.CreateUser(ctx, admin, application.CreateUserInput{
		TenantID: "upshs", Email: "atomic-reset@upshs.example", Name: "Reset User", Role: "teacher", Password: "original-password-123",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	tenantCtx := tenancy.WithActor(ctx, auth.Actor{TenantID: "upshs"})
	access, refresh, loggedInUser, expires, err := svc.Login(tenantCtx, user.Email, "original-password-123")
	if err != nil {
		t.Fatalf("login before reset: %v", err)
	}
	if access == "" || loggedInUser.ID != user.ID || expires.IsZero() {
		t.Fatalf("incomplete pre-reset session: user=%+v expires=%v", loggedInUser, expires)
	}
	const supersededToken = "superseded-password-reset-token-long-enough"
	const resetToken = "atomic-password-reset-token-that-is-long-enough"
	const siblingToken = "legacy-sibling-reset-token-that-is-long-enough"
	if err := repo.SavePasswordResetToken(ctx, "upshs", user.ID, domain.HashToken(supersededToken), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed superseded reset token: %v", err)
	}
	if err := repo.SavePasswordResetToken(ctx, "upshs", user.ID, domain.HashToken(resetToken), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed reset token: %v", err)
	}
	if err := svc.ResetPassword(tenantCtx, supersededToken, "superseded-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("superseded reset token remained usable: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO password_resets (tenant_id, user_id, token_hash, expires_at)
		VALUES ('upshs', $1, $2, NOW() + INTERVAL '1 hour')
	`, user.ID, domain.HashToken(siblingToken)); err != nil {
		t.Fatalf("seed legacy sibling reset token: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION fail_identity_credential_update() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced credential failure'; END $$
	`); err != nil {
		t.Fatalf("create failure function: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TRIGGER fail_identity_credential_update
		BEFORE UPDATE ON credentials
		FOR EACH ROW EXECUTE FUNCTION fail_identity_credential_update()
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if err := svc.ResetPassword(tenantCtx, resetToken, "replacement-password-123"); err == nil {
		t.Fatal("reset must fail when credential replacement cannot commit")
	}
	if _, err := pool.Exec(ctx, `DROP TRIGGER fail_identity_credential_update ON credentials`); err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}
	if _, err := pool.Exec(ctx, `DROP FUNCTION fail_identity_credential_update()`); err != nil {
		t.Fatalf("drop failure function: %v", err)
	}

	if err := svc.ResetPassword(tenantCtx, resetToken, "replacement-password-123"); err != nil {
		t.Fatalf("same token was not retryable after rollback: %v", err)
	}
	if _, _, _, _, err := svc.Refresh(ctx, refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("pre-reset refresh remained usable: %v", err)
	}
	if _, _, _, _, err := svc.Login(tenantCtx, user.Email, "original-password-123"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("old password remained usable: %v", err)
	}
	if _, _, _, _, err := svc.Login(tenantCtx, user.Email, "replacement-password-123"); err != nil {
		t.Fatalf("replacement password unusable: %v", err)
	}
	if err := svc.ResetPassword(tenantCtx, resetToken, "another-replacement-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("used reset token replay error=%v", err)
	}
	if err := svc.ResetPassword(tenantCtx, siblingToken, "sibling-replacement-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("sibling reset token survived successful reset: %v", err)
	}
}
