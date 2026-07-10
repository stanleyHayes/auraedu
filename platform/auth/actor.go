// Package auth provides the shared actor + authorization primitives every service
// uses to enforce the four gates (agent_plan §2 rule 5; spec §8.3):
//
//	authenticated → tenant scope matches → RBAC permission → feature enabled → resource ownership
//
// The API Gateway verifies the JWT and forwards the actor to private services via
// X-Actor-* headers. Because domain/AI services are private (Render pserv), the
// gateway is the only ingress, so these headers are trusted at the service boundary.
package auth

import (
	"net/http"
	"strings"
)

// Header names the gateway sets on requests to downstream services.
const (
	HeaderUserID      = "X-Actor-User"
	HeaderTenant      = "X-Actor-Tenant"
	HeaderRole        = "X-Actor-Role"
	HeaderPermissions = "X-Actor-Permissions"
)

// RolePlatformSuperAdmin is the company/platform super admin (spec §8.1) — may act across tenants.
const RolePlatformSuperAdmin = "platform_super_admin"

// Actor is the authenticated caller, resolved by the gateway from the JWT claims.
type Actor struct {
	UserID        string
	TenantID      string // the tenant (school code) the actor belongs to; empty for platform admins
	Role          string
	Permissions   []string
	PlatformAdmin bool // may act across all tenants
}

// FromHeaders builds an Actor from the gateway-injected headers.
func FromHeaders(h http.Header) Actor {
	var perms []string
	if raw := h.Get(HeaderPermissions); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				perms = append(perms, p)
			}
		}
	}
	role := h.Get(HeaderRole)
	return Actor{
		UserID:        h.Get(HeaderUserID),
		TenantID:      h.Get(HeaderTenant),
		Role:          role,
		Permissions:   perms,
		PlatformAdmin: role == RolePlatformSuperAdmin,
	}
}

// Authenticated reports whether a caller is present (the gateway always sets a user id).
func (a Actor) Authenticated() bool { return a.UserID != "" }

// Has reports whether the actor holds a permission. Platform super admins hold all.
func (a Actor) Has(perm string) bool {
	if a.PlatformAdmin {
		return true
	}
	for _, p := range a.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// CanAccessTenant reports whether the actor may act on the given tenant code:
// a platform admin may access any tenant; everyone else only their own.
func (a Actor) CanAccessTenant(tenantCode string) bool {
	return a.PlatformAdmin || (a.TenantID != "" && a.TenantID == tenantCode)
}
