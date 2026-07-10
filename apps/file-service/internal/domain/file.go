package domain

import "time"

// File is the aggregate root of the file service. Every record is tenant-scoped.
type File struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewFile constructs a File, enforcing invariants (tenant + name required).
func NewFile(id, tenantID, name string) (*File, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if name == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	return &File{ID: id, TenantID: tenantID, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
