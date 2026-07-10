package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StudentStatus enumerates the lifecycle states of a student record.
type StudentStatus string

const (
	StatusActive    StudentStatus = "active"
	StatusWithdrawn StudentStatus = "withdrawn"
	StatusGraduated StudentStatus = "graduated"
)

// Gender enumerates the supported gender values.
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

// Student is the aggregate root of the student service. Every record is tenant-scoped.
type Student struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	StudentCode string    `json:"student_code"`
	DateOfBirth *string   `json:"date_of_birth,omitempty"`
	Gender      *string   `json:"gender,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FullName returns the student's display name.
func (s Student) FullName() string { return strings.TrimSpace(s.FirstName + " " + s.LastName) }

// NewStudent constructs a Student, enforcing invariants.
func NewStudent(tenantID, firstName, lastName string) (*Student, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("student: generate id: %w", err)
	}
	return &Student{
		ID:          id.String(),
		TenantID:    tenantID,
		FirstName:   strings.TrimSpace(firstName),
		LastName:    strings.TrimSpace(lastName),
		StudentCode: generateStudentCode(),
		Status:      string(StatusActive),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Validate checks that the student aggregate is well-formed.
func (s Student) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(s.FirstName) == "" || strings.TrimSpace(s.LastName) == "" {
		return ErrValidation
	}
	if !isValidStatus(s.Status) {
		return ErrValidation
	}
	if s.Gender != nil && !isValidGender(*s.Gender) {
		return ErrValidation
	}
	if s.DateOfBirth != nil && !isValidDate(*s.DateOfBirth) {
		return ErrValidation
	}
	return nil
}

// ApplyUpdate mutates the student with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (s *Student) ApplyUpdate(firstName, lastName, status *string) ([]string, error) {
	var changed []string
	if firstName != nil {
		if strings.TrimSpace(*firstName) == "" {
			return nil, ErrValidation
		}
		s.FirstName = strings.TrimSpace(*firstName)
		changed = append(changed, "first_name")
	}
	if lastName != nil {
		if strings.TrimSpace(*lastName) == "" {
			return nil, ErrValidation
		}
		s.LastName = strings.TrimSpace(*lastName)
		changed = append(changed, "last_name")
	}
	if status != nil {
		if !isValidStatus(*status) {
			return nil, ErrValidation
		}
		s.Status = *status
		changed = append(changed, "status")
	}
	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func generateStudentCode() string {
	id, err := uuid.NewV7()
	if err != nil {
		return "STU-" + uuid.NewString()
	}
	return "STU-" + id.String()
}

func isValidStatus(v string) bool {
	switch StudentStatus(v) {
	case StatusActive, StatusWithdrawn, StatusGraduated:
		return true
	}
	return false
}

func isValidGender(v string) bool {
	switch Gender(v) {
	case GenderMale, GenderFemale, GenderOther:
		return true
	}
	return false
}

func isValidDate(v string) bool {
	_, err := time.Parse(time.DateOnly, v)
	return err == nil
}
