package ports

import (
	"context"

	"github.com/auraedu/academic-service/internal/domain"
)

// EventPublisher emits academic domain events.
type EventPublisher interface {
	// Publish sends an academic domain event. meta may contain extra event data such as changed_fields.
	Publish(ctx context.Context, eventType string, year *domain.AcademicYear, meta map[string]any) error
}
