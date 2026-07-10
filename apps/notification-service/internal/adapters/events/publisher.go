package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the notification service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishMessageSent emits a notification.sent.v1 CloudEvent.
func (p *Publisher) PublishMessageSent(ctx context.Context, m *domain.Message) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"message_id":   m.ID,
		"channel":      m.Channel,
		"recipient_id": m.RecipientID,
	}
	if m.TemplateID != nil {
		data["template_id"] = *m.TemplateID
	}
	event, err := tenancy.NewCloudEvent("notification.sent.v1", "notification-service", "", m.TenantID, data)
	if err != nil {
		return fmt.Errorf("notifications: build sent event: %w", err)
	}
	event.Subject = m.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishMessageFailed emits a notification.failed.v1 CloudEvent.
func (p *Publisher) PublishMessageFailed(ctx context.Context, m *domain.Message, reason string) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"message_id": m.ID,
		"channel":    m.Channel,
		"reason":     reason,
	}
	event, err := tenancy.NewCloudEvent("notification.failed.v1", "notification-service", "", m.TenantID, data)
	if err != nil {
		return fmt.Errorf("notifications: build failed event: %w", err)
	}
	event.Subject = m.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
