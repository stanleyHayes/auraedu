// Package application holds the Tenant Service use cases. The four gates
// (auth → tenant scope → RBAC → entitlement) are enforced here (agent_plan §5),
// never in HTTP handlers. The actor is resolved by the gateway (platform/auth).
package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

// FeatureManage is the RBAC permission required to change a tenant's feature flags (spec §8.2).
const FeatureManage = "features.manage"

type Service struct {
	repo  ports.Repository
	pub   ports.EventPublisher
	txt   TXTResolver
	token func() (string, error)
}

type Option func(*Service)

func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

type TXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

func WithTXTResolver(resolver TXTResolver) Option { return func(s *Service) { s.txt = resolver } }

func WithDomainTokenGenerator(generator func() (string, error)) Option {
	return func(s *Service) { s.token = generator }
}

type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _, _ string, _ map[string]any) error {
	return nil
}

func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}, txt: net.DefaultResolver, token: randomDomainToken}
	for _, o := range opts {
		o(s)
	}
	return s
}

func randomDomainToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *Service) durableTenantLifecycle() (ports.DurableTenantLifecycleRepository, bool) {
	repo, ok := s.repo.(ports.DurableTenantLifecycleRepository)
	return repo, ok
}

func withTenant(ctx context.Context, code string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: code})
}

type SubmitOnboardingInput struct {
	SchoolName           string
	AdministratorName    string
	Email                string
	Phone                *string
	CountryCode          string
	Plan                 string
	Priorities           *string
	PrivacyNoticeVersion string
	AcceptedTerms        bool
	Website              string
}

func (s *Service) SubmitOnboarding(ctx context.Context, idempotencyKey string, input SubmitOnboardingInput) (*domain.OnboardingRequest, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return nil, false, domain.ErrValidation
	}
	request, err := domain.NewOnboardingRequest(domain.OnboardingRequestInput{
		SchoolName: input.SchoolName, AdministratorName: input.AdministratorName,
		Email: input.Email, Phone: input.Phone, CountryCode: input.CountryCode,
		Plan: input.Plan, Priorities: input.Priorities,
		PrivacyNoticeVersion: input.PrivacyNoticeVersion,
		AcceptedTerms:        input.AcceptedTerms, Website: input.Website,
	})
	if err != nil {
		return nil, false, err
	}
	payload, err := json.Marshal(map[string]any{
		"school_name": request.SchoolName, "administrator_name": request.AdministratorName,
		"email": request.Email, "phone": request.Phone, "country_code": request.CountryCode,
		"plan": request.Plan, "priorities": request.Priorities,
		"privacy_notice_version": request.PrivacyNoticeVersion,
	})
	if err != nil {
		return nil, false, fmt.Errorf("onboarding: encode request: %w", err)
	}
	return s.repo.SubmitOnboarding(ctx, request, digest(idempotencyKey), digest(string(payload)), digest(request.Email))
}

func (s *Service) ListOnboarding(ctx context.Context, actor auth.Actor, limit int, cursor, status string) ([]domain.OnboardingRequest, string, error) {
	if !actor.PlatformAdmin {
		return nil, "", domain.ErrForbidden
	}
	if cursor != "" && len(cursor) != 36 {
		return nil, "", domain.ErrValidation
	}
	validStatus := status == "" || status == domain.OnboardingPending || status == domain.OnboardingApproved ||
		status == domain.OnboardingRejected || status == domain.OnboardingProvisioningFailed
	if !validStatus {
		return nil, "", domain.ErrValidation
	}
	return s.repo.ListOnboarding(auth.WithActor(ctx, actor), limit, cursor, status)
}

func (s *Service) ApproveOnboarding(ctx context.Context, actor auth.Actor, requestID, tenantCode string) (domain.OnboardingRequest, error) {
	if !actor.PlatformAdmin {
		return domain.OnboardingRequest{}, domain.ErrForbidden
	}
	request, err := s.repo.GetOnboarding(ctx, requestID)
	if err != nil {
		return domain.OnboardingRequest{}, err
	}
	tenant, err := request.Tenant(tenantCode)
	if err != nil {
		return domain.OnboardingRequest{}, err
	}
	ctx = auth.WithActor(ctx, actor)
	approved, err := s.repo.ApproveOnboarding(ctx, requestID, tenant, actor.UserID)
	if err != nil {
		return domain.OnboardingRequest{}, err
	}
	if _, durable := s.repo.(ports.DurableOnboardingRepository); durable {
		return approved, nil
	}
	if err := s.pub.Publish(ctx, "tenant.created.v1", tenant.Code, map[string]any{
		"tenant_code": tenant.Code, "name": tenant.Name, "plan": tenant.Plan,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant.created event", "err", err)
	}
	if err := s.pub.Publish(ctx, "tenant.onboarding_approved.v1", tenant.Code, map[string]any{
		"request_id": requestID, "tenant_code": tenant.Code, "plan": tenant.Plan,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish onboarding approval event", "err", err)
	}
	return approved, nil
}

func (s *Service) RejectOnboarding(ctx context.Context, actor auth.Actor, requestID, reason string) (domain.OnboardingRequest, error) {
	if !actor.PlatformAdmin {
		return domain.OnboardingRequest{}, domain.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > 500 {
		return domain.OnboardingRequest{}, domain.ErrValidation
	}
	return s.repo.RejectOnboarding(auth.WithActor(ctx, actor), requestID, reason, actor.UserID)
}

func (s *Service) ResolveOnboardingAdministrator(ctx context.Context, requestID string) (domain.OnboardingRequest, error) {
	request, err := s.repo.GetOnboarding(ctx, strings.TrimSpace(requestID))
	if err != nil {
		return domain.OnboardingRequest{}, err
	}
	if request.Status != domain.OnboardingApproved || request.TenantCode == nil || *request.TenantCode == "" {
		return domain.OnboardingRequest{}, domain.ErrNotFound
	}
	return request, nil
}

// ActivateOnboardingTenant is called only by the authenticated identity
// service after the school's administrator accepts their invitation. It is
// intentionally idempotent so acceptance can be retried safely.
func (s *Service) ActivateOnboardingTenant(ctx context.Context, code string) error {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return domain.ErrValidation
	}
	if repo, durable := s.durableTenantLifecycle(); durable {
		_, err := repo.ActivateOnboardingTenant(withTenant(ctx, code), code)
		return err
	}
	tenant, err := s.repo.GetTenant(withTenant(ctx, code), code)
	if err != nil {
		return err
	}
	if tenant.Status == "active" {
		return nil
	}
	if tenant.Status != "onboarding" {
		return domain.ErrConflict
	}
	status := "active"
	updated, err := s.repo.UpdateTenant(withTenant(ctx, code), code, domain.TenantUpdate{Status: &status})
	if err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "tenant.activated.v1", code, map[string]any{
		"tenant_code": code, "status": updated.Status,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish tenant activation event", "err", err)
	}
	return nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// ListTenants is a platform-wide operation — platform super admins only (spec §8.1).
func (s *Service) ListTenants(ctx context.Context, actor auth.Actor) ([]domain.Tenant, error) {
	ctx = auth.WithActor(ctx, actor)
	if !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListTenants(ctx)
}

// CreateTenant provisions a new school. Platform super admins only.
func (s *Service) CreateTenant(ctx context.Context, actor auth.Actor, t domain.Tenant) (domain.Tenant, error) {
	ctx = auth.WithActor(ctx, actor)
	if !actor.PlatformAdmin {
		return domain.Tenant{}, domain.ErrForbidden
	}
	if t.Domain != "" {
		return domain.Tenant{}, domain.ErrValidation
	}
	if err := t.Validate(); err != nil {
		return domain.Tenant{}, err
	}
	t.Code = strings.ToLower(strings.TrimSpace(t.Code))
	t.Status = defaultStatus(t.Status, "active")
	if err := s.repo.CreateTenant(withTenant(ctx, t.Code), t); err != nil {
		return domain.Tenant{}, err
	}
	if _, durable := s.durableTenantLifecycle(); durable {
		return t, nil
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
	ctx = auth.WithActor(ctx, actor)
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
	if _, durable := s.durableTenantLifecycle(); durable {
		return t, nil
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
	ctx = auth.WithActor(ctx, actor)
	if !actor.PlatformAdmin {
		return domain.ErrForbidden
	}
	if code == "" {
		return domain.ErrValidation
	}
	if err := s.repo.DeleteTenant(withTenant(ctx, code), code); err != nil {
		return err
	}
	if _, durable := s.durableTenantLifecycle(); durable {
		return nil
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
	ctx = auth.WithActor(ctx, actor)
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
	domainHost = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domainHost), "."))
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if strings.ContainsAny(domainHost, "/:@?#") || strings.ContainsAny(subdomain, ". /:@?#") {
		return domain.Tenant{}, domain.ErrValidation
	}
	return s.repo.ResolveTenant(ctx, domainHost, subdomain)
}

func (s *Service) RequestCustomDomain(ctx context.Context, actor auth.Actor, code, hostname string) (domain.CustomDomain, error) {
	ctx = auth.WithActor(ctx, actor)
	if !actor.CanAccessTenant(code) || !actor.Has(FeatureManage) {
		return domain.CustomDomain{}, domain.ErrForbidden
	}
	tenant, err := s.repo.GetTenant(withTenant(ctx, code), code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	if !domain.PlanAllows(tenant.Plan, "professional") {
		return domain.CustomDomain{}, domain.ErrEntitlement
	}
	features, err := s.repo.Features(withTenant(ctx, code), code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	enabled := false
	for _, feature := range features {
		if feature.Key == "custom_domain" {
			enabled = feature.Enabled
			break
		}
	}
	if !enabled {
		return domain.CustomDomain{}, flags.ErrFeatureDisabled
	}
	hostname, err = domain.NormalizeCustomDomain(hostname)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	token, err := s.token()
	if err != nil || len(token) < 32 {
		return domain.CustomDomain{}, domain.ErrUnavailable
	}
	verificationValue := "auraedu-domain-verification=" + token
	registration := domain.CustomDomain{
		TenantCode:    code,
		Hostname:      hostname,
		Status:        domain.DomainPending,
		TXTRecordName: domain.DomainTXTRecord(hostname),
	}
	created, err := s.repo.RequestCustomDomain(withTenant(ctx, code), registration, digest(verificationValue))
	if err != nil {
		return domain.CustomDomain{}, err
	}
	created.VerificationToken = verificationValue
	return created, nil
}

func (s *Service) GetCustomDomain(ctx context.Context, actor auth.Actor, code string) (domain.CustomDomain, error) {
	if !actor.CanAccessTenant(code) || !actor.Has(FeatureManage) {
		return domain.CustomDomain{}, domain.ErrForbidden
	}
	registration, _, err := s.repo.GetCustomDomain(withTenant(auth.WithActor(ctx, actor), code), code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	return visibleCustomDomain(registration, actor), nil
}

func visibleCustomDomain(registration domain.CustomDomain, actor auth.Actor) domain.CustomDomain {
	registration.VerificationToken = ""
	if !actor.PlatformAdmin {
		registration.ProviderReference = ""
	}
	return registration
}

func (s *Service) VerifyCustomDomain(ctx context.Context, actor auth.Actor, code string) (domain.CustomDomain, error) {
	if !actor.CanAccessTenant(code) || !actor.Has(FeatureManage) {
		return domain.CustomDomain{}, domain.ErrForbidden
	}
	if s.txt == nil {
		return domain.CustomDomain{}, domain.ErrUnavailable
	}
	ctx = auth.WithActor(ctx, actor)
	registration, expectedHash, err := s.repo.GetCustomDomain(withTenant(ctx, code), code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	if registration.Status == domain.DomainVerified || registration.Status == domain.DomainActive {
		return visibleCustomDomain(registration, actor), nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	values, err := s.txt.LookupTXT(lookupCtx, registration.TXTRecordName)
	if err != nil {
		return domain.CustomDomain{}, domain.ErrUnavailable
	}
	owned := false
	for _, value := range values {
		if digest(strings.TrimSpace(value)) == expectedHash {
			owned = true
			break
		}
	}
	if !owned {
		return domain.CustomDomain{}, domain.ErrConflict
	}
	return s.repo.MarkCustomDomainVerified(withTenant(ctx, code), code, time.Now().UTC())
}

func (s *Service) ActivateCustomDomain(ctx context.Context, actor auth.Actor, code, providerReference string) (domain.CustomDomain, error) {
	if !actor.PlatformAdmin {
		return domain.CustomDomain{}, domain.ErrForbidden
	}
	providerReference = strings.TrimSpace(providerReference)
	if len(providerReference) < 8 || len(providerReference) > 200 {
		return domain.CustomDomain{}, domain.ErrValidation
	}
	ctx = auth.WithActor(ctx, actor)
	registration, _, err := s.repo.GetCustomDomain(withTenant(ctx, code), code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	if registration.Status != domain.DomainVerified && registration.Status != domain.DomainActive {
		return domain.CustomDomain{}, domain.ErrConflict
	}
	if registration.Status == domain.DomainActive {
		return registration, nil
	}
	return s.repo.ActivateCustomDomain(withTenant(ctx, code), code, providerReference, time.Now().UTC())
}

func (s *Service) DeactivateCustomDomain(ctx context.Context, actor auth.Actor, code, providerReference string) (domain.CustomDomain, error) {
	if !actor.PlatformAdmin {
		return domain.CustomDomain{}, domain.ErrForbidden
	}
	providerReference = strings.TrimSpace(providerReference)
	if len(providerReference) < 8 || len(providerReference) > 200 {
		return domain.CustomDomain{}, domain.ErrValidation
	}
	ctx = auth.WithActor(ctx, actor)
	registration, _, err := s.repo.GetCustomDomain(withTenant(ctx, code), code)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	if registration.Status == domain.DomainInactive {
		return registration, nil
	}
	if registration.Status != domain.DomainActive {
		return domain.CustomDomain{}, domain.ErrConflict
	}
	return s.repo.DeactivateCustomDomain(withTenant(ctx, code), code, providerReference, time.Now().UTC())
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
	ctx = auth.WithActor(ctx, actor)
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
	ctx = auth.WithActor(ctx, actor)
	if !actor.CanAccessTenant(code) {
		return domain.Settings{}, domain.ErrForbidden
	}
	return s.repo.Settings(withTenant(ctx, code), code)
}

// UpdateSettings changes a tenant's operational settings. The actor must both
// belong to the tenant and hold the administrative feature-management permission;
// platform super admins implicitly hold all permissions.
func (s *Service) UpdateSettings(ctx context.Context, actor auth.Actor, code string, settings domain.Settings) (domain.Settings, error) {
	ctx = auth.WithActor(ctx, actor)
	if !actor.CanAccessTenant(code) {
		return domain.Settings{}, domain.ErrForbidden
	}
	if !actor.Has(FeatureManage) {
		return domain.Settings{}, domain.ErrForbidden
	}
	if err := domain.ValidateSettings(settings); err != nil {
		return domain.Settings{}, err
	}
	if err := s.repo.UpdateSettings(withTenant(ctx, code), code, settings); err != nil {
		return domain.Settings{}, err
	}
	if _, durable := s.durableTenantLifecycle(); durable {
		return settings, nil
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
	if _, durable := s.durableTenantLifecycle(); durable {
		return flag, nil
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
	if _, durable := s.durableTenantLifecycle(); durable {
		return flag, nil
	}
	eventType := "tenant.feature_disabled.v1"
	if enabled {
		eventType = "tenant.feature_enabled.v1"
	}
	// The payload must match contracts/events/tenant.feature_{enabled,disabled}.v1.json
	// (additionalProperties: false) — the override reason stays on the flag row only.
	if err := s.pub.Publish(ctx, eventType, code, map[string]any{
		"feature_key": key,
		"is_enabled":  enabled,
		"plan":        flag.PlanRequired,
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
