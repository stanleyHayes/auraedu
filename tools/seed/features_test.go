package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeatureDefaultsCoverCanonicalRegistry(t *testing.T) {
	registryPath := filepath.Join(repoRoot(), "contracts", "features", "features.yaml")

	for _, tenantCode := range []string{"upshs", "aboom"} {
		defaults, err := loadFeatureDefaults(registryPath, tenantCode)
		if err != nil {
			t.Fatalf("load defaults for %s: %v", tenantCode, err)
		}
		if len(defaults) < 40 {
			t.Fatalf("expected complete registry for %s, got %d features", tenantCode, len(defaults))
		}
		assertFeatureDefault(t, defaults, "push_notifications", true)
		assertFeatureDefault(t, defaults, "growth_crm", false)
		assertFeatureDefault(t, defaults, "growth_website_chat", false)
	}

	upshs, err := loadFeatureDefaults(registryPath, "upshs")
	if err != nil {
		t.Fatalf("load UPSHS defaults: %v", err)
	}
	assertFeatureDefault(t, upshs, "admissions", true)

	aboom, err := loadFeatureDefaults(registryPath, "aboom")
	if err != nil {
		t.Fatalf("load Aboom defaults: %v", err)
	}
	assertFeatureDefault(t, aboom, "admissions", false)
}

func TestFeatureDefaultsRejectMissingAndInvalidTenantValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "features.yaml")
	contents := []byte("features:\n  - key: public_website\n    defaults: { upshs: perhaps }\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write test registry: %v", err)
	}

	if _, err := loadFeatureDefaults(path, "upshs"); err == nil {
		t.Fatal("expected invalid default to fail")
	}
	if _, err := loadFeatureDefaults(path, "aboom"); err == nil {
		t.Fatal("expected missing tenant default to fail")
	}
}

func assertFeatureDefault(t *testing.T, defaults []featureDefault, key string, enabled bool) {
	t.Helper()
	for _, feature := range defaults {
		if feature.Key == key {
			if feature.Enabled != enabled {
				t.Fatalf("feature %s enabled=%v, want %v", key, feature.Enabled, enabled)
			}
			return
		}
	}
	t.Fatalf("feature %s missing from seed defaults", key)
}
