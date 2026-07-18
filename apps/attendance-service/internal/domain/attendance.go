// Package domain contains the attendance aggregate root and value objects.
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

// IsEmpty reports whether the date is the zero value.
func (d Date) IsEmpty() bool { return d.IsZero() }

// Status enumerates the attendance states for a student on a given date.
type Status string

const (
	StatusPresent Status = "present"
	StatusAbsent  Status = "absent"
	StatusLate    Status = "late"
	StatusExcused Status = "excused"
)

// AttendanceRecord is the aggregate root for a daily attendance mark. Every record
// is tenant-scoped; student_id and academic_year_id are kept as opaque UUIDs to avoid
// coupling this service to student/academic-year lifecycle details.
type AttendanceRecord struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	StudentID      string     `json:"student_id"`
	AcademicYearID string     `json:"academic_year_id"`
	ClassID        *string    `json:"class_id,omitempty"`
	SubjectID      *string    `json:"subject_id,omitempty"`
	Date           Date       `json:"date"`
	Status         string     `json:"status"`
	Reason         *string    `json:"reason,omitempty"`
	MarkedBy       string     `json:"marked_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

// NewAttendanceRecord constructs an AttendanceRecord, enforcing invariants.
func NewAttendanceRecord(tenantID, studentID, academicYearID, dateStr, statusStr, markedBy string, reason *string) (*AttendanceRecord, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(studentID) == "" {
		return nil, fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(academicYearID) == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	date, err := NewDate(dateStr)
	if err != nil || date.IsEmpty() {
		return nil, fmt.Errorf("%w: date must be a valid date (YYYY-MM-DD)", ErrValidation)
	}
	if strings.TrimSpace(markedBy) == "" {
		return nil, fmt.Errorf("%w: marked_by is required", ErrValidation)
	}
	status := Status(strings.TrimSpace(statusStr))
	if !isValidStatus(status) {
		return nil, fmt.Errorf("%w: status must be present, absent, late or excused", ErrValidation)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("attendance: generate id: %w", err)
	}
	now := time.Now().UTC()
	return &AttendanceRecord{
		ID:             id.String(),
		TenantID:       tenantID,
		StudentID:      strings.TrimSpace(studentID),
		AcademicYearID: strings.TrimSpace(academicYearID),
		Date:           date,
		Status:         string(status),
		Reason:         reason,
		MarkedBy:       strings.TrimSpace(markedBy),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Validate checks that the aggregate is well-formed.
func (r AttendanceRecord) Validate() error {
	if r.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(r.StudentID) == "" {
		return fmt.Errorf("%w: student_id is required", ErrValidation)
	}
	if strings.TrimSpace(r.AcademicYearID) == "" {
		return fmt.Errorf("%w: academic_year_id is required", ErrValidation)
	}
	if r.Date.IsEmpty() {
		return fmt.Errorf("%w: date must be a valid date", ErrValidation)
	}
	if strings.TrimSpace(r.MarkedBy) == "" {
		return fmt.Errorf("%w: marked_by is required", ErrValidation)
	}
	if !isValidStatus(Status(r.Status)) {
		return fmt.Errorf("%w: status must be present, absent, late or excused", ErrValidation)
	}
	return nil
}

// ApplyUpdate mutates the record with non-empty patch fields. It returns the names
// of fields that changed, or ErrValidation if a supplied value is invalid.
func (r *AttendanceRecord) ApplyUpdate(status, reason, markedBy *string) ([]string, error) {
	var changed []string

	if status != nil {
		s := Status(strings.TrimSpace(*status))
		if !isValidStatus(s) {
			return nil, fmt.Errorf("%w: status must be present, absent, late or excused", ErrValidation)
		}
		if !isValidTransition(Status(r.Status), s) {
			return nil, fmt.Errorf("%w: cannot transition status from %s to %s", ErrValidation, r.Status, *status)
		}
		r.Status = string(s)
		changed = append(changed, "status")
	}
	if reason != nil {
		r.Reason = reason
		changed = append(changed, "reason")
	}
	if markedBy != nil {
		if strings.TrimSpace(*markedBy) == "" {
			return nil, fmt.Errorf("%w: marked_by cannot be empty", ErrValidation)
		}
		r.MarkedBy = strings.TrimSpace(*markedBy)
		changed = append(changed, "marked_by")
	}

	if len(changed) > 0 {
		r.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func isValidStatus(s Status) bool {
	switch s {
	case StatusPresent, StatusAbsent, StatusLate, StatusExcused:
		return true
	}
	return false
}

// isValidTransition allows any transition between valid statuses for the minimal CRUD
// implementation; the only invalid transition is to an unknown status (handled above).
func isValidTransition(_, to Status) bool {
	return isValidStatus(to)
}
