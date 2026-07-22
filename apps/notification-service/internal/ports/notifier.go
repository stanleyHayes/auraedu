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

// ProviderReceipt correlates a provider-accepted notification with later signed
// delivery feedback. MessageID is intentionally kept out of public API models.
type ProviderReceipt struct {
	Provider  string
	MessageID string
}

// ReceiptNotifier is implemented by providers that return a stable delivery
// identifier. Existing channel adapters may continue implementing Notifier.
type ReceiptNotifier interface {
	Notifier
	SendWithReceipt(context.Context, domain.Message) (ProviderReceipt, error)
}
