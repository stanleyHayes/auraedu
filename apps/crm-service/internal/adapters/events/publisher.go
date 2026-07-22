// Package events adapts CRM domain events to the platform event bus.
//
//nolint:lll // Event payload fields remain inline so privacy review can see the complete emitted shape.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/eventbus"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

type Publisher struct{ bus *eventbus.Publisher }

var _ ports.EventPublisher = (*Publisher)(nil)

func NewPublisher(bus *eventbus.Publisher) *Publisher { return &Publisher{bus: bus} }

func (p *Publisher) LeadCreated(ctx context.Context, lead *domain.Lead) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent("lead.created.v1", "crm-service", lead.ID, lead.TenantID, map[string]any{"lead_id": lead.ID, "stage": lead.Stage, "source": lead.Source, "campaign_id": lead.CampaignID, "created_at": lead.CreatedAt})
	if err != nil {
		return fmt.Errorf("crm: build lead event: %w", err)
	}
	event.Subject = lead.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) LeadScored(ctx context.Context, tenantID, leadID string, score domain.LeadScore) error {
	if p == nil || p.bus == nil {
		return nil
	}
	positive := make([]string, 0, len(score.PositiveFactors))
	for _, factor := range score.PositiveFactors {
		positive = append(positive, factor.Code)
	}
	negative := make([]string, 0, len(score.NegativeFactors))
	for _, factor := range score.NegativeFactors {
		negative = append(negative, factor.Code)
	}
	event, err := tenancy.NewCloudEvent("lead.scored.v1", "crm-service", uuid.NewString(), tenantID, map[string]any{"lead_id": leadID, "score": score.Score, "confidence": score.Confidence, "positive_factor_codes": positive, "negative_factor_codes": negative, "rule_version": score.RuleVersion, "evaluated_at": score.EvaluatedAt})
	if err != nil {
		return fmt.Errorf("crm: build lead score event: %w", err)
	}
	event.Subject, event.Time = leadID, time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
func (p *Publisher) InteractionCreated(ctx context.Context, item *domain.Interaction) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent("lead.interaction_created.v1", "crm-service", item.ID, item.TenantID, map[string]any{"interaction_id": item.ID, "lead_id": item.LeadID, "channel": item.Channel, "direction": item.Direction, "actor_type": item.ActorType, "occurred_at": item.OccurredAt})
	if err != nil {
		return fmt.Errorf("crm: build interaction event: %w", err)
	}
	event.Subject = item.ID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) FeedbackSubmitted(ctx context.Context, feedback *domain.Feedback) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent("growth.feedback_submitted.v1", "crm-service", feedback.ID, feedback.TenantID, map[string]any{"feedback_id": feedback.ID, "interaction_id": feedback.InteractionID, "ai_run_id": feedback.AIRunID, "feedback_type": feedback.FeedbackType, "rating": feedback.Rating, "submitted_at": feedback.CreatedAt})
	if err != nil {
		return fmt.Errorf("crm: build feedback event: %w", err)
	}
	event.Subject, event.Time = feedback.ID, time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}

func (p *Publisher) CallbackRequested(ctx context.Context, callback *domain.CallbackRequest) error {
	if p == nil || p.bus == nil {
		return nil
	}
	event, err := tenancy.NewCloudEvent("growth.callback_requested.v1", "crm-service", callback.ID, callback.TenantID, map[string]any{
		"callback_request_id": callback.ID,
		"lead_id":             callback.LeadID,
		"preferred_at":        callback.PreferredAt,
		"timezone":            callback.Timezone,
		"locale":              callback.Locale,
		"status":              callback.Status,
		"requested_at":        callback.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("crm: build callback event: %w", err)
	}
	event.Subject, event.Time = callback.ID, time.Now().UTC().Format(time.RFC3339)
	return p.bus.Publish(ctx, event)
}
