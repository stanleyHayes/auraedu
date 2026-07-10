package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PaymentProvider enumerates the supported payment gateway integrations.
type PaymentProvider string

const (
	ProviderPaystack    PaymentProvider = "paystack"
	ProviderFlutterwave PaymentProvider = "flutterwave"
	ProviderMock        PaymentProvider = "mock"
)

// PaymentStatus enumerates the lifecycle states of a payment.
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusSuccess    PaymentStatus = "success"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusCancelled  PaymentStatus = "cancelled"
)

// DefaultCurrency is the currency used when none is supplied.
const DefaultCurrency = "GHS"

// Payment is the aggregate root for a payment transaction initiated against an invoice.
type Payment struct {
	ID                string          `json:"id"`
	TenantID          string          `json:"tenant_id"`
	InvoiceID         string          `json:"invoice_id"`
	AmountCents       int             `json:"amount_cents"`
	Currency          string          `json:"currency"`
	Provider          string          `json:"provider"`
	ProviderReference *string         `json:"provider_reference,omitempty"`
	Status            string          `json:"status"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	InitiatedAt       time.Time       `json:"initiated_at"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// NewPayment constructs a Payment, enforcing invariants.
func NewPayment(tenantID, invoiceID, provider, currency string, amountCents int, metadata json.RawMessage) (*Payment, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(invoiceID) == "" {
		return nil, fmt.Errorf("%w: invoice_id is required", ErrValidation)
	}
	if amountCents <= 0 {
		return nil, fmt.Errorf("%w: amount_cents must be positive", ErrValidation)
	}
	if strings.TrimSpace(currency) == "" {
		currency = DefaultCurrency
	}
	if !isValidProvider(PaymentProvider(provider)) {
		return nil, fmt.Errorf("%w: provider must be paystack, flutterwave or mock", ErrValidation)
	}
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("payments: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &Payment{
		ID:          id.String(),
		TenantID:    tenantID,
		InvoiceID:   strings.TrimSpace(invoiceID),
		AmountCents: amountCents,
		Currency:    strings.TrimSpace(strings.ToUpper(currency)),
		Provider:    strings.TrimSpace(strings.ToLower(provider)),
		Status:      string(PaymentStatusPending),
		Metadata:    metadata,
		InitiatedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (p Payment) Validate() error {
	if p.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(p.InvoiceID) == "" {
		return fmt.Errorf("%w: invoice_id is required", ErrValidation)
	}
	if p.AmountCents <= 0 {
		return fmt.Errorf("%w: amount_cents must be positive", ErrValidation)
	}
	if strings.TrimSpace(p.Currency) == "" {
		return fmt.Errorf("%w: currency is required", ErrValidation)
	}
	if !isValidProvider(PaymentProvider(p.Provider)) {
		return fmt.Errorf("%w: provider must be paystack, flutterwave or mock", ErrValidation)
	}
	if !isValidPaymentStatus(PaymentStatus(p.Status)) {
		return fmt.Errorf("%w: status must be pending, processing, success, failed or cancelled", ErrValidation)
	}
	return nil
}

// PaymentPatch carries optional update fields.
type PaymentPatch struct {
	AmountCents       *int
	Currency          *string
	Provider          *string
	ProviderReference *string
	Status            *string
	Metadata          json.RawMessage
	CompletedAt       *time.Time
}

// ApplyUpdate mutates the payment with non-nil patch fields.
func (p *Payment) ApplyUpdate(patch PaymentPatch) ([]string, error) {
	var changed []string

	if patch.AmountCents != nil {
		if *patch.AmountCents <= 0 {
			return nil, fmt.Errorf("%w: amount_cents must be positive", ErrValidation)
		}
		p.AmountCents = *patch.AmountCents
		changed = append(changed, "amount_cents")
	}
	if patch.Currency != nil {
		if strings.TrimSpace(*patch.Currency) == "" {
			return nil, fmt.Errorf("%w: currency cannot be empty", ErrValidation)
		}
		p.Currency = strings.TrimSpace(strings.ToUpper(*patch.Currency))
		changed = append(changed, "currency")
	}
	if patch.Provider != nil {
		if !isValidProvider(PaymentProvider(*patch.Provider)) {
			return nil, fmt.Errorf("%w: provider must be paystack, flutterwave or mock", ErrValidation)
		}
		p.Provider = strings.TrimSpace(strings.ToLower(*patch.Provider))
		changed = append(changed, "provider")
	}
	if patch.ProviderReference != nil {
		p.ProviderReference = patch.ProviderReference
		changed = append(changed, "provider_reference")
	}
	if patch.Status != nil {
		if !isValidPaymentStatus(PaymentStatus(*patch.Status)) {
			return nil, fmt.Errorf("%w: status must be pending, processing, success, failed or cancelled", ErrValidation)
		}
		p.Status = *patch.Status
		changed = append(changed, "status")
	}
	if patch.Metadata != nil {
		p.Metadata = patch.Metadata
		changed = append(changed, "metadata")
	}
	if patch.CompletedAt != nil {
		p.CompletedAt = patch.CompletedAt
		changed = append(changed, "completed_at")
	}

	if len(changed) > 0 {
		p.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidProvider(p PaymentProvider) bool {
	switch p {
	case ProviderPaystack, ProviderFlutterwave, ProviderMock:
		return true
	}
	return false
}

func isValidPaymentStatus(s PaymentStatus) bool {
	switch s {
	case PaymentStatusPending, PaymentStatusProcessing, PaymentStatusSuccess, PaymentStatusFailed, PaymentStatusCancelled:
		return true
	}
	return false
}
