// Package domain holds the Identity Service business rules (agent_plan §8, spec §8).
// Users authenticate with a password; a successful login yields a JWT whose claims
// the gateway turns into an Actor (tenant + role + permissions).
package domain

// User is an authenticated principal, scoped to a tenant (school). Platform super
// admins have an empty TenantID and act across tenants.
type User struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	TenantID    string   `json:"tenant_id,omitempty"`
	Role        string   `json:"role"`
	Permissions []string `json:"-"` // never serialised to clients
}
