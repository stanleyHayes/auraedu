// Package events adapts outbound report domain events to the platform eventbus.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

// Publisher adapts the platform eventbus to the report service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishReportTemplate emits a CloudEvent for the given report template domain event.
func (p *Publisher) PublishReportTemplate(ctx context.Context, eventType string, t *domain.ReportTemplate, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"report_template_id": t.ID,
		"academic_year_id":   t.AcademicYearID,
		"name":               t.Name,
		"status":             t.Status,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "report-service", "", t.TenantID, data)
	if err != nil {
		return fmt.Errorf("report: build template event: %w", err)
	}
	event.Subject = t.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishReportCard emits a CloudEvent for the given report card domain event.
func (p *Publisher) PublishReportCard(ctx context.Context, eventType string, c *domain.ReportCard, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"report_card_id":   c.ID,
		"student_id":       c.StudentID,
		"academic_year_id": c.AcademicYearID,
		"term_id":          c.TermID,
		"template_id":      c.TemplateID,
		"status":           c.Status,
	}
	if c.PDFPath != nil {
		data["pdf_path"] = *c.PDFPath
		// Consumers never see local disk paths; file_url is the download route
		// (contracts/events/report.published.v1.json).
		data["file_url"] = "/api/v1/report-cards/" + c.ID + "/download"
	}
	if c.GeneratedAt != nil {
		data["generated_at"] = c.GeneratedAt.Format(time.RFC3339)
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "report-service", "", c.TenantID, data)
	if err != nil {
		return fmt.Errorf("report: build report card event: %w", err)
	}
	event.Subject = c.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
