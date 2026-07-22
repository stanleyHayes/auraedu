// Package events publishes privacy-safe knowledge lifecycle events.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

type Publisher struct{ inner *eventbus.Publisher }

func NewPublisher(inner *eventbus.Publisher) *Publisher { return &Publisher{inner: inner} }

func (p *Publisher) Publish(ctx context.Context, eventType, tenantID string, data map[string]any) error {
	return p.publish(ctx, uuid.NewString(), eventType, tenantID, data)
}

// PublishWithID publishes a durable outbox record using the same identity for
// the CloudEvent and JetStream de-duplication key across every retry.
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	return p.publish(ctx, eventID, eventType, tenantID, data)
}

func (p *Publisher) publish(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.inner == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "knowledge-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("knowledge: build event: %w", err)
	}
	event.Type = eventType
	event.IdempotencyKey = eventID
	if subject, ok := data["source_id"].(string); ok {
		event.Subject = subject
	}
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.inner.Publish(ctx, event)
}
