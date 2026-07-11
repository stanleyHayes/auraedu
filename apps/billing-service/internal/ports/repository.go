package ports

import (
	"context"

	"github.com/auraedu/billing-service/internal/domain"
)

// PlanRepository persists Plan aggregates.
type PlanRepository interface {
	Create(ctx context.Context, p *domain.Plan) error
	GetByID(ctx context.Context, id string) (*domain.Plan, error)
	GetByCode(ctx context.Context, code string) (*domain.Plan, error)
	List(ctx context.Context, filter PlanFilter) ([]*domain.Plan, string, error)
	Update(ctx context.Context, p *domain.Plan) error
	Delete(ctx context.Context, id string) error
}

// SubscriptionRepository persists Subscription aggregates. Implementations MUST
// scope every query by tenantID (defense-in-depth with Postgres RLS).
type SubscriptionRepository interface {
	Create(ctx context.Context, tenantID string, s *domain.Subscription) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error)
	List(ctx context.Context, tenantID string, filter SubscriptionFilter) ([]*domain.Subscription, string, error)
	Update(ctx context.Context, tenantID string, s *domain.Subscription) error
	Delete(ctx context.Context, tenantID, id string) error
}

// SaaSInvoiceRepository persists SaaSInvoice aggregates. Implementations MUST
// scope every query by tenantID (defense-in-depth with Postgres RLS).
type SaaSInvoiceRepository interface {
	Create(ctx context.Context, tenantID string, i *domain.SaaSInvoice) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.SaaSInvoice, error)
	List(ctx context.Context, tenantID string, filter SaaSInvoiceFilter) ([]*domain.SaaSInvoice, string, error)
	Update(ctx context.Context, tenantID string, i *domain.SaaSInvoice) error
	Delete(ctx context.Context, tenantID, id string) error
}

// PlanFilter carries cursor pagination and optional equality filters.
type PlanFilter struct {
	Limit  int
	Cursor string
	Status string
}

// SubscriptionFilter carries cursor pagination and optional equality filters.
type SubscriptionFilter struct {
	Limit  int
	Cursor string
	Status string
	PlanID string
}

// SaaSInvoiceFilter carries cursor pagination and optional equality filters.
type SaaSInvoiceFilter struct {
	Limit          int
	Cursor         string
	Status         string
	SubscriptionID string
}
