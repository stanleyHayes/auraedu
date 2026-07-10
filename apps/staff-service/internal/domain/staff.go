package domain

import "time"

// Staff is the aggregate root of the staff service. Every record is tenant-scoped.
type Staff struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewStaff constructs a Staff, enforcing invariants (tenant + name required).
func NewStaff(id, tenantID, name string) (*Staff, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if name == "" {
		return nil, ErrValidation
	}
	now := time.Now().UTC()
	return &Staff{ID: id, TenantID: tenantID, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
