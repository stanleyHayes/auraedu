package ports

import (
	"context"

	"github.com/auraedu/notification-service/internal/domain"
)

// Notifier dispatches a message through a concrete channel.
type Notifier interface {
	// Send attempts to deliver the message over the channel. The implementation
	// MUST be idempotent: repeated calls with the same message should not produce
	// duplicate side effects.
	Send(ctx context.Context, msg domain.Message) error
}
