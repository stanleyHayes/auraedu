// Package ports defines the report service publisher boundary.
package ports

import (
	"context"
	"time"

	"github.com/auraedu/report-service/internal/domain"
)

// ReportTemplateEventData is the canonical payload shared by direct adapters
// and the transactional outbox.
func ReportTemplateEventData(t *domain.ReportTemplate, meta map[string]any) map[string]any {
	data := map[string]any{
		"report_template_id": t.ID,
		"academic_year_id":   t.AcademicYearID,
		"name":               t.Name,
		"status":             t.Status,
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

// ReportCardEventData is the canonical payload shared by direct adapters and
// the transactional outbox. Private storage keys never leave the service.
func ReportCardEventData(eventType string, c *domain.ReportCard, meta map[string]any) map[string]any {
	if eventType == "report.published.v1" {
		data := map[string]any{
			"report_card_id": c.ID,
			"student_id":     c.StudentID,
			"term_id":        c.TermID,
		}
		if c.PDFPath != nil {
			data["file_url"] = "/api/v1/report-cards/" + c.ID + "/download"
		}
		return data
	}

	data := map[string]any{
		"report_card_id":   c.ID,
		"student_id":       c.StudentID,
		"academic_year_id": c.AcademicYearID,
		"status":           c.Status,
	}
	if c.TermID != "" {
		data["term_id"] = c.TermID
	}
	if c.TemplateID != "" {
		data["template_id"] = c.TemplateID
	}
	if c.PDFPath != nil {
		data["file_url"] = "/api/v1/report-cards/" + c.ID + "/download"
	}
	if c.GeneratedAt != nil {
		data["generated_at"] = c.GeneratedAt.Format(time.RFC3339)
	}
	if changed, ok := meta["changed_fields"]; ok {
		data["changed_fields"] = changed
	}
	return data
}

// EventPublisher emits report domain events.
type EventPublisher interface {
	// PublishReportTemplate emits an event about a ReportTemplate aggregate.
	PublishReportTemplate(ctx context.Context, eventType string, t *domain.ReportTemplate, meta map[string]any) error
	// PublishReportCard emits an event about a ReportCard aggregate.
	PublishReportCard(ctx context.Context, eventType string, c *domain.ReportCard, meta map[string]any) error
}
