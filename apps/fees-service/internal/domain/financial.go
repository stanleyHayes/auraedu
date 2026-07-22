package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CurrencyBalance is an immutable projection of a learner's invoice ledger.
type CurrencyBalance struct {
	Currency           string `json:"currency"`
	TotalInvoicedCents int    `json:"total_invoiced_cents"`
	TotalPaidCents     int    `json:"total_paid_cents"`
	OutstandingCents   int    `json:"outstanding_cents"`
}

// Balance groups a learner's totals by currency so unlike monies are never summed.
type Balance struct {
	StudentID string            `json:"student_id"`
	Totals    []CurrencyBalance `json:"totals"`
}

// Receipt is the immutable evidence produced when a confirmed provider payment
// is reconciled against a learner invoice.
type Receipt struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	InvoiceID         string    `json:"invoice_id"`
	StudentID         string    `json:"student_id"`
	PaymentID         string    `json:"payment_id"`
	AmountCents       int       `json:"amount_cents"`
	AppliedCents      int       `json:"applied_cents"`
	OverpaymentCents  int       `json:"overpayment_cents"`
	Currency          string    `json:"currency"`
	ProviderReference *string   `json:"provider_reference,omitempty"`
	IssuedAt          time.Time `json:"issued_at"`
}

// NewReceipt constructs immutable reconciliation evidence.
func NewReceipt(
	tenantID, invoiceID, studentID, paymentID, currency string,
	amountCents, appliedCents int,
	providerReference *string,
	issuedAt time.Time,
) (*Receipt, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(invoiceID) == "" || strings.TrimSpace(studentID) == "" || strings.TrimSpace(paymentID) == "" {
		return nil, fmt.Errorf("%w: invoice_id, student_id and payment_id are required", ErrValidation)
	}
	if amountCents <= 0 || appliedCents < 0 || appliedCents > amountCents {
		return nil, fmt.Errorf("%w: payment amounts are invalid", ErrValidation)
	}
	if strings.TrimSpace(currency) == "" {
		return nil, fmt.Errorf("%w: currency is required", ErrValidation)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("fees: generate receipt id: %w", err)
	}
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	return &Receipt{
		ID:                id.String(),
		TenantID:          tenantID,
		InvoiceID:         strings.TrimSpace(invoiceID),
		StudentID:         strings.TrimSpace(studentID),
		PaymentID:         strings.TrimSpace(paymentID),
		AmountCents:       amountCents,
		AppliedCents:      appliedCents,
		OverpaymentCents:  amountCents - appliedCents,
		Currency:          strings.ToUpper(strings.TrimSpace(currency)),
		ProviderReference: providerReference,
		IssuedAt:          issuedAt.UTC(),
	}, nil
}
