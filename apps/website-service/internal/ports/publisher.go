// Package ports defines the website-service driven ports.
package ports

import (
	"context"

	"github.com/auraedu/website-service/internal/domain"
)

// EventPublisher emits website domain events.
type EventPublisher interface {
	// PublishPage sends a page domain event. meta may contain extra event data such as changed_fields.
	PublishPage(ctx context.Context, eventType string, page *domain.Page, meta map[string]any) error
	// PublishSection sends a section domain event. meta may contain extra event data.
	PublishSection(ctx context.Context, eventType string, section *domain.Section, meta map[string]any) error
}
