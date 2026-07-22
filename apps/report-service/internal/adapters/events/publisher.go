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

// PublishWithID emits an outbox event with a stable CloudEvent identity so
// retries remain idempotent for consumers.
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "report-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("report: build outbox event: %w", err)
	}
	event.Type = eventType
	event.IdempotencyKey = eventID
	if subject, ok := data["report_card_id"].(string); ok {
		event.Subject = subject
	} else if subject, ok := data["report_template_id"].(string); ok {
		event.Subject = subject
	}
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishReportTemplate emits a CloudEvent for the given report template domain event.
func (p *Publisher) PublishReportTemplate(ctx context.Context, eventType string, t *domain.ReportTemplate, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.ReportTemplateEventData(t, meta)
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
	data := ports.ReportCardEventData(eventType, c, meta)
	event, err := tenancy.NewCloudEvent(eventType, "report-service", "", c.TenantID, data)
	if err != nil {
		return fmt.Errorf("report: build report card event: %w", err)
	}
	event.Subject = c.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
