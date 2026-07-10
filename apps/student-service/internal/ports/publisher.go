package ports

import (
	"context"

	"github.com/auraedu/student-service/internal/domain"
)

// EventPublisher emits student domain events.
type EventPublisher interface {
	// Publish sends a student domain event. meta may contain extra event data such as changed_fields.
	Publish(ctx context.Context, eventType string, student *domain.Student, meta map[string]any) error
}
