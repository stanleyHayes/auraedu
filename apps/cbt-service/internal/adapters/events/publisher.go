// Package events adapts the platform eventbus to the CBT service publisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the CBT service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishQuestionBank emits a CloudEvent for the given question bank domain event.
func (p *Publisher) PublishQuestionBank(ctx context.Context, eventType string, q *domain.QuestionBank, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.QuestionEventData(q, meta)
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", "", q.TenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build question event: %w", err)
	}
	event.Subject = q.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishExamSession emits a CloudEvent for the given exam session domain event.
func (p *Publisher) PublishExamSession(ctx context.Context, eventType string, e *domain.ExamSession, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.ExamEventData(e, meta)
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", "", e.TenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build exam event: %w", err)
	}
	event.Subject = e.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishSubmission emits a CloudEvent for the given submission domain event.
func (p *Publisher) PublishSubmission(ctx context.Context, eventType string, s *domain.Submission, _ map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.SubmissionEventData(eventType, s)
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", "", s.TenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build submission event: %w", err)
	}
	event.Subject = s.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "cbt-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("cbt: build outbox event: %w", err)
	}
	for _, key := range []string{"submission_id", "exam_id", "question_id"} {
		if subject, ok := data[key].(string); ok && subject != "" {
			event.Subject = subject
			break
		}
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
