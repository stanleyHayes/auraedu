// Package application holds the Tenant Service use cases. The four gates
// (auth → tenant scope → RBAC → entitlement) are enforced here (agent_plan §5),
// never in HTTP handlers. The actor is resolved by the gateway (platform/auth).
package application

import (
	"context"
	"log/slog"
	"strings"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

// FeatureManage is the RBAC permission required to change a tenant's feature flags (spec §8.2).
const FeatureManage = "features.manage"

type Service struct {
	repo ports.Repository
	pub  ports.EventPublisher
}

type Option func(*Service)

func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _, _ string, _ map[string]any) error {
	return nil
}

func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ListTenants is a platform-wide operation — platform super admins only (spec §8.1).
func (s *Service) ListTenants(actor auth.Actor) ([]domain.Tenant, error) {
	if !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListTenants(), nil
}

// CreateTenant provisions a new school. Platform super admins only.
func (s *Service) CreateTenant(actor auth.Actor, t domain.Tenant) (domain.Tenant, error) {
	if !actor.PlatformAdmin {
		return domain.Tenant{}, domain.ErrForbidden
	}
	if err := t.Validate(); err != nil {
		return domain.Tenant{}, err
	}
	t.Code = strings.ToLower(strings.TrimSpace(t.Code))
	t.Status = defaultStatus(t.Status, "active")
	if err := s.repo.CreateTenant(t); err != nil {
		return domain.Tenant{}, err
	}
	if err := s.pub.Publish(context.Background(), "tenant.created.v1", t.Code, map[string]any{
		"tenant_code": t.Code,
		"name":        t.Name,
		"plan":        t.Plan,
	}); err != nil {
		slog.Default().ErrorContext(context.Background(), "failed to publish tenant.created event", "err", err)
	}
	return t, nil
}

// GetTenant returns a tenant record; the actor must belong to it (or be a platform admin).
func (s *Service) GetTenant(actor auth.Actor, code string) (domain.Tenant, error) {
	if !actor.CanAccessTenant(code) {
		return domain.Tenant{}, domain.ErrForbidden
	}
	return s.repo.GetTenant(code)
}

// Branding is PUBLIC by design: a school's logo/colors theme the login page before
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
	flag, err := s.repo.SetFeature(code, key, enabled)
	if err != nil {
		return domain.FeatureFlag{}, err
	}
	eventType := "tenant.feature_disabled.v1"
	if enabled {
		eventType = "tenant.feature_enabled.v1"
	}
	if err := s.pub.Publish(context.Background(), eventType, code, map[string]any{
		"feature_key": key,
		"is_enabled":  enabled,
		"plan":        flag.PlanRequired,
	}); err != nil {
		slog.Default().ErrorContext(context.Background(), "failed to publish tenant.feature event", "err", err)
	}
	return flag, nil
}

func defaultStatus(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
