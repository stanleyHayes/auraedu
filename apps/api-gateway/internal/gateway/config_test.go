package gateway

import (
	"net/http"
	"os"
	"testing"
)

func TestDefaultRegistryMatchesKnownRoutes(t *testing.T) {
	reg := DefaultRegistry()
	cases := []struct {
		path    string
		wantPre string
	}{
		{"/api/v1/auth/login", "/api/v1/auth"},
		{"/api/v1/students/123", "/api/v1/students"},
		{"/api/v1/ai/recommendations/run", "/api/v1/ai/recommendations"},
		{"/api/v1/roles", "/api/v1/roles"},
		{"/api/v1/permissions", "/api/v1/permissions"},
		{"/api/v1/report-cards/rc1", "/api/v1/report-cards"},
		{"/api/v1/report-templates/rt1", "/api/v1/report-templates"},
		{"/api/v1/fee-structures/fs1", "/api/v1/fee-structures"},
		{"/api/v1/invoices/inv1", "/api/v1/invoices"},
		{"/api/v1/messages/m1", "/api/v1/messages"},
		{"/api/v1/notification-templates/nt1", "/api/v1/notification-templates"},
		{"/api/v1/notification-subscriptions/ns1", "/api/v1/notification-subscriptions"},
		{"/api/v1/transactions/tx1", "/api/v1/transactions"},
		{"/api/v1/webhook-events/we1", "/api/v1/webhook-events"},
		{"/api/v1/webhooks/paystack", "/api/v1/webhooks"},
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
	if err := os.Unsetenv("JWT_SIGNING_KEY"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
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

// TestDefaultRegistryServicePorts pins the default targets to the service port
// registry in agent_plan.md Appendix D (academic 8092, attendance 8094, ...).
func TestDefaultRegistryServicePorts(t *testing.T) {
	for _, key := range []string{
		"SERVICE_IDENTITY_URL", "SERVICE_TENANT_URL", "SERVICE_STUDENT_URL",
		"SERVICE_STAFF_URL", "SERVICE_ACADEMIC_URL", "SERVICE_FILE_URL",
		"SERVICE_ATTENDANCE_URL", "SERVICE_ASSESSMENT_URL", "SERVICE_REPORT_URL",
		"SERVICE_FEES_URL", "SERVICE_PAYMENT_URL", "SERVICE_NOTIFICATION_URL",
		"SERVICE_BILLING_URL", "SERVICE_WEBSITE_URL", "SERVICE_ANALYTICS_URL",
		"SERVICE_CBT_URL", "SERVICE_AUDIT_URL",
	} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
	}

	reg := DefaultRegistry()
	cases := []struct {
		prefix string
		target string
	}{
		{"/api/v1/roles", "http://localhost:8081"},
		{"/api/v1/permissions", "http://localhost:8081"},
		{"/api/v1/attendance", "http://localhost:8094"},
		{"/api/v1/assessments", "http://localhost:8095"},
		{"/api/v1/academic", "http://localhost:8092"},
		{"/api/v1/report-cards", "http://localhost:8096"},
		{"/api/v1/report-templates", "http://localhost:8096"},
		{"/api/v1/fees", "http://localhost:8097"},
		{"/api/v1/fee-structures", "http://localhost:8097"},
		{"/api/v1/invoices", "http://localhost:8097"},
		{"/api/v1/payments", "http://localhost:8098"},
		{"/api/v1/transactions", "http://localhost:8098"},
		{"/api/v1/webhook-events", "http://localhost:8098"},
		{"/api/v1/webhooks", "http://localhost:8098"},
		{"/api/v1/notifications", "http://localhost:8099"},
		{"/api/v1/messages", "http://localhost:8099"},
		{"/api/v1/notification-templates", "http://localhost:8099"},
		{"/api/v1/notification-subscriptions", "http://localhost:8099"},
		{"/api/v1/files", "http://localhost:8093"},
		{"/api/v1/uploads", "http://localhost:8093"},
		{"/api/v1/website", "http://localhost:8101"},
		{"/api/v1/analytics", "http://localhost:8102"},
		{"/api/v1/billing", "http://localhost:8100"},
		{"/api/v1/cbt", "http://localhost:8103"},
		{"/api/v1/audit", "http://localhost:8104"},
	}
	for _, tc := range cases {
		rt, ok := reg.Match(tc.prefix)
		if !ok {
			t.Fatalf("expected match for %s", tc.prefix)
		}
		if rt.Target != tc.target {
			t.Errorf("%s target: got %q, want %q", tc.prefix, rt.Target, tc.target)
		}
	}
}

// TestDefaultRegistryRoutePolicies pins the feature-flag, public, and permission
// mappings for routes added for report/fees/payment/notification/identity services.
func TestDefaultRegistryRoutePolicies(t *testing.T) {
	reg := DefaultRegistry()
	cases := []struct {
		prefix  string
		feature string
		public  bool
		perms   map[string]string
	}{
		{"/api/v1/report-cards", "report_cards", false, map[string]string{
			http.MethodGet: "reports.read", http.MethodPost: "reports.publish",
			http.MethodPatch: "reports.publish", http.MethodDelete: "reports.publish",
		}},
		{"/api/v1/report-templates", "report_cards", false, map[string]string{
			http.MethodGet: "reports.read", http.MethodPost: "reports.publish",
			http.MethodPatch: "reports.publish", http.MethodDelete: "reports.publish",
		}},
		{"/api/v1/fee-structures", "fees", false, map[string]string{
			http.MethodGet: "fees.read", http.MethodPost: "fees.manage",
			http.MethodPatch: "fees.manage", http.MethodDelete: "fees.manage",
		}},
		{"/api/v1/invoices", "fees", false, map[string]string{
			http.MethodGet: "fees.read", http.MethodPost: "fees.manage",
			http.MethodPatch: "fees.manage", http.MethodDelete: "fees.manage",
		}},
		{"/api/v1/transactions", "online_payments", false, map[string]string{
			http.MethodGet: "payments.read",
		}},
		{"/api/v1/webhook-events", "online_payments", false, map[string]string{
			http.MethodGet: "payments.read", http.MethodPost: "payments.initiate",
		}},
		{"/api/v1/webhooks", "", true, nil},
		{"/api/v1/messages", "email_notifications", false, map[string]string{
			http.MethodGet: "notifications.read", http.MethodPost: "notifications.send",
			http.MethodPatch: "notifications.manage", http.MethodDelete: "notifications.manage",
		}},
		{"/api/v1/notification-templates", "email_notifications", false, map[string]string{
			http.MethodGet: "notifications.read", http.MethodPost: "notifications.manage",
			http.MethodPatch: "notifications.manage", http.MethodDelete: "notifications.manage",
		}},
		{"/api/v1/notification-subscriptions", "email_notifications", false, map[string]string{
			http.MethodGet: "notifications.read", http.MethodPost: "notifications.manage",
			http.MethodPatch: "notifications.manage", http.MethodDelete: "notifications.manage",
		}},
		{"/api/v1/roles", "", false, map[string]string{http.MethodGet: "users.read"}},
		{"/api/v1/permissions", "", false, map[string]string{http.MethodGet: "users.read"}},
	}
	for _, tc := range cases {
		rt, ok := reg.Match(tc.prefix)
		if !ok {
			t.Fatalf("expected match for %s", tc.prefix)
		}
		if rt.Prefix != tc.prefix {
			t.Fatalf("%s matched prefix %q", tc.prefix, rt.Prefix)
		}
		if rt.FeatureKey != tc.feature {
			t.Errorf("%s feature: got %q, want %q", tc.prefix, rt.FeatureKey, tc.feature)
		}
		if rt.Public != tc.public {
			t.Errorf("%s public: got %v, want %v", tc.prefix, rt.Public, tc.public)
		}
		if len(rt.Permissions) != len(tc.perms) {
			t.Errorf("%s permissions: got %v, want %v", tc.prefix, rt.Permissions, tc.perms)
			continue
		}
		for method, want := range tc.perms {
			if got := rt.Permissions[method]; got != want {
				t.Errorf("%s %s permission: got %q, want %q", tc.prefix, method, got, want)
			}
		}
	}
}
