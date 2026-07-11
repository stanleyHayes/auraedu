package unit

import (
	"errors"
	"testing"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

func newSvc() *application.Service { return application.NewService(memory.New()) }

var (
	anon        = auth.Actor{}
	platformAdm = auth.Actor{UserID: "p1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	upshsAdmin  = auth.Actor{UserID: "u1", TenantID: "upshs", Role: "school_admin", Permissions: []string{"features.manage"}}
	upshsUser   = auth.Actor{UserID: "u2", TenantID: "upshs", Role: "teacher"}
	aboomUser   = auth.Actor{UserID: "a1", TenantID: "aboom-ame-zion-c", Role: "teacher"}
)

func TestListTenantsRequiresPlatformAdmin(t *testing.T) {
	svc := newSvc()
	if _, err := svc.ListTenants(anon); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("anon should be forbidden, got %v", err)
	}
	if _, err := svc.ListTenants(upshsAdmin); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("school admin should not list all tenants, got %v", err)
	}
	got, err := svc.ListTenants(platformAdm)
	if err != nil || len(got) != 3 {
		t.Fatalf("platform admin should list 3 tenants, got %d err %v", len(got), err)
	}
}

func TestBrandingIsPublic(t *testing.T) {
	b, err := newSvc().Branding("upshs")
	if err != nil {
		t.Fatalf("branding should be public: %v", err)
	}
	if b.Brand.Primary != "#7B1113" {
		t.Fatalf("upshs brand = %q, want #7B1113", b.Brand.Primary)
	}
}

func TestGetTenantTenantScope(t *testing.T) {
	svc := newSvc()
	if _, err := svc.GetTenant(upshsUser, "upshs"); err != nil {
		t.Fatalf("own tenant should be readable: %v", err)
	}
	if _, err := svc.GetTenant(upshsUser, "aboom-ame-zion-c"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant read should be forbidden, got %v", err)
	}
	if _, err := svc.GetTenant(platformAdm, "aboom-ame-zion-c"); err != nil {
		t.Fatalf("platform admin should read any tenant: %v", err)
	}
}

func TestFeaturesTenantScope(t *testing.T) {
	svc := newSvc()
	if _, err := svc.Features(upshsUser, ""); !errors.Is(err, domain.ErrNoTenant) {
		t.Fatalf("empty tenant should be ErrNoTenant, got %v", err)
	}
	if _, err := svc.Features(aboomUser, "upshs"); !errors.Is(err, domain.ErrForbidden) {
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
	if _, err := svc.SetFeature(upshsUser, "upshs", "analytics", false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("teacher should not manage features, got %v", err)
	}
	// school admin may manage own tenant (analytics is professional; upshs is ai_plus → allowed)
	if _, err := svc.SetFeature(upshsAdmin, "upshs", "analytics", true); err != nil {
		t.Fatalf("school admin should manage own tenant's in-plan feature: %v", err)
	}
	// school admin may NOT manage another tenant
	if _, err := svc.SetFeature(upshsAdmin, "aboom-ame-zion-c", "analytics", true); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-tenant management should be forbidden, got %v", err)
	}
}

func TestSetFeatureEntitlement(t *testing.T) {
	svc := newSvc()
	// Aboom is on the 'starter' plan; ai_recommendations needs 'ai_plus'. Even a platform
	// admin cannot ENABLE it above the plan (spec §3.3: billing controls entitlement).
	if _, err := svc.SetFeature(platformAdm, "aboom-ame-zion-c", "ai_recommendations", true); !errors.Is(err, domain.ErrEntitlement) {
		t.Fatalf("enabling above plan should be ErrEntitlement, got %v", err)
	}
	// Disabling is always allowed.
	if _, err := svc.SetFeature(platformAdm, "aboom-ame-zion-c", "ai_recommendations", false); err != nil {
		t.Fatalf("disabling should be allowed: %v", err)
	}
	// Unknown key.
	if _, err := svc.SetFeature(platformAdm, "upshs", "not_a_feature", true); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unknown key should be ErrValidation, got %v", err)
	}
}

func enabled(t *testing.T, svc *application.Service, actor auth.Actor, code, key string) bool {
	t.Helper()
	fs, err := svc.Features(actor, code)
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
