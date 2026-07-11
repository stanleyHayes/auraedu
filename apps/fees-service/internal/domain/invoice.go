package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InvoiceStatus enumerates the lifecycle states of an invoice.
type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusPending   InvoiceStatus = "pending"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusOverdue   InvoiceStatus = "overdue"
	InvoiceStatusCancelled InvoiceStatus = "cancelled" //nolint:misspell // domain uses British spelling for status
)

// Invoice is the aggregate root for a student fee invoice.
type Invoice struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	StudentID      string    `json:"student_id"`
	FeeStructureID string    `json:"fee_structure_id"`
	AmountCents    int       `json:"amount_cents"`
	BalanceCents   int       `json:"balance_cents"`
	Status         string    `json:"status"`
	DueDate        Date      `json:"due_date,omitempty"`
	IssuedAt       time.Time `json:"issued_at"`
	Notes          *string   `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewInvoice constructs an Invoice, enforcing invariants.
func NewInvoice(tenantID, studentID, feeStructureID string, amountCents, balanceCents int, dueDate Date, notes *string) (*Invoice, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(feeStructureID) == "" {
		return nil, fmt.Errorf("%w: fee_structure_id is required", ErrValidation)
	}
	if amountCents < 0 {
		return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if balanceCents < 0 {
		balanceCents = amountCents
	}
	if balanceCents > amountCents {
		return nil, fmt.Errorf("%w: balance_cents cannot exceed amount_cents", ErrValidation)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("fees: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &Invoice{
		ID:             id.String(),
		TenantID:       tenantID,
		StudentID:      strings.TrimSpace(studentID),
		FeeStructureID: strings.TrimSpace(feeStructureID),
		AmountCents:    amountCents,
		BalanceCents:   balanceCents,
		Status:         string(InvoiceStatusPending),
		DueDate:        dueDate,
		IssuedAt:       now,
		Notes:          notes,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (i Invoice) Validate() error {
	if i.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(i.StudentID) == "" {
		return fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(i.FeeStructureID) == "" {
		return fmt.Errorf("%w: fee_structure_id is required", ErrValidation)
	}
	if i.AmountCents < 0 {
		return fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if i.BalanceCents < 0 || i.BalanceCents > i.AmountCents {
		return fmt.Errorf("%w: balance_cents must be between 0 and amount_cents", ErrValidation)
	}
	if !isValidInvoiceStatus(InvoiceStatus(i.Status)) {
		//nolint:misspell // domain uses British spelling for status
		return fmt.Errorf(
			"%w: status must be draft, pending, paid, overdue or cancelled", ErrValidation,
		)
	}
	return nil
}

// InvoicePatch carries optional update fields.
type InvoicePatch struct {
	AmountCents  *int
	BalanceCents *int
	Status       *string
	DueDate      *Date
	Notes        *string
}

// ApplyUpdate mutates the invoice with non-nil patch fields.
// When the status transitions to paid, balance_cents is zeroed unless explicitly supplied.
func (i *Invoice) ApplyUpdate(p InvoicePatch) ([]string, error) {
	var changed []string

	if p.AmountCents != nil {
		if *p.AmountCents < 0 {
			return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
		}
		i.AmountCents = *p.AmountCents
		changed = append(changed, "amount_cents")
		if p.BalanceCents == nil && i.BalanceCents > i.AmountCents {
			i.BalanceCents = i.AmountCents
			changed = append(changed, "balance_cents")
		}
	}
	if p.BalanceCents != nil {
		if *p.BalanceCents < 0 || *p.BalanceCents > i.AmountCents {
			return nil, fmt.Errorf("%w: balance_cents must be between 0 and amount_cents", ErrValidation)
		}
		i.BalanceCents = *p.BalanceCents
		changed = append(changed, "balance_cents")
	}
	if p.Status != nil {
		if !isValidInvoiceStatus(InvoiceStatus(*p.Status)) {
			//nolint:misspell // domain uses British spelling for status
			return nil, fmt.Errorf(
				"%w: status must be draft, pending, paid, overdue or cancelled", ErrValidation,
			)
		}
		oldStatus := i.Status
		i.Status = *p.Status
		changed = append(changed, "status")
		if i.Status == string(InvoiceStatusPaid) && p.BalanceCents == nil {
			i.BalanceCents = 0
			if !contains(changed, "balance_cents") {
				changed = append(changed, "balance_cents")
			}
		}
		_ = oldStatus // available for transition logging if needed
	}
	if p.DueDate != nil {
		i.DueDate = *p.DueDate
		changed = append(changed, "due_date")
	}
	if p.Notes != nil {
		i.Notes = p.Notes
		changed = append(changed, "notes")
	}

	if len(changed) > 0 {
		i.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isValidInvoiceStatus(s InvoiceStatus) bool {
	switch s {
	case InvoiceStatusDraft, InvoiceStatusPending, InvoiceStatusPaid, InvoiceStatusOverdue, InvoiceStatusCancelled:
		return true
	}
	return false
}
