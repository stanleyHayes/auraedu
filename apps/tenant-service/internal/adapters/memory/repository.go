// Package memory is an in-memory, seeded implementation of ports.Repository.
// It lets the Tenant Service run and be tested without infrastructure; the
// Postgres+RLS adapter (platform/db) replaces it in the next story. Seed data
// mirrors contracts/features/features.yaml and the two initial tenants (spec §20).
package memory

import (
	"sync"

	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
)

// catalog is the stable feature key list + plan tier (spec §9, §16).
var catalog = []domain.FeatureFlag{
	{Key: "public_website", PlanRequired: "starter"},
	{Key: "admissions", PlanRequired: "growth"},
	{Key: "student_management", PlanRequired: "starter"},
	{Key: "staff_management", PlanRequired: "starter"},
	{Key: "parent_portal", PlanRequired: "starter"},
	{Key: "student_portal", PlanRequired: "growth"},
	{Key: "teacher_portal", PlanRequired: "starter"},
	{Key: "attendance", PlanRequired: "starter"},
	{Key: "assignments", PlanRequired: "growth"},
	{Key: "assessments", PlanRequired: "growth"},
	{Key: "cbt_exams", PlanRequired: "professional"},
	{Key: "report_cards", PlanRequired: "starter"},
	{Key: "fees", PlanRequired: "growth"},
	{Key: "online_payments", PlanRequired: "professional"},
	{Key: "timetable", PlanRequired: "growth"},
	{Key: "library", PlanRequired: "professional"},
	{Key: "hostel", PlanRequired: "professional"},
	{Key: "transport", PlanRequired: "professional"},
	{Key: "announcements", PlanRequired: "starter"},
	{Key: "email_notifications", PlanRequired: "starter"},
	{Key: "sms_notifications", PlanRequired: "growth"},
	{Key: "whatsapp_notifications", PlanRequired: "professional"},
	{Key: "analytics", PlanRequired: "professional"},
	{Key: "ai_recommendations", PlanRequired: "ai_plus"},
	{Key: "ai_predictions", PlanRequired: "ai_plus"},
	{Key: "career_guidance", PlanRequired: "ai_plus"},
	{Key: "billing", PlanRequired: "core"},
	{Key: "custom_domain", PlanRequired: "professional"},
}

func planOf(key string) string {
	for _, f := range catalog {
		if f.Key == key {
			return f.PlanRequired
		}
	}
	return ""
}

// Repository is the seeded in-memory tenant + feature-flag store.
type Repository struct {
	mu      sync.RWMutex
	tenants map[string]domain.Tenant
	order   []string
	enabled map[string]map[string]bool // tenant code -> feature key -> enabled
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
		tenants: map[string]domain.Tenant{},
		enabled: map[string]map[string]bool{},
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

func (r *Repository) ListTenants() []domain.Tenant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.Tenant, 0, len(r.order))
	for _, code := range r.order {
		out = append(out, r.tenants[code])
	}
	return out
}

func (r *Repository) GetTenant(code string) (domain.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tenants[code]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

func (r *Repository) Features(code string) ([]domain.FeatureFlag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	on, ok := r.enabled[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := make([]domain.FeatureFlag, 0, len(catalog))
	for _, f := range catalog {
		out = append(out, domain.FeatureFlag{Key: f.Key, Enabled: on[f.Key], PlanRequired: f.PlanRequired})
	}
	return out, nil
}

func (r *Repository) SetFeature(code, key string, enabled bool) (domain.FeatureFlag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	on, ok := r.enabled[code]
	if !ok {
		return domain.FeatureFlag{}, domain.ErrNotFound
	}
	if planOf(key) == "" {
		return domain.FeatureFlag{}, domain.ErrValidation
	}
	on[key] = enabled
	return domain.FeatureFlag{Key: key, Enabled: enabled, PlanRequired: planOf(key)}, nil
}
