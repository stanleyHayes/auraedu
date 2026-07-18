package unit

import (
	"testing"

	"github.com/auraedu/assessment-service/internal/domain"
)

func TestAggregateGrades_Empty(t *testing.T) {
	book := domain.AggregateGrades(nil)
	if len(book.Subjects) != 0 {
		t.Fatalf("expected no subjects, got %d", len(book.Subjects))
	}
	if book.Overall.AssessmentCount != 0 || book.Overall.TotalScore != 0 || book.Overall.TotalMaxScore != 0 {
		t.Fatalf("expected zero overall aggregate, got %+v", book.Overall)
	}
	if book.Overall.Average != nil || book.Overall.WeightedAverage != nil {
		t.Fatalf("expected nil averages for empty gradebook, got %+v", book.Overall)
	}
}

func TestAggregateGrades_SingleSubject(t *testing.T) {
	rows := []domain.GradeRow{
		{SubjectID: subject1, Score: 80, MaxScore: 100},
		{SubjectID: subject1, Score: 60, MaxScore: 100},
	}
	book := domain.AggregateGrades(rows)
	if len(book.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(book.Subjects))
	}
	got := book.Subjects[0]
	if got.SubjectID != subject1 {
		t.Fatalf("wrong subject: %q", got.SubjectID)
	}
	if got.AssessmentCount != 2 || got.TotalScore != 140 || got.TotalMaxScore != 200 {
		t.Fatalf("wrong totals: %+v", got)
	}
	if got.Average == nil || *got.Average != 70 {
		t.Fatalf("expected average 70, got %v", got.Average)
	}
	if got.WeightedAverage == nil || *got.WeightedAverage != 70 {
		t.Fatalf("expected weighted average 70, got %v", got.WeightedAverage)
	}
}

// TestAggregateGrades_Weighted exercises the case where the simple average of
// percentages differs from the max-score-weighted average:
// 10/10 (100%) and 50/100 (50%) -> average 75, weighted 60/110 = 54.55.
func TestAggregateGrades_Weighted(t *testing.T) {
	rows := []domain.GradeRow{
		{SubjectID: subject1, Score: 10, MaxScore: 10},
		{SubjectID: subject1, Score: 50, MaxScore: 100},
	}
	book := domain.AggregateGrades(rows)
	if len(book.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(book.Subjects))
	}
	got := book.Subjects[0]
	if got.Average == nil || *got.Average != 75 {
		t.Fatalf("expected average 75, got %v", got.Average)
	}
	if got.WeightedAverage == nil || *got.WeightedAverage != 54.55 {
		t.Fatalf("expected weighted average 54.55, got %v", got.WeightedAverage)
	}
}

func TestAggregateGrades_MultiSubjectAndOverall(t *testing.T) {
	rows := []domain.GradeRow{
		{SubjectID: subject1, Score: 90, MaxScore: 100},
		{SubjectID: subject2, Score: 40, MaxScore: 50},
		{SubjectID: subject2, Score: 30, MaxScore: 50},
	}
	book := domain.AggregateGrades(rows)
	if len(book.Subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(book.Subjects))
	}
	// Subjects are ordered by subject_id for deterministic output
	// (subject2 "3333..." sorts before subject1 "dddd...").
	if book.Subjects[0].SubjectID != subject2 || book.Subjects[1].SubjectID != subject1 {
		t.Fatalf("subjects not ordered: %+v", book.Subjects)
	}
	if got := book.Subjects[0]; got.AssessmentCount != 2 || got.TotalScore != 70 || got.TotalMaxScore != 100 {
		t.Fatalf("wrong subject2 totals: %+v", got)
	}
	overall := book.Overall
	if overall.AssessmentCount != 3 || overall.TotalScore != 160 || overall.TotalMaxScore != 200 {
		t.Fatalf("wrong overall totals: %+v", overall)
	}
	if overall.WeightedAverage == nil || *overall.WeightedAverage != 80 {
		t.Fatalf("expected overall weighted average 80, got %v", overall.WeightedAverage)
	}
	// (90 + 80 + 60) / 3 = 76.67
	if overall.Average == nil || *overall.Average != 76.67 {
		t.Fatalf("expected overall average 76.67, got %v", overall.Average)
	}
}

func TestAggregateGrades_SkipsNonPositiveMaxScore(t *testing.T) {
	rows := []domain.GradeRow{
		{SubjectID: subject1, Score: 5, MaxScore: 0},
		{SubjectID: subject1, Score: 80, MaxScore: 100},
	}
	book := domain.AggregateGrades(rows)
	if book.Overall.AssessmentCount != 1 {
		t.Fatalf("expected 1 counted row, got %+v", book.Overall)
	}
	if *book.Overall.Average != 80 || *book.Overall.WeightedAverage != 80 {
		t.Fatalf("unexpected averages: %+v", book.Overall)
	}
}
