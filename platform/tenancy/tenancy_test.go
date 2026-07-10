package tenancy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
)

func TestFromRequestHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderTenantID, "upshs")
	req.Header.Set(HeaderRequestID, "req-1")

	tc, err := FromRequest(req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.TenantID != "upshs" || tc.RequestID != "req-1" {
		t.Fatalf("unexpected tenant context: %+v", tc)
	}
}

func TestFromRequestJWT(t *testing.T) {
	key := []byte("test-key")
	token, err := auth.Sign(auth.Claims{
		Subject:   "u1",
		TenantID:  "upshs",
		Role:      "teacher",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	tc, err := FromRequest(req, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.TenantID != "upshs" || tc.ActorID != "u1" || tc.ActorRole != "teacher" {
		t.Fatalf("unexpected tenant context: %+v", tc)
	}
}

func TestFromRequestMissingTenant(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if _, err := FromRequest(req, nil); err != ErrMissingTenant {
		t.Fatalf("expected ErrMissingTenant, got %v", err)
	}
}

func TestMiddlewareEnforcesTenant(t *testing.T) {
	handler := Middleware(nil, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(HeaderTenantID, "upshs")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = WithContext(ctx, TenantContext{TenantID: "upshs", RequestID: "r1"})

	if TenantID(ctx) != "upshs" {
		t.Fatalf("expected upshs, got %s", TenantID(ctx))
	}
	if RequestID(ctx) != "r1" {
		t.Fatalf("expected r1, got %s", RequestID(ctx))
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("expected no tenant context")
	}
}

func TestCacheKeyAndFilePath(t *testing.T) {
	if got := CacheKey("upshs", "session:abc"); got != "tenant:upshs:session:abc" {
		t.Fatalf("unexpected cache key: %s", got)
	}
	if got := FilePath("upshs", "students/photo.jpg"); got != "/upshs/students/photo.jpg" {
		t.Fatalf("unexpected file path: %s", got)
	}
}

func TestValidateAccess(t *testing.T) {
	upshsActor := auth.Actor{UserID: "u1", TenantID: "upshs"}
	otherActor := auth.Actor{UserID: "u2", TenantID: "aboom-ame-zion-c"}
	adminActor := auth.Actor{UserID: "s1", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}

	if err := ValidateAccess(upshsActor, "upshs"); err != nil {
		t.Fatalf("upshs actor should access upshs: %v", err)
	}
	if err := ValidateAccess(otherActor, "upshs"); err != ErrTenantMismatch {
		t.Fatalf("expected tenant mismatch, got %v", err)
	}
	if err := ValidateAccess(adminActor, "upshs"); err != nil {
		t.Fatalf("admin should access any tenant: %v", err)
	}
}

func TestCloudEventValidate(t *testing.T) {
	valid := CloudEvent{SpecVersion: "1.0", Type: "student.enrolled", ID: "evt-1", TenantID: "upshs"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}

	invalid := CloudEvent{SpecVersion: "1.0", Type: "student.enrolled", ID: "evt-2"}
	if err := invalid.Validate(); err != ErrMissingEventTenant {
		t.Fatalf("expected missing tenant error, got %v", err)
	}
}
