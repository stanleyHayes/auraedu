// Package domain holds the Identity Service business rules (agent_plan §8, spec §8).
package domain

// UserStatus is the lifecycle state of a user account.
type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusInactive UserStatus = "inactive"
	StatusLocked   UserStatus = "locked"
)

// User is an authenticated principal, scoped to a tenant (school). Platform super
// admins have an empty TenantID and act across tenants.
type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	TenantID    string     `json:"tenant_id,omitempty"`
	Role        string     `json:"role"`
	Permissions []string   `json:"permissions,omitempty"`
	Status      UserStatus `json:"status"`
}
