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

// PublishPayment emits a CloudEvent for a payment domain event.
func (p *Publisher) PublishPayment(ctx context.Context, eventType string, payment *domain.Payment, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "payment-service", "", payment.TenantID, PaymentEventData(payment, meta))
	if err != nil {
		return fmt.Errorf("payments: build payment event: %w", err)
	}
	event.Subject = payment.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PaymentEventData builds the event data payload. It carries the fields required by
// contracts/events/payment.received.v1.json (payment_id, invoice_id, amount) and
// payment.failed.v1.json (payment_id), plus gateway and operational extras.
func PaymentEventData(payment *domain.Payment, meta map[string]any) map[string]any {
	data := map[string]any{
		"payment_id":   payment.ID,
		"invoice_id":   payment.InvoiceID,
		"amount":       payment.AmountCents,
		"amount_cents": payment.AmountCents,
		"currency":     payment.Currency,
		"provider":     payment.Provider,
		"gateway":      payment.Provider,
		"status":       payment.Status,
		"initiated_at": payment.InitiatedAt.Format(time.RFC3339),
	}
	if payment.ProviderReference != nil {
		data["provider_reference"] = *payment.ProviderReference
	}
	if payment.CompletedAt != nil {
		data["completed_at"] = payment.CompletedAt.Format(time.RFC3339)
	}
	for k, v := range meta {
		data[k] = v
	}
	return data
}
