package gateway

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestDefaultRegistryMatchesKnownRoutes(t *testing.T) {
	reg := DefaultRegistry()
	cases := []struct {
		path    string
		wantPre string
	}{
		{"/api/v1/auth/login", "/api/v1/auth/login"},
		{"/api/v1/auth/mfa/verify", "/api/v1/auth/mfa/verify"},
		{"/api/v1/auth/me", "/api/v1/auth"},
		{"/api/v1/public/invites/token/accept", "/api/v1/public/invites"},
		{"/api/v1/public/onboarding-requests", "/api/v1/public/onboarding-requests"},
		{"/api/v1/public/billing/plans", "/api/v1/public/billing/plans"},
		{"/api/v1/public/callback-requests", "/api/v1/public/callback-requests"},
		{"/api/v1/public/programmes", "/api/v1/public/programmes"},
		{"/api/v1/programmes", "/api/v1/programmes"},
		{"/api/v1/intakes/example", "/api/v1/intakes"},
		{"/api/v1/callback-requests", "/api/v1/callback-requests"},
		{"/api/v1/students/123", "/api/v1/students"},
		{"/api/v1/academic-years", "/api/v1/academic-years"},
		{"/api/v1/ai/recommendations/run", "/api/v1/ai/recommendations"},
		{"/api/v1/roles", "/api/v1/roles"},
		{"/api/v1/permissions", "/api/v1/permissions"},
		{"/api/v1/report-cards/rc1", "/api/v1/report-cards"},
		{"/api/v1/report-templates/rt1", "/api/v1/report-templates"},
		{"/api/v1/fee-structures/fs1", "/api/v1/fee-structures"},
		{"/api/v1/invoices/inv1", "/api/v1/invoices"},
		{"/api/v1/timetable/entry1", "/api/v1/timetable"},
		{"/api/v1/messages/m1", "/api/v1/messages"},
		{"/api/v1/analytics/metrics", "/api/v1/analytics"},
		{"/api/v1/notification-templates/nt1", "/api/v1/notification-templates"},
		{"/api/v1/notification-subscriptions/ns1", "/api/v1/notification-subscriptions"},
		{"/api/v1/transactions/tx1", "/api/v1/transactions"},
		{"/api/v1/webhook-events/we1", "/api/v1/webhook-events"},
		{"/api/v1/webhooks/twilio", "/api/v1/webhooks/twilio"},
		{"/api/v1/webhooks/resend", "/api/v1/webhooks/resend"},
		{"/api/v1/email-preferences/unsubscribe", "/api/v1/email-preferences/unsubscribe"},
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
	if _, ok := reg.Match("/api/v1/students-attacker"); ok {
		t.Fatal("route matching must be segment-aware")
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
	t.Setenv("ENVIRONMENT", "development")
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

func TestLoadConfigRejectsWildcardAllCORSInProduction(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-key")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("GATEWAY_CORS_ORIGINS", "*")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected production wildcard-all CORS origin to fail")
	}
}

func TestLoadConfigRequiresHTTPSOriginsInProduction(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-key")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("GATEWAY_CORS_ORIGINS", "http://app.auraedu.com")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected plaintext production CORS origin to fail")
	}
}

func TestLoadConfigAcceptsOwnedHTTPSOriginsInProduction(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-key")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("GATEWAY_CORS_ORIGINS", "https://auraedu.com,https://*.auraedu.com")
	t.Setenv("GATEWAY_TRUSTED_PROXY", "render")

	if _, err := LoadConfig(); err != nil {
		t.Fatalf("expected owned HTTPS origins to pass: %v", err)
	}
}

func TestLoadConfigRequiresTrustedProxyInProduction(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-key")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("GATEWAY_CORS_ORIGINS", "https://auraedu.com")
	t.Setenv("GATEWAY_TRUSTED_PROXY", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected missing production trusted proxy to fail")
	}
}

func TestLoadConfigDoesNotExposeInternalActorHeadersToBrowsers(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-key")
	if err := os.Unsetenv("GATEWAY_CORS_HEADERS"); err != nil {
		t.Fatalf("unset headers: %v", err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	for _, header := range cfg.CORSHeaders {
		if strings.HasPrefix(strings.ToLower(header), "x-actor-") {
			t.Fatalf("internal trust header exposed through CORS: %s", header)
		}
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
		"SERVICE_CBT_URL", "SERVICE_AUDIT_URL", "SERVICE_CRM_URL",
		"SERVICE_ADMISSIONS_URL",
		"SERVICE_MARKET_INTELLIGENCE_URL",
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
		{"/api/v1/timetable", "http://localhost:8092"},
		{"/api/v1/report-cards", "http://localhost:8096"},
		{"/api/v1/report-templates", "http://localhost:8096"},
		{"/api/v1/fees", "http://localhost:8097"},
		{"/api/v1/fee-structures", "http://localhost:8097"},
		{"/api/v1/invoices", "http://localhost:8097"},
		{"/api/v1/payments", "http://localhost:8098"},
		{"/api/v1/transactions", "http://localhost:8098"},
		{"/api/v1/webhook-events", "http://localhost:8098"},
		{"/api/v1/webhooks/twilio", "http://localhost:8099"},
		{"/api/v1/webhooks/resend", "http://localhost:8099"},
		{"/api/v1/email-preferences/unsubscribe", "http://localhost:8099"},
		{"/api/v1/webhooks", "http://localhost:8098"},
		{"/api/v1/notifications", "http://localhost:8099"},
		{"/api/v1/messages", "http://localhost:8099"},
		{"/api/v1/communication-journeys", "http://localhost:8099"},
		{"/api/v1/notification-templates", "http://localhost:8099"},
		{"/api/v1/notification-subscriptions", "http://localhost:8099"},
		{"/api/v1/files", "http://localhost:8093"},
		{"/api/v1/uploads", "http://localhost:8093"},
		{"/api/v1/public/leads", "http://localhost:8105"},
		{"/api/v1/public/feedback", "http://localhost:8105"},
		{"/api/v1/public/callback-requests", "http://localhost:8105"},
		{"/api/v1/callback-requests", "http://localhost:8105"},
		{"/api/v1/public/programmes", "http://localhost:8114"},
		{"/api/v1/programmes", "http://localhost:8114"},
		{"/api/v1/intakes", "http://localhost:8114"},
		{"/api/v1/leads", "http://localhost:8105"},
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
		{"/api/v1/timetable", "timetable", false, map[string]string{
			http.MethodGet: "academic.read", http.MethodPost: "academic.manage",
			http.MethodPatch: "academic.manage", http.MethodDelete: "academic.manage",
		}},
		{"/api/v1/transactions", "online_payments", false, map[string]string{
			http.MethodGet: "payments.read",
		}},
		{"/api/v1/webhook-events", "online_payments", false, map[string]string{
			http.MethodGet: "payments.configure", http.MethodPost: "payments.configure",
		}},
		{"/api/v1/webhooks", "", true, nil},
		{"/api/v1/webhooks/twilio", "", true, nil},
		{"/api/v1/webhooks/resend", "", true, nil},
		{"/api/v1/email-preferences/unsubscribe", "", true, nil},
		{"/api/v1/messages", "email_notifications", false, map[string]string{
			http.MethodGet: "notifications.read", http.MethodPost: "notifications.send",
			http.MethodPatch: "notifications.manage", http.MethodDelete: "notifications.manage",
		}},
		{"/api/v1/communication-journeys", "growth_crm", false, map[string]string{
			http.MethodGet: "notifications.read", http.MethodPost: "notifications.manage",
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
		{"/api/v1/public/leads", "growth_crm", true, nil},
		{"/api/v1/public/feedback", "growth_crm", true, nil},
		{"/api/v1/public/callback-requests", "growth_crm", true, nil},
		{"/api/v1/public/programmes", "admissions", true, nil},
		{"/api/v1/programmes", "admissions", false, map[string]string{
			http.MethodGet: "admissions.application.read", http.MethodPost: "admissions.catalogue.manage", http.MethodPatch: "admissions.catalogue.manage",
		}},
		{"/api/v1/intakes", "admissions", false, map[string]string{http.MethodPatch: "admissions.catalogue.manage"}},
		{"/api/v1/callback-requests", "growth_crm", false, map[string]string{http.MethodGet: "crm.lead.read"}},
		{"/api/v1/public/onboarding-requests", "", true, nil},
		{"/api/v1/public/billing/plans", "", true, nil},
		{"/api/v1/auth/login", "", true, nil},
		{"/api/v1/auth/mfa/verify", "", true, nil},
		{"/api/v1/auth/refresh", "", true, nil},
		{"/api/v1/auth/forgot-password", "", true, nil},
		{"/api/v1/auth/reset-password", "", true, nil},
		{"/api/v1/auth/me", "", false, nil},
		{"/api/v1/leads", "growth_crm", false, map[string]string{
			http.MethodGet: "crm.lead.read", http.MethodPatch: "crm.lead.update",
		}},
	}
	for _, tc := range cases {
		rt, ok := reg.Match(tc.prefix)
		if !ok {
			t.Fatalf("expected match for %s", tc.prefix)
		}
		expectedPrefix := tc.prefix
		if tc.prefix == "/api/v1/auth/me" {
			expectedPrefix = "/api/v1/auth"
		}
		if rt.Prefix != expectedPrefix {
			t.Fatalf("%s matched prefix %q, want %q", tc.prefix, rt.Prefix, expectedPrefix)
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

func TestPublicCredentialRoutesRequireResolvedTenant(t *testing.T) {
	for _, path := range []string{"/api/v1/auth/forgot-password", "/api/v1/auth/reset-password", "/api/v1/auth/mfa/verify"} {
		route, ok := DefaultRegistry().Match(path)
		if !ok || !route.Public || route.TenantOptional {
			t.Fatalf("credential route must be public but tenant-bound: path=%s route=%+v", path, route)
		}
	}
}

func TestResendWebhookIsPublicTenantOptionalAndNotificationOwned(t *testing.T) {
	route, ok := DefaultRegistry().Match("/api/v1/webhooks/resend")
	if !ok || !route.Public || !route.TenantOptional || route.Target != "http://localhost:8099" {
		t.Fatalf("unexpected Resend webhook route: %+v", route)
	}
}

func TestTwilioWebhookIsPublicTenantOptionalAndNotificationOwned(t *testing.T) {
	route, ok := DefaultRegistry().Match("/api/v1/webhooks/twilio")
	if !ok || !route.Public || !route.TenantOptional || route.Target != "http://localhost:8099" {
		t.Fatalf("unexpected Twilio webhook route: %+v", route)
	}
}

func TestGrowthExecutiveAnalyticsUsesDedicatedPermission(t *testing.T) {
	route, ok := DefaultRegistry().Match("/api/v1/analytics/executive/growth")
	if !ok {
		t.Fatal("expected Growth executive analytics route")
	}
	if route.FeatureKey != "analytics" || route.Permission != "analytics.executive.read" {
		t.Fatalf("unexpected Growth executive route policy: %+v", route)
	}
	metrics, ok := DefaultRegistry().Match("/api/v1/analytics/metrics")
	if !ok || metrics.Permission != "analytics.view" {
		t.Fatalf("unexpected metrics route policy: %+v", metrics)
	}
	query, ok := DefaultRegistry().Match("/api/v1/analytics/executive/query")
	if !ok || query.Permission != "analytics.executive.read" {
		t.Fatalf("unexpected executive query policy: %+v", query)
	}
}
