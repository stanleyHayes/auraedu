package tenancy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
)

func TestFromRequestHeader(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
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
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	if _, err := FromRequest(req, nil); !errors.Is(err, ErrMissingTenant) {
		t.Fatalf("expected ErrMissingTenant, got %v", err)
	}
}

func TestMiddlewareEnforcesTenant(t *testing.T) {
	handler := Middleware(nil, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
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
	if err := ValidateAccess(otherActor, "upshs"); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("expected tenant mismatch, got %v", err)
	}
	if err := ValidateAccess(adminActor, "upshs"); err != nil {
		t.Fatalf("admin should access any tenant: %v", err)
	}
}

func TestCloudEventValidate(t *testing.T) {
	valid := CloudEvent{SpecVersion: "1.0", Type: "student.enrolled.v1", Source: "student-service", ID: "evt-1", Time: "2026-07-20T10:00:00Z", TenantID: "upshs", Data: json.RawMessage(`{"student_id":"s1"}`)}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}

	invalid := valid
	invalid.TenantID = ""
	if err := invalid.Validate(); !errors.Is(err, ErrMissingEventTenant) {
		t.Fatalf("expected missing tenant error, got %v", err)
	}

	for name, testCase := range map[string]struct {
		mutate   func(*CloudEvent)
		expected error
	}{
		"unversioned type": {func(event *CloudEvent) { event.Type = "student.enrolled" }, ErrUnversionedEventType},
		"missing source":   {func(event *CloudEvent) { event.Source = "" }, ErrMissingEventSource},
		"missing time":     {func(event *CloudEvent) { event.Time = "" }, ErrMissingEventTime},
		"invalid time":     {func(event *CloudEvent) { event.Time = "not-a-time" }, ErrInvalidEventTime},
		"missing data":     {func(event *CloudEvent) { event.Data = nil }, ErrMissingEventData},
		"array data":       {func(event *CloudEvent) { event.Data = json.RawMessage(`[]`) }, ErrInvalidEventData},
		"malformed data":   {func(event *CloudEvent) { event.Data = json.RawMessage(`{`) }, ErrInvalidEventData},
	} {
		t.Run(name, func(t *testing.T) {
			event := valid
			testCase.mutate(&event)
			if err := event.Validate(); !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
}
