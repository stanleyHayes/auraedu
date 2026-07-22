package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com","unexpected":true}`))
	rec := httptest.NewRecorder()
	var body forgotPasswordRequest

	if decodeJSON(rec, req, &body) {
		t.Fatal("decodeJSON accepted an unknown field")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPublicInviteAcceptanceAliasIsRegistered(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(nil).Register(mux)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/invites/one-time-token/accept", strings.NewReader(`{"unexpected":true}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "validation_error") {
		t.Fatalf("public alias not routed through strict invite decoder: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAuthorizationDiscoveryUsesGeneratedRegistry(t *testing.T) {
	handler := &Handler{}

	permissionsResponse := httptest.NewRecorder()
	handler.listPermissions(permissionsResponse, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/permissions", nil))
	var permissions struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(permissionsResponse.Body.Bytes(), &permissions); err != nil {
		t.Fatalf("decode permissions: %v", err)
	}
	if len(permissions.Data) != len(auth.KnownPermissions()) || !containsString(permissions.Data, "files.update") {
		t.Fatalf("permissions endpoint drifted from registry: %+v", permissions.Data)
	}

	rolesResponse := httptest.NewRecorder()
	handler.listRoles(rolesResponse, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/roles", nil))
	var roles struct {
		Data []struct {
			Role  string `json:"role"`
			Scope string `json:"scope"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rolesResponse.Body.Bytes(), &roles); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	if len(roles.Data) != len(auth.KnownRoles()) || !containsRole(roles.Data, "applicant", "own_applications") || !containsRole(roles.Data, "support_agent", "limited_platform_support") {
		t.Fatalf("roles endpoint drifted from registry: %+v", roles.Data)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsRole(values []struct {
	Role  string `json:"role"`
	Scope string `json:"scope"`
}, role, scope string) bool {
	for _, value := range values {
		if value.Role == role && value.Scope == scope {
			return true
		}
	}
	return false
}

func TestDecodeJSONRejectsMultipleValues(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"email":"a@example.com"}{"email":"b@example.com"}`))
	rec := httptest.NewRecorder()
	var body forgotPasswordRequest

	if decodeJSON(rec, req, &body) {
		t.Fatal("decodeJSON accepted multiple JSON values")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUnavailableMapsToServiceUnavailable(t *testing.T) {
	err := errors.Join(domain.ErrUnavailable, errors.New("notification service timed out"))
	if got := mapStatus(err); got != http.StatusServiceUnavailable {
		t.Fatalf("mapStatus = %d, want %d", got, http.StatusServiceUnavailable)
	}
	if got := codeFor(err); got != "service_unavailable" {
		t.Fatalf("codeFor = %q, want service_unavailable", got)
	}
}
