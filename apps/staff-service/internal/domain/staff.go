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
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	StaffType string    `json:"staff_type"`
	Email     *string   `json:"email,omitempty"`
	StaffCode string    `json:"staff_code"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	return nil
}

// ApplyUpdate mutates the staff with non-empty patch fields. It returns the
// names of fields that changed, or ErrValidation if a supplied value is invalid.
func (s *Staff) ApplyUpdate(firstName, lastName, staffType, email, status *string) ([]string, error) {
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
		if *email != "" && !isValidEmail(*email) {
			return nil, ErrValidation
		}
		s.Email = email
		changed = append(changed, "email")
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
