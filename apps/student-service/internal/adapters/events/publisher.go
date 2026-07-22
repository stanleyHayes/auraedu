// Package events adapts outbound student domain events to the platform eventbus.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	"github.com/google/uuid"
)

// Publisher adapts the platform eventbus to the student service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// Publish emits a CloudEvent for the given student domain event.
// If student is nil, the caller must supply tenant_id (and optionally guardian_id) in meta.
func (p *Publisher) Publish(ctx context.Context, eventType string, student *domain.Student, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.StudentEventData(student, meta)

	tenantID := ""
	if student != nil {
		tenantID = student.TenantID
	} else if meta != nil {
		if v, ok := meta["tenant_id"].(string); ok {
			tenantID = v
		}
	}
	if tenantID == "" {
		return fmt.Errorf("student: cannot build event %q without tenant_id", eventType)
	}

	// Generate the event id up front: eventbus.Publish validates the CloudEvent
	// (id required) before it can deliver, so an empty id would silently drop the event.
	event, err := tenancy.NewCloudEvent(eventType, "student-service", uuid.Must(uuid.NewV7()).String(), tenantID, data)
	if err != nil {
		return fmt.Errorf("student: build event: %w", err)
	}
	if student != nil {
		event.Subject = student.ID
	} else if meta != nil {
		if v, ok := meta["guardian_id"].(string); ok {
			event.Subject = v
		}
	}
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "student-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("student: build outbox event: %w", err)
	}
	studentID, studentOK := data["student_id"].(string)
	guardianID, guardianOK := data["guardian_id"].(string)
	if studentOK && studentID != "" {
		event.Subject = studentID
	} else if guardianOK && guardianID != "" {
		event.Subject = guardianID
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
