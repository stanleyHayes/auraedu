// Package domain contains the academic-service aggregate roots and value objects.
package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Date is a calendar-date value that marshals to and from "YYYY-MM-DD" JSON strings.
type Date struct{ time.Time }

// NewDate parses a Date from a string. It returns a zero Date for an empty string.
func NewDate(v string) (Date, error) {
	if v == "" {
		return Date{}, nil
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return Date{}, err
	}
	return Date{t}, nil
}

// String returns the date in YYYY-MM-DD format.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Format(time.DateOnly)
}

// MarshalJSON implements json.Marshaler.
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		d.Time = time.Time{}
		return nil
	}
	parsed, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

func (d Date) IsEmpty() bool { return d.IsZero() }

// AcademicYearStatus enumerates the lifecycle states of an academic year.
type AcademicYearStatus string

const (
	StatusActive   AcademicYearStatus = "active"
	StatusArchived AcademicYearStatus = "archived"
)

// AcademicYear is the aggregate root for academic years. Every record is tenant-scoped.
type AcademicYear struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	StartDate Date      `json:"start_date"`
	EndDate   Date      `json:"end_date"`
	Status    string    `json:"status"`
	IsCurrent bool      `json:"is_current"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewAcademicYear constructs an AcademicYear, enforcing invariants.
func NewAcademicYear(tenantID, name, code, startDate, endDate string, isCurrent bool) (*AcademicYear, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(code) == "" {
		code = generateYearCode(startDate)
	}
	start, err := NewDate(startDate)
	if err != nil {
		return nil, fmt.Errorf("%w: start_date must be a valid date (YYYY-MM-DD)", ErrValidation)
	}
	end, err := NewDate(endDate)
	if err != nil {
		return nil, fmt.Errorf("%w: end_date must be a valid date (YYYY-MM-DD)", ErrValidation)
	}
	if end.Before(start.Time) || end.Equal(start.Time) {
		return nil, fmt.Errorf("%w: end_date must be after start_date", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("academic: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &AcademicYear{
		ID:        id.String(),
		TenantID:  tenantID,
		Name:      strings.TrimSpace(name),
		Code:      strings.TrimSpace(code),
		StartDate: start,
		EndDate:   end,
		Status:    string(StatusActive),
		IsCurrent: isCurrent,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (y AcademicYear) Validate() error {
	if y.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(y.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(y.Code) == "" {
		return fmt.Errorf("%w: code is required", ErrValidation)
	}
	if y.StartDate.IsEmpty() || y.EndDate.IsEmpty() {
		return fmt.Errorf("%w: start_date and end_date must be valid dates", ErrValidation)
	}
	if y.EndDate.Before(y.StartDate.Time) || y.EndDate.Equal(y.StartDate.Time) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrValidation)
	}
	if !isValidYearStatus(y.Status) {
		return fmt.Errorf("%w: status must be active or archived", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the academic year with non-empty patch fields.
// It returns the names of fields that changed, or ErrValidation if a supplied value is invalid.
func (y *AcademicYear) ApplyUpdate(name, code, startDate, endDate, status *string, isCurrent *bool) ([]string, error) {
	var changed []string

	if name != nil {
		if strings.TrimSpace(*name) == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		y.Name = strings.TrimSpace(*name)
		changed = append(changed, "name")
	}
	if code != nil {
		if strings.TrimSpace(*code) == "" {
			return nil, fmt.Errorf("%w: code cannot be empty", ErrValidation)
		}
		y.Code = strings.TrimSpace(*code)
		changed = append(changed, "code")
	}
	if startDate != nil {
		start, err := NewDate(*startDate)
		if err != nil {
			return nil, fmt.Errorf("%w: start_date must be a valid date", ErrValidation)
		}
		y.StartDate = start
		changed = append(changed, "start_date")
	}
	if endDate != nil {
		end, err := NewDate(*endDate)
		if err != nil {
			return nil, fmt.Errorf("%w: end_date must be a valid date", ErrValidation)
		}
		y.EndDate = end
		changed = append(changed, "end_date")
	}
	if status != nil {
		if !isValidYearStatus(*status) {
			return nil, fmt.Errorf("%w: status must be active or archived", ErrValidation)
		}
		y.Status = *status
		changed = append(changed, "status")
	}
	if isCurrent != nil {
		y.IsCurrent = *isCurrent
		changed = append(changed, "is_current")
	}

	if y.EndDate.Before(y.StartDate.Time) || y.EndDate.Equal(y.StartDate.Time) {
		return nil, fmt.Errorf("%w: end_date must be after start_date", ErrValidation)
	}

	if len(changed) > 0 {
		y.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// Term is a minimal aggregate representing a term within an academic year.
type Term struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	AcademicYearID string    `json:"academic_year_id"`
	Name           string    `json:"name"`
	StartDate      Date      `json:"start_date"`
	EndDate        Date      `json:"end_date"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewTerm constructs a Term, enforcing invariants.
func NewTerm(tenantID, academicYearID, name, startDate, endDate string) (*Term, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if academicYearID == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrValidation)
	}
	start, err := NewDate(startDate)
	if err != nil {
		return nil, fmt.Errorf("%w: start_date must be a valid date (YYYY-MM-DD)", ErrValidation)
	}
	end, err := NewDate(endDate)
	if err != nil {
		return nil, fmt.Errorf("%w: end_date must be a valid date (YYYY-MM-DD)", ErrValidation)
	}
	if end.Before(start.Time) || end.Equal(start.Time) {
		return nil, fmt.Errorf("%w: end_date must be after start_date", ErrValidation)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("academic: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &Term{
		ID:             id.String(),
		TenantID:       tenantID,
		AcademicYearID: academicYearID,
		Name:           strings.TrimSpace(name),
		StartDate:      start,
		EndDate:        end,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the term aggregate is well-formed.
func (t Term) Validate() error {
	if t.TenantID == "" {
		return ErrMissingTenant
	}
	if t.AcademicYearID == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if t.StartDate.IsEmpty() || t.EndDate.IsEmpty() {
		return fmt.Errorf("%w: start_date and end_date must be valid dates", ErrValidation)
	}
	if t.EndDate.Before(t.StartDate.Time) || t.EndDate.Equal(t.StartDate.Time) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrValidation)
	}
	return nil
}

func isValidYearStatus(v string) bool {
	switch AcademicYearStatus(v) {
	case StatusActive, StatusArchived:
		return true
	}
	return false
}

func generateYearCode(startDate string) string {
	if d, err := NewDate(startDate); err == nil && !d.IsEmpty() {
		return fmt.Sprintf("AY-%d", d.Year())
	}
	return "AY-" + uuid.NewString()[:8]
}
