package unit

import (
	"testing"

	"github.com/auraedu/tenant-service/internal/adapters/memory"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

func newSvc() *application.Service { return application.NewService(memory.New()) }

func TestSeededTenants(t *testing.T) {
	svc := newSvc()
	if got := len(svc.ListTenants()); got != 3 {
		t.Fatalf("expected 3 seeded tenants, got %d", got)
	}
	upshs, err := svc.GetTenant("upshs")
	if err != nil {
		t.Fatalf("upshs not found: %v", err)
	}
	if upshs.Branding.Brand.Primary != "#7B1113" {
		t.Fatalf("upshs brand = %q, want #7B1113", upshs.Branding.Brand.Primary)
	}
}

func TestUnknownTenantNotFound(t *testing.T) {
	if _, err := newSvc().GetTenant("nope"); err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFeaturesRequireTenant(t *testing.T) {
	if _, err := newSvc().Features(""); err != domain.ErrNoTenant {
		t.Fatalf("expected ErrNoTenant, got %v", err)
	}
}

func TestFeatureFlagsPerTenant(t *testing.T) {
	svc := newSvc()
	// UPSHS has AI recommendations on; Aboom does not (spec §20).
	if !enabled(t, svc, "upshs", "ai_recommendations") {
		t.Fatal("expected ai_recommendations enabled for upshs")
	}
	if enabled(t, svc, "aboom-ame-zion-c", "ai_recommendations") {
		t.Fatal("expected ai_recommendations disabled for aboom")
	}
}

func TestSetFeatureToggles(t *testing.T) {
	svc := newSvc()
	f, err := svc.SetFeature("aboom-ame-zion-c", "ai_recommendations", true)
	if err != nil {
		t.Fatalf("SetFeature: %v", err)
	}
	if !f.Enabled {
		t.Fatal("flag should be enabled after SetFeature(true)")
	}
	if !enabled(t, svc, "aboom-ame-zion-c", "ai_recommendations") {
		t.Fatal("snapshot should reflect the toggle")
	}
	if _, err := svc.SetFeature("aboom-ame-zion-c", "not_a_feature", true); err != domain.ErrValidation {
		t.Fatalf("expected ErrValidation for unknown key, got %v", err)
	}
}

func enabled(t *testing.T, svc *application.Service, code, key string) bool {
	t.Helper()
	fs, err := svc.Features(code)
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
