package ports

import (
	"context"

	"github.com/auraedu/platform/tenancy"
)

// Handler receives a CloudEvent that the subscriber has pulled from the event bus.
// The context carries the tenant context (TenantID and optional ActorID).
type Handler func(ctx context.Context, event tenancy.CloudEvent) error

// Subscriber consumes CloudEvents from the event bus and dispatches them to a Handler.
type Subscriber interface {
	Start(ctx context.Context, handler Handler) error
	Stop() error
}
