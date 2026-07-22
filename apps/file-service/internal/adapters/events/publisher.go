// Package events publishes file-service domain events.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the file service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// Publish emits a CloudEvent for the given file domain event.
func (p *Publisher) Publish(ctx context.Context, eventType string, file *domain.FileUpload, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.FileEventData(file, meta)
	event, err := tenancy.NewCloudEvent(eventType, "file-service", "", file.TenantID, data)
	if err != nil {
		return fmt.Errorf("file: build event: %w", err)
	}
	event.Subject = file.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "file-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("file: build outbox event: %w", err)
	}
	if fileID, ok := data["file_id"].(string); ok && fileID != "" {
		event.Subject = fileID
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
