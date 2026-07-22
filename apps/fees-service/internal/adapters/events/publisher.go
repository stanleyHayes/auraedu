// Package events publishes fees-service domain events.
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

func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "fees-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("fees: build outbox event: %w", err)
	}
	event.Type = eventType
	event.IdempotencyKey = eventID
	if subject, ok := data["invoice_id"].(string); ok {
		event.Subject = subject
	}
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishFeeStructure emits a CloudEvent for a fee structure domain event.
func (p *Publisher) PublishFeeStructure(ctx context.Context, eventType string, fs *domain.FeeStructure, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := ports.FeeStructureEventData(fs, meta)
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
	data := ports.InvoiceEventData(eventType, inv, meta)
	event, err := tenancy.NewCloudEvent(eventType, "fees-service", "", inv.TenantID, data)
	if err != nil {
		return fmt.Errorf("fees: build invoice event: %w", err)
	}
	event.Subject = inv.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
