package unit

import (
	"testing"

	"github.com/auraedu/cbt-service/internal/domain"
)

func TestNewSubmission_RequiresTenant(t *testing.T) {
	if _, err := domain.NewSubmission("", exam1, studentA); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewSubmission_RequiresExamSessionID(t *testing.T) {
	if _, err := domain.NewSubmission(tenantA, "", studentA); err == nil {
		t.Fatal("expected error when exam_session_id is empty")
	}
}

func TestNewSubmission_RequiresStudentID(t *testing.T) {
	if _, err := domain.NewSubmission(tenantA, exam1, ""); err == nil {
		t.Fatal("expected error when student_id is empty")
	}
}

func TestNewSubmission_Valid(t *testing.T) {
	s, err := domain.NewSubmission(tenantA, exam1, studentA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != string(domain.SubmissionStatusInProgress) {
		t.Fatalf("expected in_progress status, got %q", s.Status)
	}
	if s.Answers == nil {
		t.Fatal("expected answers map to be initialized")
	}
}

func TestSubmission_Submit(t *testing.T) {
	s, err := domain.NewSubmission(tenantA, exam1, studentA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.Submit(map[string]string{qid1: "4"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != string(domain.SubmissionStatusSubmitted) {
		t.Fatalf("expected submitted status, got %q", s.Status)
	}
	if s.SubmittedAt == nil {
		t.Fatal("expected submitted_at to be set")
	}
}

func TestSubmission_Grade(t *testing.T) {
	s, err := domain.NewSubmission(tenantA, exam1, studentA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.Status = string(domain.SubmissionStatusSubmitted)
	if err := s.Grade(8, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != string(domain.SubmissionStatusGraded) {
		t.Fatalf("expected graded status, got %q", s.Status)
	}
	if s.Score == nil || *s.Score != 8 {
		t.Fatalf("expected score 8, got %v", s.Score)
	}
	if s.MaxScore != 10 {
		t.Fatalf("expected max_score 10, got %d", s.MaxScore)
	}
}

func TestSubmission_Grade_RejectsNegativeScore(t *testing.T) {
	s, err := domain.NewSubmission(tenantA, exam1, studentA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.Status = string(domain.SubmissionStatusSubmitted)
	if err := s.Grade(-1, 10); err == nil {
		t.Fatal("expected error for negative score")
	}
}

func TestSubmission_Grade_RejectsExcessScore(t *testing.T) {
	s, err := domain.NewSubmission(tenantA, exam1, studentA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s.Status = string(domain.SubmissionStatusSubmitted)
	if err := s.Grade(11, 10); err == nil {
		t.Fatal("expected error for score exceeding max_score")
	}
}
