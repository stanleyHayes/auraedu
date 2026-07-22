// Package events adapts the platform eventbus to the tenant service EventPublisher port.
package events

import (
	"context"
	"fmt"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/google/uuid"
)

// Publisher adapts the platform eventbus to the tenant service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher. A nil bus disables publishing.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// Publish emits a CloudEvent for the given tenant domain event on the JetStream AURA stream.
func (p *Publisher) Publish(ctx context.Context, eventType, tenantCode string, payload map[string]any) error {
	return p.PublishWithID(ctx, newEventID(), eventType, tenantCode, payload)
}

// PublishWithID emits a replay-safe outbox event. The stable outbox UUID is
// used as both CloudEvent id and JetStream idempotency key.
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantCode string, payload map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "tenant-service", eventID, tenantCode, payload)
	if err != nil {
		return fmt.Errorf("tenant: build event %q: %w", eventType, err)
	}
	event.IdempotencyKey = eventID
	return p.bus.Publish(ctx, event)
}

// newEventID generates the CloudEvent id. The platform eventbus validates the
// event before it can auto-assign an id, so callers must supply one.
func newEventID() string { return uuid.Must(uuid.NewV7()).String() }
