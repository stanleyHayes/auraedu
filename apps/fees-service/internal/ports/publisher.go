// Package ports defines the fees-service boundary interfaces.
package ports

import (
	"context"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
)

// EventPublisher emits fees domain events.
type EventPublisher interface {
	// PublishFeeStructure sends a fee structure domain event.
	PublishFeeStructure(ctx context.Context, eventType string, fs *domain.FeeStructure, meta map[string]any) error
	// PublishInvoice sends an invoice domain event.
	PublishInvoice(ctx context.Context, eventType string, inv *domain.Invoice, meta map[string]any) error
}

func FeeStructureEventData(fs *domain.FeeStructure, meta map[string]any) map[string]any {
	data := map[string]any{
		"fee_structure_id": fs.ID,
		"class_id":         nil,
	}
	for _, key := range []string{"class_id", "amount"} {
		if value, ok := meta[key]; ok {
			data[key] = value
		}
	}
	return data
}

func InvoiceEventData(eventType string, inv *domain.Invoice, meta map[string]any) map[string]any {
	if eventType == "invoice.created.v1" {
		return map[string]any{
			"invoice_id": inv.ID,
			"student_id": inv.StudentID,
			"amount_due": float64(inv.AmountCents) / 100,
			"due_date":   inv.DueDate.String(),
		}
	}
	data := map[string]any{
		"invoice_id":       inv.ID,
		"student_id":       inv.StudentID,
		"fee_structure_id": inv.FeeStructureID,
		"amount_due":       float64(inv.AmountCents) / 100,
		"balance_due":      float64(inv.BalanceCents) / 100,
		"amount_cents":     inv.AmountCents,
		"balance_cents":    inv.BalanceCents,
		"status":           inv.Status,
		"due_date":         inv.DueDate.String(),
		"issued_at":        inv.IssuedAt.Format(time.RFC3339),
	}
	if inv.Notes != nil {
		data["notes"] = *inv.Notes
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}
