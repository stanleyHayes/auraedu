package gateway

import (
	"context"
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
			BySubdomain: map[string]string{"upshs": "upshs", "aboom": "aboom"},
		},
		Flags: &stubs.FeatureFlagClient{
			Defaults: map[string]bool{"student_management": true, "ai_predictions": true, "file_management": true, "fees": true, "email_notifications": true},
			TenantOverrides: map[string]map[string]bool{
				"upshs": {"cbt_exams": true},
				"aboom": {"cbt_exams": false, "email_notifications": false},
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
