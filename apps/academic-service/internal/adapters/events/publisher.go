// Package events adapts the platform eventbus to the academic-service EventPublisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the academic service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishYear emits a CloudEvent for the given academic year domain event.
func (p *Publisher) PublishYear(ctx context.Context, eventType string, year *domain.AcademicYear, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"year_id":    year.ID,
		"name":       year.Name,
		"start_date": year.StartDate,
		"end_date":   year.EndDate,
	}
	for k, v := range meta {
		data[k] = v
	}
	return p.publish(ctx, eventType, year.TenantID, year.ID, data)
}

// PublishTerm emits a CloudEvent for the given term domain event.
func (p *Publisher) PublishTerm(ctx context.Context, eventType string, term *domain.Term, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"term_id":          term.ID,
		"academic_year_id": term.AcademicYearID,
		"name":             term.Name,
		"start_date":       term.StartDate,
		"end_date":         term.EndDate,
	}
	for k, v := range meta {
		data[k] = v
	}
	return p.publish(ctx, eventType, term.TenantID, term.ID, data)
}

// PublishClass emits a CloudEvent for the given class domain event. The payload
// conforms to contracts/events/academic.class_created.v1.json for created events.
func (p *Publisher) PublishClass(ctx context.Context, eventType string, class *domain.Class, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"class_id":         class.ID,
		"name":             class.Name,
		"academic_year_id": class.AcademicYearID,
	}
	for k, v := range meta {
		data[k] = v
	}
	return p.publish(ctx, eventType, class.TenantID, class.ID, data)
}

// PublishSubject emits a CloudEvent for the given subject domain event. The payload
// conforms to contracts/events/academic.subject_created.v1.json for created events.
func (p *Publisher) PublishSubject(ctx context.Context, eventType string, subject *domain.Subject, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"subject_id": subject.ID,
		"name":       subject.Name,
		"code":       subject.Code,
	}
	for k, v := range meta {
		data[k] = v
	}
	return p.publish(ctx, eventType, subject.TenantID, subject.ID, data)
}

func (p *Publisher) publish(ctx context.Context, eventType, tenantID, subjectID string, data map[string]any) error {
	event, err := tenancy.NewCloudEvent(eventType, "academic-service", "", tenantID, data)
	if err != nil {
		return fmt.Errorf("academic: build event: %w", err)
	}
	event.Subject = subjectID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
