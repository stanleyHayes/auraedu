// Package memory is an in-memory, seeded implementation of ports.Repository.
// It lets the Tenant Service run and be tested without infrastructure; the
// Postgres+RLS adapter (platform/db) replaces it in production. Seed data
// mirrors contracts/features/features.yaml and the two initial tenants (spec §20).
package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

// Repository is the seeded in-memory tenant + feature-flag store.
type Repository struct {
	mu                sync.RWMutex
	tenants           map[string]domain.Tenant
	order             []string
	enabled           map[string]map[string]bool // tenant code -> feature key -> enabled
	settings          map[string]domain.Settings
	onboarding        map[string]*domain.OnboardingRequest
	onboardingOrder   []string
	idempotency       map[string]string
	payloadHashes     map[string]string
	emailFingerprints map[string]string
	customDomains     map[string]domain.CustomDomain
	domainChallenges  map[string]string
}

var _ ports.Repository = (*Repository)(nil)

func setOf(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// New returns a seeded repository (UPSHS, Aboom, Cape Coast Prep).
func New() *Repository {
	r := &Repository{
		tenants:           map[string]domain.Tenant{},
		enabled:           map[string]map[string]bool{},
		settings:          map[string]domain.Settings{},
		onboarding:        map[string]*domain.OnboardingRequest{},
		idempotency:       map[string]string{},
		payloadHashes:     map[string]string{},
		emailFingerprints: map[string]string{},
		customDomains:     map[string]domain.CustomDomain{},
		domainChallenges:  map[string]string{},
	}
	add := func(t domain.Tenant, enabled map[string]bool) {
		r.tenants[t.Code] = t
		r.order = append(r.order, t.Code)
		r.enabled[t.Code] = enabled
	}

	add(domain.Tenant{
		Code: "upshs", Name: "University Practice SHS", Short: "UPSHS", Status: "active",
		Plan: "ai_plus", Branding: domain.Branding{Brand: domain.Brand{Primary: "#7B1113", Secondary: "#1E7D52"}},
	}, setOf(
		"public_website", "admissions", "student_management", "staff_management", "parent_portal",
		"student_portal", "teacher_portal", "attendance", "assignments", "assessments", "cbt_exams",
		"report_cards", "fees", "timetable", "announcements", "email_notifications", "analytics",
		"ai_recommendations", "ai_predictions", "career_guidance", "billing", "custom_domain",
	))

	add(domain.Tenant{
		Code: "aboom-ame-zion-c", Name: "Aboom AME Zion C Basic School", Short: "Aboom", Status: "active",
		Plan: "starter", Branding: domain.Branding{Brand: domain.Brand{Primary: "#1E7D52"}},
	}, setOf(
		"public_website", "student_management", "staff_management", "parent_portal", "teacher_portal",
		"attendance", "assessments", "report_cards", "fees", "announcements", "email_notifications", "billing",
	))

	add(domain.Tenant{
		Code: "cape-coast-prep", Name: "Cape Coast Prep", Short: "Cape Coast", Status: "onboarding",
		Plan: "professional", Branding: domain.Branding{Brand: domain.Brand{Primary: "#2456A6"}},
	}, setOf(
		"public_website", "admissions", "student_management", "staff_management", "parent_portal",
		"teacher_portal", "attendance", "assessments", "report_cards", "fees", "online_payments",
		"announcements", "email_notifications", "billing",
	))

	return r
}

func (r *Repository) SubmitOnboarding(
	_ context.Context,
	request *domain.OnboardingRequest,
	idempotencyHash string,
	payloadHash string,
	emailFingerprint string,
) (*domain.OnboardingRequest, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.idempotency[idempotencyHash]; ok {
		if r.payloadHashes[idempotencyHash] != payloadHash {
			return nil, false, domain.ErrConflict
		}
		stored := *r.onboarding[id]
		return &stored, false, nil
	}
	if id, ok := r.emailFingerprints[emailFingerprint]; ok {
		stored := *r.onboarding[id]
		return &stored, false, nil
	}
	stored := *request
	r.onboarding[request.ID] = &stored
	r.onboardingOrder = append(r.onboardingOrder, request.ID)
	r.idempotency[idempotencyHash] = request.ID
	r.payloadHashes[idempotencyHash] = payloadHash
	r.emailFingerprints[emailFingerprint] = request.ID
	return request, true, nil
}

func (r *Repository) ListOnboarding(_ context.Context, limit int, cursor, status string) ([]domain.OnboardingRequest, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	out := make([]domain.OnboardingRequest, 0, limit)
	started := cursor == ""
	for i := len(r.onboardingOrder) - 1; i >= 0; i-- {
		id := r.onboardingOrder[i]
		if !started {
			started = id == cursor
			continue
		}
		item := r.onboarding[id]
		if status != "" && item.Status != status {
			continue
		}
		if len(out) == limit {
			return out, out[len(out)-1].ID, nil
		}
		out = append(out, *item)
	}
	return out, "", nil
}

func (r *Repository) GetOnboarding(_ context.Context, requestID string) (domain.OnboardingRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.onboarding[requestID]
	if !ok {
		return domain.OnboardingRequest{}, domain.ErrNotFound
	}
	return *item, nil
}

func (r *Repository) ApproveOnboarding(_ context.Context, requestID string, tenant domain.Tenant, _ string) (domain.OnboardingRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.onboarding[requestID]
	if !ok {
		return domain.OnboardingRequest{}, domain.ErrNotFound
	}
	if item.Status != domain.OnboardingPending {
		return domain.OnboardingRequest{}, domain.ErrConflict
	}
	if _, exists := r.tenants[tenant.Code]; exists {
		return domain.OnboardingRequest{}, domain.ErrConflict
	}
	now := time.Now().UTC()
	item.Status, item.TenantCode, item.DecidedAt = domain.OnboardingApproved, &tenant.Code, &now
	r.tenants[tenant.Code] = tenant
	r.order = append(r.order, tenant.Code)
	r.enabled[tenant.Code] = defaultEnabledForPlan(tenant.Plan)
	r.settings[tenant.Code] = domain.Settings{PrimaryContactEmail: item.Email}
	delete(r.emailFingerprints, fingerprintLookup(r.emailFingerprints, requestID))
	return *item, nil
}

func (r *Repository) RejectOnboarding(_ context.Context, requestID, reason, _ string) (domain.OnboardingRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.onboarding[requestID]
	if !ok {
		return domain.OnboardingRequest{}, domain.ErrNotFound
	}
	if item.Status != domain.OnboardingPending {
		return domain.OnboardingRequest{}, domain.ErrConflict
	}
	now := time.Now().UTC()
	item.Status, item.DecisionReason, item.DecidedAt = domain.OnboardingRejected, &reason, &now
	delete(r.emailFingerprints, fingerprintLookup(r.emailFingerprints, requestID))
	return *item, nil
}

func fingerprintLookup(values map[string]string, requestID string) string {
	for fingerprint, id := range values {
		if id == requestID {
			return fingerprint
		}
	}
	return ""
}

func (r *Repository) ListTenants(_ context.Context) ([]domain.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Tenant, 0, len(r.order))
	for _, code := range r.order {
		out = append(out, r.tenants[code])
	}
	return out, nil
}

func (r *Repository) GetTenant(_ context.Context, code string) (domain.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tenants[code]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

func (r *Repository) CreateTenant(_ context.Context, t domain.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenants[t.Code]; exists {
		return domain.ErrValidation
	}
	r.tenants[t.Code] = t
	r.order = append(r.order, t.Code)
	r.enabled[t.Code] = defaultEnabledForPlan(t.Plan)
	r.settings[t.Code] = domain.Settings{}
	return nil
}

func defaultEnabledForPlan(plan string) map[string]bool {
	m := map[string]bool{}
	for _, f := range domain.FeatureCatalog() {
		m[f.Key] = domain.PlanAllows(plan, f.PlanRequired)
	}
	return m
}

func (r *Repository) UpdateTenant(_ context.Context, code string, upd domain.TenantUpdate) (domain.Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tenants[code]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	t = t.ApplyUpdate(upd)
	if err := t.Validate(); err != nil {
		return domain.Tenant{}, err
	}
	r.tenants[code] = t
	return t, nil
}

func (r *Repository) DeleteTenant(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[code]; !ok {
		return domain.ErrNotFound
	}
	delete(r.tenants, code)
	delete(r.enabled, code)
	delete(r.settings, code)
	delete(r.customDomains, code)
	delete(r.domainChallenges, code)
	for i, c := range r.order {
		if c == code {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func (r *Repository) ResolveTenant(_ context.Context, domainHost, subdomain string) (domain.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if domainHost != "" {
		for _, t := range r.tenants {
			if t.Status == "active" && strings.EqualFold(t.Domain, domainHost) {
				return t, nil
			}
		}
	}
	if subdomain != "" {
		if t, ok := r.tenants[subdomain]; ok && t.Status == "active" {
			return t, nil
		}
	}
	return domain.Tenant{}, domain.ErrNotFound
}

func (r *Repository) Features(_ context.Context, code string) ([]domain.FeatureFlag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	on, ok := r.enabled[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	catalog := domain.FeatureCatalog()
	out := make([]domain.FeatureFlag, 0, len(catalog))
	for _, f := range catalog {
		out = append(out, domain.FeatureFlag{Key: f.Key, Enabled: on[f.Key], PlanRequired: f.PlanRequired})
	}
	return out, nil
}

func (r *Repository) SetFeature(_ context.Context, code, key string, enabled bool, _ string) (domain.FeatureFlag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	on, ok := r.enabled[code]
	if !ok {
		return domain.FeatureFlag{}, domain.ErrNotFound
	}
	plan, known := domain.FeaturePlan(key)
	if !known {
		return domain.FeatureFlag{}, domain.ErrValidation
	}
	on[key] = enabled
	return domain.FeatureFlag{Key: key, Enabled: enabled, PlanRequired: plan}, nil
}

func (r *Repository) Settings(_ context.Context, code string) (domain.Settings, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.tenants[code]; !ok {
		return domain.Settings{}, domain.ErrNotFound
	}
	return r.settings[code], nil
}

func (r *Repository) UpdateSettings(_ context.Context, code string, s domain.Settings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[code]; !ok {
		return domain.ErrNotFound
	}
	r.settings[code] = s
	return nil
}

func (r *Repository) RequestCustomDomain(_ context.Context, registration domain.CustomDomain, challengeHash string) (domain.CustomDomain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[registration.TenantCode]; !ok {
		return domain.CustomDomain{}, domain.ErrNotFound
	}
	if current, ok := r.customDomains[registration.TenantCode]; ok && current.Status == domain.DomainActive {
		return domain.CustomDomain{}, domain.ErrConflict
	}
	for code, existing := range r.customDomains {
		if code != registration.TenantCode && strings.EqualFold(existing.Hostname, registration.Hostname) {
			return domain.CustomDomain{}, domain.ErrConflict
		}
	}
	r.customDomains[registration.TenantCode] = registration
	r.domainChallenges[registration.TenantCode] = challengeHash
	return registration, nil
}

func (r *Repository) GetCustomDomain(_ context.Context, code string) (domain.CustomDomain, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	registration, ok := r.customDomains[code]
	if !ok {
		return domain.CustomDomain{}, "", domain.ErrNotFound
	}
	return registration, r.domainChallenges[code], nil
}

func (r *Repository) MarkCustomDomainVerified(_ context.Context, code string, verifiedAt time.Time) (domain.CustomDomain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	registration, ok := r.customDomains[code]
	if !ok {
		return domain.CustomDomain{}, domain.ErrNotFound
	}
	registration.Status = domain.DomainVerified
	registration.VerifiedAt = &verifiedAt
	r.customDomains[code] = registration
	return registration, nil
}

func (r *Repository) ActivateCustomDomain(_ context.Context, code, providerReference string, activatedAt time.Time) (domain.CustomDomain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	registration, ok := r.customDomains[code]
	if !ok {
		return domain.CustomDomain{}, domain.ErrNotFound
	}
	if registration.Status != domain.DomainVerified {
		return domain.CustomDomain{}, domain.ErrConflict
	}
	registration.Status = domain.DomainActive
	registration.ProviderReference = providerReference
	registration.ActivatedAt = &activatedAt
	r.customDomains[code] = registration
	tenant := r.tenants[code]
	tenant.Domain = registration.Hostname
	r.tenants[code] = tenant
	return registration, nil
}

func (r *Repository) DeactivateCustomDomain(_ context.Context, code, providerReference string, deactivatedAt time.Time) (domain.CustomDomain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	registration, ok := r.customDomains[code]
	if !ok {
		return domain.CustomDomain{}, domain.ErrNotFound
	}
	if registration.Status != domain.DomainActive {
		return domain.CustomDomain{}, domain.ErrConflict
	}
	registration.Status = domain.DomainInactive
	registration.ProviderReference = providerReference
	registration.DeactivatedAt = &deactivatedAt
	r.customDomains[code] = registration
	tenant := r.tenants[code]
	tenant.Domain = ""
	r.tenants[code] = tenant
	return registration, nil
}
