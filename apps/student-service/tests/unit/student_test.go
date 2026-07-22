package unit

import (
	"testing"

	"github.com/auraedu/student-service/internal/domain"
)

func TestNewStudent_RequiresTenant(t *testing.T) {
	if _, err := domain.NewStudent("", "Ada", "Lovelace"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewStudent_RequiresFirstName(t *testing.T) {
	if _, err := domain.NewStudent("tenant-1", "", "Lovelace"); err == nil {
		t.Fatal("expected error when first_name is empty")
	}
}

func TestNewStudent_RequiresLastName(t *testing.T) {
	if _, err := domain.NewStudent("tenant-1", "Ada", ""); err == nil {
		t.Fatal("expected error when last_name is empty")
	}
}

func TestNewStudent_Valid(t *testing.T) {
	e, err := domain.NewStudent("tenant-1", "Ada", "Lovelace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
	if e.FirstName != "Ada" || e.LastName != "Lovelace" {
		t.Fatalf("name not set: got %q %q", e.FirstName, e.LastName)
	}
	if e.Status != string(domain.StatusActive) {
		t.Fatalf("expected active status, got %q", e.Status)
	}
	if e.StudentCode == "" {
		t.Fatal("expected student_code to be generated")
	}
}

func TestStudent_ApplyUpdate(t *testing.T) {
	e, err := domain.NewStudent("tenant-1", "Ada", "Lovelace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first := "Augusta"
	status := string(domain.StatusGraduated)
	changed, err := e.ApplyUpdate(&first, nil, &status, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if e.FirstName != first {
		t.Fatalf("first name not updated: got %q", e.FirstName)
	}
	if e.Status != status {
		t.Fatalf("status not updated: got %q", e.Status)
	}
}

func TestStudent_ApplyUpdateLinksAndClearsIdentity(t *testing.T) {
	e, err := domain.NewStudent("tenant-1", "Ada", "Lovelace")
	if err != nil {
		t.Fatal(err)
	}
	userID := "33333333-3333-4333-8333-333333333333"
	if _, err := e.ApplyUpdate(nil, nil, nil, &userID); err != nil || e.UserID == nil || *e.UserID != userID {
		t.Fatalf("identity link failed: user_id=%v err=%v", e.UserID, err)
	}
	empty := ""
	if _, err := e.ApplyUpdate(nil, nil, nil, &empty); err != nil || e.UserID != nil {
		t.Fatalf("identity unlink failed: user_id=%v err=%v", e.UserID, err)
	}
	invalid := "not-a-uuid"
	if _, err := e.ApplyUpdate(nil, nil, nil, &invalid); err == nil {
		t.Fatal("invalid identity link must be rejected")
	}
}

func TestStudent_InvalidStatus(t *testing.T) {
	e, err := domain.NewStudent("tenant-1", "Ada", "Lovelace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.Status = "unknown"
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestStudent_InvalidGender(t *testing.T) {
	e, err := domain.NewStudent("tenant-1", "Ada", "Lovelace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := "robot"
	e.Gender = &bad
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid gender")
	}
}
