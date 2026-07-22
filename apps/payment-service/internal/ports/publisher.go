// Package ports defines the payment service application boundaries.
package ports

import (
	"context"
	"time"

	"github.com/auraedu/payment-service/internal/domain"
)

// EventPublisher emits payment domain events.
type EventPublisher interface {
	// PublishPayment sends a payment domain event.
	PublishPayment(ctx context.Context, eventType string, p *domain.Payment, meta map[string]any) error
}

// PaymentEventData is the canonical public payload for eventType. Keeping it
// at the port boundary lets direct publishers and the transactional outbox
// produce byte-for-byte equivalent, contract-specific data.
func PaymentEventData(eventType string, payment *domain.Payment, meta map[string]any) map[string]any {
	switch eventType {
	case "payment.received.v1":
		return map[string]any{
			"payment_id": payment.ID,
			"invoice_id": payment.InvoiceID,
			"amount":     payment.AmountCents,
			"gateway":    payment.Provider,
		}
	case "payment.failed.v1":
		data := map[string]any{
			"payment_id": payment.ID,
			"invoice_id": payment.InvoiceID,
		}
		if reason, ok := meta["reason"]; ok {
			data["reason"] = reason
		}
		return data
	}

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
	for _, key := range []string{"changed_fields", "failure_reason"} {
		if value, ok := meta[key]; ok {
			data[key] = value
		}
	}
	return data
}
