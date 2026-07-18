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
	y, err := domain.NewAcademicYear("tenant-1", "2025/26", "", "2025-09-01", "2026-07-31", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestTerm_ApplyUpdate(t *testing.T) {
	term, err := domain.NewTerm("tenant-1", "ay-1", "Term 1", "2025-09-01", "2025-12-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "First Term"
	end := "2026-01-15"
	changed, err := term.ApplyUpdate(&name, nil, &end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if term.Name != name {
		t.Fatalf("name not updated: got %q", term.Name)
	}
	if term.EndDate.String() != end {
		t.Fatalf("end_date not updated: got %q", term.EndDate.String())
	}
}

func TestTerm_ApplyUpdate_EndBeforeStart(t *testing.T) {
	term, err := domain.NewTerm("tenant-1", "ay-1", "Term 1", "2025-09-01", "2025-12-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := "2025-08-01"
	if _, err := term.ApplyUpdate(nil, nil, &bad); err == nil {
		t.Fatal("expected error when end_date is not after start_date")
	}
}

func TestNewClass_RequiresTenant(t *testing.T) {
	if _, err := domain.NewClass("", "ay-1", "Class 1", nil, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewClass_RequiresAcademicYear(t *testing.T) {
	if _, err := domain.NewClass("tenant-1", "", "Class 1", nil, nil); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewClass_RequiresName(t *testing.T) {
	if _, err := domain.NewClass("tenant-1", "ay-1", "  ", nil, nil); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewClass_RejectsNonPositiveCapacity(t *testing.T) {
	zero := 0
	if _, err := domain.NewClass("tenant-1", "ay-1", "Class 1", nil, &zero); err == nil {
		t.Fatal("expected error when capacity is zero")
	}
	negative := -5
	if _, err := domain.NewClass("tenant-1", "ay-1", "Class 1", nil, &negative); err == nil {
		t.Fatal("expected error when capacity is negative")
	}
}

func TestNewClass_Valid(t *testing.T) {
	teacher := "teacher-1"
	capacity := 45
	c, err := domain.NewClass("tenant-1", "ay-1", "Form 2", &teacher, &capacity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", c.TenantID)
	}
	if c.Name != "Form 2" {
		t.Fatalf("name not set: got %q", c.Name)
	}
	if c.AcademicYearID != "ay-1" {
		t.Fatalf("academic_year_id not set: got %q", c.AcademicYearID)
	}
	if c.ClassTeacherID == nil || *c.ClassTeacherID != teacher {
		t.Fatalf("class_teacher_id not set: got %v", c.ClassTeacherID)
	}
	if c.Capacity == nil || *c.Capacity != capacity {
		t.Fatalf("capacity not set: got %v", c.Capacity)
	}
}

func TestNewClass_EmptyTeacherNormalizesToNil(t *testing.T) {
	empty := "  "
	c, err := domain.NewClass("tenant-1", "ay-1", "Class 1", &empty, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ClassTeacherID != nil {
		t.Fatalf("expected empty class_teacher_id to normalize to nil, got %q", *c.ClassTeacherID)
	}
}

func TestClass_ApplyUpdate(t *testing.T) {
	c, err := domain.NewClass("tenant-1", "ay-1", "Class 1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "Class 1A"
	teacher := "teacher-9"
	capacity := 40
	changed, err := c.ApplyUpdate(&name, &teacher, &capacity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if c.Name != name || c.ClassTeacherID == nil || *c.ClassTeacherID != teacher || c.Capacity == nil || *c.Capacity != capacity {
		t.Fatalf("class not updated: %+v", c)
	}
}

func TestClass_ApplyUpdate_ClearTeacher(t *testing.T) {
	teacher := "teacher-1"
	c, err := domain.NewClass("tenant-1", "ay-1", "Class 1", &teacher, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	empty := ""
	changed, err := c.ApplyUpdate(nil, &empty, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed[0] != "class_teacher_id" {
		t.Fatalf("expected class_teacher_id change, got %v", changed)
	}
	if c.ClassTeacherID != nil {
		t.Fatalf("expected teacher cleared, got %q", *c.ClassTeacherID)
	}
}

func TestClass_ApplyUpdate_RejectsNonPositiveCapacity(t *testing.T) {
	c, err := domain.NewClass("tenant-1", "ay-1", "Class 1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	zero := 0
	if _, err := c.ApplyUpdate(nil, nil, &zero); err == nil {
		t.Fatal("expected error when capacity is zero")
	}
}

func TestNewSubject_RequiresTenant(t *testing.T) {
	if _, err := domain.NewSubject("", "Mathematics", nil, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewSubject_RequiresName(t *testing.T) {
	if _, err := domain.NewSubject("tenant-1", " ", nil, nil); err == nil {
		t.Fatal("expected error when name is empty")
	}
}

func TestNewSubject_Valid(t *testing.T) {
	code := "MATH"
	desc := "Core mathematics"
	s, err := domain.NewSubject("tenant-1", "Mathematics", &code, &desc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TenantID != "tenant-1" {
		t.Fatalf("tenant not set: got %q", s.TenantID)
	}
	if s.Name != "Mathematics" {
		t.Fatalf("name not set: got %q", s.Name)
	}
	if s.Code == nil || *s.Code != code {
		t.Fatalf("code not set: got %v", s.Code)
	}
	if s.Description == nil || *s.Description != desc {
		t.Fatalf("description not set: got %v", s.Description)
	}
}

func TestNewSubject_EmptyCodeNormalizesToNil(t *testing.T) {
	empty := ""
	s, err := domain.NewSubject("tenant-1", "Mathematics", &empty, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Code != nil {
		t.Fatalf("expected empty code to normalize to nil, got %q", *s.Code)
	}
}

func TestSubject_ApplyUpdate(t *testing.T) {
	s, err := domain.NewSubject("tenant-1", "Mathematics", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name := "Further Mathematics"
	code := "FMATH"
	changed, err := s.ApplyUpdate(&name, &code, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if s.Name != name || s.Code == nil || *s.Code != code {
		t.Fatalf("subject not updated: %+v", s)
	}
}

func TestSubject_ApplyUpdate_RejectsEmptyName(t *testing.T) {
	s, err := domain.NewSubject("tenant-1", "Mathematics", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	empty := "  "
	if _, err := s.ApplyUpdate(&empty, nil, nil); err == nil {
		t.Fatal("expected error when name is empty")
	}
}
