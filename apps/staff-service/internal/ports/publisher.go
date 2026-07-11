// Package ports defines the staff service publisher boundary.
package ports

import (
	"context"

	"github.com/auraedu/staff-service/internal/domain"
)

// EventPublisher emits staff domain events.
type EventPublisher interface {
	// Publish sends a staff domain event. meta may contain extra event data such as changed_fields.
	Publish(ctx context.Context, eventType string, staff *domain.Staff, meta map[string]any) error
}
