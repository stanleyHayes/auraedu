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
