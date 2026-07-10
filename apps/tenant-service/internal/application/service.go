// Package application holds the Tenant Service use cases. RBAC + plan-gating +
// event emission are enforced here (agent_plan §5), not in HTTP handlers.
package application

import (
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

type Service struct {
	repo ports.Repository
}

func NewService(repo ports.Repository) *Service { return &Service{repo: repo} }

func (s *Service) ListTenants() []domain.Tenant { return s.repo.ListTenants() }

func (s *Service) GetTenant(code string) (domain.Tenant, error) { return s.repo.GetTenant(code) }

// Branding returns just the tenant's visual identity (loaded by web/mobile at startup).
func (s *Service) Branding(code string) (domain.Branding, error) {
	t, err := s.repo.GetTenant(code)
	if err != nil {
		return domain.Branding{}, err
	}
	return t.Branding, nil
}

// Features returns the feature-flag snapshot for a tenant (spec §3). Requires tenant context.
func (s *Service) Features(code string) ([]domain.FeatureFlag, error) {
	if code == "" {
		return nil, domain.ErrNoTenant
	}
	return s.repo.Features(code)
}

// SetFeature enables/disables a feature for a tenant (platform super admin, spec §3.3).
func (s *Service) SetFeature(code, key string, enabled bool) (domain.FeatureFlag, error) {
	if code == "" {
		return domain.FeatureFlag{}, domain.ErrNoTenant
	}
	// TODO(AURA-5.3): emit tenant.feature_enabled/disabled via platform/eventbus; enforce plan gating.
	return s.repo.SetFeature(code, key, enabled)
}
