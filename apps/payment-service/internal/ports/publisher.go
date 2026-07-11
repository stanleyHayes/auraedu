// Package ports defines the payment service application boundaries.
package ports

import (
	"context"

	"github.com/auraedu/payment-service/internal/domain"
)

// EventPublisher emits payment domain events.
type EventPublisher interface {
	// PublishPayment sends a payment domain event.
	PublishPayment(ctx context.Context, eventType string, p *domain.Payment, meta map[string]any) error
}
