package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Recurrence enumerates the supported fee billing cadences.
type Recurrence string

const (
	RecurrenceOneTime  Recurrence = "one_time"
	RecurrenceTermly   Recurrence = "termly"
	RecurrenceMonthly  Recurrence = "monthly"
	RecurrenceAnnually Recurrence = "annually"
)

// Target enumerates the population a fee structure applies to.
type Target string

const (
	TargetAllStudents     Target = "all_students"
	TargetSpecificStudent Target = "specific_student"
)

// Status enumerates the lifecycle states of a fee structure.
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// DefaultCurrency is the currency used when none is supplied.
const DefaultCurrency = "GHS"

// FeeStructure is the aggregate root for a billable fee template.
type FeeStructure struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Name           string    `json:"name"`
	AcademicYearID string    `json:"academic_year_id"`
	AmountCents    int       `json:"amount_cents"`
	Currency       string    `json:"currency"`
	Recurrence     string    `json:"recurrence"`
	Target         string    `json:"target"`
	DueDay         *int      `json:"due_day,omitempty"`
	Description    *string   `json:"description,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewFeeStructure constructs a FeeStructure, enforcing invariants.
func NewFeeStructure(tenantID, name, academicYearID, currency, recurrence, target string, amountCents int, dueDay *int, description *string) (*FeeStructure, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if amountCents < 0 {
		return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if strings.TrimSpace(currency) == "" {
		currency = DefaultCurrency
	}
	if !isValidRecurrence(Recurrence(recurrence)) {
		return nil, fmt.Errorf("%w: recurrence must be one_time, termly, monthly or annually", ErrValidation)
	}
	if !isValidTarget(Target(target)) {
		return nil, fmt.Errorf("%w: target must be all_students or specific_student", ErrValidation)
	}
	if dueDay != nil && (*dueDay < 1 || *dueDay > 31) {
		return nil, fmt.Errorf("%w: due_day must be between 1 and 31", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("fees: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &FeeStructure{
		ID:             id.String(),
		TenantID:       tenantID,
		Name:           strings.TrimSpace(name),
		AcademicYearID: strings.TrimSpace(academicYearID),
		AmountCents:    amountCents,
		Currency:       strings.TrimSpace(strings.ToUpper(currency)),
		Recurrence:     string(Recurrence(recurrence)),
		Target:         string(Target(target)),
		DueDay:         dueDay,
		Description:    description,
		Status:         string(StatusActive),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (f FeeStructure) Validate() error {
	if f.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(f.AcademicYearID) == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if f.AmountCents < 0 {
		return fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
	}
	if strings.TrimSpace(f.Currency) == "" {
		return fmt.Errorf("%w: currency is required", ErrValidation)
	}
	if !isValidRecurrence(Recurrence(f.Recurrence)) {
		return fmt.Errorf("%w: recurrence must be one_time, termly, monthly or annually", ErrValidation)
	}
	if !isValidTarget(Target(f.Target)) {
		return fmt.Errorf("%w: target must be all_students or specific_student", ErrValidation)
	}
	if f.DueDay != nil && (*f.DueDay < 1 || *f.DueDay > 31) {
		return fmt.Errorf("%w: due_day must be between 1 and 31", ErrValidation)
	}
	if !isValidFeeStructureStatus(Status(f.Status)) {
		return fmt.Errorf("%w: status must be active or archived", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the fee structure with non-nil patch fields.
func (f *FeeStructure) ApplyUpdate(p FeeStructurePatch) ([]string, error) {
	var changed []string

	if p.Name != nil {
		if strings.TrimSpace(*p.Name) == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		f.Name = strings.TrimSpace(*p.Name)
		changed = append(changed, "name")
	}
	if p.AcademicYearID != nil {
		if strings.TrimSpace(*p.AcademicYearID) == "" {
			return nil, fmt.Errorf("%w: academic_year_id cannot be empty", ErrValidation)
		}
		f.AcademicYearID = strings.TrimSpace(*p.AcademicYearID)
		changed = append(changed, "academic_year_id")
	}
	if p.AmountCents != nil {
		if *p.AmountCents < 0 {
			return nil, fmt.Errorf("%w: amount_cents cannot be negative", ErrValidation)
		}
		f.AmountCents = *p.AmountCents
		changed = append(changed, "amount_cents")
	}
	if p.Currency != nil {
		if strings.TrimSpace(*p.Currency) == "" {
			return nil, fmt.Errorf("%w: currency cannot be empty", ErrValidation)
		}
		f.Currency = strings.TrimSpace(strings.ToUpper(*p.Currency))
		changed = append(changed, "currency")
	}
	if p.Recurrence != nil {
		if !isValidRecurrence(Recurrence(*p.Recurrence)) {
			return nil, fmt.Errorf("%w: recurrence must be one_time, termly, monthly or annually", ErrValidation)
		}
		f.Recurrence = *p.Recurrence
		changed = append(changed, "recurrence")
	}
	if p.Target != nil {
		if !isValidTarget(Target(*p.Target)) {
			return nil, fmt.Errorf("%w: target must be all_students or specific_student", ErrValidation)
		}
		f.Target = *p.Target
		changed = append(changed, "target")
	}
	if p.DueDay != nil {
		if *p.DueDay < 1 || *p.DueDay > 31 {
			return nil, fmt.Errorf("%w: due_day must be between 1 and 31", ErrValidation)
		}
		f.DueDay = p.DueDay
		changed = append(changed, "due_day")
	}
	if p.Description != nil {
		f.Description = p.Description
		changed = append(changed, "description")
	}
	if p.Status != nil {
		if !isValidFeeStructureStatus(Status(*p.Status)) {
			return nil, fmt.Errorf("%w: status must be active or archived", ErrValidation)
		}
		f.Status = *p.Status
		changed = append(changed, "status")
	}

	if len(changed) > 0 {
		f.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// FeeStructurePatch carries optional update fields.
type FeeStructurePatch struct {
	Name           *string
	AcademicYearID *string
	AmountCents    *int
	Currency       *string
	Recurrence     *string
	Target         *string
	DueDay         *int
	Description    *string
	Status         *string
}

func isValidRecurrence(r Recurrence) bool {
	switch r {
	case RecurrenceOneTime, RecurrenceTermly, RecurrenceMonthly, RecurrenceAnnually:
		return true
	}
	return false
}

func isValidTarget(t Target) bool {
	switch t {
	case TargetAllStudents, TargetSpecificStudent:
		return true
	}
	return false
}

func isValidFeeStructureStatus(s Status) bool {
	switch s {
	case StatusActive, StatusArchived:
		return true
	}
	return false
}
