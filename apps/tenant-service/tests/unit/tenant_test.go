package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

func newSvc() *application.Service { return application.NewService(memory.New()) }

type txtResolver struct {
	records map[string][]string
	err     error
}

func (r *txtResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	return r.records[name], r.err
}

var (
	ctx         = context.Background()
	anon        = auth.Actor{}
	platformAdm = auth.Actor{UserID: "p1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	upshsAdmin  = auth.Actor{UserID: "u1", TenantID: "upshs", Role: "school_admin", Permissions: []string{"features.manage"}}
	upshsUser   = auth.Actor{UserID: "u2", TenantID: "upshs", Role: "teacher"}
	aboomUser   = auth.Actor{UserID: "a1", TenantID: "aboom-ame-zion-c", Role: "teacher"}
)

func TestListTenantsRequiresPlatformAdmin(t *testing.T) {
	svc := newSvc()
	if _, err := svc.ListTenants(ctx, anon); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("anon should be forbidden, got %v", err)
	}
	if _, err := svc.ListTenants(ctx, upshsAdmin); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin should not list all tenants, got %v", err)
	}
	got, err := svc.ListTenants(ctx, platformAdm)
	if err != nil || len(got) != 3 {
		t.Fatalf("platform admin should list 3 tenants, got %d err %v", len(got), err)
	}
}

func TestBrandingIsPublic(t *testing.T) {
	b, err := newSvc().Branding(ctx, "upshs")
	if err != nil {
		t.Fatalf("branding should be public: %v", err)
	}
	if b.Brand.Primary != "#7B1113" {
		t.Fatalf("upshs brand = %q, want #7B1113", b.Brand.Primary)
	}
}

func TestGetTenantTenantScope(t *testing.T) {
	svc := newSvc()
	if _, err := svc.GetTenant(ctx, upshsUser, "upshs"); err != nil {
		t.Fatalf("own tenant should be readable: %v", err)
	}
	if _, err := svc.GetTenant(ctx, upshsUser, "aboom-ame-zion-c"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant read should be forbidden, got %v", err)
	}
	if _, err := svc.GetTenant(ctx, platformAdm, "aboom-ame-zion-c"); err != nil {
		t.Fatalf("platform admin should read any tenant: %v", err)
	}
}

func TestFeaturesIsPublicForResolvedTenant(t *testing.T) {
	svc := newSvc()
	fs, err := svc.Features(ctx, anon, "upshs")
	if err != nil {
		t.Fatalf("public feature snapshot should succeed: %v", err)
	}
	if len(fs) == 0 {
		t.Fatal("public feature snapshot should return flags")
	}
	// Unknown tenants still 404 even for public callers.
	if _, err := svc.Features(ctx, anon, "no-such-school"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown tenant should be not found, got %v", err)
	}
}

func TestFeaturesTenantScope(t *testing.T) {
	svc := newSvc()
	if _, err := svc.Features(ctx, upshsUser, ""); !errors.Is(err, domain.ErrNoTenant) {
		t.Fatalf("empty tenant should be ErrNoTenant, got %v", err)
	}
	if _, err := svc.Features(ctx, aboomUser, "upshs"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant feature read should be forbidden, got %v", err)
	}
	if !enabled(t, svc, upshsUser, "upshs", "ai_recommendations") {
		t.Fatal("upshs should have ai_recommendations on")
	}
	if enabled(t, svc, aboomUser, "aboom-ame-zion-c", "ai_recommendations") {
		t.Fatal("aboom should have ai_recommendations off")
	}
}

func TestSetFeatureRBACAndScope(t *testing.T) {
	svc := newSvc()
	// teacher lacks features.manage
	if _, err := svc.SetFeature(ctx, upshsUser, "upshs", "analytics", false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("teacher should not manage features, got %v", err)
	}
	// school admin may manage own tenant (analytics is professional; upshs is ai_plus → allowed)
	if _, err := svc.SetFeature(ctx, upshsAdmin, "upshs", "analytics", true); err != nil {
		t.Fatalf("school admin should manage own tenant's in-plan feature: %v", err)
	}
	// school admin may NOT manage another tenant
	if _, err := svc.SetFeature(ctx, upshsAdmin, "aboom-ame-zion-c", "analytics", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant management should be forbidden, got %v", err)
	}
}

func TestOverrideFeatureRequiresPlatformAdmin(t *testing.T) {
	svc := newSvc()
	reason := "trial unlock"
	if _, err := svc.OverrideFeature(ctx, upshsAdmin, "upshs", "analytics", true, reason); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin should not override features, got %v", err)
	}
	if _, err := svc.OverrideFeature(ctx, platformAdm, "no-such-tenant", "analytics", true, reason); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown tenant should be not found, got %v", err)
	}
	// upshs is ai_plus, analytics is professional → allowed.
	if _, err := svc.OverrideFeature(ctx, platformAdm, "upshs", "analytics", true, reason); err != nil {
		t.Fatalf("platform admin should override feature: %v", err)
	}
}

func TestSettingsTenantScope(t *testing.T) {
	svc := newSvc()
	if _, err := svc.Settings(ctx, aboomUser, "upshs"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant settings read should be forbidden, got %v", err)
	}
	s, err := svc.Settings(ctx, upshsUser, "upshs")
	if err != nil {
		t.Fatalf("own tenant settings read: %v", err)
	}
	if s.Locale != "" {
		t.Fatalf("default locale should be empty, got %q", s.Locale)
	}
}

func TestUpdateSettings(t *testing.T) {
	svc := newSvc()
	if _, err := svc.UpdateSettings(ctx, upshsUser, "upshs", domain.Settings{
		Locale: "en-GH", Timezone: "Africa/Accra", AcademicYearStartMonth: 9,
	}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("teacher must not update tenant settings, got %v", err)
	}
	updated, err := svc.UpdateSettings(ctx, upshsAdmin, "upshs", domain.Settings{
		Locale:                 "en-GH",
		Timezone:               "Africa/Accra",
		AcademicYearStartMonth: 9,
	})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if updated.Locale != "en-GH" {
		t.Fatalf("locale = %q, want en-GH", updated.Locale)
	}
	s, err := svc.Settings(ctx, upshsUser, "upshs")
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if s.Timezone != "Africa/Accra" {
		t.Fatalf("timezone = %q, want Africa/Accra", s.Timezone)
	}
	if _, err := svc.UpdateSettings(ctx, upshsAdmin, "upshs", domain.Settings{AcademicYearStartMonth: 13}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid month should be validation error, got %v", err)
	}
}

func TestCreateTenantSeedsDefaultsByPlan(t *testing.T) {
	svc := newSvc()
	created, err := svc.CreateTenant(ctx, platformAdm, domain.Tenant{
		Code:   "growth-school",
		Name:   "Growth School",
		Status: "active",
		Plan:   "growth",
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if created.Plan != "growth" {
		t.Fatalf("plan = %q, want growth", created.Plan)
	}
	fs, err := svc.Features(ctx, platformAdm, created.Code)
	if err != nil {
		t.Fatalf("features: %v", err)
	}
	for _, f := range fs {
		want := domain.PlanAllows("growth", f.PlanRequired)
		if f.Enabled != want {
			t.Fatalf("feature %q enabled = %v, want %v", f.Key, f.Enabled, want)
		}
	}
}

func TestOverrideFeatureEntitlement(t *testing.T) {
	svc := newSvc()
	if _, err := svc.OverrideFeature(ctx, platformAdm, "aboom-ame-zion-c", "ai_recommendations", true, "trial"); !errors.Is(err, domain.ErrEntitlement) {
		t.Fatalf("enabling above plan should be ErrEntitlement, got %v", err)
	}
}

func TestSetFeatureEntitlement(t *testing.T) {
	svc := newSvc()
	// Aboom is on the 'starter' plan; ai_recommendations needs 'ai_plus'. Even a platform
	// admin cannot ENABLE it above the plan (spec §3.3: billing controls entitlement).
	if _, err := svc.SetFeature(ctx, platformAdm, "aboom-ame-zion-c", "ai_recommendations", true); !errors.Is(err, domain.ErrEntitlement) {
		t.Fatalf("enabling above plan should be ErrEntitlement, got %v", err)
	}
	// Disabling is always allowed.
	if _, err := svc.SetFeature(ctx, platformAdm, "aboom-ame-zion-c", "ai_recommendations", false); err != nil {
		t.Fatalf("disabling should be allowed: %v", err)
	}
	// Unknown key.
	if _, err := svc.SetFeature(ctx, platformAdm, "upshs", "not_a_feature", true); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unknown key should be ErrValidation, got %v", err)
	}
}

func TestUpdateTenantRequiresPlatformAdmin(t *testing.T) {
	svc := newSvc()
	name := "Updated Name"
	if _, err := svc.UpdateTenant(ctx, upshsAdmin, "upshs", domain.TenantUpdate{Name: &name}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin should not update tenant, got %v", err)
	}
	if _, err := svc.UpdateTenant(ctx, platformAdm, "no-such-tenant", domain.TenantUpdate{Name: &name}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown tenant should be not found, got %v", err)
	}
	updated, err := svc.UpdateTenant(ctx, platformAdm, "upshs", domain.TenantUpdate{Name: &name})
	if err != nil {
		t.Fatalf("platform admin should update tenant: %v", err)
	}
	if updated.Name != name {
		t.Fatalf("name = %q, want %q", updated.Name, name)
	}
}

func TestUpdateTenantValidation(t *testing.T) {
	svc := newSvc()
	empty := ""
	if _, err := svc.UpdateTenant(ctx, platformAdm, "upshs", domain.TenantUpdate{Name: &empty}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty name should be validation error, got %v", err)
	}
	badStatus := "deleted"
	if _, err := svc.UpdateTenant(ctx, platformAdm, "upshs", domain.TenantUpdate{Status: &badStatus}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid status should be validation error, got %v", err)
	}
	badPlan := "ultimate"
	if _, err := svc.UpdateTenant(ctx, platformAdm, "upshs", domain.TenantUpdate{Plan: &badPlan}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid plan should be validation error, got %v", err)
	}
	domainHost := "upshs.edu.gh"
	if _, err := svc.UpdateTenant(ctx, platformAdm, "upshs", domain.TenantUpdate{Domain: &domainHost}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("generic update must not bypass domain verification, got %v", err)
	}
}

func TestResolveTenantBySubdomain(t *testing.T) {
	svc := newSvc()
	if _, err := svc.ResolveTenant(ctx, "", ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty lookup should be validation error, got %v", err)
	}
	if _, err := svc.ResolveTenant(ctx, "", "no-such-tenant"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown subdomain should be not found, got %v", err)
	}
	if _, err := svc.ResolveTenant(ctx, "", "cape-coast-prep"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("onboarding tenant must not resolve publicly, got %v", err)
	}
	got, err := svc.ResolveTenant(ctx, "", "upshs")
	if err != nil {
		t.Fatalf("resolve upshs subdomain: %v", err)
	}
	if got.Code != "upshs" {
		t.Fatalf("resolved code = %q, want upshs", got.Code)
	}
}

func TestResolveTenantByDomain(t *testing.T) {
	resolver := &txtResolver{records: map[string][]string{}}
	svc := application.NewService(memory.New(), application.WithTXTResolver(resolver), application.WithDomainTokenGenerator(func() (string, error) {
		return strings.Repeat("a", 64), nil
	}))
	domainHost := "UPSHS.edu.gh."
	registration, err := svc.RequestCustomDomain(ctx, upshsAdmin, "upshs", domainHost)
	if err != nil {
		t.Fatalf("request custom domain: %v", err)
	}
	if registration.Hostname != "upshs.edu.gh" || registration.Status != domain.DomainPending || registration.VerificationToken == "" {
		t.Fatalf("unexpected registration: %+v", registration)
	}
	if _, err := svc.ResolveTenant(ctx, registration.Hostname, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unverified domain must not resolve, got %v", err)
	}
	if _, err := svc.VerifyCustomDomain(ctx, upshsAdmin, "upshs"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("missing TXT value must not verify, got %v", err)
	}
	resolver.records[registration.TXTRecordName] = []string{"unrelated", registration.VerificationToken}
	verified, err := svc.VerifyCustomDomain(ctx, upshsAdmin, "upshs")
	if err != nil || verified.Status != domain.DomainVerified || verified.VerifiedAt == nil {
		t.Fatalf("verify custom domain: %+v err=%v", verified, err)
	}
	read, err := svc.GetCustomDomain(ctx, upshsAdmin, "upshs")
	if err != nil || read.VerificationToken != "" {
		t.Fatalf("stored challenge must never expose its token: %+v err=%v", read, err)
	}
	if _, err := svc.ActivateCustomDomain(ctx, upshsAdmin, "upshs", "render-domain-123"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin must not attest provider TLS, got %v", err)
	}
	active, err := svc.ActivateCustomDomain(ctx, platformAdm, "upshs", "render-domain-123")
	if err != nil || active.Status != domain.DomainActive || active.ActivatedAt == nil {
		t.Fatalf("activate custom domain: %+v err=%v", active, err)
	}
	visible, err := svc.GetCustomDomain(ctx, upshsAdmin, "upshs")
	if err != nil || visible.ProviderReference != "" {
		t.Fatalf("tenant-facing domain state leaked provider reference: %+v err=%v", visible, err)
	}
	got, err := svc.ResolveTenant(ctx, registration.Hostname, "")
	if err != nil {
		t.Fatalf("resolve by domain: %v", err)
	}
	if got.Code != "upshs" {
		t.Fatalf("resolved code = %q, want upshs", got.Code)
	}
	if _, err := svc.DeactivateCustomDomain(ctx, upshsAdmin, "upshs", "render-removal-123"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin must not attest provider removal, got %v", err)
	}
	inactive, err := svc.DeactivateCustomDomain(ctx, platformAdm, "upshs", "render-removal-123")
	if err != nil || inactive.Status != domain.DomainInactive || inactive.DeactivatedAt == nil {
		t.Fatalf("deactivate custom domain: %+v err=%v", inactive, err)
	}
	if _, err := svc.ResolveTenant(ctx, registration.Hostname, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("inactive domain must not resolve, got %v", err)
	}
}

func TestCustomDomainSecurityAndValidation(t *testing.T) {
	svc := newSvc()
	if _, err := svc.RequestCustomDomain(ctx, upshsUser, "upshs", "school.edu.gh"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("teacher must not request domain, got %v", err)
	}
	if _, err := svc.RequestCustomDomain(ctx, upshsAdmin, "aboom-ame-zion-c", "school.edu.gh"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant request must be forbidden, got %v", err)
	}
	aboomAdmin := auth.Actor{UserID: "a2", TenantID: "aboom-ame-zion-c", Role: "school_admin", Permissions: []string{"features.manage"}}
	if _, err := svc.RequestCustomDomain(ctx, aboomAdmin, "aboom-ame-zion-c", "school.edu.gh"); !errors.Is(err, domain.ErrEntitlement) {
		t.Fatalf("starter plan must not request custom domain, got %v", err)
	}
	for _, invalid := range []string{"localhost", "127.0.0.1", "evil..edu.gh", "school.auraedu.com", "https://school.edu.gh"} {
		if _, err := domain.NormalizeCustomDomain(invalid); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("invalid domain %q accepted: %v", invalid, err)
		}
	}
}

func TestDeleteTenantRequiresPlatformAdmin(t *testing.T) {
	svc := newSvc()
	if err := svc.DeleteTenant(ctx, upshsAdmin, "upshs"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin should not delete tenant, got %v", err)
	}
	if err := svc.DeleteTenant(ctx, platformAdm, "no-such-tenant"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown tenant should be not found, got %v", err)
	}
	if err := svc.DeleteTenant(ctx, platformAdm, "upshs"); err != nil {
		t.Fatalf("platform admin should delete tenant: %v", err)
	}
	if _, err := svc.GetTenant(ctx, platformAdm, "upshs"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted tenant should be not found, got %v", err)
	}
}

func enabled(t *testing.T, svc *application.Service, actor auth.Actor, code, key string) bool {
	t.Helper()
	fs, err := svc.Features(ctx, actor, code)
	if err != nil {
		t.Fatalf("Features(%q): %v", code, err)
	}
	for _, f := range fs {
		if f.Key == key {
			return f.Enabled
		}
	}
	t.Fatalf("feature %q not in catalog", key)
	return false
}
