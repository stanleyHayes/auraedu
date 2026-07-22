// Package events publishes market-intelligence lifecycle events.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

type Publisher struct{ bus *eventbus.Publisher }

func New(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	event, e := tenancy.NewCloudEvent(eventType, "market-intelligence-service", eventID, tenantID, data)
	if e != nil {
		return fmt.Errorf("intelligence event: %w", e)
	}
	event.Type = eventType
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
