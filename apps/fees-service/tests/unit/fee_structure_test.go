package unit

import (
	"testing"

	"github.com/auraedu/fees-service/internal/domain"
)

func TestNewFeeStructure_RequiresTenant(t *testing.T) {
	if _, err := domain.NewFeeStructure("", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, nil, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewFeeStructure_RequiresName(t *testing.T) {
	if _, err := domain.NewFeeStructure("tenant-1", "", "ay-1", "GHS", "termly", "all_students", 10000, nil, nil); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewFeeStructure_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewFeeStructure("tenant-1", "Tuition", "", "GHS", "termly", "all_students", 10000, nil, nil); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewFeeStructure_RequiresNonNegativeAmount(t *testing.T) {
	if _, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", -1, nil, nil); err == nil {
		t.Fatal("expected error when amount_cents is negative")
	}
}

func TestNewFeeStructure_DefaultsCurrency(t *testing.T) {
	fs, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "", "termly", "all_students", 10000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.Currency != "GHS" {
		t.Fatalf("expected default currency GHS, got %q", fs.Currency)
	}
}

func TestNewFeeStructure_RequiresValidRecurrence(t *testing.T) {
	if _, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "daily", "all_students", 10000, nil, nil); err == nil {
		t.Fatal("expected error for invalid recurrence")
	}
}

func TestNewFeeStructure_RequiresValidTarget(t *testing.T) {
	if _, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "everyone", 10000, nil, nil); err == nil {
		t.Fatal("expected error for invalid target")
	}
}

func TestNewFeeStructure_RequiresValidDueDay(t *testing.T) {
	day := 32
	if _, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, &day, nil); err == nil {
		t.Fatal("expected error when due_day > 31")
	}
	day = 0
	if _, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, &day, nil); err == nil {
		t.Fatal("expected error when due_day < 1")
	}
}

func TestNewFeeStructure_Valid(t *testing.T) {
	fs, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.Name != "Tuition" {
		t.Fatalf("name not set: got %q", fs.Name)
	}
	if fs.Status != string(domain.StatusActive) {
		t.Fatalf("status not active: got %q", fs.Status)
	}
}

func TestFeeStructure_ApplyUpdate(t *testing.T) {
	fs, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "Updated Tuition"
	amount := 15000
	status := string(domain.StatusArchived)
	changed, err := fs.ApplyUpdate(domain.FeeStructurePatch{
		Name:        &name,
		AmountCents: &amount,
		Status:      &status,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if fs.Name != name || fs.AmountCents != amount || fs.Status != status {
		t.Fatalf("update not applied: %+v", fs)
	}
}

func TestFeeStructure_ApplyUpdate_InvalidStatus(t *testing.T) {
	fs, err := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "deleted"
	if _, err := fs.ApplyUpdate(domain.FeeStructurePatch{Status: &status}); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestFeeStructure_Validate_InvalidStatus(t *testing.T) {
	fs, _ := domain.NewFeeStructure("tenant-1", "Tuition", "ay-1", "GHS", "termly", "all_students", 10000, nil, nil)
	fs.Status = "unknown"
	if err := fs.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
