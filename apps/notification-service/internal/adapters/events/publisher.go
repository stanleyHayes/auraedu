// Package events publishes notification-service domain events.
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

// PublishWithID publishes a durable outbox record with a stable CloudEvent ID,
// which also becomes the JetStream deduplication key.
func (p *Publisher) PublishWithID(ctx context.Context, id, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "notification-service", id, tenantID, data)
	if err != nil {
		return fmt.Errorf("notifications: build outbox event: %w", err)
	}
	if messageID, ok := data["message_id"].(string); ok && messageID != "" {
		event.Subject = messageID
	} else if journeyID, ok := data["journey_id"].(string); ok && journeyID != "" {
		event.Subject = journeyID
	}
	event.IdempotencyKey = id
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishMessageSent emits a notification.sent.v1 CloudEvent.
func (p *Publisher) PublishMessageSent(ctx context.Context, m *domain.Message) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.MessageSentEventData(m)
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
	data := ports.MessageFailedEventData(m, reason)
	event, err := tenancy.NewCloudEvent("notification.failed.v1", "notification-service", "", m.TenantID, data)
	if err != nil {
		return fmt.Errorf("notifications: build failed event: %w", err)
	}
	event.Subject = m.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
