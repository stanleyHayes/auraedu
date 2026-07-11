// Package memory is an in-memory, seeded implementation of ports.Repository.
// It lets the Tenant Service run and be tested without infrastructure; the
// Postgres+RLS adapter (platform/db) replaces it in production. Seed data
// mirrors contracts/features/features.yaml and the two initial tenants (spec §20).
package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

// Repository is the seeded in-memory tenant + feature-flag store.
type Repository struct {
	mu       sync.RWMutex
	tenants  map[string]domain.Tenant
	order    []string
	enabled  map[string]map[string]bool // tenant code -> feature key -> enabled
	settings map[string]domain.Settings
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
		tenants:  map[string]domain.Tenant{},
		enabled:  map[string]map[string]bool{},
		settings: map[string]domain.Settings{},
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
			if strings.EqualFold(t.Domain, domainHost) {
				return t, nil
			}
		}
	}
	if subdomain != "" {
		if t, ok := r.tenants[subdomain]; ok {
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
