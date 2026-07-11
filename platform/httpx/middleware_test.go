package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/platform/auth"
)

func TestRequestIDMiddlewarePreservesInbound(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.Header.Set(RequestIDHeader, "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got != "existing-id" {
		t.Fatalf("expected existing-id, got %q", got)
	}
}

func TestRequestIDMiddlewareGeneratesMissing(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got == "" {
		t.Fatal("expected generated request id")
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := CORS(DefaultCORS())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected *, got %q", got)
	}
}

func TestErrorResponses(t *testing.T) {
	cases := []struct {
		fn       func(http.ResponseWriter, *http.Request)
		expected int
		code     ErrorCode
	}{
		{func(w http.ResponseWriter, r *http.Request) { Forbidden(w, r, "") }, http.StatusForbidden, ErrForbidden},
		{func(w http.ResponseWriter, r *http.Request) { FeatureDisabled(w, r, "billing") }, http.StatusForbidden, ErrFeatureDisabled},
		{func(w http.ResponseWriter, r *http.Request) { TenantMismatch(w, r) }, http.StatusForbidden, ErrTenantMismatch},
		{func(w http.ResponseWriter, r *http.Request) { ValidationError(w, r, nil) }, http.StatusUnprocessableEntity, ErrValidation},
		{func(w http.ResponseWriter, r *http.Request) { NotFound(w, r, "student") }, http.StatusNotFound, ErrNotFound},
		{func(w http.ResponseWriter, r *http.Request) { Unauthorized(w, r, "") }, http.StatusUnauthorized, ErrUnauthorized},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		c.fn(rec, httptest.NewRequestWithContext(context.Background(), "GET", "/", nil))
		if rec.Code != c.expected {
			t.Fatalf("%s: expected %d, got %d", c.code, c.expected, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s: expected json, got %s", c.code, ct)
		}
	}
}

func TestRequirePermissionAllowsHolder(t *testing.T) {
	handler := RequirePermission("students.read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.Header.Set(auth.HeaderUserID, "u1")
	req.Header.Set(auth.HeaderTenant, "upshs")
	req.Header.Set(auth.HeaderRole, "teacher")
	req.Header.Set(auth.HeaderPermissions, "students.read,students.write")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequirePermissionAllowsPlatformAdmin(t *testing.T) {
	handler := RequirePermission("anything.manage")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	req.Header.Set(auth.HeaderUserID, "admin1")
	req.Header.Set(auth.HeaderRole, auth.RolePlatformSuperAdmin)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequirePermissionRejectsMissingPermission(t *testing.T) {
	handler := RequirePermission("students.delete")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/students/1", nil)
	req.Header.Set(auth.HeaderUserID, "u1")
	req.Header.Set(auth.HeaderTenant, "upshs")
	req.Header.Set(auth.HeaderRole, "teacher")
	req.Header.Set(auth.HeaderPermissions, "students.read")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequirePermissionRejectsUnauthenticated(t *testing.T) {
	handler := RequirePermission("students.read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
