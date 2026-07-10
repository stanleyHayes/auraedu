package ports

import (
	"context"

	"github.com/auraedu/file-service/internal/domain"
)

// EventPublisher emits file domain events.
type EventPublisher interface {
	// Publish sends a file domain event. meta may contain extra event data such as changed_fields.
	Publish(ctx context.Context, eventType string, file *domain.FileUpload, meta map[string]any) error
}
