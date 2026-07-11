package ports

import (
	"context"

	"github.com/auraedu/billing-service/internal/domain"
)

// EventPublisher emits billing domain events.
type EventPublisher interface {
	PublishSubscription(ctx context.Context, eventType string, s *domain.Subscription, meta map[string]any) error
	PublishPlan(ctx context.Context, eventType string, p *domain.Plan, meta map[string]any) error
	PublishInvoice(ctx context.Context, eventType string, i *domain.SaaSInvoice, meta map[string]any) error
}
