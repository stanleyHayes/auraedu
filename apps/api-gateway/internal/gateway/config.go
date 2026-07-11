// Package gateway implements the api-gateway reverse proxy, middleware, and route registry.
package gateway

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/platform/config"
)

type ServiceRegistry []Route

type Route struct {
	Prefix      string
	Target      string
	FeatureKey  string
	Permission  string
	Permissions map[string]string
	Public      bool
}

func DefaultRegistry() ServiceRegistry {
	return ServiceRegistry{
		{Prefix: "/api/v1/identity", Target: envURL("SERVICE_IDENTITY_URL", "http://localhost:8081"), FeatureKey: "", Public: true},
		{Prefix: "/api/v1/tenants", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), FeatureKey: "billing", Public: false},
		{Prefix: "/api/v1/super-admin", Target: envURL("SERVICE_TENANT_URL", "http://localhost:8082"), FeatureKey: "billing", Public: false},
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
		{Prefix: "/api/v1/attendance", Target: envURL("SERVICE_ATTENDANCE_URL", "http://localhost:8092"), FeatureKey: "attendance"},
		{Prefix: "/api/v1/assessments", Target: envURL("SERVICE_ASSESSMENT_URL", "http://localhost:8093"), FeatureKey: "assessments"},
		{
			Prefix:     "/api/v1/academic",
			Target:     envURL("SERVICE_ACADEMIC_URL", "http://localhost:8092"),
			FeatureKey: "academic_management",
			Permissions: map[string]string{
				http.MethodGet:  "academic.read",
				http.MethodPost: "academic.manage",
			},
		},
		{Prefix: "/api/v1/fees", Target: envURL("SERVICE_FEES_URL", "http://localhost:8095"), FeatureKey: "fees"},
		{Prefix: "/api/v1/payments", Target: envURL("SERVICE_PAYMENT_URL", "http://localhost:8096"), FeatureKey: "online_payments"},
		{Prefix: "/api/v1/notifications", Target: envURL("SERVICE_NOTIFICATION_URL", "http://localhost:8097"), FeatureKey: "email_notifications"},
		{Prefix: "/api/v1/files/webhook", Target: envURL("SERVICE_FILE_URL", "http://localhost:8098"), Public: true},
		{Prefix: "/api/v1/files", Target: envURL("SERVICE_FILE_URL", "http://localhost:8098"), FeatureKey: "file_management", Permissions: map[string]string{
			http.MethodGet:    "files.read",
			http.MethodPost:   "files.upload",
			http.MethodPatch:  "files.update",
			http.MethodDelete: "files.delete",
		}},
		{Prefix: "/api/v1/uploads", Target: envURL("SERVICE_FILE_URL", "http://localhost:8098"), FeatureKey: "file_management", Permissions: map[string]string{
			http.MethodPost: "files.upload",
		}},
		{Prefix: "/api/v1/website", Target: envURL("SERVICE_WEBSITE_URL", "http://localhost:8099"), FeatureKey: "public_website"},
		{Prefix: "/api/v1/analytics", Target: envURL("SERVICE_ANALYTICS_URL", "http://localhost:8100"), FeatureKey: "analytics"},
		{Prefix: "/api/v1/billing", Target: envURL("SERVICE_BILLING_URL", "http://localhost:8101"), FeatureKey: "billing"},
		{Prefix: "/api/v1/cbt", Target: envURL("SERVICE_CBT_URL", "http://localhost:8102"), FeatureKey: "cbt_exams"},
		{Prefix: "/api/v1/audit", Target: envURL("SERVICE_AUDIT_URL", "http://localhost:8103"), FeatureKey: ""},
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
		return v
	}
	return fallback
}

type Config struct {
	Port            int
	SigningKey      []byte
	RedisURL        string
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
	return &Config{
		Port:        config.Port(8080),
		SigningKey:  []byte(key),
		RedisURL:    config.Getenv("REDIS_URL", "redis://localhost:6379"),
		CORSOrigins: splitList(config.Getenv("GATEWAY_CORS_ORIGINS", "*")),
		CORSMethods: splitList(config.Getenv("GATEWAY_CORS_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")),
		CORSHeaders: splitList(config.Getenv(
			"GATEWAY_CORS_HEADERS",
			"Authorization,Content-Type,X-Request-Id,X-Tenant-ID,X-Actor-User,X-Actor-Tenant,X-Actor-Role,X-Actor-Permissions",
		)),
		RateLimitRPS:    rps,
		RateLimitBurst:  burst,
		RateLimitWindow: time.Second,
		Registry:        DefaultRegistry(),
	}, nil
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
		if strings.HasPrefix(path, rt.Prefix) {
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
