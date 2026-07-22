// Package events adapts outbound staff domain events to the platform eventbus.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
)

// Publisher adapts the platform eventbus to the staff service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// Publish emits a CloudEvent for the given staff domain event.
func (p *Publisher) Publish(ctx context.Context, eventType string, staff *domain.Staff, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.StaffEventData(staff, meta)
	event, err := tenancy.NewCloudEvent(eventType, "staff-service", "", staff.TenantID, data)
	if err != nil {
		return fmt.Errorf("staff: build event: %w", err)
	}
	event.Subject = staff.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishWithID emits a replay-safe outbox event with a stable CloudEvent ID.
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "staff-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("staff: build outbox event: %w", err)
	}
	staffID, ok := data["staff_id"].(string)
	if ok && staffID != "" {
		event.Subject = staffID
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
