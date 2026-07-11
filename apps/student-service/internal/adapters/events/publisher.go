package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
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
	data := map[string]any{}
	if student != nil {
		data["student_id"] = student.ID
	}
	for k, v := range meta {
		data[k] = v
	}

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

	event, err := tenancy.NewCloudEvent(eventType, "student-service", "", tenantID, data)
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
