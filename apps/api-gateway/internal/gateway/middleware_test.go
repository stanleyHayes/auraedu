package gateway

import (
	"bytes"
	"context"
	"errors"
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
		{Prefix: "/api/v1/public/onboarding-requests", Target: "http://localhost:8082", Public: true, TenantOptional: true},
		{Prefix: "/api/v1/public/invites", Target: "http://localhost:8081", Public: true, TenantOptional: true},
		{Prefix: "/api/v1/public/assistant", Target: "http://localhost:8111", Public: true, FeatureKey: "growth_website_chat"},
		{Prefix: "/api/v1/identity", Target: "http://localhost:8081", Public: true},
		{Prefix: "/api/v1/students", Target: "http://localhost:8090", FeatureKey: "student_management"},
		{Prefix: "/api/v1/cbt", Target: "http://localhost:8102", FeatureKey: "cbt_exams"},
		{Prefix: "/api/v1/ai/predictions", Target: "http://localhost:8201", FeatureKey: "ai_predictions", Permission: "ai.view_predictions"},
		{Prefix: "/api/v1/files", Target: "http://localhost:8098", FeatureKey: "file_management", Permissions: map[string]string{
			http.MethodGet: "files.read", http.MethodPost: "files.upload", http.MethodPatch: "files.update", http.MethodDelete: "files.delete",
		}},
		{Prefix: "/api/v1/uploads", Target: "http://localhost:8098", FeatureKey: "file_management", Permissions: map[string]string{
			http.MethodPost: "files.upload",
		}},
		{Prefix: "/api/v1/webhooks", Target: "http://localhost:8098", Public: true},
		{Prefix: "/api/v1/invoices", Target: "http://localhost:8097", FeatureKey: "fees", Permissions: map[string]string{
			http.MethodGet: "fees.read", http.MethodPost: "fees.manage",
		}},
		{Prefix: "/api/v1/messages", Target: "http://localhost:8099", FeatureKey: "email_notifications", Permissions: map[string]string{
			http.MethodGet: "notifications.read", http.MethodPost: "notifications.send",
		}},
	}
	proxy, err := NewReverseProxy(cfg.Registry, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		panic(err)
	}

	return &Builder{
		Log:      slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		Config:   cfg,
		Registry: cfg.Registry,
		Proxy:    proxy,
		Tenant: &stubs.TenantResolver{
			BySubdomain:         map[string]string{"upshs": "upshs", "aboom": "aboom"},
			SubdomainBaseDomain: "auraedu.test",
		},
		Flags: &stubs.FeatureFlagClient{
			Defaults: map[string]bool{"student_management": true, "ai_predictions": true, "file_management": true, "fees": true, "email_notifications": true, "growth_website_chat": true},
			TenantOverrides: map[string]map[string]bool{
				"upshs": {"cbt_exams": true},
				"aboom": {"cbt_exams": false, "email_notifications": false},
			},
		},
	}
}

func TestPublicInviteAcceptanceNeedsNoSessionOrTenantAndDoesNotLeakTokenToRateLimitStorage(t *testing.T) {
	b := testBuilder()
	var logs bytes.Buffer
	b.Log = slog.New(slog.NewJSONHandler(&logs, nil))
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return true, nil }}
	b.RateLimiter = &TokenBucket{Store: store, RPS: 1, Burst: 1}
	called := false
	handler := b.chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if !ActorFrom(r.Context()).IsEmpty() || TenantIDFrom(r.Context()) != "" {
			t.Fatal("invite acceptance invented an actor or tenant")
		}
	}))
	const secret = "secret-one-time-invite-token"
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/public/invites/"+secret+"/accept",
		strings.NewReader(`{"name":"Ama","password":"strong-password"}`),
	)
	request.RemoteAddr = "203.0.113.25:443"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !called || response.Code != http.StatusOK {
		t.Fatalf("public invite did not reach upstream: called=%v status=%d body=%s", called, response.Code, response.Body.String())
	}
	calls := store.Calls()
	if len(calls) != 1 || strings.Contains(calls[0], secret) || !strings.Contains(calls[0], "/api/v1/public/invites") {
		t.Fatalf("rate-limit storage exposed the token or missed canonical route: %v", calls)
	}
	if strings.Contains(logs.String(), secret) || !strings.Contains(logs.String(), `"path":"/api/v1/public/invites"`) {
		t.Fatalf("access log exposed the token or missed canonical route: %s", logs.String())
	}
}

func TestBuildUsesConfiguredReadinessChecks(t *testing.T) {
	b := testBuilder()
	health := NewHealth("api-gateway", "test")
	health.AddReadinessCheck("redis", func() error { return errors.New("redis unavailable") })
	b.Health = health

	recorder := httptest.NewRecorder()
	b.Build().ServeHTTP(
		recorder,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ready", nil),
	)

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"check":"redis"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicOnboardingSkipsTenantResolutionAndUsesIPRateLimit(t *testing.T) {
	b := testBuilder()
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return true, nil }}
	b.RateLimiter = &TokenBucket{Store: store, RPS: 1, Burst: 1}
	called := false
	handler := b.chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if tenant := TenantIDFrom(r.Context()); tenant != "" {
			t.Errorf("platform public route should not invent tenant context, got %q", tenant)
		}
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/onboarding-requests", strings.NewReader(`{}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.17")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if !called || rr.Code != http.StatusOK {
		t.Fatalf("public onboarding should reach handler: called=%v status=%d body=%s", called, rr.Code, rr.Body.String())
	}
	if calls := store.Calls(); len(calls) != 1 || !strings.Contains(calls[0], "public:203.0.113.17") {
		t.Fatalf("expected IP-scoped rate-limit key, got %v", calls)
	}
}

func TestPublicTenantRouteUsesTenantAndClientRateLimitKey(t *testing.T) {
	b := testBuilder()
	store := &mocks.RedisStore{AllowFunc: func(string) (bool, error) { return true, nil }}
	b.RateLimiter = &TokenBucket{Store: store, RPS: 1, Burst: 1}
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/assistant/messages", strings.NewReader(`{}`))
	req.Header.Set("X-Tenant-ID", "upshs")
	req.Header.Set("X-Forwarded-For", "203.0.113.44")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if calls := store.Calls(); len(calls) != 1 || !strings.Contains(calls[0], "upshs:public:203.0.113.44") {
		t.Fatalf("expected tenant+IP rate-limit key, got %v", calls)
	}
}

func TestRenderClientAddressIgnoresSpoofedForwardedFor(t *testing.T) {
	b := testBuilder()
	b.Config.TrustedProxy = "render"
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	req.Header.Set("CF-Connecting-IP", "198.51.100.24")

	if got := b.clientAddress(req); got != "198.51.100.24" {
		t.Fatalf("client address=%q, want Cloudflare address", got)
	}

	req.Header.Del("CF-Connecting-IP")
	if got := b.clientAddress(req); got != "192.0.2.10" {
		t.Fatalf("fallback address=%q, want direct peer instead of spoofable forwarding header", got)
	}
}

func TestPublicRouteFailsClosedWhenRateLimiterIsUnavailable(t *testing.T) {
	b := testBuilder()
	b.RateLimiter = &TokenBucket{Store: &mocks.RedisStore{AllowFunc: func(string) (bool, error) {
		return false, errors.New("redis unavailable")
	}}}
	handler := b.chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("public handler should not run") }))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/assistant/messages", strings.NewReader(`{}`))
	req.Header.Set("X-Tenant-ID", "upshs")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "rate_limiter_unavailable") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/identity/login", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/v1/students/1", nil)
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

func TestGatewaySetsSensitiveAPIResponseHeaders(t *testing.T) {
	b := testBuilder()
	b.Config.Environment = "production"
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/identity/login", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	want := map[string]string{
		"Cache-Control":             "no-store",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
		"Referrer-Policy":           "no-referrer",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
	}
	for header, expected := range want {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("%s=%q, want %q", header, got, expected)
		}
	}
}

func TestCORSAllowsOwnedTenantSubdomain(t *testing.T) {
	b := testBuilder()
	b.Config.CORSOrigins = []string{"https://auraedu.com", "https://*.auraedu.com"}

	if !b.allowOrigin("https://upshs.auraedu.com") {
		t.Fatal("expected owned tenant subdomain to be allowed")
	}
	if b.allowOrigin("https://auraedu.com.attacker.example") {
		t.Fatal("lookalike domain must not be allowed")
	}
	if b.allowOrigin("http://upshs.auraedu.com") {
		t.Fatal("scheme downgrade must not be allowed")
	}
}

func TestCORSAllowsOnlyExactVerifiedCustomDomain(t *testing.T) {
	b := testBuilder()
	b.Config.CORSOrigins = []string{"https://auraedu.com", "https://*.auraedu.com"}
	b.Tenant = &stubs.TenantResolver{ByHost: map[string]string{"school.edu.gh": "upshs"}}

	if !b.allowRequestOrigin(context.Background(), "https://school.edu.gh") {
		t.Fatal("verified exact custom domain should be allowed")
	}
	for _, origin := range []string{
		"https://upshs.attacker.example",
		"http://school.edu.gh",
		"https://school.edu.gh:8443",
		"https://school.edu.gh.attacker.example",
	} {
		if b.allowRequestOrigin(context.Background(), origin) {
			t.Fatalf("unverified custom origin %q was allowed", origin)
		}
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	b := testBuilder()
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler without auth")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/1", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/identity/login", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/1", nil)
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

func TestAuthRejectsInvalidTokenBeforeUnpermissionedRoute(t *testing.T) {
	b := testBuilder()
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer attacker-controlled-token")
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called || rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "unauthorized") {
		t.Fatalf("called=%v status=%d body=%s", called, rr.Code, rr.Body.String())
	}
}

func TestAuthRejectsInvalidOptionalTokenOnPublicRoute(t *testing.T) {
	b := testBuilder()
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/identity/login", nil)
	req.Header.Set("Authorization", "Bearer attacker-controlled-token")
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if called || rr.Code != http.StatusUnauthorized {
		t.Fatalf("called=%v status=%d body=%s", called, rr.Code, rr.Body.String())
	}
}

func TestTenantRejectsJWTClaimSwitchToAnotherSchool(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "upshs", Role: "teacher"})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("cross-tenant token must not reach handler")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "aboom")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "tenant_mismatch") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestTenantRequired(t *testing.T) {
	b := testBuilder()
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler without tenant")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "unauthorized") {
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://upshs.auraedu.test/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if tenant != "upshs" {
		t.Fatalf("tenant: got %q, want upshs", tenant)
	}
}

func TestTenantDoesNotResolveFromAttackerControlledFirstLabel(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "upshs", Role: "teacher"})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("attacker-controlled hostname reached tenant handler")
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://upshs.attacker.example/api/v1/students/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFeatureFlagDisabled(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "aboom", Role: "teacher"})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler when feature disabled")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/cbt/exams", nil)
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

func TestFeatureFlagGateFailsClosedWhenClientMissing(t *testing.T) {
	b := testBuilder()
	b.Flags = nil
	token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: "upshs", Role: "teacher"})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler without a feature-flag client")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/cbt/exams", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "feature_disabled") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFeatureFlagTenantMatrix(t *testing.T) {
	b := testBuilder()
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		tenant string
		want   int
	}{
		{tenant: "upshs", want: http.StatusNoContent},
		{tenant: "aboom", want: http.StatusForbidden},
	} {
		t.Run(tc.tenant, func(t *testing.T) {
			token := signTestToken(b, auth.Claims{Subject: "u1", TenantID: tc.tenant, Role: "teacher"})
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/cbt/exams", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Tenant-ID", tc.tenant)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rr.Code, tc.want, rr.Body.String())
			}
		})
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/1", nil)
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

func TestPermissionAllowsAuthorizedActor(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"ai.view_predictions"},
	})
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/ai/predictions/students/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("handler should be called for permitted actor")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPermissionDeniesUnauthorizedActor(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"students.read"},
	})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler without permission")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/ai/predictions/students/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
	if !strings.Contains(rr.Body.String(), "permission_denied") {
		t.Fatalf("expected permission_denied error, got %q", rr.Body.String())
	}
}

func TestPermissionSkippedForPublicRoute(t *testing.T) {
	b := testBuilder()
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/identity/login", nil)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("public route should not require permission")
	}
}

func TestPermissionAllowsPlatformAdmin(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:  "admin1",
		TenantID: "upshs",
		Role:     auth.RolePlatformSuperAdmin,
	})
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/ai/predictions/students/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("platform admin should bypass permission check")
	}
}

func TestPermissionRespectsMethodMap(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"files.read"},
	})
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/files/123", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("GET /files should be allowed with files.read")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestUploadsPermissionAllowsAuthorizedActor(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"files.upload"},
	})
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/uploads/signed", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("POST /uploads should be allowed with files.upload")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestUploadsPermissionDeniesUnauthorizedActor(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"files.read"},
	})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("POST /uploads should be denied without files.upload")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/uploads/signed", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
	if !strings.Contains(rr.Body.String(), "permission_denied") {
		t.Fatalf("expected permission_denied error, got %q", rr.Body.String())
	}
}

func TestPermissionMethodMapDeniesMissingPermission(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"files.read"},
	})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("POST /files should be denied without files.upload")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
	if !strings.Contains(rr.Body.String(), "permission_denied") {
		t.Fatalf("expected permission_denied error, got %q", rr.Body.String())
	}
}

func TestAuthAllowsPublicWebhookRoute(t *testing.T) {
	b := testBuilder()
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	// Payment provider webhooks carry no user JWT (same pattern as /api/v1/files/webhook).
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/webhooks/paystack", nil)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("public webhook route should reach handler without a token")
	}
}

func TestInvoicesPermissionAllowsReadActor(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "accountant",
		Permissions: []string{"fees.read"},
	})
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/invoices/inv1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("GET /invoices should be allowed with fees.read")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestInvoicesPermissionDeniesWriteWithoutManage(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "accountant",
		Permissions: []string{"fees.read"},
	})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("POST /invoices should be denied without fees.manage")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
	if !strings.Contains(rr.Body.String(), "permission_denied") {
		t.Fatalf("expected permission_denied error, got %q", rr.Body.String())
	}
}

func TestMessagesRouteAllowedWithPermissionAndFlag(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "upshs",
		Role:        "teacher",
		Permissions: []string{"notifications.read"},
	})
	called := false
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/messages/m1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "upshs")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("GET /messages should be allowed with notifications.read and email_notifications enabled")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMessagesFeatureFlagDisabled(t *testing.T) {
	b := testBuilder()
	token := signTestToken(b, auth.Claims{
		Subject:     "u1",
		TenantID:    "aboom",
		Role:        "teacher",
		Permissions: []string{"notifications.read"},
	})
	handler := b.chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler when email_notifications disabled")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/messages/m1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Tenant-ID", "aboom")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusForbidden)
	}
	if !strings.Contains(rr.Body.String(), "feature_disabled") {
		t.Fatalf("expected feature_disabled error, got %q", rr.Body.String())
	}
}
