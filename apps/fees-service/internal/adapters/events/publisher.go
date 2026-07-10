package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the fees service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishFeeStructure emits a CloudEvent for a fee structure domain event.
func (p *Publisher) PublishFeeStructure(ctx context.Context, eventType string, fs *domain.FeeStructure, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"fee_structure_id": fs.ID,
		"name":             fs.Name,
		"academic_year_id": fs.AcademicYearID,
		"amount_cents":     fs.AmountCents,
		"currency":         fs.Currency,
		"recurrence":       fs.Recurrence,
		"target":           fs.Target,
		"class_id":         nil,
	}
	if fs.DueDay != nil {
		data["due_day"] = *fs.DueDay
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "fees-service", "", fs.TenantID, data)
	if err != nil {
		return fmt.Errorf("fees: build fee structure event: %w", err)
	}
	event.Subject = fs.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishInvoice emits a CloudEvent for an invoice domain event.
func (p *Publisher) PublishInvoice(ctx context.Context, eventType string, inv *domain.Invoice, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
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
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "fees-service", "", inv.TenantID, data)
	if err != nil {
		return fmt.Errorf("fees: build invoice event: %w", err)
	}
	event.Subject = inv.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
