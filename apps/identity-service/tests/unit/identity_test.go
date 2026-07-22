package unit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/domain"
	identitytenancy "github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
)

func newSvc(t *testing.T) (*application.Service, *events.RecordingPublisher) {
	t.Helper()
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	pub := events.NewRecordingPublisher()
	return application.NewService(repo, nil, pub, []byte("test-signing-key"), time.Hour, 7*24*time.Hour,
		application.WithTransactionalNotifier(&recordingNotifier{})), pub
}

func loginRefresh(ctx context.Context, t *testing.T, svc *application.Service) string {
	t.Helper()
	access, refresh, user, expires, err := svc.Login(ctx, "e.mensah@upshs.edu.gh", "password123")
	if err != nil || access == "" || refresh == "" || user.ID == "" || expires.IsZero() {
		t.Fatalf("login failed: user=%+v expires=%v err=%v", user, expires, err)
	}
	return refresh
}

func loginError(ctx context.Context, svc *application.Service, email, password string) error {
	access, refresh, user, expires, err := svc.Login(ctx, email, password)
	_ = access
	_ = refresh
	_ = user
	_ = expires
	return err
}

func refreshError(ctx context.Context, svc *application.Service, token string) error {
	access, refresh, user, expires, err := svc.Refresh(ctx, token)
	_ = access
	_ = refresh
	_ = user
	_ = expires
	return err
}

type recordingNotifier struct{ deliveries int }

func (n *recordingNotifier) Deliver(_ context.Context, _, _, _ string, _ map[string]any) error {
	n.deliveries++
	return nil
}

type retryingActivator struct {
	calls int
	fail  bool
}

func (a *retryingActivator) Activate(_ context.Context, tenantID string) error {
	a.calls++
	if tenantID != "upshs" {
		return errors.New("unexpected tenant")
	}
	if a.fail {
		return errors.New("tenant service unavailable")
	}
	return nil
}

func TestPasswordHashVerify(t *testing.T) {
	cred, err := domain.NewCredential("s3cret!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !cred.Verify("s3cret!") {
		t.Fatal("correct password should verify")
	}
	if cred.Verify("wrong") {
		t.Fatal("wrong password must not verify")
	}
}

func TestLoginIssuesUsableToken(t *testing.T) {
	svc, _ := newSvc(t)
	access, _, user, _, err := svc.Login(tenantContext("upshs"), "e.mensah@upshs.edu.gh", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if user.Role != "teacher" || user.TenantID != "upshs" {
		t.Fatalf("user mismatch: %+v", user)
	}
	claims, err := svc.Verify(access)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "u-teacher" || claims.TenantID != "upshs" || !contains(claims.Permissions, "attendance.mark") {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if claims.FeaturesHash != "" {
		t.Fatalf("features_hash claim missing/empty expected, got %q", claims.FeaturesHash)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, _ := newSvc(t)
	err := loginError(tenantContext("upshs"), svc, "e.mensah@upshs.edu.gh", "nope")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownEmailDoesNotEnumerate(t *testing.T) {
	svc, _ := newSvc(t)
	err := loginError(context.Background(), svc, "ghost@nowhere.gh", "password123")
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestSuperAdminHasNoTenantAndIsPlatformAdmin(t *testing.T) {
	svc, _ := newSvc(t)
	access, refresh, user, expires, err := svc.Login(context.Background(), "super@auraedu.dev", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if refresh == "" || user.ID == "" || expires.IsZero() {
		t.Fatalf("incomplete super-admin session: user=%+v expires=%v", user, expires)
	}
	claims, err := svc.Verify(access)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Role != auth.RolePlatformSuperAdmin || claims.TenantID != "" {
		t.Fatalf("super admin claims wrong: %+v", claims)
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	svc, _ := newSvc(t)
	refresh := loginRefresh(tenantContext("upshs"), t, svc)
	access2, refresh2, user, _, err := svc.Refresh(context.Background(), refresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if user.ID != "u-teacher" {
		t.Fatalf("user mismatch: %+v", user)
	}
	if access2 == "" || refresh2 == "" || refresh2 == refresh {
		t.Fatal("tokens should be rotated")
	}
	if err := refreshError(context.Background(), svc, refresh); err == nil {
		t.Fatal("old refresh token should be rejected")
	}
}

func TestRefreshTokenCanBeConsumedOnlyOnceConcurrently(t *testing.T) {
	svc, _ := newSvc(t)
	refresh := loginRefresh(tenantContext("upshs"), t, svc)

	type refreshResult struct {
		refresh string
		err     error
	}
	results := make(chan refreshResult, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, successor, _, _, refreshErr := svc.Refresh(context.Background(), refresh)
			results <- refreshResult{refresh: successor, err: refreshErr}
		}()
	}
	wg.Wait()
	close(results)

	succeeded, rejected := 0, 0
	successor := ""
	for result := range results {
		switch {
		case result.err == nil:
			succeeded++
			successor = result.refresh
		case errors.Is(result.err, domain.ErrExpiredToken):
			rejected++
		default:
			t.Fatalf("unexpected refresh result: %v", result.err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent refresh results: succeeded=%d rejected=%d", succeeded, rejected)
	}
	if err := refreshError(context.Background(), svc, successor); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("replay did not revoke the successor family: %v", err)
	}
}

func TestLogoutWithRotatedPredecessorRevokesTheWholeSessionFamily(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := tenantContext("upshs")
	predecessor := loginRefresh(ctx, t, svc)
	access, successor, user, expires, err := svc.Refresh(ctx, predecessor)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if access == "" || user.ID == "" || expires.IsZero() {
		t.Fatalf("incomplete rotated session: user=%+v expires=%v", user, expires)
	}
	actor := auth.Actor{UserID: "u-teacher", TenantID: "upshs", Role: "teacher"}
	if err := svc.Logout(ctx, actor, predecessor); err != nil {
		t.Fatalf("logout through predecessor: %v", err)
	}
	if err := refreshError(ctx, svc, successor); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("successor remained usable after family logout: %v", err)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := tenantContext("upshs")
	refresh := loginRefresh(ctx, t, svc)
	actor := auth.Actor{UserID: "u-teacher", TenantID: "upshs", Role: "teacher"}
	if err := svc.Logout(ctx, actor, refresh); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if err := refreshError(ctx, svc, refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("refresh after logout should fail, got %v", err)
	}
}

func TestLogoutCrossTenantForbidden(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := tenantContext("upshs")
	refresh := loginRefresh(ctx, t, svc)
	other := auth.Actor{UserID: "u-other", TenantID: "aboom", Role: "teacher"}
	if err := svc.Logout(ctx, other, refresh); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant logout should be forbidden, got %v", err)
	}
}

func TestRevokeSessionByHash(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := tenantContext("upshs")
	refresh := loginRefresh(ctx, t, svc)
	tokenHash := domain.HashToken(refresh)
	actor := auth.Actor{UserID: "u-teacher", TenantID: "upshs", Role: "teacher"}
	if err := svc.RevokeSession(ctx, actor, tokenHash); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, _, _, _, err := svc.Refresh(ctx, refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("refresh after revoke should fail, got %v", err)
	}
}

func TestUserCRUD(t *testing.T) {
	svc, pub := newSvc(t)
	actor := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.create", "users.read", "users.update", "roles.assign"}}
	ctx := tenantContext("upshs")

	if _, err := svc.CreateUser(ctx, actor, application.CreateUserInput{Email: "weak@upshs.edu.gh", Name: "Weak User", Role: "teacher", Password: "too-short"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("weak administrative password accepted: %v", err)
	}
	u, err := svc.CreateUser(ctx, actor, application.CreateUserInput{Email: "new@upshs.edu.gh", Name: "New User", Role: "teacher", Password: "passphrase-1234"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.TenantID != "upshs" {
		t.Fatalf("tenant mismatch: %q", u.TenantID)
	}

	users, err := svc.ListUsers(ctx, actor)
	if err != nil || len(users) != 3 {
		t.Fatalf("list users: %d, %v", len(users), err)
	}

	got, err := svc.GetUser(ctx, actor, u.ID)
	if err != nil || got.Email != "new@upshs.edu.gh" {
		t.Fatalf("get user: %v", err)
	}

	role := "principal"
	updated, err := svc.UpdateUser(ctx, actor, u.ID, application.UpdateUserInput{Role: &role})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Role != "principal" {
		t.Fatalf("role not updated: %q", updated.Role)
	}
	if len(pub.Events) != 1 || pub.Events[0].Type != "user.role_changed.v1" {
		t.Fatalf("expected role_changed event, got %+v", pub.Events)
	}

	if err := svc.DeleteUser(ctx, actor, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := svc.GetUser(ctx, actor, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestAuthorizationRegistryRejectsUnknownAndEscalatedGrants(t *testing.T) {
	svc, _ := newSvc(t)
	actor := auth.Actor{
		UserID: "u-admin", TenantID: "upshs", Role: "school_admin",
		Permissions: []string{"users.create", "roles.assign", "students.read"},
	}
	ctx := context.Background()

	if _, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "unknown-role@upshs.example", Name: "Unknown", Role: "wizard",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unknown role should fail validation, got %v", err)
	}
	if _, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "unknown-permission@upshs.example", Name: "Unknown", Role: "teacher",
		Permissions: []string{"students.read", "root.everything"},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unknown permission should fail validation, got %v", err)
	}
	if _, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "escalated@upshs.example", Name: "Escalated", Role: "teacher",
		Permissions: []string{"billing.manage"},
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("granting an unheld permission should be forbidden, got %v", err)
	}
	if _, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "platform@upshs.example", Name: "Platform", Role: auth.RolePlatformSuperAdmin,
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("tenant actor assigning platform role should be forbidden, got %v", err)
	}
}

func TestAuthorizationMutationsRequireTheSpecificPermission(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	role := "principal"
	name := "Renamed Teacher"
	updateOnly := auth.Actor{UserID: "update", TenantID: "upshs", Permissions: []string{"users.update"}}
	if _, err := svc.UpdateUser(ctx, updateOnly, "u-teacher", application.UpdateUserInput{Role: &role}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("users.update must not assign roles, got %v", err)
	}
	assignOnly := auth.Actor{UserID: "assign", TenantID: "upshs", Permissions: []string{"roles.assign", "attendance.mark"}}
	if _, err := svc.UpdateUser(ctx, assignOnly, "u-teacher", application.UpdateUserInput{Name: &name}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("roles.assign must not update profiles, got %v", err)
	}
	invalid := domain.UserStatus("deleted")
	if _, err := svc.UpdateUser(ctx, updateOnly, "u-teacher", application.UpdateUserInput{Status: &invalid}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unknown status should fail validation, got %v", err)
	}
	if _, err := svc.AssignRole(ctx, assignOnly, "u-teacher", role, []string{"attendance.mark"}); err != nil {
		t.Fatalf("roles.assign should grant a held permission: %v", err)
	}
}

func TestPrivilegeAndStatusChangesRevokeExistingRefreshSessions(t *testing.T) {
	t.Run("role promotion", func(t *testing.T) {
		svc, _ := newSvc(t)
		refresh := loginRefresh(tenantContext("upshs"), t, svc)
		platform := auth.Actor{UserID: "u-super", PlatformAdmin: true, Role: auth.RolePlatformSuperAdmin, Permissions: []string{"roles.assign"}}
		if _, err := svc.AssignRole(context.Background(), platform, "u-teacher", "school_admin", nil); err != nil {
			t.Fatalf("promote user: %v", err)
		}
		if err := refreshError(context.Background(), svc, refresh); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("pre-promotion refresh token remained usable: %v", err)
		}
	})

	t.Run("account lock", func(t *testing.T) {
		svc, _ := newSvc(t)
		refresh := loginRefresh(tenantContext("upshs"), t, svc)
		locked := domain.StatusLocked
		admin := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.update"}}
		if _, err := svc.UpdateUser(context.Background(), admin, "u-teacher", application.UpdateUserInput{Status: &locked}); err != nil {
			t.Fatalf("lock user: %v", err)
		}
		if err := refreshError(context.Background(), svc, refresh); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("locked user's refresh token remained usable: %v", err)
		}
	})
}

func TestInviteRequiresRoleAssignmentAuthority(t *testing.T) {
	svc, _ := newSvc(t)
	actor := auth.Actor{UserID: "creator", TenantID: "upshs", Permissions: []string{"users.create"}}
	if _, err := svc.InviteUser(context.Background(), actor, application.InviteInput{
		Email: "invite@upshs.example", Role: "teacher",
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("user creation alone must not grant invite roles, got %v", err)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	svc, pub := newSvc(t)
	ctx := tenantContext("upshs")
	if err := svc.RequestPasswordReset(ctx, "e.mensah@upshs.edu.gh"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if len(pub.Events) != 0 {
		t.Fatalf("password reset secrets must not be published, got %+v", pub.Events)
	}
}

func TestLoginAndPasswordResetAreBoundToResolvedTenant(t *testing.T) {
	repo, err := memory.New()
	if err != nil {
		t.Fatal(err)
	}
	aboomCredential, err := domain.NewCredential("aboom-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateUser(context.Background(), domain.User{
		ID:       "u-aboom-shared",
		TenantID: "aboom",
		Email:    "e.mensah@upshs.edu.gh",
		Name:     "Abena Mensah",
		Role:     "teacher",
		Status:   domain.StatusActive,
	}, aboomCredential); err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("test-signing-key"), time.Hour, 7*24*time.Hour,
		application.WithTransactionalNotifier(&recordingNotifier{}))

	upshsAccess, upshsRefresh, upshsUser, upshsExpiry, err := svc.Login(
		tenantContext("upshs"), "e.mensah@upshs.edu.gh", "password123",
	)
	if err != nil || upshsAccess == "" || upshsRefresh == "" || upshsExpiry.IsZero() || upshsUser.TenantID != "upshs" {
		t.Fatalf("upshs login crossed tenant: user=%+v err=%v", upshsUser, err)
	}
	aboomAccess, aboomRefresh, aboomUser, aboomExpiry, err := svc.Login(
		tenantContext("aboom"), "e.mensah@upshs.edu.gh", "aboom-password",
	)
	if err != nil || aboomAccess == "" || aboomRefresh == "" || aboomExpiry.IsZero() || aboomUser.TenantID != "aboom" {
		t.Fatalf("aboom login crossed tenant: user=%+v err=%v", aboomUser, err)
	}

	const resetToken = "tenant-bound-reset-token-which-is-long-enough"
	existingAccess, existingRefresh, existingUser, existingExpiry, err := svc.Login(
		tenantContext("upshs"), "e.mensah@upshs.edu.gh", "password123",
	)
	if err != nil || existingAccess == "" || existingUser.ID == "" || existingExpiry.IsZero() {
		t.Fatal(err)
	}
	if err := repo.SavePasswordResetToken(context.Background(), "upshs", "u-teacher", domain.HashToken(resetToken), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(tenantContext("upshs"), resetToken, "too-short"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("weak reset password accepted: %v", err)
	}
	if err := svc.ResetPassword(tenantContext("aboom"), resetToken, "new-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("cross-tenant reset was not denied: %v", err)
	}
	if err := svc.ResetPassword(tenantContext("upshs"), resetToken, "new-password-123"); err != nil {
		t.Fatalf("right-tenant reset failed after denied attempt: %v", err)
	}
	if _, _, _, _, err := svc.Refresh(context.Background(), existingRefresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("password reset did not revoke the existing session: %v", err)
	}
}

func TestNewPasswordResetRevokesPriorUnusedToken(t *testing.T) {
	repo, err := memory.New()
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("reset-supersession-key"), time.Hour, 7*24*time.Hour)
	const first = "first-reset-token-that-is-long-enough"
	const replacement = "replacement-reset-token-long-enough"
	expires := time.Now().Add(time.Hour)
	if err := repo.SavePasswordResetToken(context.Background(), "upshs", "u-teacher", domain.HashToken(first), expires); err != nil {
		t.Fatalf("first reset: %v", err)
	}
	if err := repo.SavePasswordResetToken(context.Background(), "upshs", "u-teacher", domain.HashToken(replacement), expires); err != nil {
		t.Fatalf("replacement reset: %v", err)
	}
	if err := svc.ResetPassword(tenantContext("upshs"), first, "superseded-reset-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("superseded reset remained usable: %v", err)
	}
	if err := svc.ResetPassword(tenantContext("upshs"), replacement, "replacement-reset-123"); err != nil {
		t.Fatalf("replacement reset unusable: %v", err)
	}
}

func TestInviteFlow(t *testing.T) {
	svc, pub := newSvc(t)
	actor := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.create", "roles.assign"}}
	ctx := context.Background()

	token, err := svc.InviteUser(ctx, actor, application.InviteInput{Email: "invited@upshs.edu.gh", Role: "teacher"})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if token == "" {
		t.Fatal("expected invite token")
	}
	if len(pub.Events) != 0 {
		t.Fatalf("invite secrets must not be published, got %+v", pub.Events)
	}

	if _, err := svc.AcceptInvite(ctx, token, "Invited User", "too-short"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("weak invite password accepted: %v", err)
	}
	u, err := svc.AcceptInvite(ctx, token, "Invited User", "welcome-user-123")
	if err != nil {
		t.Fatalf("accept invite: %v", err)
	}
	if u.Email != "invited@upshs.edu.gh" || u.Role != "teacher" {
		t.Fatalf("invited user mismatch: %+v", u)
	}
}

func TestInviteActivatesCredentiallessUserWithoutReplacingExistingCredential(t *testing.T) {
	svc, _ := newSvc(t)
	actor := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.create", "roles.assign"}}
	ctx := tenantContext("upshs")
	credentialless, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "credentialless@upshs.edu.gh", Name: "Credentialless User", Role: "teacher",
	})
	if err != nil {
		t.Fatalf("create credentialless user: %v", err)
	}
	token, err := svc.InviteUser(ctx, actor, application.InviteInput{Email: credentialless.Email, Role: "teacher"})
	if err != nil {
		t.Fatalf("invite credentialless user: %v", err)
	}
	activated, err := svc.AcceptInvite(ctx, token, credentialless.Name, "credentialless-password-123")
	if err != nil || activated.ID != credentialless.ID {
		t.Fatalf("activate existing user: user=%+v err=%v", activated, err)
	}
	if _, _, _, _, err := svc.Login(ctx, credentialless.Email, "credentialless-password-123"); err != nil {
		t.Fatalf("provisioned credential unusable: %v", err)
	}
	replayed, err := svc.AcceptInvite(ctx, token, credentialless.Name, "credentialless-password-123")
	if err != nil || replayed.ID != credentialless.ID {
		t.Fatalf("exact invite replay did not resume: user=%+v err=%v", replayed, err)
	}
	if _, err := svc.AcceptInvite(ctx, token, credentialless.Name, "different-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("invite replay replaced an existing credential: %v", err)
	}
}

func TestNewInviteRevokesPriorUnusedToken(t *testing.T) {
	svc, _ := newSvc(t)
	actor := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.create", "roles.assign"}}
	input := application.InviteInput{Email: "superseded@upshs.edu.gh", Role: "teacher"}
	first, err := svc.InviteUser(tenantContext("upshs"), actor, input)
	if err != nil {
		t.Fatalf("first invite: %v", err)
	}
	second, err := svc.InviteUser(tenantContext("upshs"), actor, input)
	if err != nil {
		t.Fatalf("replacement invite: %v", err)
	}
	if _, err := svc.AcceptInvite(context.Background(), first, "Superseded User", "superseded-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("superseded token remained usable: %v", err)
	}
	if _, err := svc.AcceptInvite(context.Background(), second, "Superseded User", "superseded-password-123"); err != nil {
		t.Fatalf("replacement token unusable: %v", err)
	}
}

func TestUsedInviteCannotResumeAfterOriginalExpiry(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("memory repository: %v", err)
	}
	repo.WithClock(func() time.Time { return now })
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("invite-expiry-signing-key"), time.Hour, 7*24*time.Hour,
		application.WithTransactionalNotifier(&recordingNotifier{})).WithClock(func() time.Time { return now })
	actor := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.create", "roles.assign"}}
	token, err := svc.InviteUser(tenantContext("upshs"), actor, application.InviteInput{Email: "expires@upshs.edu.gh", Role: "teacher"})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := svc.AcceptInvite(context.Background(), token, "Expiring User", "expiring-password-123"); err != nil {
		t.Fatalf("initial acceptance: %v", err)
	}
	now = now.Add(8 * 24 * time.Hour)
	if _, err := svc.AcceptInvite(context.Background(), token, "Expiring User", "expiring-password-123"); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("expired used invite resumed: %v", err)
	}
}

func TestSchoolAdminInviteAcceptanceResumesAfterActivationFailure(t *testing.T) {
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	activator := &retryingActivator{fail: true}
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("test-signing-key"), time.Hour, 7*24*time.Hour,
		application.WithTransactionalNotifier(&recordingNotifier{}),
		application.WithTenantActivator(activator),
	)
	actor := auth.Actor{UserID: "u-super", PlatformAdmin: true, Permissions: []string{"users.create"}}
	token, err := svc.InviteUser(context.Background(), actor, application.InviteInput{
		TenantID: "upshs", Email: "founder@upshs.edu.gh", Role: "school_admin",
	})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := svc.AcceptInvite(context.Background(), token, "School Founder", "welcome-user-123"); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("first acceptance should report dependency failure, got %v", err)
	}
	activator.fail = false
	user, err := svc.AcceptInvite(context.Background(), token, "School Founder", "welcome-user-123")
	if err != nil {
		t.Fatalf("retry acceptance: %v", err)
	}
	if user.Email != "founder@upshs.edu.gh" || activator.calls != 2 {
		t.Fatalf("retry did not resume exact acceptance: user=%+v calls=%d", user, activator.calls)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func tenantContext(tenantID string) context.Context {
	return identitytenancy.WithActor(context.Background(), auth.Actor{TenantID: tenantID})
}
