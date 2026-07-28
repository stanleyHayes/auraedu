package unit

import (
	"net/http"
	"testing"
	"time"

	"github.com/auraedu/audit-service/internal/adapters/memory"
	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/platform/httpx"
	"github.com/google/uuid"
)

func TestHandler_ListAuditLogs_Filters(t *testing.T) {
	repo := memory.NewRepository()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	actorID := uuid.NewString()
	seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", actorID, base)
	seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", uuid.NewString(), base)
	seedFullLog(t, repo, tenantAID, "invoice.created.v1", "billing-service", actorID, base)
	seedFullLog(t, repo, tenantAID, "student.created.v1", "student-service", actorID, base.Add(72*time.Hour))
	mux := newTestMux(repo)

	path := "/api/v1/audit/logs?event_type=student.created.v1&actor_id=" + actorID +
		"&source_service=student-service&from=2026-01-09T00:00:00Z&to=2026-01-11T00:00:00Z"
	rec := doGet(t, mux, path, tenantHeaders(tenantAID, application.PermRead))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	env := decodeBody[listEnvelope](t, rec)
	if len(env.Data) != 1 {
		t.Fatalf("expected exactly 1 filtered log, got %d", len(env.Data))
	}
	if env.Data[0]["actor_id"] != actorID {
		t.Fatalf("actor_id mismatch: %v", env.Data[0]["actor_id"])
	}
}

func TestHandler_ListAuditLogs_InvalidActorID(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	rec := doGet(t, mux, "/api/v1/audit/logs?actor_id=not-a-uuid", tenantHeaders(tenantAID, application.PermRead))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: got %d, want 422", rec.Code)
	}
	body := decodeBody[map[string]any](t, rec)
	if body["error"] != string(httpx.ErrValidation) {
		t.Fatalf("error code: got %v, want validation_error", body["error"])
	}
}

func TestHandler_ListAuditLogs_InvalidTimeRange(t *testing.T) {
	mux := newTestMux(memory.NewRepository())

	for _, path := range []string{
		"/api/v1/audit/logs?from=not-a-time",
		"/api/v1/audit/logs?to=yesterday",
		"/api/v1/audit/logs?from=2026-01-12T00:00:00Z&to=2026-01-10T00:00:00Z",
	} {
		rec := doGet(t, mux, path, tenantHeaders(tenantAID, application.PermRead))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status: got %d, want 422", path, rec.Code)
		}
		body := decodeBody[map[string]any](t, rec)
		if body["error"] != string(httpx.ErrValidation) {
			t.Fatalf("%s error code: got %v, want validation_error", path, body["error"])
		}
	}
}
