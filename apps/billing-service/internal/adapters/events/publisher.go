package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/billing-service/internal/domain"
	"github.com/auraedu/billing-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
)

// Publisher adapts the platform eventbus to the billing service EventPublisher port.
type Publisher struct {
	bus *eventbus.Publisher
}

var _ ports.EventPublisher = (*Publisher)(nil)

// NewPublisher wraps a platform eventbus publisher.
func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

// PublishSubscription emits a CloudEvent for a subscription domain event.
func (p *Publisher) PublishSubscription(ctx context.Context, eventType string, s *domain.Subscription, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"subscription_id": s.ID,
		"tenant_id":       s.TenantID,
		"plan_id":         s.PlanID,
		"status":          s.Status,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "billing-service", "", s.TenantID, data)
	if err != nil {
		return fmt.Errorf("billing: build subscription event: %w", err)
	}
	event.Subject = s.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishPlan emits a CloudEvent for a plan domain event.
func (p *Publisher) PublishPlan(ctx context.Context, eventType string, plan *domain.Plan, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"plan_id":     plan.ID,
		"plan_code":   plan.Code,
		"name":        plan.Name,
		"price_cents": plan.PriceCents,
		"currency":    plan.Currency,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "billing-service", "", "", data)
	if err != nil {
		return fmt.Errorf("billing: build plan event: %w", err)
	}
	event.Subject = plan.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishInvoice emits a CloudEvent for a SaaS invoice domain event.
func (p *Publisher) PublishInvoice(ctx context.Context, eventType string, i *domain.SaaSInvoice, meta map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	data := map[string]any{
		"invoice_id":      i.ID,
		"tenant_id":       i.TenantID,
		"subscription_id": i.SubscriptionID,
		"amount_cents":    i.AmountCents,
		"status":          i.Status,
	}
	for k, v := range meta {
		data[k] = v
	}
	event, err := tenancy.NewCloudEvent(eventType, "billing-service", "", i.TenantID, data)
	if err != nil {
		return fmt.Errorf("billing: build invoice event: %w", err)
	}
	event.Subject = i.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
