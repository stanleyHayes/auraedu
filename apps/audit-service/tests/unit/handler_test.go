package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	svchttp "github.com/auraedu/audit-service/internal/adapters/http"
	"github.com/auraedu/audit-service/internal/adapters/memory"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

func newTestMux(repo *memory.Repository) *http.ServeMux {
	query := application.NewQuery(repo)
	health := httpx.NewHealth("audit-service", "test")
	mux := http.NewServeMux()
	svchttp.NewHandler(health, query).Register(mux)
	return mux
}

// doGet issues a GET against the mux with the gateway-injected actor headers.
func doGet(t *testing.T, mux *http.ServeMux, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func tenantHeaders(tenantID string, perms string) map[string]string {
	return map[string]string{
		auth.HeaderUserID:      "user-1",
		auth.HeaderTenant:      tenantID,
		auth.HeaderRole:        "tenant_admin",
		auth.HeaderPermissions: perms,
		tenancy.HeaderTenantID: tenantID,
	}
}

func platformAdminHeaders() map[string]string {
	return map[string]string{
		auth.HeaderUserID: "admin-1",
		auth.HeaderRole:   auth.RolePlatformSuperAdmin,
	}
}

type listEnvelope struct {
	Data       []map[string]any `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v (body: %s)", err, rec.Body.String())
	}
	return out
}

func TestHandler_ListAuditLogs_OK(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "user-1")
	seedLog(t, repo, tenantAID, "student.updated.v1", "")
	mux := newTestMux(repo)

	rec := doGet(t, mux, "/api/v1/audit/logs", tenantHeaders(tenantAID, application.PermRead))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	env := decodeBody[listEnvelope](t, rec)
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(env.Data))
	}
	first := env.Data[0]
	if first["event_type"] != "student.updated.v1" {
		t.Fatalf("expected newest first, got event_type %v", first["event_type"])
	}
	if first["id"] == nil || first["tenant_id"] != tenantAID {
		t.Fatalf("id/tenant_id mismatch: %v", first)
	}
	if first["occurred_at"] == nil || first["occurred_at"] == "" {
		t.Fatalf("occurred_at missing: %v", first)
	}
	// Empty actor/resource fields map to null per the contract.
	if v, ok := first["actor_id"]; !ok || v != nil {
		t.Fatalf("expected actor_id null, got %v", first["actor_id"])
	}
	if v, ok := first["resource_type"]; !ok || v != "student" {
		t.Fatalf("expected resource_type student, got %v", first["resource_type"])
	}
	if env.NextCursor != nil {
		t.Fatalf("expected next_cursor null, got %v", *env.NextCursor)
	}
}

func TestHandler_ListAuditLogs_ContractPath(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "user-1")
	mux := newTestMux(repo)

	// contracts/openapi/audit.v1.yaml declares GET /audit-logs under /api/v1.
	rec := doGet(t, mux, "/api/v1/audit-logs", tenantHeaders(tenantAID, application.PermRead))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	env := decodeBody[listEnvelope](t, rec)
	if len(env.Data) != 1 {
		t.Fatalf("expected 1 log, got %d", len(env.Data))
	}
}

func TestHandler_ListAuditLogs_Unauthenticated(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	rec := doGet(t, mux, "/api/v1/audit/logs", map[string]string{
		tenancy.HeaderTenantID: tenantAID,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", rec.Code)
	}
	body := decodeBody[map[string]any](t, rec)
	if body["error"] != string(httpx.ErrForbidden) {
		t.Fatalf("error code: got %v, want forbidden", body["error"])
	}
}

func TestHandler_ListAuditLogs_MissingTenant(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	headers := tenantHeaders(tenantAID, application.PermRead)
	delete(headers, tenancy.HeaderTenantID)
	rec := doGet(t, mux, "/api/v1/audit/logs", headers)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", rec.Code)
	}
	body := decodeBody[map[string]any](t, rec)
	if body["error"] != string(httpx.ErrTenantMismatch) {
		t.Fatalf("error code: got %v, want tenant_mismatch", body["error"])
	}
}

func TestHandler_ListAuditLogs_TenantMismatch(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	// Actor belongs to tenant B but requests tenant A's logs.
	headers := tenantHeaders(tenantBID, application.PermRead)
	headers[tenancy.HeaderTenantID] = tenantAID
	rec := doGet(t, mux, "/api/v1/audit/logs", headers)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", rec.Code)
	}
	body := decodeBody[map[string]any](t, rec)
	if body["error"] != string(httpx.ErrForbidden) {
		t.Fatalf("error code: got %v, want forbidden", body["error"])
	}
}

func TestHandler_ListAuditLogs_MissingPermission(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	rec := doGet(t, mux, "/api/v1/audit/logs", tenantHeaders(tenantAID, "students.read"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", rec.Code)
	}
	body := decodeBody[map[string]any](t, rec)
	if body["error"] != string(httpx.ErrForbidden) {
		t.Fatalf("error code: got %v, want forbidden", body["error"])
	}
}

func TestHandler_ListAuditLogs_PlatformAdminCrossTenant(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "user-1")
	seedLog(t, repo, tenantBID, "invoice.created.v1", "user-2")
	mux := newTestMux(repo)

	// No tenant header: the platform super admin reads across tenants.
	rec := doGet(t, mux, "/api/v1/audit/logs?limit=50", platformAdminHeaders())
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	env := decodeBody[listEnvelope](t, rec)
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 logs across tenants, got %d", len(env.Data))
	}
}

func TestHandler_ListAuditLogs_Pagination(t *testing.T) {
	repo := memory.NewRepository()
	seedLog(t, repo, tenantAID, "student.created.v1", "")
	seedLog(t, repo, tenantAID, "student.updated.v1", "")
	mux := newTestMux(repo)

	rec := doGet(t, mux, "/api/v1/audit/logs?limit=1", tenantHeaders(tenantAID, application.PermRead))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	env := decodeBody[listEnvelope](t, rec)
	if len(env.Data) != 1 {
		t.Fatalf("expected 1 log, got %d", len(env.Data))
	}
	if env.NextCursor == nil || *env.NextCursor == "" {
		t.Fatal("expected next_cursor on first page")
	}

	rec2 := doGet(t, mux, "/api/v1/audit/logs?limit=1&cursor="+*env.NextCursor, tenantHeaders(tenantAID, application.PermRead))
	if rec2.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec2.Code)
	}
	env2 := decodeBody[listEnvelope](t, rec2)
	if len(env2.Data) != 1 {
		t.Fatalf("expected 1 log on page 2, got %d", len(env2.Data))
	}
	if env2.Data[0]["id"] == env.Data[0]["id"] {
		t.Fatal("page 2 returned the same log as page 1")
	}
	// The adapters emit a cursor whenever a page is full (same semantics as
	// the Postgres List); a final fetch returns an empty page with null cursor.
	if env2.NextCursor == nil || *env2.NextCursor == "" {
		t.Fatal("expected next_cursor on full last page")
	}
	rec3 := doGet(t, mux, "/api/v1/audit/logs?limit=1&cursor="+*env2.NextCursor, tenantHeaders(tenantAID, application.PermRead))
	if rec3.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec3.Code)
	}
	env3 := decodeBody[listEnvelope](t, rec3)
	if len(env3.Data) != 0 {
		t.Fatalf("expected 0 logs on final page, got %d", len(env3.Data))
	}
	if env3.NextCursor != nil {
		t.Fatalf("expected next_cursor null on final page, got %v", *env3.NextCursor)
	}
}

func TestHandler_ListAuditLogs_InvalidCursor(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	rec := doGet(t, mux, "/api/v1/audit/logs?cursor=not-a-uuid", tenantHeaders(tenantAID, application.PermRead))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: got %d, want 422", rec.Code)
	}
	body := decodeBody[map[string]any](t, rec)
	if body["error"] != string(httpx.ErrValidation) {
		t.Fatalf("error code: got %v, want validation_error", body["error"])
	}
}
