package unit

import (
	"testing"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
)

func TestNewExamSession_RequiresTenant(t *testing.T) {
	if _, err := domain.NewExamSession("", "Midterm", ay1, subject1, []string{qid1}, 60, nil, nil); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewExamSession_RequiresTitle(t *testing.T) {
	if _, err := domain.NewExamSession(tenantA, "", ay1, subject1, []string{qid1}, 60, nil, nil); err == nil {
		t.Fatal("expected error when title is empty")
	}
}

func TestNewExamSession_RequiresAcademicYearID(t *testing.T) {
	if _, err := domain.NewExamSession(tenantA, "Midterm", "", subject1, []string{qid1}, 60, nil, nil); err == nil {
		t.Fatal("expected error when academic_year_id is empty")
	}
}

func TestNewExamSession_RequiresSubjectID(t *testing.T) {
	if _, err := domain.NewExamSession(tenantA, "Midterm", ay1, "", []string{qid1}, 60, nil, nil); err == nil {
		t.Fatal("expected error when subject_id is empty")
	}
}

func TestNewExamSession_RequiresQuestionIDs(t *testing.T) {
	if _, err := domain.NewExamSession(tenantA, "Midterm", ay1, subject1, []string{}, 60, nil, nil); err == nil {
		t.Fatal("expected error when question_ids is empty")
	}
}

func TestNewExamSession_RequiresPositiveDuration(t *testing.T) {
	if _, err := domain.NewExamSession(tenantA, "Midterm", ay1, subject1, []string{qid1}, 0, nil, nil); err == nil {
		t.Fatal("expected error when duration_minutes is zero")
	}
}

func TestNewExamSession_RequiresEndAfterStart(t *testing.T) {
	start := time.Now().UTC()
	end := start.Add(-time.Hour)
	if _, err := domain.NewExamSession(tenantA, "Midterm", ay1, subject1, []string{qid1}, 60, &start, &end); err == nil {
		t.Fatal("expected error when end_at is before start_at")
	}
}

func TestNewExamSession_Valid(t *testing.T) {
	e, err := domain.NewExamSession(tenantA, "Midterm", ay1, subject1, []string{qid1, qid2}, 60, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Status != string(domain.ExamStatusDraft) {
		t.Fatalf("expected draft status, got %q", e.Status)
	}
	if len(e.QuestionIDs) != 2 {
		t.Fatalf("expected 2 question ids, got %d", len(e.QuestionIDs))
	}
}

func TestExamSession_IsActive(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	e, err := domain.NewExamSession(tenantA, "Midterm", ay1, subject1, []string{qid1}, 60, &start, &end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.Status = string(domain.ExamStatusActive)
	if !e.IsActive(now) {
		t.Fatal("expected exam session to be active")
	}
	if e.IsActive(now.Add(2 * time.Hour)) {
		t.Fatal("expected exam session to be inactive after end_at")
	}
}

func TestExamSession_ApplyUpdate_Activates(t *testing.T) {
	e, err := domain.NewExamSession(tenantA, "Midterm", ay1, subject1, []string{qid1}, 60, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	status := string(domain.ExamStatusActive)
	changed, err := e.ApplyUpdate(nil, nil, nil, nil, nil, &status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 1 || changed[0] != "status" {
		t.Fatalf("expected status changed, got %v", changed)
	}
	if e.Status != status {
		t.Fatalf("expected active status, got %q", e.Status)
	}
}
