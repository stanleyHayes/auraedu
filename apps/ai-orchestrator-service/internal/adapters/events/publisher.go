// Package events publishes assistant domain events to the canonical event stream.
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

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	return p.publish(ctx, eventID, eventType, tenantID, data)
}

func (p *Publisher) publish(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.inner == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "ai-orchestrator-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("assistant: build event: %w", err)
	}
	event.Type, event.Time, event.IdempotencyKey = eventType, time.Now().UTC().Format(time.RFC3339), eventID
	if subject, ok := data["message_id"].(string); ok {
		event.Subject = subject
	}
	return p.inner.Publish(ctx, event)
}
