// Package events publishes content lifecycle events.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

type Publisher struct{ bus *eventbus.Publisher }

func New(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

func (p *Publisher) Publish(ctx context.Context, eventType, tenantID string, data map[string]any) error {
	return p.PublishWithID(ctx, uuid.NewString(), eventType, tenantID, data)
}

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "content-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("content event: %w", err)
	}
	event.Type, event.IdempotencyKey, event.Time = eventType, eventID, time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
