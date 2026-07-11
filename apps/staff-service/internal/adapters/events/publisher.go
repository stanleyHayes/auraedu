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
	data := map[string]any{
		"staff_id":   staff.ID,
		"staff_type": staff.StaffType,
		"name":       staff.FullName(),
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "staff-service", "", staff.TenantID, data)
	if err != nil {
		return fmt.Errorf("staff: build event: %w", err)
	}
	event.Subject = staff.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
