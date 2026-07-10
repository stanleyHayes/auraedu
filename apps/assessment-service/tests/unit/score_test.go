package unit

import (
	"testing"

	"github.com/auraedu/assessment-service/internal/domain"
)

const (
	tenantA  = "11111111-1111-1111-1111-111111111111"
	ay1      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	subject1 = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	student1 = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	staff1   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
)

func TestNewScore_RequiresTenant(t *testing.T) {
	if _, err := domain.NewScore("", "assessment-1", student1, 85, staff1, "", 100); err == nil {
		t.Fatal("expected error when tenant_id is empty")
	}
}

func TestNewScore_RequiresAssessmentID(t *testing.T) {
	if _, err := domain.NewScore(tenantA, "", student1, 85, staff1, "", 100); err == nil {
		t.Fatal("expected error when assessment_id is empty")
	}
}

func TestNewScore_RequiresStudentID(t *testing.T) {
	if _, err := domain.NewScore(tenantA, "assessment-1", "", 85, staff1, "", 100); err == nil {
		t.Fatal("expected error when student_id is empty")
	}
}

func TestNewScore_RequiresRecordedBy(t *testing.T) {
	if _, err := domain.NewScore(tenantA, "assessment-1", student1, 85, "", "", 100); err == nil {
		t.Fatal("expected error when recorded_by is empty")
	}
}

func TestNewScore_RequiresNonNegativeScore(t *testing.T) {
	if _, err := domain.NewScore(tenantA, "assessment-1", student1, -1, staff1, "", 100); err == nil {
		t.Fatal("expected error when score is negative")
	}
}

func TestNewScore_ScoreCannotExceedMaxScore(t *testing.T) {
	if _, err := domain.NewScore(tenantA, "assessment-1", student1, 101, staff1, "", 100); err == nil {
		t.Fatal("expected error when score exceeds max_score")
	}
}

func TestNewScore_Valid(t *testing.T) {
	s, err := domain.NewScore(tenantA, "assessment-1", student1, 85, staff1, "Good work", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TenantID != tenantA {
		t.Fatalf("tenant not set: got %q", s.TenantID)
	}
	if s.Score != 85 {
		t.Fatalf("score not set: got %d", s.Score)
	}
	if s.Notes == nil || *s.Notes != "Good work" {
		t.Fatalf("notes not set: got %v", s.Notes)
	}
}

func TestScore_ApplyUpdate(t *testing.T) {
	s, err := domain.NewScore(tenantA, "assessment-1", student1, 75, staff1, "", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	score := 80
	notes := "Improved"
	changed, err := s.ApplyUpdate(&score, &notes, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed fields, got %v", changed)
	}
	if s.Score != score {
		t.Fatalf("score not updated: got %d", s.Score)
	}
	if s.Notes == nil || *s.Notes != notes {
		t.Fatalf("notes not updated: got %v", s.Notes)
	}
}

func TestScore_ApplyUpdate_ScoreExceedsMaxScore(t *testing.T) {
	s, err := domain.NewScore(tenantA, "assessment-1", student1, 75, staff1, "", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	score := 101
	if _, err := s.ApplyUpdate(&score, nil, 100); err == nil {
		t.Fatal("expected error when updated score exceeds max_score")
	}
}

func TestScore_ApplyUpdate_NegativeScore(t *testing.T) {
	s, err := domain.NewScore(tenantA, "assessment-1", student1, 75, staff1, "", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	score := -1
	if _, err := s.ApplyUpdate(&score, nil, 100); err == nil {
		t.Fatal("expected error when updated score is negative")
	}
}

func TestScore_Validate_NegativeScore(t *testing.T) {
	s, _ := domain.NewScore(tenantA, "assessment-1", student1, 75, staff1, "", 100)
	s.Score = -5
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for negative score")
	}
}
