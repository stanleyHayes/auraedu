package gateway

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/api-gateway/internal/mocks"
	"github.com/auraedu/api-gateway/internal/stubs"
	"github.com/auraedu/platform/auth"
)

func testBuilder() *Builder {
	cfg := &Config{
		Port:           8080,
		SigningKey:     []byte("test-signing-key"),
		CORSOrigins:    []string{"*"},
		CORSMethods:    []string{"GET", "POST", "OPTIONS"},
		CORSHeaders:    []string{"Authorization", "Content-Type", "X-Request-Id", "X-Tenant-ID"},
		RateLimitRPS:   10,
		RateLimitBurst: 20,
	}
	cfg.Registry = ServiceRegistry{
		{Prefix: "/api/v1/identity", Target: "http://localhost:8081", Public: true},
		{Prefix: "/api/v1/students", Target: "http://localhost:8090", FeatureKey: "student_management"},
		{Prefix: "/api/v1/cbt", Target: "http://localhost:8102", FeatureKey: "cbt_exams"},
	}
	proxy, _ := NewReverseProxy(cfg.Registry, slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	return &Builder{
		Log:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		Config:   cfg,
		Registry: cfg.Registry,
		Proxy:    proxy,
		Tenant: &stubs.TenantResolver{
			BySubdomain: map[string]string{"upshs": "upshs", "aboom": "aboom"},
		},
		Flags: &stubs.FeatureFlagClient{
			Defaults: map[string]bool{"student_management": true},
			TenantOverrides: map[string]map[string]bool{
				"upshs": {"cbt_exams": true},
				"aboom": {"cbt_exams": false},
			},
		},
	}
}

func signTestToken(b *Builder, claims auth.Claims) string {
	claims.ExpiresAt = time.Now().Add(time.Hour).Unix()
	token, err := auth.Sign(claims, b.Config.SigningKey)
	if err != nil {
		panic(err)
	}
	return token
}

func TestRequestIDGeneratedAndPropagated(t *testing.T) {
	b := testBuilder()
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if RequestIDFrom(r.Context()) == "" {
			t.Error("expected request id in context")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/1", nil)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("handler not called")
	}
	if rr.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected X-Request-Id response header")
	}
}

func TestCORSPreflight(t *testing.T) {
	b := testBuilder()
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/students/1", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("cors origin: got %q", got)
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	b := testBuilder()
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler without auth")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/1", nil)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthAllowsPublicRoute(t *testing.T) {
	b := testBuilder()
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/login", nil)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("public route should reach handler")
	}
}

func TestAuthAcceptsValidToken(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:  "u1",
		TenantID: "upshs",
		Role:     "teacher",
	})

	var actor ActorContext
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor = ActorFrom(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if actor.UserID != "u1" {
		t.Fatalf("actor user id: got %q, want u1", actor.UserID)
	}
}

func TestTenantRequired(t *testing.T) {
	b := testBuilder()
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler without tenant")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "tenant_required") {
		t.Fatalf("expected tenant_required error, got %q", body)
	}
}

func TestTenantResolvedFromSubdomain(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "upshs", Role: "teacher"})
	var tenant string
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant = TenantIDFrom(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "http://upshs.auraedu.test/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if tenant != "upshs" {
		t.Fatalf("tenant: got %q, want upshs", tenant)
	}
}

func TestFeatureFlagDisabled(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "aboom", Role: "teacher"})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler when feature disabled")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cbt/exams", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "aboom")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
	if !strings.Contains(rr.Body.String(), "feature_disabled") {
		t.Fatalf("expected feature_disabled error")
	}
}

func TestRateLimitBlocksWhenExhausted(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "upshs", Role: "teacher"})
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return false, nil }}
	b.RateLimiter = &TokenBucket{Store: store, RPS: 1, Burst: 1}

	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler when rate limited")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}
