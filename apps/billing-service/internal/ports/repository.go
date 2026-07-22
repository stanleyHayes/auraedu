// Package ports defines the inbound and outbound ports for the billing service.
package ports

import (
	"context"
	"encoding/json"

	"github.com/auraedu/billing-service/internal/domain"
)

const (
	BillingMutationSubscriptionCreate = "subscription_create"
	BillingMutationSubscriptionUpdate = "subscription_update"
	BillingMutationInvoiceCreate      = "invoice_create"
)

type LifecycleEvent struct {
	EventType string
	TenantID  string
	Payload   map[string]any
}

type SubscriptionLifecycleRepository interface {
	CommitSubscriptionLifecycle(context.Context, string, string, *domain.Subscription, []LifecycleEvent) error
}

type InvoiceLifecycleRepository interface {
	CommitInvoiceLifecycle(context.Context, string, string, *domain.SaaSInvoice, []LifecycleEvent) error
}

type OutboxEvent struct {
	ID, TenantID, EventType string
	Payload                 json.RawMessage
}

type OutboxRepository interface {
	ClaimPendingBillingEvents(context.Context, int) ([]OutboxEvent, error)
	MarkBillingEventPublished(context.Context, string) error
	MarkBillingEventFailed(context.Context, string, string) error
}

func SubscriptionEventData(s *domain.Subscription, meta map[string]any) map[string]any {
	data := map[string]any{
		"subscription_id": s.ID,
		"tenant_id":       s.TenantID,
		"plan_id":         s.PlanID,
		"status":          s.Status,
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

func PlanEventData(plan *domain.Plan, meta map[string]any) map[string]any {
	data := map[string]any{
		"plan_id":     plan.ID,
		"plan_code":   plan.Code,
		"name":        plan.Name,
		"price_cents": plan.PriceCents,
		"currency":    plan.Currency,
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

func InvoiceEventData(invoice *domain.SaaSInvoice, meta map[string]any) map[string]any {
	data := map[string]any{
		"invoice_id":      invoice.ID,
		"tenant_id":       invoice.TenantID,
		"subscription_id": invoice.SubscriptionID,
		"amount_cents":    invoice.AmountCents,
		"status":          invoice.Status,
	}
	for key, value := range meta {
		data[key] = value
	}
	return data
}

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
