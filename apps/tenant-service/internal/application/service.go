// Package application holds the Tenant Service use cases. The four gates
// (auth → tenant scope → RBAC → entitlement) are enforced here (agent_plan §5),
// never in HTTP handlers. The actor is resolved by the gateway (platform/auth).
package application

import (
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

// FeatureManage is the RBAC permission required to change a tenant's feature flags (spec §8.2).
const FeatureManage = "features.manage"

type Service struct {
	repo ports.Repository
}

func NewService(repo ports.Repository) *Service { return &Service{repo: repo} }

// ListTenants is a platform-wide operation — platform super admins only (spec §8.1).
func (s *Service) ListTenants(actor auth.Actor) ([]domain.Tenant, error) {
	if !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListTenants(), nil
}

// GetTenant returns a tenant record; the actor must belong to it (or be a platform admin).
func (s *Service) GetTenant(actor auth.Actor, code string) (domain.Tenant, error) {
	if !actor.CanAccessTenant(code) {
		return domain.Tenant{}, domain.ErrForbidden
	}
	return s.repo.GetTenant(code)
}

// Branding is PUBLIC by design: a school's logo/colours theme the login page before
// authentication (BRAND.md §5, DESIGN_SYSTEM §17). It exposes no sensitive data.
func (s *Service) Branding(code string) (domain.Branding, error) {
	t, err := s.repo.GetTenant(code)
	if err != nil {
		return domain.Branding{}, err
	}
	return t.Branding, nil
}

// Features returns the feature snapshot for a tenant. Requires a resolved tenant and
// that the actor belongs to it (or is a platform admin) — prevents cross-tenant reads.
func (s *Service) Features(actor auth.Actor, code string) ([]domain.FeatureFlag, error) {
	if code == "" {
		return nil, domain.ErrNoTenant
	}
	if !actor.CanAccessTenant(code) {
		return nil, domain.ErrForbidden
	}
	return s.repo.Features(code)
}

// SetFeature enables/disables a feature for a tenant. Enforces all four gates:
// RBAC (features.manage) + tenant scope + entitlement (can't enable above the plan, spec §3.3).
func (s *Service) SetFeature(actor auth.Actor, code, key string, enabled bool) (domain.FeatureFlag, error) {
	if code == "" {
		return domain.FeatureFlag{}, domain.ErrNoTenant
	}
	if !actor.Has(FeatureManage) || !actor.CanAccessTenant(code) {
		return domain.FeatureFlag{}, domain.ErrForbidden
	}
	if enabled {
		tenant, err := s.repo.GetTenant(code)
		if err != nil {
			return domain.FeatureFlag{}, err
		}
		plan, known := domain.FeaturePlan(key)
		if !known {
			return domain.FeatureFlag{}, domain.ErrValidation
		}
		if !domain.PlanAllows(tenant.Plan, plan) {
			return domain.FeatureFlag{}, domain.ErrEntitlement
		}
	}
	// TODO(AURA-5.3): emit tenant.feature_enabled/disabled via platform/eventbus.
	return s.repo.SetFeature(code, key, enabled)
}
