package domain

import (
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Guardian represents a parent or legal guardian of one or more students.
type Guardian struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Relationship string    `json:"relationship"`
	Phone        *string   `json:"phone,omitempty"`
	Email        *string   `json:"email,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FullName returns the guardian's display name.
func (g Guardian) FullName() string { return strings.TrimSpace(g.FirstName + " " + g.LastName) }

// NewGuardian constructs a Guardian, enforcing invariants.
func NewGuardian(tenantID, firstName, lastName, relationship string) (*Guardian, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" || strings.TrimSpace(relationship) == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return &Guardian{
		ID:           id.String(),
		TenantID:     tenantID,
		FirstName:    strings.TrimSpace(firstName),
		LastName:     strings.TrimSpace(lastName),
		Relationship: strings.TrimSpace(relationship),
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Validate checks that the guardian aggregate is well-formed.
func (g Guardian) Validate() error {
	if g.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(g.FirstName) == "" || strings.TrimSpace(g.LastName) == "" || strings.TrimSpace(g.Relationship) == "" {
		return ErrValidation
	}
	if g.Email != nil && *g.Email != "" {
		if _, err := mail.ParseAddress(*g.Email); err != nil {
			return ErrValidation
		}
	}
	return nil
}

// ApplyUpdate mutates the guardian with non-empty patch fields.
func (g *Guardian) ApplyUpdate(firstName, lastName, relationship, phone, email *string) ([]string, error) {
	var changed []string
	if firstName != nil {
		if strings.TrimSpace(*firstName) == "" {
			return nil, ErrValidation
		}
		g.FirstName = strings.TrimSpace(*firstName)
		changed = append(changed, "first_name")
	}
	if lastName != nil {
		if strings.TrimSpace(*lastName) == "" {
			return nil, ErrValidation
		}
		g.LastName = strings.TrimSpace(*lastName)
		changed = append(changed, "last_name")
	}
	if relationship != nil {
		if strings.TrimSpace(*relationship) == "" {
			return nil, ErrValidation
		}
		g.Relationship = strings.TrimSpace(*relationship)
		changed = append(changed, "relationship")
	}
	if phone != nil {
		g.Phone = phone
		changed = append(changed, "phone")
	}
	if email != nil {
		if *email != "" {
			if _, err := mail.ParseAddress(*email); err != nil {
				return nil, ErrValidation
			}
		}
		g.Email = email
		changed = append(changed, "email")
	}
	if len(changed) > 0 {
		g.UpdatedAt = time.Now().UTC()
	}
	return changed, nil
}

// StudentGuardian links a student to a guardian within a tenant.
type StudentGuardian struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	StudentID    string    `json:"student_id"`
	GuardianID   string    `json:"guardian_id"`
	Relationship *string   `json:"relationship,omitempty"`
	IsPrimary    bool      `json:"is_primary"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewStudentGuardian creates a link record after validating required IDs.
func NewStudentGuardian(tenantID, studentID, guardianID string, relationship *string, isPrimary bool) (*StudentGuardian, error) {
	if tenantID == "" || studentID == "" || guardianID == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return &StudentGuardian{
		ID:           id.String(),
		TenantID:     tenantID,
		StudentID:    studentID,
		GuardianID:   guardianID,
		Relationship: relationship,
		IsPrimary:    isPrimary,
		CreatedAt:    now,
	}, nil
}
