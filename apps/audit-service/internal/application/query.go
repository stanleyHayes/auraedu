package application

import (
	"context"
	"fmt"

	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

// PermRead is the RBAC permission required to read audit logs.
const PermRead = "audit.read"

// Query is the audit read use case (AURA-23.2). The gates
// (authenticated → tenant scope → RBAC) are enforced here, never in HTTP
// handlers (agent_plan §5). Tenant actors see only their own tenant's logs;
// platform super admins without a tenant context read across tenants.
type Query struct {
	repo ports.Repository
}

// NewQuery creates a new audit query use case backed by the given repository.
func NewQuery(repo ports.Repository) *Query {
	return &Query{repo: repo}
}

// ListAuditLogs returns a page of audit logs for the actor's scope, newest-first.
// A platform super admin with no tenant context receives a cross-tenant page;
// any other actor must carry a tenant context that matches their own tenant.
func (q *Query) ListAuditLogs(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	if !actor.Authenticated() {
		return nil, "", domain.ErrForbidden
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" && !actor.PlatformAdmin {
		return nil, "", domain.ErrMissingTenant
	}
	if tenantID != "" && !actor.CanAccessTenant(tenantID) {
		return nil, "", domain.ErrForbidden
	}
	if !actor.Has(PermRead) {
		return nil, "", domain.ErrForbidden
	}
	if cursor != "" {
		if _, err := uuid.Parse(cursor); err != nil {
			return nil, "", fmt.Errorf("%w: cursor is invalid", domain.ErrValidation)
		}
	}

	limit = normalizeLimit(limit)
	// Propagate the actor so platform/db sets app.is_platform_admin for RLS.
	ctx = auth.WithActor(ctx, actor)
	if tenantID == "" {
		return q.repo.ListAll(ctx, limit, cursor)
	}
	return q.repo.List(ctx, tenantID, limit, cursor)
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}
