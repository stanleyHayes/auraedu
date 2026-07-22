// Package events adapts the platform eventbus to the attendance service publisher port.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the attendance service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// Publish emits a CloudEvent for the given attendance domain event.
func (p *Publisher) Publish(ctx context.Context, eventType string, record *domain.AttendanceRecord, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.AttendanceEventData(record, meta)
	event, err := tenancy.NewCloudEvent(eventType, "attendance-service", "", record.TenantID, data)
	if err != nil {
		return fmt.Errorf("attendance: build event: %w", err)
	}
	event.Subject = record.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "attendance-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("attendance: build outbox event: %w", err)
	}
	if id, ok := data["attendance_id"].(string); ok && id != "" {
		event.Subject = id
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
