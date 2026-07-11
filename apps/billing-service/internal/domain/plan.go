// Package domain contains the billing aggregates and value objects.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BillingInterval enumerates the supported plan cadences.
type BillingInterval string

const (
	BillingIntervalMonthly BillingInterval = "monthly"
	BillingIntervalYearly  BillingInterval = "yearly"
)

// PlanStatus enumerates the lifecycle states of a plan.
type PlanStatus string

const (
	PlanStatusActive   PlanStatus = "active"
	PlanStatusArchived PlanStatus = "archived"
)

// DefaultPlanCurrency is the currency used when none is supplied.
const DefaultPlanCurrency = "GHS"

// Plan is the aggregate root for a SaaS pricing plan.
type Plan struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Code            string    `json:"code"`
	Description     *string   `json:"description,omitempty"`
	PriceCents      int       `json:"price_cents"`
	Currency        string    `json:"currency"`
	BillingInterval string    `json:"billing_interval"`
	Features        []string  `json:"features"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// NewPlan constructs a Plan, enforcing invariants.
func NewPlan(name, code, currency, billingInterval string, priceCents int, description *string, features []string) (*Plan, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("%w: code is required", ErrValidation)
	}
	if priceCents < 0 {
		return nil, fmt.Errorf("%w: price_cents cannot be negative", ErrValidation)
	}
	if strings.TrimSpace(currency) == "" {
		currency = DefaultPlanCurrency
	}
	if !isValidBillingInterval(BillingInterval(billingInterval)) {
		return nil, fmt.Errorf("%w: billing_interval must be monthly or yearly", ErrValidation)
	}
	if features == nil {
		features = []string{}
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("billing: generate plan id: %w", err)
	}
	now := time.Now().UTC()
	return &Plan{
		ID:              id.String(),
		Name:            strings.TrimSpace(name),
		Code:            strings.ToLower(strings.TrimSpace(code)),
		Description:     description,
		PriceCents:      priceCents,
		Currency:        strings.ToUpper(strings.TrimSpace(currency)),
		BillingInterval: string(BillingInterval(billingInterval)),
		Features:        features,
		Status:          string(PlanStatusActive),
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (p Plan) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(p.Code) == "" {
		return fmt.Errorf("%w: code is required", ErrValidation)
	}
	if p.PriceCents < 0 {
		return fmt.Errorf("%w: price_cents cannot be negative", ErrValidation)
	}
	if strings.TrimSpace(p.Currency) == "" {
		return fmt.Errorf("%w: currency is required", ErrValidation)
	}
	if !isValidBillingInterval(BillingInterval(p.BillingInterval)) {
		return fmt.Errorf("%w: billing_interval must be monthly or yearly", ErrValidation)
	}
	if !isValidPlanStatus(PlanStatus(p.Status)) {
		return fmt.Errorf("%w: status must be active or archived", ErrValidation)
	}
	return nil
}

// HasFeature reports whether the plan includes a feature key.
func (p Plan) HasFeature(key string) bool {
	for _, f := range p.Features {
		if f == key {
			return true
		}
	}
	return false
}

// PlanPatch carries optional update fields.
type PlanPatch struct {
	Name            *string
	Code            *string
	Description     *string
	PriceCents      *int
	Currency        *string
	BillingInterval *string
	Features        *[]string
	Status          *string
}

// ApplyUpdate mutates the plan with non-nil patch fields.
func (p *Plan) ApplyUpdate(patch PlanPatch) ([]string, error) {
	var changed []string

	if patch.Name != nil {
		if strings.TrimSpace(*patch.Name) == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		p.Name = strings.TrimSpace(*patch.Name)
		changed = append(changed, "name")
	}
	if patch.Code != nil {
		if strings.TrimSpace(*patch.Code) == "" {
			return nil, fmt.Errorf("%w: code cannot be empty", ErrValidation)
		}
		p.Code = strings.ToLower(strings.TrimSpace(*patch.Code))
		changed = append(changed, "code")
	}
	if patch.Description != nil {
		p.Description = patch.Description
		changed = append(changed, "description")
	}
	if patch.PriceCents != nil {
		if *patch.PriceCents < 0 {
			return nil, fmt.Errorf("%w: price_cents cannot be negative", ErrValidation)
		}
		p.PriceCents = *patch.PriceCents
		changed = append(changed, "price_cents")
	}
	if patch.Currency != nil {
		if strings.TrimSpace(*patch.Currency) == "" {
			return nil, fmt.Errorf("%w: currency cannot be empty", ErrValidation)
		}
		p.Currency = strings.ToUpper(strings.TrimSpace(*patch.Currency))
		changed = append(changed, "currency")
	}
	if patch.BillingInterval != nil {
		if !isValidBillingInterval(BillingInterval(*patch.BillingInterval)) {
			return nil, fmt.Errorf("%w: billing_interval must be monthly or yearly", ErrValidation)
		}
		p.BillingInterval = *patch.BillingInterval
		changed = append(changed, "billing_interval")
	}
	if patch.Features != nil {
		p.Features = *patch.Features
		if p.Features == nil {
			p.Features = []string{}
		}
		changed = append(changed, "features")
	}
	if patch.Status != nil {
		if !isValidPlanStatus(PlanStatus(*patch.Status)) {
			return nil, fmt.Errorf("%w: status must be active or archived", ErrValidation)
		}
		p.Status = *patch.Status
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		p.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidBillingInterval(i BillingInterval) bool {
	switch i {
	case BillingIntervalMonthly, BillingIntervalYearly:
		return true
	}
	return false
}

func isValidPlanStatus(s PlanStatus) bool {
	switch s {
	case PlanStatusActive, PlanStatusArchived:
		return true
	}
	return false
}
