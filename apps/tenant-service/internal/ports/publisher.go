package ports

import "context"

// EventPublisher emits tenant domain events.
type EventPublisher interface {
	Publish(ctx context.Context, eventType, tenantCode string, payload map[string]any) error
}
