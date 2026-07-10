package unit

import (
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/application"
	"github.com/auraedu/identity-service/internal/domain"
)

func newSvc(t *testing.T) *application.Service {
	t.Helper()
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return application.NewService(repo, []byte("test-signing-key"), time.Hour)
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
	svc := newSvc(t)
	token, user, _, err := svc.Login("e.mensah@upshs.edu.gh", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if user.Role != "teacher" || user.TenantID != "upshs" {
		t.Fatalf("user mismatch: %+v", user)
	}
	actor, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if actor.UserID != "u-teacher" || actor.TenantID != "upshs" || !actor.Has("attendance.mark") {
		t.Fatalf("actor mismatch: %+v", actor)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	if _, _, _, err := newSvc(t).Login("e.mensah@upshs.edu.gh", "nope"); err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownEmailDoesNotEnumerate(t *testing.T) {
	// Unknown email returns the SAME error as a wrong password.
	if _, _, _, err := newSvc(t).Login("ghost@nowhere.gh", "password123"); err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestSuperAdminHasNoTenantAndIsPlatformAdmin(t *testing.T) {
	svc := newSvc(t)
	token, _, _, err := svc.Login("super@auraedu.dev", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	actor, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !actor.PlatformAdmin || actor.TenantID != "" {
		t.Fatalf("super admin actor wrong: %+v", actor)
	}
}
