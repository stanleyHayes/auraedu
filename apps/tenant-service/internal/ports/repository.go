// Package ports defines the tenant service repository boundary.
package ports

import (
	"context"

	"github.com/auraedu/tenant-service/internal/domain"
)

// Repository stores tenants and their feature flags. Adapters implement the
// context-aware interface so that platform/db can set app.tenant_id for RLS.
type Repository interface {
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	GetTenant(ctx context.Context, code string) (domain.Tenant, error)
	CreateTenant(ctx context.Context, t domain.Tenant) error
	UpdateTenant(ctx context.Context, code string, upd domain.TenantUpdate) (domain.Tenant, error)
	DeleteTenant(ctx context.Context, code string) error
	ResolveTenant(ctx context.Context, domain, subdomain string) (domain.Tenant, error)
	Features(ctx context.Context, code string) ([]domain.FeatureFlag, error)
	SetFeature(ctx context.Context, code, key string, enabled bool) (domain.FeatureFlag, error)
}
