package domain

import "time"

// Academic is the aggregate root of the academic service. Every record is tenant-scoped.
type Academic struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewAcademic constructs a Academic, enforcing invariants (tenant + name required).
func NewAcademic(id, tenantID, name string) (*Academic, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if name == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	return &Academic{ID: id, TenantID: tenantID, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
