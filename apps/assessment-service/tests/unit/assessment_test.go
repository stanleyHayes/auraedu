package unit

import (
	"testing"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
)

func TestNewAssessment_RequiresTenant(t *testing.T) {
	if _, err := domain.NewAssessment("", ay1, subject1, "test", "Midterm", "", 100, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty (tenant isolation invariant)")
	}
}

func TestNewAssessment_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewAssessment(tenantA, "", subject1, "test", "Midterm", "", 100, nil); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewAssessment_RequiresSubjectID(t *testing.T) {
	if _, err := domain.NewAssessment(tenantA, ay1, "", "test", "Midterm", "", 100, nil); err == nil {
		t.Fatal("expected error when subject_id is empty")
	}
}

func TestNewAssessment_RequiresTitle(t *testing.T) {
	if _, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "", "", 100, nil); err == nil {
		t.Fatal("expected error when title is empty")
	}
}

func TestNewAssessment_RequiresValidType(t *testing.T) {
	if _, err := domain.NewAssessment(tenantA, ay1, subject1, "quiz", "Midterm", "", 100, nil); err == nil {
		t.Fatal("expected error when type is invalid")
	}
}

func TestNewAssessment_RequiresPositiveMaxScore(t *testing.T) {
	if _, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 0, nil); err == nil {
		t.Fatal("expected error when max_score is zero")
	}
	if _, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", -5, nil); err == nil {
		t.Fatal("expected error when max_score is negative")
	}
}

func TestNewAssessment_Valid(t *testing.T) {
	due := time.Date(2025, 10, 15, 9, 0, 0, 0, time.UTC)
	a, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "Term 1 test", 100, &due)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.TenantID != tenantA {
		t.Fatalf("tenant not set: got %q", a.TenantID)
	}
	if a.Status != string(domain.StatusDraft) {
		t.Fatalf("expected draft status, got %q", a.Status)
	}
	if a.DueDate == nil || !a.DueDate.Equal(due) {
		t.Fatalf("due_date not set: got %v", a.DueDate)
	}
	if a.Description == nil || *a.Description != "Term 1 test" {
		t.Fatalf("description not set: got %v", a.Description)
	}
}

func TestAssessment_ApplyUpdate(t *testing.T) {
	a, err := domain.NewAssessment(tenantA, ay1, subject1, "assignment", "Homework 1", "", 20, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	title := "Homework 1 (updated)"
	maxScore := 30
	status := string(domain.StatusPublished)
	changed, err := a.ApplyUpdate(&title, nil, nil, &maxScore, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed fields, got %v", changed)
	}
	if a.Title != title {
		t.Fatalf("title not updated: got %q", a.Title)
	}
	if a.MaxScore != maxScore {
		t.Fatalf("max_score not updated: got %d", a.MaxScore)
	}
	if a.Status != status {
		t.Fatalf("status not updated: got %q", a.Status)
	}
}

func TestAssessment_ApplyUpdate_InvalidStatus(t *testing.T) {
	a, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 100, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := "unknown"
	if _, err := a.ApplyUpdate(nil, nil, nil, nil, nil, &status); err == nil {
		t.Fatal("expected error for invalid status update")
	}
}

func TestAssessment_ApplyUpdate_InvalidTransition(t *testing.T) {
	a, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 100, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a.Status = string(domain.StatusArchived)
	status := string(domain.StatusPublished)
	if _, err := a.ApplyUpdate(nil, nil, nil, nil, nil, &status); err == nil {
		t.Fatal("expected error for invalid status transition archived -> published")
	}
}

func TestAssessment_ApplyUpdate_PublishedEventDetected(t *testing.T) {
	a, err := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 100, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != string(domain.StatusDraft) {
		t.Fatal("expected initial status draft")
	}
	status := string(domain.StatusPublished)
	changed, err := a.ApplyUpdate(nil, nil, nil, nil, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed[0] != "status" {
		t.Fatalf("expected status changed, got %v", changed)
	}
	if a.Status != string(domain.StatusPublished) {
		t.Fatal("expected published status")
	}
}

func TestAssessment_Validate_InvalidType(t *testing.T) {
	a, _ := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 100, nil)
	a.Type = "unknown"
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestAssessment_Validate_InvalidStatus(t *testing.T) {
	a, _ := domain.NewAssessment(tenantA, ay1, subject1, "test", "Midterm", "", 100, nil)
	a.Status = "unknown"
	if err := a.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
