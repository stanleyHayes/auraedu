// Package events adapts the platform eventbus to the billing service publisher port.
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
	data := ports.SubscriptionEventData(s, meta)
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
	data := ports.PlanEventData(plan, meta)
	rawTenantID, exists := meta["tenant_id"]
	if !exists {
		return fmt.Errorf("billing: build plan event: tenant_id is required")
	}
	tenantID, ok := rawTenantID.(string)
	if !ok {
		return fmt.Errorf("billing: build plan event: tenant_id must be a string")
	}
	if tenantID == "" {
		return fmt.Errorf("billing: build plan event: tenant_id is required")
	}
	event, err := tenancy.NewCloudEvent(eventType, "billing-service", "", tenantID, data)
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
	data := ports.InvoiceEventData(i, meta)
	event, err := tenancy.NewCloudEvent(eventType, "billing-service", "", i.TenantID, data)
	if err != nil {
		return fmt.Errorf("billing: build invoice event: %w", err)
	}
	event.Subject = i.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

// PublishWithID publishes a durable outbox event using its stable ID for both
// CloudEvent identity and JetStream deduplication.
func (p *Publisher) PublishWithID(ctx context.Context, eventID, eventType, tenantID string, data map[string]any) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent(eventType, "billing-service", eventID, tenantID, data)
	if err != nil {
		return fmt.Errorf("billing: build outbox event: %w", err)
	}
	for _, key := range []string{"subscription_id", "invoice_id", "plan_id"} {
		if subject, ok := data[key].(string); ok && subject != "" {
			event.Subject = subject
			break
		}
	}
	event.IdempotencyKey = eventID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
