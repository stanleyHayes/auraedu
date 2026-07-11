// Package ports defines the fees-service boundary interfaces.
package ports

import (
	"context"

	"github.com/auraedu/fees-service/internal/domain"
)

// EventPublisher emits fees domain events.
type EventPublisher interface {
	// PublishFeeStructure sends a fee structure domain event.
	PublishFeeStructure(ctx context.Context, eventType string, fs *domain.FeeStructure, meta map[string]any) error
	// PublishInvoice sends an invoice domain event.
	PublishInvoice(ctx context.Context, eventType string, inv *domain.Invoice, meta map[string]any) error
}
