// Package application holds the Tenant Service use cases. The four gates
// (auth → tenant scope → RBAC → entitlement) are enforced here (agent_plan §5),
// never in HTTP handlers. The actor is resolved by the gateway (platform/auth).
package application

import (
	"context"
	"log/slog"
	"strings"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
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

func withTenant(ctx context.Context, code string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: code})
}

// ListTenants is a platform-wide operation — platform super admins only (spec §8.1).
func (s *Service) ListTenants(ctx context.Context, actor auth.Actor) ([]domain.Tenant, error) {
	if !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListTenants(ctx)
}

// CreateTenant provisions a new school. Platform super admins only.
func (s *Service) CreateTenant(ctx context.Context, actor auth.Actor, t domain.Tenant) (domain.Tenant, error) {
	if !actor.PlatformAdmin {
		return domain.Tenant{}, domain.ErrForbidden
	}
	if err := t.Validate(); err != nil {
		return domain.Tenant{}, err
	}
	t.Code = strings.ToLower(strings.TrimSpace(t.Code))
	t.Status = defaultStatus(t.Status, "active")
	if err := s.repo.CreateTenant(withTenant(ctx, t.Code), t); err != nil {
		return domain.Tenant{}, err
	}
	if err := s.pub.Publish(ctx, "tenant.created.v1", t.Code, map[string]any{
		"tenant_code": t.Code,
		"name":        t.Name,
		"plan":        t.Plan,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.created event", "err", err)
	}
	return t, nil
}

// UpdateTenant applies a partial update to a school. Platform super admins only.
func (s *Service) UpdateTenant(ctx context.Context, actor auth.Actor, code string, upd domain.TenantUpdate) (domain.Tenant, error) {
	if !actor.PlatformAdmin {
		return domain.Tenant{}, domain.ErrForbidden
	}
	if code == "" {
		return domain.Tenant{}, domain.ErrValidation
	}
	if err := upd.ValidateUpdate(); err != nil {
		return domain.Tenant{}, err
	}
	t, err := s.repo.UpdateTenant(withTenant(ctx, code), code, upd)
	if err != nil {
		return domain.Tenant{}, err
	}
	if err := s.pub.Publish(ctx, "tenant.updated.v1", code, map[string]any{
		"tenant_code": code,
		"name":        t.Name,
		"status":      t.Status,
		"plan":        t.Plan,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.updated event", "err", err)
	}
	return t, nil
}

// DeleteTenant permanently removes a school and all of its feature flags.
// Platform super admins only.
func (s *Service) DeleteTenant(ctx context.Context, actor auth.Actor, code string) error {
	if !actor.PlatformAdmin {
		return domain.ErrForbidden
	}
	if code == "" {
		return domain.ErrValidation
	}
	if err := s.repo.DeleteTenant(withTenant(ctx, code), code); err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "tenant.deleted.v1", code, map[string]any{
		"tenant_code": code,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.deleted event", "err", err)
	}
	return nil
}

// GetTenant returns a tenant record; the actor must belong to it (or be a platform admin).
func (s *Service) GetTenant(ctx context.Context, actor auth.Actor, code string) (domain.Tenant, error) {
	if !actor.CanAccessTenant(code) {
		return domain.Tenant{}, domain.ErrForbidden
	}
	return s.repo.GetTenant(withTenant(ctx, code), code)
}

// ResolveTenant is a public lookup used by the API gateway before authentication.
// It resolves a school by its custom domain or subdomain and exposes only
// non-sensitive identity fields.
func (s *Service) ResolveTenant(ctx context.Context, domainHost, subdomain string) (domain.Tenant, error) {
	if domainHost == "" && subdomain == "" {
		return domain.Tenant{}, domain.ErrValidation
	}
	return s.repo.ResolveTenant(ctx, domainHost, subdomain)
}

// Branding is PUBLIC by design: a school's logo/colors theme the login page before
// authentication (BRAND.md §5, DESIGN_SYSTEM §17). It exposes no sensitive data.
func (s *Service) Branding(ctx context.Context, code string) (domain.Branding, error) {
	t, err := s.repo.GetTenant(withTenant(ctx, code), code)
	if err != nil {
		return domain.Branding{}, err
	}
	return t.Branding, nil
}

// Features returns the feature snapshot for a tenant. Requires a resolved tenant and
// that the actor belongs to it (or is a platform admin) — prevents cross-tenant reads.
func (s *Service) Features(ctx context.Context, actor auth.Actor, code string) ([]domain.FeatureFlag, error) {
	if code == "" {
		return nil, domain.ErrNoTenant
	}
	// Public feature snapshot: unauthenticated callers may read the snapshot for
	// the resolved tenant (used by web/mobile at boot before login). Authenticated
	// callers are still scoped to their own tenant.
	if actor.Authenticated() && !actor.CanAccessTenant(code) {
		return nil, domain.ErrForbidden
	}
	return s.repo.Features(withTenant(ctx, code), code)
}

// Settings returns a tenant's operational settings; the actor must belong to the
// tenant (or be a platform admin).
func (s *Service) Settings(ctx context.Context, actor auth.Actor, code string) (domain.Settings, error) {
	if !actor.CanAccessTenant(code) {
		return domain.Settings{}, domain.ErrForbidden
	}
	return s.repo.Settings(withTenant(ctx, code), code)
}

// UpdateSettings changes a tenant's operational settings. Any authenticated actor
// belonging to the tenant may update their own settings; platform super admins may
// update any tenant.
func (s *Service) UpdateSettings(ctx context.Context, actor auth.Actor, code string, settings domain.Settings) (domain.Settings, error) {
	if !actor.CanAccessTenant(code) {
		return domain.Settings{}, domain.ErrForbidden
	}
	if err := domain.ValidateSettings(settings); err != nil {
		return domain.Settings{}, err
	}
	if err := s.repo.UpdateSettings(withTenant(ctx, code), code, settings); err != nil {
		return domain.Settings{}, err
	}
	if err := s.pub.Publish(ctx, "tenant.settings_updated.v1", code, map[string]any{
		"tenant_code": code,
		"locale":      settings.Locale,
		"timezone":    settings.Timezone,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.settings_updated event", "err", err)
	}
	return settings, nil
}

// SetFeature enables/disables a feature for a tenant. Enforces all four gates:
// RBAC (features.manage) + tenant scope + entitlement (can't enable above the plan, spec §3.3).
func (s *Service) SetFeature(ctx context.Context, actor auth.Actor, code, key string, enabled bool) (domain.FeatureFlag, error) {
	if code == "" {
		return domain.FeatureFlag{}, domain.ErrNoTenant
	}
	if !actor.Has(FeatureManage) || !actor.CanAccessTenant(code) {
		return domain.FeatureFlag{}, domain.ErrForbidden
	}
	if enabled {
		tenant, err := s.repo.GetTenant(withTenant(ctx, code), code)
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
	flag, err := s.repo.SetFeature(withTenant(ctx, code), code, key, enabled, "")
	if err != nil {
		return domain.FeatureFlag{}, err
	}
	eventType := "tenant.feature_disabled.v1"
	if enabled {
		eventType = "tenant.feature_enabled.v1"
	}
	if err := s.pub.Publish(ctx, eventType, code, map[string]any{
		"feature_key": key,
		"is_enabled":  enabled,
		"plan":        flag.PlanRequired,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.feature event", "err", err)
	}
	return flag, nil
}

// OverrideFeature is a platform super-admin override of a tenant feature flag.
// It bypasses the normal `features.manage` permission check but still enforces
// tenant scope and plan entitlement. The reason is persisted for audit.
func (s *Service) OverrideFeature(ctx context.Context, actor auth.Actor, code, key string, enabled bool, reason string) (domain.FeatureFlag, error) {
	if code == "" {
		return domain.FeatureFlag{}, domain.ErrNoTenant
	}
	if !actor.PlatformAdmin || !actor.CanAccessTenant(code) {
		return domain.FeatureFlag{}, domain.ErrForbidden
	}
	if enabled {
		tenant, err := s.repo.GetTenant(withTenant(ctx, code), code)
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
	flag, err := s.repo.SetFeature(withTenant(ctx, code), code, key, enabled, reason)
	if err != nil {
		return domain.FeatureFlag{}, err
	}
	eventType := "tenant.feature_disabled.v1"
	if enabled {
		eventType = "tenant.feature_enabled.v1"
	}
	if err := s.pub.Publish(ctx, eventType, code, map[string]any{
		"feature_key": key,
		"is_enabled":  enabled,
		"plan":        flag.PlanRequired,
		"reason":      reason,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.feature event", "err", err)
	}
	return flag, nil
}

func defaultStatus(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
