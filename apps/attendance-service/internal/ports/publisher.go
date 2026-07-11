// Package ports defines the inbound and outbound ports for the attendance service.
package ports

import (
	"context"

	"github.com/auraedu/attendance-service/internal/domain"
)

// EventPublisher emits attendance domain events.
type EventPublisher interface {
	// Publish sends an attendance domain event. meta may contain extra event data such as changed_fields.
	Publish(ctx context.Context, eventType string, record *domain.AttendanceRecord, meta map[string]any) error
}
