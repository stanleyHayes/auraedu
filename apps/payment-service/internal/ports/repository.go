// Package ports defines the payment service repository boundaries.
package ports

import (
	"context"

	"github.com/auraedu/payment-service/internal/domain"
)

// PaymentRepository persists Payment aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type PaymentRepository interface {
	Create(ctx context.Context, tenantID string, p *domain.Payment) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Payment, error)
	GetByProviderReference(ctx context.Context, tenantID, provider, reference string) (*domain.Payment, error)
	List(ctx context.Context, tenantID string, filter PaymentFilter) ([]*domain.Payment, string, error)
	Update(ctx context.Context, tenantID string, p *domain.Payment) error
	Delete(ctx context.Context, tenantID, id string) error
}

// TransactionRepository persists Transaction aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type TransactionRepository interface {
	Create(ctx context.Context, tenantID string, t *domain.Transaction) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Transaction, error)
	ListByPayment(ctx context.Context, tenantID, paymentID string, filter TransactionFilter) ([]*domain.Transaction, string, error)
}

// WebhookEventRepository persists WebhookEvent aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type WebhookEventRepository interface {
	Create(ctx context.Context, tenantID string, w *domain.WebhookEvent) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.WebhookEvent, error)
	Update(ctx context.Context, tenantID string, w *domain.WebhookEvent) error
	List(ctx context.Context, tenantID string, filter WebhookEventFilter) ([]*domain.WebhookEvent, string, error)
}

// PaymentFilter carries cursor pagination and optional equality filters.
type PaymentFilter struct {
	Limit     int
	Cursor    string
	Status    string
	Provider  string
	InvoiceID string
}

// TransactionFilter carries cursor pagination.
type TransactionFilter struct {
	Limit  int
	Cursor string
}

// WebhookEventFilter carries cursor pagination and optional equality filters.
type WebhookEventFilter struct {
	Limit     int
	Cursor    string
	Provider  string
	EventType string
}
