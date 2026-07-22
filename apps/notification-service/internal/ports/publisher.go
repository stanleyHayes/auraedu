// Package ports defines the notification-service boundary interfaces.
package ports

import (
	"context"

	"github.com/auraedu/notification-service/internal/domain"
)

// EventPublisher emits notification domain events.
type EventPublisher interface {
	// PublishMessageSent emits a notification.sent.v1 event.
	PublishMessageSent(ctx context.Context, m *domain.Message) error
	// PublishMessageFailed emits a notification.failed.v1 event.
	PublishMessageFailed(ctx context.Context, m *domain.Message, reason string) error
}

// MessageSentEventData is the canonical privacy-minimized public payload used
// by both direct and transactional-outbox publication.
func MessageSentEventData(message *domain.Message) map[string]any {
	return map[string]any{
		"message_id":   message.ID,
		"channel":      message.Channel,
		"recipient_id": message.RecipientID,
	}
}

// MessageFailedEventData excludes recipient, content and raw provider errors.
// Detailed diagnostics remain in the tenant-scoped message record; the public
// integration event carries only a stable privacy-safe outcome code.
func MessageFailedEventData(message *domain.Message, _ string) map[string]any {
	return map[string]any{
		"message_id": message.ID,
		"channel":    message.Channel,
		"reason":     "delivery_failed",
	}
}
