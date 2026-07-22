// Package events adapts outbound payment domain events to the platform eventbus.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
	"github.com/auraedu/payment-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the payment service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishWithID sends a transactional-outbox event with a stable identity for
// JetStream and consumer deduplication.
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "payment-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("payments: build outbox event: %w", err)
	}
	event.Type = eventType
	event.IdempotencyKey = eventID
	if subject, ok := data["payment_id"].(string); ok {
		event.Subject = subject
	}
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishPayment emits a CloudEvent for a payment domain event.
func (p *Publisher) PublishPayment(ctx context.Context, eventType string, payment *domain.Payment, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(
		eventType,
		"payment-service",
		"",
		payment.TenantID,
		ports.PaymentEventData(eventType, payment, meta),
	)
	if err != nil {
		return fmt.Errorf("payments: build payment event: %w", err)
	}
	event.Subject = payment.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PaymentEventData is retained as a compatibility alias for existing callers.
func PaymentEventData(eventType string, payment *domain.Payment, meta map[string]any) map[string]any {
	return ports.PaymentEventData(eventType, payment, meta)
}
