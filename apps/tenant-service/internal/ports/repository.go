package ports

import "github.com/auraedu/tenant-service/internal/domain"

// Repository stores tenants and their feature flags. The memory adapter seeds it
// today; the Postgres adapter (with RLS) is the next story (AURA-5.x / platform/db).
type Repository interface {
	ListTenants() []domain.Tenant
	GetTenant(code string) (domain.Tenant, error)
	Features(code string) ([]domain.FeatureFlag, error)
	SetFeature(code, key string, enabled bool) (domain.FeatureFlag, error)
}
