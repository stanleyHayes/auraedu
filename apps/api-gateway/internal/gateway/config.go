// Package gateway implements the api-gateway reverse proxy, middleware, and route registry.
package gateway

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/platform/config"
)

type ServiceRegistry []Route

type Route struct {
	Prefix         string
	Target         string
	FeatureKey     string
	Permission     string
	Permissions    map[string]string
	Public         bool
	TenantOptional bool
}

func DefaultRegistry() ServiceRegistry {
	// Default targets follow the service port registry in agent_plan.md Appendix D.
	return ServiceRegistry{
		{Prefix: "/api/v1/auth/login", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Public: true},
		{Prefix: "/api/v1/auth/mfa/verify", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Public: true},
		{Prefix: "/api/v1/auth/refresh", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Public: true},
		{Prefix: "/api/v1/auth/forgot-password", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Public: true},
		{Prefix: "/api/v1/auth/reset-password", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Public: true},
		{Prefix: "/api/v1/auth", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081")},
		{Prefix: "/api/v1/public/invites", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/users", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), FeatureKey: "", Permissions: map[string]string{
			http.MethodGet:    "users.read",
			http.MethodPost:   "users.create",
			http.MethodPut:    "users.update",
			http.MethodPatch:  "users.update",
			http.MethodDelete: "users.delete",
		}},
		{Prefix: "/api/v1/roles", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Permissions: map[string]string{
			http.MethodGet: "users.read",
		}},
		{Prefix: "/api/v1/permissions", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), Permissions: map[string]string{
			http.MethodGet: "users.read",
		}},
		{Prefix: "/api/v1/public/onboarding-requests", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/public/billing/plans", Target: envURL("SERVICE_BILLING_URL", "http://localhost:8100"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/super-admin/onboarding-requests", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), TenantOptional: true},
		{Prefix: "/api/v1/super-admin/tenants", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), TenantOptional: true},
		{Prefix: "/api/v1/super-admin/features", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), TenantOptional: true},
		{Prefix: "/api/v1/tenants", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), Public: true},
		{Prefix: "/api/v1/super-admin", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), FeatureKey: "billing", Public: false},
		{Prefix: "/api/v1/features", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), Public: true},
		{Prefix: "/api/v1/students", Target: envURL("SERVICE_STUDENT_URL", "http://localhost:8090"), FeatureKey: "student_management", Permissions: map[string]string{
			http.MethodGet:    "students.read",
			http.MethodPost:   "students.create",
			http.MethodPatch:  "students.update",
			http.MethodDelete: "students.delete",
		}},
		{
			Prefix:     "/api/v1/guardians",
			Target:     envURL("SERVICE_STUDENT_URL", "http://localhost:8090"),
			FeatureKey: "student_management",
			Permissions: map[string]string{
				http.MethodGet:    "students.read",
				http.MethodPost:   "students.create",
				http.MethodPatch:  "students.update",
				http.MethodDelete: "students.delete",
			},
		},
		{Prefix: "/api/v1/staff", Target: envURL("SERVICE_STAFF_URL", "http://localhost:8091"), FeatureKey: "staff_management"},
		{Prefix: "/api/v1/attendance", Target: envURL("SERVICE_ATTENDANCE_URL", "http://localhost:8094"), FeatureKey: "attendance"},
		{Prefix: "/api/v1/assessments", Target: envURL("SERVICE_ASSESSMENT_URL", "http://localhost:8095"), FeatureKey: "assessments"},
		{
			Prefix:     "/api/v1/assignments",
			Target:     envURL("SERVICE_ASSESSMENT_URL", "http://localhost:8095"),
			FeatureKey: "assignments",
			Permissions: map[string]string{
				http.MethodGet:    "assessments.read",
				http.MethodPost:   "assessments.manage",
				http.MethodPatch:  "assessments.manage",
				http.MethodDelete: "assessments.manage",
			},
		},
		{
			Prefix:     "/api/v1/gradebook",
			Target:     envURL("SERVICE_ASSESSMENT_URL", "http://localhost:8095"),
			FeatureKey: "assessments",
			Permissions: map[string]string{
				http.MethodGet: "assessments.read",
			},
		},
		{
			Prefix:     "/api/v1/academic",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "academic_management",
			Permissions: map[string]string{
				http.MethodGet:    "academic.read",
				http.MethodPost:   "academic.manage",
				http.MethodPatch:  "academic.manage",
				http.MethodDelete: "academic.manage",
			},
		},
		{
			Prefix:     "/api/v1/academic-years",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "academic_management",
			Permissions: map[string]string{
				http.MethodGet:    "academic.read",
				http.MethodPost:   "academic.manage",
				http.MethodPatch:  "academic.manage",
				http.MethodDelete: "academic.manage",
			},
		},
		{
			Prefix:     "/api/v1/terms",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "academic_management",
			Permissions: map[string]string{
				http.MethodGet:    "academic.read",
				http.MethodPost:   "academic.manage",
				http.MethodPatch:  "academic.manage",
				http.MethodDelete: "academic.manage",
			},
		},
		{
			Prefix:     "/api/v1/classes",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "academic_management",
			Permissions: map[string]string{
				http.MethodGet:    "academic.read",
				http.MethodPost:   "academic.manage",
				http.MethodPatch:  "academic.manage",
				http.MethodDelete: "academic.manage",
			},
		},
		{
			Prefix:     "/api/v1/subjects",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "academic_management",
			Permissions: map[string]string{
				http.MethodGet:    "academic.read",
				http.MethodPost:   "academic.manage",
				http.MethodPatch:  "academic.manage",
				http.MethodDelete: "academic.manage",
			},
		},
		{
			Prefix:     "/api/v1/timetable",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "timetable",
			Permissions: map[string]string{
				http.MethodGet:    "academic.read",
				http.MethodPost:   "academic.manage",
				http.MethodPatch:  "academic.manage",
				http.MethodDelete: "academic.manage",
			},
		},
		{
			Prefix:     "/api/v1/report-cards",
			Target:     envURL("SERVICE_REPORT_URL", "http://localhost:8096"),
			FeatureKey: "report_cards",
			Permissions: map[string]string{
				http.MethodGet:    "reports.read",
				http.MethodPost:   "reports.publish",
				http.MethodPatch:  "reports.publish",
				http.MethodDelete: "reports.publish",
			},
		},
		{
			Prefix:     "/api/v1/report-templates",
			Target:     envURL("SERVICE_REPORT_URL", "http://localhost:8096"),
			FeatureKey: "report_cards",
			Permissions: map[string]string{
				http.MethodGet:    "reports.read",
				http.MethodPost:   "reports.publish",
				http.MethodPatch:  "reports.publish",
				http.MethodDelete: "reports.publish",
			},
		},
		{Prefix: "/api/v1/fees", Target: envURL("SERVICE_FEES_URL", "http://localhost:8097"), FeatureKey: "fees"},
		{
			Prefix:     "/api/v1/fee-structures",
			Target:     envURL("SERVICE_FEES_URL", "http://localhost:8097"),
			FeatureKey: "fees",
			Permissions: map[string]string{
				http.MethodGet:    "fees.read",
				http.MethodPost:   "fees.manage",
				http.MethodPatch:  "fees.manage",
				http.MethodDelete: "fees.manage",
			},
		},
		{
			Prefix:     "/api/v1/invoices",
			Target:     envURL("SERVICE_FEES_URL", "http://localhost:8097"),
			FeatureKey: "fees",
			Permissions: map[string]string{
				http.MethodGet:    "fees.read",
				http.MethodPost:   "fees.manage",
				http.MethodPatch:  "fees.manage",
				http.MethodDelete: "fees.manage",
			},
		},
		{
			Prefix:     "/api/v1/balances",
			Target:     envURL("SERVICE_FEES_URL", "http://localhost:8097"),
			FeatureKey: "fees",
			Permissions: map[string]string{
				http.MethodGet: "fees.read",
			},
		},
		{
			Prefix:     "/api/v1/receipts",
			Target:     envURL("SERVICE_FEES_URL", "http://localhost:8097"),
			FeatureKey: "fees",
			Permissions: map[string]string{
				http.MethodGet: "fees.read",
			},
		},
		{Prefix: "/api/v1/payments", Target: envURL("SERVICE_PAYMENT_URL", "http://localhost:8098"), FeatureKey: "online_payments"},
		{
			Prefix:     "/api/v1/transactions",
			Target:     envURL("SERVICE_PAYMENT_URL", "http://localhost:8098"),
			FeatureKey: "online_payments",
			Permissions: map[string]string{
				http.MethodGet: "payments.read",
			},
		},
		{
			Prefix:     "/api/v1/webhook-events",
			Target:     envURL("SERVICE_PAYMENT_URL", "http://localhost:8098"),
			FeatureKey: "online_payments",
			Permissions: map[string]string{
				http.MethodGet:  "payments.configure",
				http.MethodPost: "payments.configure",
			},
		},
		// Provider webhooks carry no user JWT; their signatures are verified by
		// the owning service before any state change.
		{Prefix: "/api/v1/webhooks/twilio", Target: envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/webhooks/resend", Target: envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/webhooks", Target: envURL("SERVICE_PAYMENT_URL", "http://localhost:8098"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/email-preferences/unsubscribe", Target: envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"), Public: true, TenantOptional: true},
		{Prefix: "/api/v1/notifications", Target: envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"), FeatureKey: "email_notifications"},
		{
			Prefix:     "/api/v1/messages",
			Target:     envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"),
			FeatureKey: "email_notifications",
			Permissions: map[string]string{
				http.MethodGet:    "notifications.read",
				http.MethodPost:   "notifications.send",
				http.MethodPatch:  "notifications.manage",
				http.MethodDelete: "notifications.manage",
			},
		},
		{
			Prefix:     "/api/v1/communication-journeys",
			Target:     envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"),
			FeatureKey: "growth_crm",
			Permissions: map[string]string{
				http.MethodGet:  "notifications.read",
				http.MethodPost: "notifications.manage",
			},
		},
		{
			Prefix:     "/api/v1/notification-templates",
			Target:     envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"),
			FeatureKey: "email_notifications",
			Permissions: map[string]string{
				http.MethodGet:    "notifications.read",
				http.MethodPost:   "notifications.manage",
				http.MethodPatch:  "notifications.manage",
				http.MethodDelete: "notifications.manage",
			},
		},
		{
			Prefix:     "/api/v1/notification-subscriptions",
			Target:     envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"),
			FeatureKey: "email_notifications",
			Permissions: map[string]string{
				http.MethodGet:    "notifications.read",
				http.MethodPost:   "notifications.manage",
				http.MethodPatch:  "notifications.manage",
				http.MethodDelete: "notifications.manage",
			},
		},
		{
			Prefix:      "/api/v1/device-tokens",
			Target:      envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"),
			FeatureKey:  "notifications",
			Permissions: map[string]string{http.MethodPost: "notifications.read", http.MethodDelete: "notifications.read"},
		},
		{
			Prefix:     "/api/v1/announcements",
			Target:     envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8099"),
			FeatureKey: "announcements",
			Permissions: map[string]string{
				http.MethodGet:    "notifications.read",
				http.MethodPost:   "notifications.manage",
				http.MethodDelete: "notifications.manage",
			},
		},
		{Prefix: "/api/v1/files/webhook", Target: envURL("SERVICE_FILE_URL", "http://localhost:8093"), Public: true},
		{Prefix: "/api/v1/files", Target: envURL("SERVICE_FILE_URL", "http://localhost:8093"), FeatureKey: "file_management", Permissions: map[string]string{
			http.MethodGet:    "files.read",
			http.MethodPost:   "files.upload",
			http.MethodPatch:  "files.update",
			http.MethodDelete: "files.delete",
		}},
		{Prefix: "/api/v1/uploads", Target: envURL("SERVICE_FILE_URL", "http://localhost:8093"), FeatureKey: "file_management", Permissions: map[string]string{
			http.MethodPost: "files.upload",
		}},
		{Prefix: "/api/v1/public/leads", Target: envURL("SERVICE_CRM_URL", "http://localhost:8105"), FeatureKey: "growth_crm", Public: true},
		{Prefix: "/api/v1/public/feedback", Target: envURL("SERVICE_CRM_URL", "http://localhost:8105"), FeatureKey: "growth_crm", Public: true},
		{Prefix: "/api/v1/public/callback-requests", Target: envURL("SERVICE_CRM_URL", "http://localhost:8105"), FeatureKey: "growth_crm", Public: true},
		{Prefix: "/api/v1/public/assistant", Target: envURL("SERVICE_AI_ORCHESTRATOR_URL", "http://localhost:8111"), FeatureKey: "growth_website_chat", Public: true},
		// The orchestrator owns the exact action allowlist and independent-review
		// lifecycle; path-specific permissions are enforced in that service.
		{Prefix: "/api/v1/ai/actions", Target: envURL("SERVICE_AI_ORCHESTRATOR_URL", "http://localhost:8111"), FeatureKey: "growth_autonomous_actions"},
		{Prefix: "/api/v1/public/programmes", Target: envURL("SERVICE_ADMISSIONS_URL", "http://localhost:8114"), FeatureKey: "admissions", Public: true},
		{Prefix: "/api/v1/callback-requests", Target: envURL("SERVICE_CRM_URL", "http://localhost:8105"), FeatureKey: "growth_crm", Permissions: map[string]string{
			http.MethodGet: "crm.lead.read",
		}},
		{Prefix: "/api/v1/leads", Target: envURL("SERVICE_CRM_URL", "http://localhost:8105"), FeatureKey: "growth_crm", Permissions: map[string]string{
			http.MethodGet:   "crm.lead.read",
			http.MethodPatch: "crm.lead.update",
		}},
		{Prefix: "/api/v1/knowledge", Target: envURL(
			"SERVICE_KNOWLEDGE_URL", "http://localhost:8110",
		), FeatureKey: "growth_website_chat", Permissions: map[string]string{
			http.MethodGet:  "knowledge.read",
			http.MethodPost: "knowledge.manage",
		}},
		// Campaign actions use path-specific permissions (create, approve,
		// publish, budget approval) enforced by campaign-service. The gateway
		// still authenticates, resolves the tenant and gates the feature.
		{Prefix: "/api/v1/campaigns", Target: envURL("SERVICE_CAMPAIGN_URL", "http://localhost:8113"), FeatureKey: "growth_campaigns"},
		// Content owns brand policy, immutable creative versions and four-eyes
		// review. Path-specific generate/review permissions are enforced there.
		{Prefix: "/api/v1/content", Target: envURL("SERVICE_CONTENT_URL", "http://localhost:8116"), FeatureKey: "growth_content_ai"},
		// Applicant-owned and staff review paths carry different permissions;
		// admissions-service enforces the path-specific RBAC + ownership rules.
		{Prefix: "/api/v1/programmes", Target: envURL("SERVICE_ADMISSIONS_URL", "http://localhost:8114"), FeatureKey: "admissions", Permissions: map[string]string{
			http.MethodGet: "admissions.application.read", http.MethodPost: "admissions.catalogue.manage", http.MethodPatch: "admissions.catalogue.manage",
		}},
		{Prefix: "/api/v1/intakes", Target: envURL("SERVICE_ADMISSIONS_URL", "http://localhost:8114"), FeatureKey: "admissions", Permissions: map[string]string{
			http.MethodPatch: "admissions.catalogue.manage",
		}},
		{Prefix: "/api/v1/applications", Target: envURL("SERVICE_ADMISSIONS_URL", "http://localhost:8114"), FeatureKey: "admissions"},
		// Source kind determines the enterprise feature and each lifecycle action
		// has distinct permissions, both enforced by market-intelligence-service.
		{Prefix: "/api/v1/intelligence", Target: envURL("SERVICE_MARKET_INTELLIGENCE_URL", "http://localhost:8115")},
		{Prefix: "/api/v1/website", Target: envURL("SERVICE_WEBSITE_URL", "http://localhost:8101"), FeatureKey: "public_website"},
		{Prefix: "/api/v1/analytics/executive/growth", Target: envURL(
			"SERVICE_ANALYTICS_URL", "http://localhost:8102",
		), FeatureKey: "analytics", Permission: "analytics.executive.read"},
		{Prefix: "/api/v1/analytics/executive/query", Target: envURL(
			"SERVICE_ANALYTICS_URL", "http://localhost:8102",
		), FeatureKey: "analytics", Permission: "analytics.executive.read"},
		{Prefix: "/api/v1/analytics", Target: envURL("SERVICE_ANALYTICS_URL", "http://localhost:8102"), FeatureKey: "analytics", Permission: "analytics.view"},
		{Prefix: "/api/v1/billing", Target: envURL("SERVICE_BILLING_URL", "http://localhost:8100"), FeatureKey: "billing"},
		{Prefix: "/api/v1/cbt", Target: envURL("SERVICE_CBT_URL", "http://localhost:8103"), FeatureKey: "cbt_exams"},
		{
			Prefix:     "/api/v1/audit",
			Target:     envURL("SERVICE_AUDIT_URL", "http://localhost:8104"),
			FeatureKey: "",
			Permissions: map[string]string{
				http.MethodGet: "audit.read",
			},
		},
		{
			Prefix:     "/api/v1/ai/recommendations",
			Target:     envURL("SERVICE_AI_RECOMMENDATION_URL", "http://localhost:8200"),
			FeatureKey: "ai_recommendations",
			Permission: "ai.view_recommendations",
		},
		{
			Prefix:     "/api/v1/ai/predictions",
			Target:     envURL("SERVICE_AI_PREDICTION_URL", "http://localhost:8201"),
			FeatureKey: "ai_predictions",
			Permission: "ai.view_predictions",
		},
		{
			Prefix:     "/api/v1/ai/career-guidance",
			Target:     envURL("SERVICE_CAREER_GUIDANCE_URL", "http://localhost:8112"),
			FeatureKey: "career_guidance",
			Permission: "ai.view_guidance",
		},
	}
}

func envURL(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return withScheme(v)
	}
	return fallback
}

// withScheme defaults scheme-less targets (e.g. Render hostport values like
// "identity-service:8081") to http so url.Parse in the proxy doesn't reject
// them at runtime.
func withScheme(target string) string {
	if strings.Contains(target, "://") {
		return target
	}
	return "http://" + target
}

type Config struct {
	Port            int
	Environment     string
	SigningKey      []byte
	RedisURL        string
	TrustedProxy    string
	CORSOrigins     []string
	CORSMethods     []string
	CORSHeaders     []string
	RateLimitRPS    float64
	RateLimitBurst  int
	RateLimitWindow time.Duration
	Registry        ServiceRegistry
}

func LoadConfig() (*Config, error) {
	key, err := config.MustGetenv("JWT_SIGNING_KEY")
	if err != nil {
		return nil, err
	}
	rps, err := strconv.ParseFloat(config.Getenv("RATE_LIMIT_RPS", "20"), 64)
	if err != nil {
		rps = 20
	}
	burst, err := strconv.Atoi(config.Getenv("RATE_LIMIT_BURST", "40"))
	if err != nil {
		burst = int(rps * 2)
	}
	corsOrigins := splitList(config.Getenv("GATEWAY_CORS_ORIGINS", "*"))
	environment := strings.ToLower(config.Getenv("ENVIRONMENT", "development"))
	trustedProxy := strings.ToLower(config.Getenv("GATEWAY_TRUSTED_PROXY", ""))
	if environment == "production" {
		if err := validateProductionCORSOrigins(corsOrigins); err != nil {
			return nil, err
		}
		if trustedProxy != "render" {
			return nil, fmt.Errorf("GATEWAY_TRUSTED_PROXY must be render in production")
		}
	}
	return &Config{
		Port:         config.Port(8080),
		Environment:  environment,
		SigningKey:   []byte(key),
		RedisURL:     config.Getenv("REDIS_URL", "redis://localhost:6379"),
		TrustedProxy: trustedProxy,
		CORSOrigins:  corsOrigins,
		CORSMethods:  splitList(config.Getenv("GATEWAY_CORS_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
		CORSHeaders: splitList(config.Getenv(
			"GATEWAY_CORS_HEADERS",
			"Authorization,Content-Type,Idempotency-Key,X-Request-Id,X-Tenant-Code,X-Tenant-ID",
		)),
		RateLimitRPS:    rps,
		RateLimitBurst:  burst,
		RateLimitWindow: time.Second,
		Registry:        DefaultRegistry(),
	}, nil
}

func validateProductionCORSOrigins(origins []string) error {
	if len(origins) == 0 {
		return fmt.Errorf("GATEWAY_CORS_ORIGINS is required in production")
	}
	for _, origin := range origins {
		if origin == "*" {
			return fmt.Errorf("GATEWAY_CORS_ORIGINS must not contain a wildcard-all origin in production")
		}
		parsed, err := url.Parse(origin)
		validOrigin := err == nil && parsed.Scheme == "https" && parsed.Host != "" &&
			parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" &&
			(parsed.Path == "" || parsed.Path == "/")
		if !validOrigin {
			return fmt.Errorf("GATEWAY_CORS_ORIGINS contains an invalid production origin %q", origin)
		}
		hostname := parsed.Hostname()
		if strings.Contains(hostname, "*") && (!strings.HasPrefix(hostname, "*.") || strings.Count(hostname, "*") != 1) {
			return fmt.Errorf("GATEWAY_CORS_ORIGINS contains an invalid wildcard origin %q", origin)
		}
	}
	return nil
}

func corsOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}
	requestOrigin, err := url.Parse(origin)
	validOrigin := err == nil && requestOrigin.Scheme != "" && requestOrigin.Host != "" &&
		requestOrigin.Path == "" && requestOrigin.RawQuery == "" && requestOrigin.Fragment == ""
	if !validOrigin {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		pattern, err := url.Parse(allowed)
		if err != nil || pattern.Scheme != requestOrigin.Scheme || pattern.Port() != requestOrigin.Port() {
			continue
		}
		patternHost := pattern.Hostname()
		if !strings.HasPrefix(patternHost, "*.") {
			continue
		}
		baseHost := strings.TrimPrefix(patternHost, "*.")
		requestHost := requestOrigin.Hostname()
		if requestHost != baseHost && strings.HasSuffix(requestHost, "."+baseHost) {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (r ServiceRegistry) Match(path string) (Route, bool) {
	for _, rt := range r {
		if path == rt.Prefix || strings.HasPrefix(path, rt.Prefix+"/") {
			return rt, true
		}
	}
	return Route{}, false
}

func (rt Route) StripPrefix(path string) string {
	if path == rt.Prefix {
		return "/"
	}
	return strings.TrimPrefix(path, rt.Prefix)
}

type ConfigError struct {
	Msg string
}

func (e *ConfigError) Error() string { return fmt.Sprintf("gateway config: %s", e.Msg) }
