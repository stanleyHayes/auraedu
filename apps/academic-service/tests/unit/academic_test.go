package unit

import (
	"testing"

	"github.com/auraedu/academic-service/internal/domain"
)

func TestNewAcademicYear_RequiresTenant(t *testing.T) {
	if _, err := domain.NewAcademicYear("", "2025/26", "", "2025-09-01", "2026-07-31", false); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewAcademicYear_RequiresName(t *testing.T) {
	if _, err := domain.NewAcademicYear("tenant-1", "", "", "2025-09-01", "2026-07-31", false); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewAcademicYear_RequiresValidDates(t *testing.T) {
	if _, err := domain.NewAcademicYear("tenant-1", "2025/26", "", "not-a-date", "2026-07-31", false); err == nil {
		t.Fatal("expected error when start_date is invalid")
	}
	if _, err := domain.NewAcademicYear("tenant-1", "2025/26", "", "2025-09-01", "not-a-date", false); err == nil {
		t.Fatal("expected error when end_date is invalid")
	}
}

func TestNewAcademicYear_EndAfterStart(t *testing.T) {
	if _, err := domain.NewAcademicYear("tenant-1", "2025/26", "", "2026-07-31", "2025-09-01", false); err == nil {
		t.Fatal("expected error when end_date is not after start_date")
	}
}

func TestNewAcademicYear_Valid(t *testing.T) {
	y, err := domain.NewAcademicYear("tenant-1", "2025/26", "AY-2025", "2025-09-01", "2026-07-31", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if y.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", y.TenantID)
	}
	if y.Name != "2025/26" {
		t.Fatalf("name not set: got %q", y.Name)
	}
	if y.Code != "AY-2025" {
		t.Fatalf("code not set: got %q", y.Code)
	}
	if y.Status != string(domain.StatusActive) {
		t.Fatalf("expected active status, got %q", y.Status)
	}
	if !y.IsCurrent {
		t.Fatal("expected is_current to be true")
	}
}

func TestNewAcademicYear_GeneratesCode(t *testing.T) {
	y, err := domain.NewAcademicYear("tenant-1", "2025/26", "", "2025-09-01", "2026-07-31", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if y.Code == "" {
		t.Fatal("expected code to be generated")
	}
}

func TestAcademicYear_ApplyUpdate(t *testing.T) {
	y, err := domain.NewAcademicYear("tenant-1", "2025/26", "", "2025-09-01", "2026-07-31", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "2025/2026 Academic Year"
	status := string(domain.StatusArchived)
	changed, err := y.ApplyUpdate(&name, nil, nil, nil, &status, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if y.Name != name {
		t.Fatalf("name not updated: got %q", y.Name)
	}
	if y.Status != status {
		t.Fatalf("status not updated: got %q", y.Status)
	}
}

func TestAcademicYear_InvalidStatus(t *testing.T) {
	y, _ := domain.NewAcademicYear("tenant-1", "2025/26", "", "2025-09-01", "2026-07-31", false)
	y.Status = "unknown"
	if err := y.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestNewTerm_RequiresAcademicYear(t *testing.T) {
	if _, err := domain.NewTerm("tenant-1", "", "Term 1", "2025-09-01", "2025-12-31"); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewTerm_Valid(t *testing.T) {
	term, err := domain.NewTerm("tenant-1", "ay-1", "Term 1", "2025-09-01", "2025-12-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if term.AcademicYearID != "ay-1" {
		t.Fatalf("academic_year_id not set: got %q", term.AcademicYearID)
	}
	if term.Name != "Term 1" {
		t.Fatalf("name not set: got %q", term.Name)
	}
}
