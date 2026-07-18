package unit

import (
	"testing"

	"github.com/auraedu/report-service/internal/domain"
)

func scoreEntry(t *testing.T, cardID, subjectID, sourceKey string, score float64, max *float64) *domain.ScoreEntry {
	t.Helper()
	e, err := domain.NewScoreEntry(tenantA, cardID, studentA, subjectID, sourceKey, "evt-1", score, max)
	if err != nil {
		t.Fatalf("new score entry: %v", err)
	}
	return e
}

func TestNewScoreEntry_Validation(t *testing.T) {
	if _, err := domain.NewScoreEntry("", "card", studentA, subject1, "key", "evt", 1, nil); err == nil {
		t.Fatal("expected error for empty tenant")
	}
	if _, err := domain.NewScoreEntry(tenantA, "", studentA, subject1, "key", "evt", 1, nil); err == nil {
		t.Fatal("expected error for empty report card")
	}
	if _, err := domain.NewScoreEntry(tenantA, "card", studentA, subject1, "", "evt", 1, nil); err == nil {
		t.Fatal("expected error for empty source key")
	}
	if _, err := domain.NewScoreEntry(tenantA, "card", studentA, subject1, "key", "evt", -1, nil); err == nil {
		t.Fatal("expected error for negative score")
	}
}

func TestNewAttendanceEntry_Validation(t *testing.T) {
	if _, err := domain.NewAttendanceEntry(tenantA, "card", studentA, "08/07/2026", "present", "evt"); err == nil {
		t.Fatal("expected error for non-ISO date")
	}
	if _, err := domain.NewAttendanceEntry(tenantA, "card", studentA, "2026-07-08", "unknown", "evt"); err == nil {
		t.Fatal("expected error for invalid status")
	}
	e, err := domain.NewAttendanceEntry(tenantA, "card", studentA, "2026-07-08", "late", "evt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Date.Format("2006-01-02") != "2026-07-08" || e.Status != domain.AttendanceStatusLate {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestAggregateScores_GroupsPerSubject(t *testing.T) {
	max100 := 100.0
	entries := []*domain.ScoreEntry{
		scoreEntry(t, "card", subject1, "a1", 40, &max100),
		scoreEntry(t, "card", subject1, "a2", 32, &max100),
		scoreEntry(t, "card", "bbbbbbbb-0000-0000-0000-000000000000", "a3", 9, nil),
	}
	got := domain.AggregateScores(entries)
	if len(got) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(got))
	}
	var subj *domain.SubjectScore
	for i := range got {
		if got[i].Label == subject1 {
			subj = &got[i]
		}
	}
	if subj == nil {
		t.Fatalf("subject group missing: %+v", got)
	}
	if subj.Score != 72 || subj.Count != 2 {
		t.Fatalf("unexpected aggregate: %+v", subj)
	}
	pct, ok := subj.Percentage()
	if !ok || pct != 36 { // 72 / 200 * 100
		t.Fatalf("expected 36%%, got %v (ok=%v)", pct, ok)
	}
}

func TestAggregateScores_NoSubjectFallsBackToAssessment(t *testing.T) {
	entries := []*domain.ScoreEntry{scoreEntry(t, "card", "", assessment1, 55, nil)}
	got := domain.AggregateScores(entries)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	if got[0].Label != "Assessment 12345678" {
		t.Fatalf("unexpected fallback label: %q", got[0].Label)
	}
	if _, ok := got[0].Percentage(); ok {
		t.Fatal("no max score: percentage must be unavailable")
	}
}

func TestSummarizeAttendance_CountsAndRate(t *testing.T) {
	mk := func(status string) *domain.AttendanceEntry {
		e, err := domain.NewAttendanceEntry(tenantA, "card", studentA, "2026-07-08", status, "evt")
		if err != nil {
			t.Fatalf("new attendance entry: %v", err)
		}
		return e
	}
	s := domain.SummarizeAttendance([]*domain.AttendanceEntry{mk("present"), mk("present"), mk("late"), mk("absent"), mk("excused")})
	if s.Present != 2 || s.Late != 1 || s.Absent != 1 || s.Excused != 1 || s.Total() != 5 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	rate, ok := s.Rate()
	if !ok || rate != 60 { // (2 present + 1 late) / 5
		t.Fatalf("expected 60%%, got %v (ok=%v)", rate, ok)
	}
	if _, ok := (domain.AttendanceSummary{}).Rate(); ok {
		t.Fatal("empty summary must have no rate")
	}
}

func TestNewEventDraftReportCard_AllowsEmptyYearAndTemplate(t *testing.T) {
	card, err := domain.NewEventDraftReportCard(tenantA, studentA, "", term1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Status != string(domain.ReportCardStatusDraft) || card.TermID != term1 {
		t.Fatalf("unexpected card: %+v", card)
	}
	if err := card.Validate(); err != nil {
		t.Fatalf("event draft must validate: %v", err)
	}
	if _, err := domain.NewEventDraftReportCard(tenantA, "", "", ""); err == nil {
		t.Fatal("expected error for empty student")
	}
}
