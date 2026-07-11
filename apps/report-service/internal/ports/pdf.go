// Package ports defines the report service PDF generator boundary.
package ports

import (
	"context"

	"github.com/auraedu/report-service/internal/domain"
)

// PDFGenerator renders a report card to a PDF byte slice.
type PDFGenerator interface {
	// GenerateReportCard returns the PDF bytes for the given report card.
	GenerateReportCard(ctx context.Context, card *domain.ReportCard) ([]byte, error)
}
