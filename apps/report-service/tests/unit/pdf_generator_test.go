package unit

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/auraedu/report-service/internal/adapters/pdf"
	"github.com/auraedu/report-service/internal/domain"
)

func testCard(t *testing.T) *domain.ReportCard {
	t.Helper()
	card, err := domain.NewEventDraftReportCard(tenantA, studentA, ay1, term1)
	if err != nil {
		t.Fatalf("new card: %v", err)
	}
	return card
}

func testTemplate(t *testing.T) *domain.ReportTemplate {
	t.Helper()
	tmpl, err := domain.NewReportTemplate(tenantA, "Midterm Template", ay1, "Well done this term.")
	if err != nil {
		t.Fatalf("new template: %v", err)
	}
	return tmpl
}

func generate(t *testing.T, doc *domain.ReportCardDocument) []byte {
	t.Helper()
	out, err := pdf.NewGenerator().GenerateReportCard(context.Background(), doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatal("expected PDF output")
	}
	return out
}

func TestPDFGenerator_RendersEntriesTemplateAndIdentity(t *testing.T) {
	max100, max50 := 100.0, 50.0
	doc := &domain.ReportCardDocument{
		Card:     testCard(t),
		Template: testTemplate(t),
		Scores: []domain.SubjectScore{
			{Label: "Mathematics", Score: 72, MaxScore: &max100, Count: 2},
			{Label: "Science", Score: 40, MaxScore: &max50, Count: 1},
		},
		Attendance:  domain.AttendanceSummary{Present: 2, Absent: 1, Late: 1},
		GeneratedAt: time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
	}
	out := generate(t, doc)

	for _, want := range []string{
		"Midterm Template",       // template name as title
		"Well done this term.",   // template body as remarks
		"AuraEDU Report Card",    // tenant header block
		tenantA,                  // school/tenant identifier
		studentA,                 // student identity
		term1,                    // period
		"2026-07-18T10:00:00Z",   // generated-at
		"Mathematics", "Science", // score rows
		"Subject", "Score", "Max", // table header
		"72", "100", "72.0", // score / max / percentage
		"Present: 2", "Absent: 1", // attendance summary
		"Attendance rate: 75.0%", // (2 present + 1 late) / 4
	} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("PDF missing %q", want)
		}
	}
}

func TestPDFGenerator_EmptyState(t *testing.T) {
	doc := &domain.ReportCardDocument{
		Card:        testCard(t),
		GeneratedAt: time.Now().UTC(),
	}
	out := generate(t, doc)

	for _, want := range []string{
		"Student Report Card", // fallback title without a template
		"No scores recorded yet.",
		"No attendance recorded yet.",
	} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("PDF missing %q", want)
		}
	}
}

func TestPDFGenerator_RequiresCard(t *testing.T) {
	if _, err := pdf.NewGenerator().GenerateReportCard(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil document")
	}
	if _, err := pdf.NewGenerator().GenerateReportCard(context.Background(), &domain.ReportCardDocument{}); err == nil {
		t.Fatal("expected error for document without card")
	}
}
