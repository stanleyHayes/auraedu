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
	changed, err := e.ApplyUpdate(&first, nil, &staffType, nil, &status, nil)
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
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.Status = "unknown"
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestStaff_InvalidStaffType(t *testing.T) {
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.StaffType = "robot"
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid staff_type")
	}
}

func TestStaff_InvalidEmail(t *testing.T) {
	bad := "not-an-email"
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.Email = &bad
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid email")
	}
}

func TestStaff_UserIDMustBeUUID(t *testing.T) {
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	invalid := "not-a-user-id"
	e.UserID = &invalid
	if err := e.Validate(); err == nil {
		t.Fatal("expected invalid identity user id to be rejected")
	}
}

func TestStaff_ApplyUpdateClearsOptionalIdentityAndEmail(t *testing.T) {
	e, err := domain.NewStaff("tenant-1", "Ama", "Mensah", "teacher")
	if err != nil {
		t.Fatal(err)
	}
	email := "ama@school.edu.gh"
	userID := "33333333-3333-4333-8333-333333333333"
	e.Email, e.UserID = &email, &userID
	empty := ""
	changed, err := e.ApplyUpdate(nil, nil, nil, &empty, nil, &empty)
	if err != nil {
		t.Fatalf("clear optional fields: %v", err)
	}
	if e.Email != nil || e.UserID != nil {
		t.Fatalf("optional links were not cleared: email=%v user_id=%v", e.Email, e.UserID)
	}
	if len(changed) != 2 || changed[0] != "email" || changed[1] != "user_id" {
		t.Fatalf("unexpected changed fields: %v", changed)
	}
}

func TestStaff_ActivateDeactivate(t *testing.T) {
	e, err := domain.NewStaff("tenant-1", "Kwame", "Nkrumah", "teacher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.Deactivate()
	if e.Status != string(domain.StatusInactive) {
		t.Fatalf("expected inactive, got %q", e.Status)
	}
	e.Activate()
	if e.Status != string(domain.StatusActive) {
		t.Fatalf("expected active, got %q", e.Status)
	}
}

func TestNewAssignmentValidatesCrossServiceIdentifiers(t *testing.T) {
	staffID := "33333333-3333-4333-8333-333333333333"
	classID := "44444444-4444-4444-8444-444444444444"
	subjectID := "55555555-5555-4555-8555-555555555555"
	role := "  Mathematics teacher  "
	assignment, err := domain.NewAssignment("school-one", staffID, classID, &subjectID, &role)
	if err != nil {
		t.Fatalf("new assignment: %v", err)
	}
	if assignment.StaffID != staffID || assignment.ClassID != classID || assignment.SubjectID == nil || *assignment.SubjectID != subjectID {
		t.Fatalf("unexpected assignment: %+v", assignment)
	}
	if assignment.Role == nil || *assignment.Role != "Mathematics teacher" {
		t.Fatalf("role was not normalized: %+v", assignment.Role)
	}
	if _, err := domain.NewAssignment("school-one", "invalid", classID, nil, nil); err == nil {
		t.Fatal("invalid staff identifier must be rejected")
	}
	if _, err := domain.NewAssignment("school-one", staffID, "invalid", nil, nil); err == nil {
		t.Fatal("invalid class identifier must be rejected")
	}
}
