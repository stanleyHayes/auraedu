// Package pdf renders report card PDFs.
package pdf

import (
	"bytes"
	"context"
	"fmt"

	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
	"github.com/jung-kurt/gofpdf"
)

// Generator is a simple PDF renderer implementing ports.PDFGenerator.
type Generator struct{}

var _ ports.PDFGenerator = (*Generator)(nil)

// NewGenerator creates a new PDF generator.
func NewGenerator() *Generator { return &Generator{} }

// GenerateReportCard returns a placeholder PDF containing the report card metadata.
func (g *Generator) GenerateReportCard(ctx context.Context, card *domain.ReportCard) ([]byte, error) {
	_ = ctx
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "AuraEDU Report Card")
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 8, fmt.Sprintf("Report Card ID: %s", card.ID))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Student ID: %s", card.StudentID))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Academic Year ID: %s", card.AcademicYearID))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Template ID: %s", card.TemplateID))
	pdf.Ln(8)
	pdf.Cell(0, 8, fmt.Sprintf("Status: %s", card.Status))
	pdf.Ln(12)

	pdf.SetFont("Arial", "I", 10)
	pdf.MultiCell(0, 6, "This is a placeholder report card. Real marks and attendance integration will be added in a future story.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf: render report card: %w", err)
	}
	return buf.Bytes(), nil
}
