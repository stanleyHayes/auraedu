package unit

import (
	"testing"

	"github.com/auraedu/staff-service/internal/domain"
)

func TestNewStaff_RequiresTenant(t *testing.T) {
	if _, err := domain.NewStaff("", "Kwame", "Nkrumah", "teacher"); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewStaff_RequiresFirstName(t *testing.T) {
	if _, err := domain.NewStaff("tenant-1", "", "Nkrumah", "teacher"); err == nil {
		t.Fatal("expected error when first_name is empty")
	}
}

func TestNewStaff_RequiresLastName(t *testing.T) {
	if _, err := domain.NewStaff("tenant-1", "Kwame", "", "teacher"); err == nil {
		t.Fatal("expected error when last_name is empty")
	}
}

func TestNewStaff_RequiresValidStaffType(t *testing.T) {
	if _, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "invalid"); err == nil {
		t.Fatal("expected error for invalid staff_type")
	}
}

func TestNewStaff_Valid(t *testing.T) {
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", e.TenantID)
	}
	if e.FirstName != "Kwame" || e.LastName != "Nkrumah" {
		t.Fatalf("name not set: got %q %q", e.FirstName, e.LastName)
	}
	if e.StaffType != "teacher" {
		t.Fatalf("staff_type not set: got %q", e.StaffType)
	}
	if e.Status != string(domain.StatusActive) {
		t.Fatalf("expected active status, got %q", e.Status)
	}
	if e.StaffCode == "" {
		t.Fatal("expected staff_code to be generated")
	}
	if !e.IsActive() {
		t.Fatal("expected staff to be active")
	}
}

func TestStaff_ApplyUpdate(t *testing.T) {
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first := "Kofi"
	staffType := "non_teaching"
	status := string(domain.StatusInactive)
	changed, err := e.ApplyUpdate(&first, nil, &staffType, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if e.FirstName != first {
		t.Fatalf("first name not updated: got %q", e.FirstName)
	}
	if e.StaffType != staffType {
		t.Fatalf("staff_type not updated: got %q", e.StaffType)
	}
	if e.Status != status {
		t.Fatalf("status not updated: got %q", e.Status)
	}
	if e.IsActive() {
		t.Fatal("expected staff to be inactive")
	}
}

func TestStaff_InvalidStatus(t *testing.T) {
	e, _ := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	e.Status = "unknown"
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestStaff_InvalidStaffType(t *testing.T) {
	e, _ := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	e.StaffType = "robot"
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid staff_type")
	}
}

func TestStaff_InvalidEmail(t *testing.T) {
	bad := "not-an-email"
	e, _ := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	e.Email = &bad
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid email")
	}
}

func TestStaff_ActivateDeactivate(t *testing.T) {
	e, _ := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	e.Deactivate()
	if e.Status != string(domain.StatusInactive) {
		t.Fatalf("expected inactive, got %q", e.Status)
	}
	e.Activate()
	if e.Status != string(domain.StatusActive) {
		t.Fatalf("expected active, got %q", e.Status)
	}
}
