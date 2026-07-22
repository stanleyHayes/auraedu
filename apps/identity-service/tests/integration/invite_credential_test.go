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

type acceptingInviteNotifier struct{}

func (acceptingInviteNotifier) Deliver(context.Context, string, string, string, map[string]any) error {
	return nil
}

func TestInviteProvisioningActivatesOnlyCredentiallessExistingUser(t *testing.T) {
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
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("invite-credential-signing-key"), time.Hour, 7*24*time.Hour,
		application.WithTransactionalNotifier(acceptingInviteNotifier{}))
	actor := auth.Actor{
		TenantID: "upshs", Role: "school_admin",
		Permissions: []string{"users.create", "roles.assign"},
	}
	tenantCtx := tenancy.WithActor(ctx, actor)

	rollbackUser, err := svc.CreateUser(tenantCtx, actor, application.CreateUserInput{
		Email: "rollback-credentialless@upshs.example", Name: "Rollback User", Role: "teacher",
	})
	if err != nil {
		t.Fatalf("create rollback user: %v", err)
	}
	rollbackToken, err := svc.InviteUser(tenantCtx, actor, application.InviteInput{Email: rollbackUser.Email, Role: "teacher"})
	if err != nil {
		t.Fatalf("invite rollback user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION reject_invite_credential() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected invite credential failure'; END $$;
		CREATE TRIGGER reject_invite_credential
		BEFORE INSERT ON credentials FOR EACH ROW EXECUTE FUNCTION reject_invite_credential();
	`); err != nil {
		t.Fatalf("install credential failure trigger: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, rollbackToken, rollbackUser.Name, "rollback-password-123"); err == nil {
		t.Fatal("injected credential failure unexpectedly accepted invite")
	}
	verificationTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin invite rollback verification: %v", err)
	}
	if err := identitydb.SetTenantContext(ctx, verificationTx, "", true); err != nil {
		rollbackTx(ctx, t, verificationTx)
		t.Fatalf("set privileged verification context: %v", err)
	}
	var inviteUnused bool
	if err := verificationTx.QueryRow(ctx, `SELECT used_at IS NULL FROM invites WHERE token_hash = $1`, domain.HashToken(rollbackToken)).Scan(&inviteUnused); err != nil {
		rollbackTx(ctx, t, verificationTx)
		t.Fatalf("inspect rolled back invite: %v", err)
	}
	if err := verificationTx.Rollback(ctx); err != nil {
		t.Fatalf("close invite rollback verification: %v", err)
	}
	if !inviteUnused {
		t.Fatal("credential failure consumed invite outside the user transaction")
	}
	if _, err := pool.Exec(ctx, `DROP TRIGGER reject_invite_credential ON credentials; DROP FUNCTION reject_invite_credential()`); err != nil {
		t.Fatalf("remove credential failure trigger: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, rollbackToken, rollbackUser.Name, "rollback-password-123"); err != nil {
		t.Fatalf("rolled-back invite was not retryable: %v", err)
	}

	firstToken, err := svc.InviteUser(tenantCtx, actor, application.InviteInput{Email: "superseded@upshs.example", Role: "teacher"})
	if err != nil {
		t.Fatalf("first superseded invite: %v", err)
	}
	replacementToken, err := svc.InviteUser(tenantCtx, actor, application.InviteInput{Email: "superseded@upshs.example", Role: "teacher"})
	if err != nil {
		t.Fatalf("replacement invite: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, firstToken, "Superseded User", "superseded-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("superseded PostgreSQL invite remained usable: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, replacementToken, "Superseded User", "superseded-password-123"); err != nil {
		t.Fatalf("replacement PostgreSQL invite unusable: %v", err)
	}

	credentialless, err := svc.CreateUser(tenantCtx, actor, application.CreateUserInput{
		Email: "existing-credentialless@upshs.example", Name: "Credentialless", Role: "teacher",
	})
	if err != nil {
		t.Fatalf("create credentialless user: %v", err)
	}
	found, existingCredential, ok, err := repo.FindByEmail(tenantCtx, credentialless.Email)
	if err != nil || !ok || found.ID != credentialless.ID || len(existingCredential.Hash) != 0 {
		t.Fatalf("nullable credential lookup: user_id=%q hash_len=%d found=%v err=%v", found.ID, len(existingCredential.Hash), ok, err)
	}
	token, err := svc.InviteUser(tenantCtx, actor, application.InviteInput{Email: credentialless.Email, Role: "teacher"})
	if err != nil {
		t.Fatalf("invite credentialless user: %v", err)
	}
	activated, err := svc.AcceptInvite(ctx, token, credentialless.Name, "credentialless-password-123")
	if err != nil || activated.ID != credentialless.ID {
		t.Fatalf("activate credentialless user: user=%+v err=%v", activated, err)
	}
	if _, _, _, _, err := svc.Login(tenancy.WithActor(ctx, auth.Actor{TenantID: "upshs"}), credentialless.Email, "credentialless-password-123"); err != nil {
		t.Fatalf("installed credential unusable: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, token, credentialless.Name, "different-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("replayed invite replaced credential: %v", err)
	}

	credentialed, err := svc.CreateUser(tenantCtx, actor, application.CreateUserInput{
		Email: "existing-credentialed@upshs.example", Name: "Credentialed", Role: "teacher", Password: "existing-password-123",
	})
	if err != nil {
		t.Fatalf("create credentialed user: %v", err)
	}
	credentialedToken, err := svc.InviteUser(tenantCtx, actor, application.InviteInput{Email: credentialed.Email, Role: "teacher"})
	if err != nil {
		t.Fatalf("invite credentialed user: %v", err)
	}
	if _, err := svc.AcceptInvite(ctx, credentialedToken, credentialed.Name, "attacker-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("existing credential was replaceable: %v", err)
	}
	if _, _, _, _, err := svc.Login(tenancy.WithActor(ctx, auth.Actor{TenantID: "upshs"}), credentialed.Email, "existing-password-123"); err != nil {
		t.Fatalf("original credential changed: %v", err)
	}
}
