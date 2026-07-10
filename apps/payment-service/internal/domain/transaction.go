package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TransactionType enumerates the direction of a transaction.
type TransactionType string

const (
	TransactionTypeDebit  TransactionType = "debit"
	TransactionTypeCredit TransactionType = "credit"
	TransactionTypeRefund TransactionType = "refund"
)

// TransactionStatus enumerates the lifecycle states of a transaction.
type TransactionStatus string

const (
	TransactionStatusPending TransactionStatus = "pending"
	TransactionStatusSuccess TransactionStatus = "success"
	TransactionStatusFailed  TransactionStatus = "failed"
)

// Transaction is the aggregate root for a ledger entry against a payment.
type Transaction struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	PaymentID   string    `json:"payment_id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	AmountCents int       `json:"amount_cents"`
	Reference   string    `json:"reference"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewTransaction constructs a Transaction, enforcing invariants.
func NewTransaction(tenantID, paymentID, txType, status, reference string, amountCents int) (*Transaction, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(paymentID) == "" {
		return nil, fmt.Errorf("%w: payment_id is required", ErrValidation)
	}
	if strings.TrimSpace(reference) == "" {
		return nil, fmt.Errorf("%w: reference is required", ErrValidation)
	}
	if amountCents < 0 {
		return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if !isValidTransactionType(TransactionType(txType)) {
		return nil, fmt.Errorf("%w: type must be debit, credit or refund", ErrValidation)
	}
	if !isValidTransactionStatus(TransactionStatus(status)) {
		return nil, fmt.Errorf("%w: status must be pending, success or failed", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("payments: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &Transaction{
		ID:          id.String(),
		TenantID:    tenantID,
		PaymentID:   strings.TrimSpace(paymentID),
		Type:        strings.TrimSpace(strings.ToLower(txType)),
		Status:      strings.TrimSpace(strings.ToLower(status)),
		AmountCents: amountCents,
		Reference:   strings.TrimSpace(reference),
		CreatedAt:   now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (t Transaction) Validate() error {
	if t.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(t.PaymentID) == "" {
		return fmt.Errorf("%w: payment_id is required", ErrValidation)
	}
	if strings.TrimSpace(t.Reference) == "" {
		return fmt.Errorf("%w: reference is required", ErrValidation)
	}
	if t.AmountCents < 0 {
		return fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if !isValidTransactionType(TransactionType(t.Type)) {
		return fmt.Errorf("%w: type must be debit, credit or refund", ErrValidation)
	}
	if !isValidTransactionStatus(TransactionStatus(t.Status)) {
		return fmt.Errorf("%w: status must be pending, success or failed", ErrValidation)
	}
	return nil
}

func isValidTransactionType(t TransactionType) bool {
	switch t {
	case TransactionTypeDebit, TransactionTypeCredit, TransactionTypeRefund:
		return true
	}
	return false
}

func isValidTransactionStatus(s TransactionStatus) bool {
	switch s {
	case TransactionStatusPending, TransactionStatusSuccess, TransactionStatusFailed:
		return true
	}
	return false
}
