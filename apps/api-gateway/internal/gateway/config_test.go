package gateway

import (
	"os"
	"testing"
)

func TestDefaultRegistryMatchesKnownRoutes(t *testing.T) {
	reg := DefaultRegistry()
	cases := []struct {
		path    string
		wantPre string
	}{
		{"/api/v1/identity/login", "/api/v1/identity"},
		{"/api/v1/students/123", "/api/v1/students"},
		{"/api/v1/ai/recommendations/run", "/api/v1/ai/recommendations"},
	}
	for _, tc := range cases {
		rt, ok := reg.Match(tc.path)
		if !ok {
			t.Fatalf("expected match for %s", tc.path)
		}
		if rt.Prefix != tc.wantPre {
			t.Fatalf("prefix: got %q, want %q", rt.Prefix, tc.wantPre)
		}
	}

	if _, ok := reg.Match("/unknown"); ok {
		t.Fatal("should not match unknown path")
	}
}

func TestRouteStripPrefix(t *testing.T) {
	rt := Route{Prefix: "/api/v1/students"}
	if got := rt.StripPrefix("/api/v1/students/123"); got != "/123" {
		t.Fatalf("strip: got %q, want %q", got, "/123")
	}
	if got := rt.StripPrefix("/api/v1/students"); got != "/" {
		t.Fatalf("strip exact prefix: got %q, want %q", got, "/")
	}
}

func TestLoadConfigRequiresSigningKey(t *testing.T) {
	os.Unsetenv("JWT_SIGNING_KEY")
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when JWT_SIGNING_KEY missing")
	}
}

func TestLoadConfigReadsEnv(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-key")
	t.Setenv("RATE_LIMIT_RPS", "50")
	t.Setenv("RATE_LIMIT_BURST", "100")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if string(cfg.SigningKey) != "test-key" {
		t.Fatalf("signing key mismatch")
	}
	if cfg.RateLimitRPS != 50 {
		t.Fatalf("rps: got %v, want 50", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 100 {
		t.Fatalf("burst: got %v, want 100", cfg.RateLimitBurst)
	}
}
