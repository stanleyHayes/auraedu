package unit

import (
	"testing"

	"github.com/auraedu/attendance-service/internal/domain"
)

func TestNewAttendanceRecord_RequiresTenant(t *testing.T) {
	if _, err := domain.NewAttendanceRecord("", "student-1", "ay-1", "2025-09-01", "present", "staff-1", nil); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewAttendanceRecord_RequiresStudentID(t *testing.T) {
	if _, err := domain.NewAttendanceRecord("tenant-1", "", "ay-1", "2025-09-01", "present", "staff-1", nil); err == nil {
		t.Fatal("expected error when student_id is empty")
	}
}

func TestNewAttendanceRecord_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewAttendanceRecord("tenant-1", "student-1", "", "2025-09-01", "present", "staff-1", nil); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewAttendanceRecord_RequiresDate(t *testing.T) {
	if _, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "", "present", "staff-1", nil); err == nil {
		t.Fatal("expected error when date is empty")
	}
	if _, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "not-a-date", "present", "staff-1", nil); err == nil {
		t.Fatal("expected error when date is invalid")
	}
}

func TestNewAttendanceRecord_RequiresMarkedBy(t *testing.T) {
	if _, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "2025-09-01", "present", "", nil); err == nil {
		t.Fatal("expected error when marked_by is empty")
	}
}

func TestNewAttendanceRecord_RequiresValidStatus(t *testing.T) {
	if _, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "2025-09-01", "unknown", "staff-1", nil); err == nil {
		t.Fatal("expected error when status is invalid")
	}
}

func TestNewAttendanceRecord_Valid(t *testing.T) {
	rec, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "2025-09-01", "present", "staff-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", rec.TenantID)
	}
	if rec.StudentID != "student-1" {
		t.Fatalf("student_id not set: got %q", rec.StudentID)
	}
	if rec.Date.String() != "2025-09-01" {
		t.Fatalf("date not set: got %q", rec.Date.String())
	}
	if rec.Status != string(domain.StatusPresent) {
		t.Fatalf("status not set: got %q", rec.Status)
	}
}

func TestAttendanceRecord_ApplyUpdate(t *testing.T) {
	rec, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "2025-09-01", "absent", "staff-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := string(domain.StatusExcused)
	reason := "doctor appointment"
	markedBy := "staff-2"
	changed, err := rec.ApplyUpdate(&status, &reason, &markedBy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if rec.Status != status {
		t.Fatalf("status not updated: got %q", rec.Status)
	}
	if rec.Reason == nil || *rec.Reason != reason {
		t.Fatalf("reason not updated: got %v", rec.Reason)
	}
	if rec.MarkedBy != markedBy {
		t.Fatalf("marked_by not updated: got %q", rec.MarkedBy)
	}
}

func TestAttendanceRecord_ApplyUpdate_InvalidStatus(t *testing.T) {
	rec, err := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "2025-09-01", "present", "staff-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "unknown"
	if _, err := rec.ApplyUpdate(&status, nil, nil); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestAttendanceRecord_InvalidStatus(t *testing.T) {
	rec, _ := domain.NewAttendanceRecord("tenant-1", "student-1", "ay-1", "2025-09-01", "present", "staff-1", nil)
	rec.Status = "unknown"
	if err := rec.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
