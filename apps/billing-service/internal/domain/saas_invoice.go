// Package domain contains the billing aggregates and value objects.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SaaSInvoiceStatus enumerates the lifecycle states of a SaaS invoice.
type SaaSInvoiceStatus string

const (
	SaaSInvoiceStatusDraft         SaaSInvoiceStatus = "draft"
	SaaSInvoiceStatusOpen          SaaSInvoiceStatus = "open"
	SaaSInvoiceStatusPaid          SaaSInvoiceStatus = "paid"
	SaaSInvoiceStatusUncollectible SaaSInvoiceStatus = "uncollectible"
	SaaSInvoiceStatusVoid          SaaSInvoiceStatus = "void"
)

// SaaSInvoice is the aggregate root for a tenant subscription invoice.
type SaaSInvoice struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	SubscriptionID string     `json:"subscription_id"`
	AmountCents    int        `json:"amount_cents"`
	Status         string     `json:"status"`
	DueDate        *time.Time `json:"due_date,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// NewSaaSInvoice constructs a SaaSInvoice, enforcing invariants.
func NewSaaSInvoice(tenantID, subscriptionID string, amountCents int, dueDate *time.Time) (*SaaSInvoice, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(subscriptionID) == "" {
		return nil, fmt.Errorf("%w: subscription_id is required", ErrValidation)
	}
	if amountCents < 0 {
		return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("billing: generate invoice id: %w", err)
	}
	now := time.Now().UTC()
	return &SaaSInvoice{
		ID:             id.String(),
		TenantID:       strings.TrimSpace(tenantID),
		SubscriptionID: strings.TrimSpace(subscriptionID),
		AmountCents:    amountCents,
		Status:         string(SaaSInvoiceStatusDraft),
		DueDate:        normalizeTimePtr(dueDate),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (i SaaSInvoice) Validate() error {
	if strings.TrimSpace(i.TenantID) == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(i.SubscriptionID) == "" {
		return fmt.Errorf("%w: subscription_id is required", ErrValidation)
	}
	if i.AmountCents < 0 {
		return fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if !isValidSaaSInvoiceStatus(SaaSInvoiceStatus(i.Status)) {
		return fmt.Errorf("%w: status must be draft, open, paid, uncollectible or void", ErrValidation)
	}
	return nil
}

// MarkPaid transitions the invoice to paid.
func (i *SaaSInvoice) MarkPaid() error {
	if i.Status == string(SaaSInvoiceStatusVoid) {
		return fmt.Errorf("%w: cannot pay a void invoice", ErrInvalidStatus)
	}
	if i.Status == string(SaaSInvoiceStatusPaid) {
		return nil
	}
	now := time.Now().UTC()
	i.Status = string(SaaSInvoiceStatusPaid)
	i.PaidAt = &now
	i.UpdatedAt = now
	return nil
}

// MarkVoid transitions the invoice to void.
func (i *SaaSInvoice) MarkVoid() error {
	if i.Status == string(SaaSInvoiceStatusPaid) {
		return fmt.Errorf("%w: cannot void a paid invoice", ErrInvalidStatus)
	}
	if i.Status == string(SaaSInvoiceStatusVoid) {
		return nil
	}
	i.Status = string(SaaSInvoiceStatusVoid)
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// SaaSInvoicePatch carries optional update fields.
type SaaSInvoicePatch struct {
	AmountCents *int
	Status      *string
	DueDate     *time.Time
	PaidAt      *time.Time
}

// ApplyUpdate mutates the invoice with non-nil patch fields.
func (i *SaaSInvoice) ApplyUpdate(patch SaaSInvoicePatch) ([]string, error) {
	var changed []string

	if patch.AmountCents != nil {
		if *patch.AmountCents < 0 {
			return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
		}
		i.AmountCents = *patch.AmountCents
		changed = append(changed, "amount_cents")
	}
	if patch.Status != nil {
		if !isValidSaaSInvoiceStatus(SaaSInvoiceStatus(*patch.Status)) {
			return nil, fmt.Errorf("%w: status must be draft, open, paid, uncollectible or void", ErrValidation)
		}
		i.Status = *patch.Status
		changed = append(changed, "status")
	}
	if patch.DueDate != nil {
		i.DueDate = normalizeTimePtr(patch.DueDate)
		changed = append(changed, "due_date")
	}
	if patch.PaidAt != nil {
		i.PaidAt = normalizeTimePtr(patch.PaidAt)
		changed = append(changed, "paid_at")
	}

	if len(changed) > 0 {
		i.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidSaaSInvoiceStatus(s SaaSInvoiceStatus) bool {
	switch s {
	case SaaSInvoiceStatusDraft, SaaSInvoiceStatusOpen, SaaSInvoiceStatusPaid, SaaSInvoiceStatusUncollectible, SaaSInvoiceStatusVoid:
		return true
	}
	return false
}
