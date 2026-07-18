// Package pdf renders report card PDFs.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	"github.com/jung-kurt/gofpdf"
)

// Generator renders report card PDFs with gofpdf: tenant header, student
// identity, subject scores table and attendance summary. Compression is left
// off so the (small) output stays inspectable.
type Generator struct{}

var _ ports.PDFGenerator = (*Generator)(nil)

// NewGenerator creates a new PDF generator.
func NewGenerator() *Generator { return &Generator{} }

// GenerateReportCard renders the report card document to PDF bytes.
func (g *Generator) GenerateReportCard(ctx context.Context, doc *domain.ReportCardDocument) ([]byte, error) {
	_ = ctx
	if doc == nil || doc.Card == nil {
		return nil, fmt.Errorf("pdf: report card document with a card is required")
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	g.header(pdf, doc)
	g.identity(pdf, doc)
	g.scoresTable(pdf, doc)
	g.attendanceSummary(pdf, doc)
	g.remarks(pdf, doc)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf: render report card: %w", err)
	}
	return buf.Bytes(), nil
}

// header renders the school/tenant block and the report title. The title comes
// from the assigned template when present.
func (g *Generator) header(pdf *gofpdf.Fpdf, doc *domain.ReportCardDocument) {
	title := "Student Report Card"
	if doc.Template != nil && doc.Template.Name != "" {
		title = doc.Template.Name
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 6, fmt.Sprintf("School (tenant): %s", doc.Card.TenantID), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "AuraEDU Report Card", "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 10, title, "", 1, "C", false, 0, "")
	pdf.Ln(2)
}

// identity renders the student / period / generated-at block.
func (g *Generator) identity(pdf *gofpdf.Fpdf, doc *domain.ReportCardDocument) {
	card := doc.Card
	generatedAt := doc.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	pdf.SetFont("Arial", "", 11)
	rows := [][2]string{
		{"Student ID:", card.StudentID},
		{"Report Card ID:", card.ID},
		{"Academic Year:", orDash(card.AcademicYearID)},
		{"Period (term):", orDash(card.TermID)},
		{"Generated at:", generatedAt.UTC().Format(time.RFC3339)},
	}
	for _, r := range rows {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(45, 6, r[0], "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.CellFormat(0, 6, r[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)
}

// scoresTable renders the aggregated subject scores, or an empty state.
func (g *Generator) scoresTable(pdf *gofpdf.Fpdf, doc *domain.ReportCardDocument) {
	g.sectionTitle(pdf, "Subject Scores")
	if len(doc.Scores) == 0 {
		pdf.SetFont("Arial", "I", 10)
		pdf.CellFormat(0, 6, "No scores recorded yet.", "", 1, "L", false, 0, "")
		pdf.Ln(4)
		return
	}

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(90, 7, "Subject", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 7, "Score", "1", 0, "R", true, 0, "")
	pdf.CellFormat(30, 7, "Max", "1", 0, "R", true, 0, "")
	pdf.CellFormat(30, 7, "%", "1", 1, "R", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, s := range doc.Scores {
		pdf.CellFormat(90, 7, truncate(s.Label, 60), "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, formatScore(s.Score), "1", 0, "R", false, 0, "")
		max := "-"
		if s.MaxScore != nil {
			max = formatScore(*s.MaxScore)
		}
		pdf.CellFormat(30, 7, max, "1", 0, "R", false, 0, "")
		pct := "-"
		if p, ok := s.Percentage(); ok {
			pct = fmt.Sprintf("%.1f", p)
		}
		pdf.CellFormat(30, 7, pct, "1", 1, "R", false, 0, "")
	}
	pdf.Ln(4)
}

// attendanceSummary renders the attendance counts and rate, or an empty state.
func (g *Generator) attendanceSummary(pdf *gofpdf.Fpdf, doc *domain.ReportCardDocument) {
	g.sectionTitle(pdf, "Attendance Summary")
	a := doc.Attendance
	if a.Total() == 0 {
		pdf.SetFont("Arial", "I", 10)
		pdf.CellFormat(0, 6, "No attendance recorded yet.", "", 1, "L", false, 0, "")
		pdf.Ln(4)
		return
	}

	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Present: %d   Absent: %d   Late: %d   Excused: %d   Total days: %d",
		a.Present, a.Absent, a.Late, a.Excused, a.Total()), "", 1, "L", false, 0, "")
	if rate, ok := a.Rate(); ok {
		pdf.CellFormat(0, 6, fmt.Sprintf("Attendance rate: %.1f%%", rate), "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)
}

// remarks renders the template body as a remarks block when one is assigned.
func (g *Generator) remarks(pdf *gofpdf.Fpdf, doc *domain.ReportCardDocument) {
	if doc.Template == nil || doc.Template.BodyTemplate == "" {
		return
	}
	g.sectionTitle(pdf, "Remarks")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, doc.Template.BodyTemplate, "", "L", false)
}

func (g *Generator) sectionTitle(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatScore(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "~"
}
