package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
)

// Publisher adapts the platform eventbus to the website service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishPage emits a CloudEvent for the given page domain event.
func (p *Publisher) PublishPage(ctx context.Context, eventType string, page *domain.Page, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"page_id": page.ID,
		"slug":    page.Slug,
		"title":   page.Title,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "website-service", "", page.TenantID, data)
	if err != nil {
		return fmt.Errorf("website: build page event: %w", err)
	}
	event.Subject = page.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishSection emits a CloudEvent for the given section domain event.
func (p *Publisher) PublishSection(ctx context.Context, eventType string, section *domain.Section, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"section_id": section.ID,
		"page_id":    section.PageID,
		"type":       section.Type,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "website-service", "", section.TenantID, data)
	if err != nil {
		return fmt.Errorf("website: build section event: %w", err)
	}
	event.Subject = section.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// RecordingPublisher records events for tests.
type RecordingPublisher struct {
	Events []struct {
		Type      string
		TenantID  string
		SubjectID string
		Payload   map[string]any
	}
}

var _ ports.EventPublisher = (*RecordingPublisher)(nil)

// NewRecordingPublisher creates a new recording publisher.
func NewRecordingPublisher() *RecordingPublisher { return &RecordingPublisher{} }

func (r *RecordingPublisher) PublishPage(ctx context.Context, eventType string, page *domain.Page, meta map[string]any) error {
	return r.record(eventType, page.TenantID, page.ID, meta)
}

func (r *RecordingPublisher) PublishSection(ctx context.Context, eventType string, section *domain.Section, meta map[string]any) error {
	return r.record(eventType, section.TenantID, section.ID, meta)
}

func (r *RecordingPublisher) record(eventType, tenantID, subjectID string, meta map[string]any) error {
	r.Events = append(r.Events, struct {
		Type      string
		TenantID  string
		SubjectID string
		Payload   map[string]any
	}{eventType, tenantID, subjectID, meta})
	return nil
}
