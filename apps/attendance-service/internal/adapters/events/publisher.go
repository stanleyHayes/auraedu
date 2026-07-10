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
	data := map[string]any{
		"attendance_id":    record.ID,
		"student_id":       record.StudentID,
		"academic_year_id": record.AcademicYearID,
		"date":             record.Date.String(),
		"status":           record.Status,
		"marked_by":        record.MarkedBy,
	}
	if record.Reason != nil {
		data["reason"] = *record.Reason
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "attendance-service", "", record.TenantID, data)
	if err != nil {
		return fmt.Errorf("attendance: build event: %w", err)
	}
	event.Subject = record.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
