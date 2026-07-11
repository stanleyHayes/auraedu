package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

func newSvc(t *testing.T) (*application.Service, *events.RecordingPublisher) {
	t.Helper()
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	pub := events.NewRecordingPublisher()
	return application.NewService(repo, nil, pub, []byte("test-signing-key"), time.Hour, 7*24*time.Hour), pub
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
	access, _, user, _, err := svc.Login(context.Background(), "e.mensah@upshs.edu.gh", "password123")
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
	_, _, _, _, err := svc.Login(context.Background(), "e.mensah@upshs.edu.gh", "nope") //nolint:dogsled // Login returns a 5-tuple; only the error is checked here
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownEmailDoesNotEnumerate(t *testing.T) {
	svc, _ := newSvc(t)
	_, _, _, _, err := svc.Login(context.Background(), "ghost@nowhere.gh", "password123") //nolint:dogsled // Login returns a 5-tuple; only the error is checked here
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestSuperAdminHasNoTenantAndIsPlatformAdmin(t *testing.T) {
	svc, _ := newSvc(t)
	access, _, _, _, err := svc.Login(context.Background(), "super@auraedu.dev", "password123") //nolint:dogsled // Login returns a 5-tuple; only access token is needed
	if err != nil {
		t.Fatalf("login: %v", err)
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
	_, refresh, _, _, err := svc.Login(context.Background(), "e.mensah@upshs.edu.gh", "password123") //nolint:dogsled // Login returns a 5-tuple; only refresh token is needed
	if err != nil {
		t.Fatalf("login: %v", err)
	}
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
	if _, _, _, _, err := svc.Refresh(context.Background(), refresh); err == nil {
		t.Fatal("old refresh token should be rejected")
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	_, refresh, _, _, err := svc.Login(ctx, "e.mensah@upshs.edu.gh", "password123") //nolint:dogsled // only refresh token needed
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	actor := auth.Actor{UserID: "u-teacher", TenantID: "upshs", Role: "teacher"}
	if err := svc.Logout(ctx, actor, refresh); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, _, _, _, err := svc.Refresh(ctx, refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("refresh after logout should fail, got %v", err)
	}
}

func TestLogoutCrossTenantForbidden(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	_, refresh, _, _, err := svc.Login(ctx, "e.mensah@upshs.edu.gh", "password123") //nolint:dogsled // only refresh token needed
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	other := auth.Actor{UserID: "u-other", TenantID: "aboom", Role: "teacher"}
	if err := svc.Logout(ctx, other, refresh); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant logout should be forbidden, got %v", err)
	}
}

func TestRevokeSessionByHash(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()
	_, refresh, _, _, err := svc.Login(ctx, "e.mensah@upshs.edu.gh", "password123") //nolint:dogsled // only refresh token needed
	if err != nil {
		t.Fatalf("login: %v", err)
	}
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
	ctx := context.Background()

	u, err := svc.CreateUser(ctx, actor, application.CreateUserInput{Email: "new@upshs.edu.gh", Name: "New User", Role: "teacher", Password: "pass1234"})
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

func TestPasswordResetFlow(t *testing.T) {
	svc, pub := newSvc(t)
	ctx := context.Background()
	if err := svc.RequestPasswordReset(ctx, "e.mensah@upshs.edu.gh"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if len(pub.Events) != 1 || pub.Events[0].Type != "notification.requested.v1" {
		t.Fatalf("expected notification event, got %+v", pub.Events)
	}
}

func TestInviteFlow(t *testing.T) {
	svc, pub := newSvc(t)
	actor := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{"users.create"}}
	ctx := context.Background()

	token, err := svc.InviteUser(ctx, actor, application.InviteInput{Email: "invited@upshs.edu.gh", Role: "teacher"})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if token == "" {
		t.Fatal("expected invite token")
	}
	if len(pub.Events) != 1 || pub.Events[0].Type != "notification.requested.v1" {
		t.Fatalf("expected notification event, got %+v", pub.Events)
	}

	u, err := svc.AcceptInvite(ctx, token, "Invited User", "welcome1")
	if err != nil {
		t.Fatalf("accept invite: %v", err)
	}
	if u.Email != "invited@upshs.edu.gh" || u.Role != "teacher" {
		t.Fatalf("invited user mismatch: %+v", u)
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
