package ports

import "context"

// Subscriber consumes external events and forwards them into the application layer.
// Implementations are typically messaging adapters (e.g. NATS JetStream).
type Subscriber interface {
	Start(ctx context.Context) error
	Stop() error
}
