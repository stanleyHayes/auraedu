package domain

import "time"

// Student is the aggregate root of the student service. Every record is tenant-scoped.
type Student struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewStudent constructs a Student, enforcing invariants (tenant + name required).
func NewStudent(id, tenantID, name string) (*Student, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if name == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	return &Student{ID: id, TenantID: tenantID, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
