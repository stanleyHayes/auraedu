// Package domain contains the billing aggregates and value objects.
//
//nolint:misspell // British spelling "cancelled" is intentional for the billing domain.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SubscriptionStatus enumerates the lifecycle states of a subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusTrialing  SubscriptionStatus = "trialing"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

// Subscription is the aggregate root for a tenant's SaaS subscription.
type Subscription struct {
	ID                 string     `json:"id"`
	TenantID           string     `json:"tenant_id"`
	PlanID             string     `json:"plan_id"`
	Status             string     `json:"status"`
	CurrentPeriodStart time.Time  `json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `json:"current_period_end"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// NewSubscription constructs a Subscription, enforcing invariants.
func NewSubscription(tenantID, planID string, periodStart, periodEnd time.Time, status string, trialEndsAt *time.Time) (*Subscription, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(planID) == "" {
		return nil, fmt.Errorf("%w: plan_id is required", ErrValidation)
	}
	if status == "" {
		status = string(SubscriptionStatusActive)
	}
	if !isValidSubscriptionStatus(SubscriptionStatus(status)) {
		return nil, fmt.Errorf("%w: status must be trialing, active, past_due or cancelled", ErrValidation)
	}
	if periodEnd.Before(periodStart) {
		return nil, fmt.Errorf("%w: current_period_end must be after current_period_start", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("billing: generate subscription id: %w", err)
	}
	now := time.Now().UTC()
	return &Subscription{
		ID:                 id.String(),
		TenantID:           strings.TrimSpace(tenantID),
		PlanID:             strings.TrimSpace(planID),
		Status:             status,
		CurrentPeriodStart: periodStart.UTC(),
		CurrentPeriodEnd:   periodEnd.UTC(),
		TrialEndsAt:        normalizeTimePtr(trialEndsAt),
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (s Subscription) Validate() error {
	if strings.TrimSpace(s.TenantID) == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(s.PlanID) == "" {
		return fmt.Errorf("%w: plan_id is required", ErrValidation)
	}
	if !isValidSubscriptionStatus(SubscriptionStatus(s.Status)) {
		return fmt.Errorf("%w: status must be trialing, active, past_due or cancelled", ErrValidation)
	}
	if s.CurrentPeriodEnd.Before(s.CurrentPeriodStart) {
		return fmt.Errorf("%w: current_period_end must be after current_period_start", ErrValidation)
	}
	return nil
}

// ChangePlan updates the subscription to a new plan and extends the current period.
func (s *Subscription) ChangePlan(newPlanID string) error {
	if strings.TrimSpace(newPlanID) == "" {
		return fmt.Errorf("%w: plan_id is required", ErrValidation)
	}
	if s.Status == string(SubscriptionStatusCancelled) {
		return fmt.Errorf("%w: cannot change plan on cancelled subscription", ErrInvalidStatus)
	}
	s.PlanID = strings.TrimSpace(newPlanID)
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// SubscriptionPatch carries optional update fields.
type SubscriptionPatch struct {
	Status             *string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	TrialEndsAt        *time.Time
	CancelledAt        *time.Time
}

// ApplyUpdate mutates the subscription with non-nil patch fields.
func (s *Subscription) ApplyUpdate(patch SubscriptionPatch) ([]string, error) {
	var changed []string

	if patch.Status != nil {
		if !isValidSubscriptionStatus(SubscriptionStatus(*patch.Status)) {
			return nil, fmt.Errorf("%w: status must be trialing, active, past_due or cancelled", ErrValidation)
		}
		s.Status = *patch.Status
		changed = append(changed, "status")
	}
	if patch.CurrentPeriodStart != nil {
		s.CurrentPeriodStart = patch.CurrentPeriodStart.UTC()
		changed = append(changed, "current_period_start")
	}
	if patch.CurrentPeriodEnd != nil {
		s.CurrentPeriodEnd = patch.CurrentPeriodEnd.UTC()
		changed = append(changed, "current_period_end")
	}
	if patch.TrialEndsAt != nil {
		s.TrialEndsAt = normalizeTimePtr(patch.TrialEndsAt)
		changed = append(changed, "trial_ends_at")
	}
	if patch.CancelledAt != nil {
		s.CancelledAt = normalizeTimePtr(patch.CancelledAt)
		changed = append(changed, "cancelled_at")
	}

	if s.CurrentPeriodEnd.Before(s.CurrentPeriodStart) {
		return nil, fmt.Errorf("%w: current_period_end must be after current_period_start", ErrValidation)
	}

	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidSubscriptionStatus(s SubscriptionStatus) bool {
	switch s {
	case SubscriptionStatusTrialing, SubscriptionStatusActive, SubscriptionStatusPastDue, SubscriptionStatusCancelled:
		return true
	}
	return false
}

func normalizeTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
