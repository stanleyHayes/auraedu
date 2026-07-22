// Package events adapts the platform eventbus to the assessment-service EventPublisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the assessment service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishAssessment emits a CloudEvent for the given assessment domain event.
func (p *Publisher) PublishAssessment(ctx context.Context, eventType string, a *domain.Assessment, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.AssessmentEventData(a, meta)
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", "", a.TenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build event: %w", err)
	}
	event.Subject = a.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishAssignment emits a CloudEvent for the given assignment domain event.
// The payload conforms to contracts/events/assignment.published.v1.json.
func (p *Publisher) PublishAssignment(ctx context.Context, eventType string, a *domain.Assessment, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.AssignmentEventData(a, meta)
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", "", a.TenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build assignment event: %w", err)
	}
	event.Subject = a.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishScore emits a CloudEvent for the given score domain event.
func (p *Publisher) PublishScore(ctx context.Context, eventType string, s *domain.Score, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.ScoreEventData(s, meta)
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", "", s.TenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build score event: %w", err)
	}
	event.Subject = s.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "assessment-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("assessment: build outbox event: %w", err)
	}
	for _, key := range []string{"score_id", "assignment_id", "assessment_id"} {
		if id, ok := data[key].(string); ok && id != "" {
			event.Subject = id
			break
		}
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
