package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StaffType enumerates the supported staff classifications.
type StaffType string

const (
	StaffTypeTeacher     StaffType = "teacher"
	StaffTypeNonTeaching StaffType = "non_teaching"
)

// StaffStatus enumerates the lifecycle states of a staff record.
type StaffStatus string

const (
	StatusActive   StaffStatus = "active"
	StatusInactive StaffStatus = "inactive"
)

// Staff is the aggregate root of the staff service. Every record is tenant-scoped.
type Staff struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	StaffType string  `json:"staff_type"`
	Email     *string `json:"email,omitempty"`
	// UserID is the soft link to the identity-service user that owns this staff
	// profile. Cross-service foreign keys are intentionally avoided.
	UserID    *string   `json:"user_id,omitempty"`
	StaffCode string    `json:"staff_code"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Assignment links an active teacher to a class and, optionally, a subject.
// Class and subject identifiers are soft cross-service references by design.
type Assignment struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	StaffID    string    `json:"staff_id"`
	ClassID    string    `json:"class_id"`
	SubjectID  *string   `json:"subject_id,omitempty"`
	Role       *string   `json:"role,omitempty"`
	AssignedAt time.Time `json:"assigned_at"`
}

// NewAssignment constructs a tenant-owned teacher assignment.
func NewAssignment(tenantID, staffID, classID string, subjectID, role *string) (*Assignment, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if _, err := uuid.Parse(staffID); err != nil {
		return nil, ErrValidation
	}
	if _, err := uuid.Parse(classID); err != nil {
		return nil, ErrValidation
	}
	if subjectID != nil {
		value := strings.TrimSpace(*subjectID)
		if value == "" {
			subjectID = nil
		} else if _, err := uuid.Parse(value); err != nil {
			return nil, ErrValidation
		} else {
			subjectID = &value
		}
	}
	if role != nil {
		value := strings.TrimSpace(*role)
		if len(value) > 100 {
			return nil, ErrValidation
		}
		if value == "" {
			role = nil
		} else {
			role = &value
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("staff: generate assignment id: %w", err)
	}
	return &Assignment{
		ID:         id.String(),
		TenantID:   tenantID,
		StaffID:    staffID,
		ClassID:    classID,
		SubjectID:  subjectID,
		Role:       role,
		AssignedAt: time.Now().UTC(),
	}, nil
}

// FullName returns the staff member's display name.
func (s Staff) FullName() string { return strings.TrimSpace(s.FirstName + " " + s.LastName) }

// IsActive reports whether the staff record is active.
func (s Staff) IsActive() bool { return s.Status == string(StatusActive) }

// Activate marks the staff record as active.
func (s *Staff) Activate() {
	s.Status = string(StatusActive)
	s.UpdatedAt = time.Now().UTC()
}

// Deactivate marks the staff record as inactive.
func (s *Staff) Deactivate() {
	s.Status = string(StatusInactive)
	s.UpdatedAt = time.Now().UTC()
}

// NewStaff constructs a Staff, enforcing invariants.
func NewStaff(tenantID, firstName, lastName, staffType string) (*Staff, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return nil, ErrValidation
	}
	if !isValidStaffType(staffType) {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("staff: generate id: %w", err)
	}
	return &Staff{
		ID:        id.String(),
		TenantID:  tenantID,
		FirstName: strings.TrimSpace(firstName),
		LastName:  strings.TrimSpace(lastName),
		StaffType: staffType,
		StaffCode: generateStaffCode(),
		Status:    string(StatusActive),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks that the staff aggregate is well-formed.
func (s Staff) Validate() error {
	if s.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(s.FirstName) == "" || strings.TrimSpace(s.LastName) == "" {
		return ErrValidation
	}
	if !isValidStaffType(s.StaffType) {
		return ErrValidation
	}
	if !isValidStatus(s.Status) {
		return ErrValidation
	}
	if s.Email != nil && *s.Email != "" && !isValidEmail(*s.Email) {
		return ErrValidation
	}
	if s.UserID != nil {
		if _, err := uuid.Parse(*s.UserID); err != nil {
			return ErrValidation
		}
	}
	return nil
}

// ApplyUpdate mutates the staff with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (s *Staff) ApplyUpdate(firstName, lastName, staffType, email, status, userID *string) ([]string, error) {
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
	if staffType != nil {
		if !isValidStaffType(*staffType) {
			return nil, ErrValidation
		}
		s.StaffType = *staffType
		changed = append(changed, "staff_type")
	}
	if email != nil {
		value := strings.TrimSpace(*email)
		if value == "" {
			s.Email = nil
		} else {
			if !isValidEmail(value) {
				return nil, ErrValidation
			}
			s.Email = &value
		}
		changed = append(changed, "email")
	}
	if status != nil {
		if !isValidStatus(*status) {
			return nil, ErrValidation
		}
		s.Status = *status
		changed = append(changed, "status")
	}
	if userID != nil {
		value := strings.TrimSpace(*userID)
		if value == "" {
			s.UserID = nil
		} else {
			if _, err := uuid.Parse(value); err != nil {
				return nil, ErrValidation
			}
			s.UserID = &value
		}
		changed = append(changed, "user_id")
	}
	if len(changed) > 0 {
		s.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

func generateStaffCode() string {
	id, err := uuid.NewV7()
	if err != nil {
		return "STF-" + uuid.NewString()
	}
	return "STF-" + id.String()
}

func isValidStaffType(v string) bool {
	switch StaffType(v) {
	case StaffTypeTeacher, StaffTypeNonTeaching:
		return true
	}
	return false
}

func isValidStatus(v string) bool {
	switch StaffStatus(v) {
	case StatusActive, StatusInactive:
		return true
	}
	return false
}

func isValidEmail(v string) bool {
	// Minimal RFC-ish check; services should validate more strictly at the edge.
	v = strings.TrimSpace(v)
	if len(v) == 0 || len(v) > 254 {
		return false
	}
	at := strings.LastIndex(v, "@")
	if at <= 0 || at == len(v)-1 {
		return false
	}
	return strings.Contains(v[at+1:], ".")
}
