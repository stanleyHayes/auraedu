package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpadapter "github.com/auraedu/attendance-service/internal/adapters/http"
	"github.com/auraedu/attendance-service/internal/application"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

func newBulkMux(gates *flags.StaticSnapshot) *http.ServeMux {
	svc := application.NewService(
		&fakeRepo{},
		application.WithPublisher(&fakePublisher{}),
		application.WithFeatureGate(gates),
		application.WithLearnerScopeResolver(scopeResolver{ids: []string{bulkStu1, bulkStu2}, classIDs: []string{bulkClass}}),
	)
	mux := http.NewServeMux()
	httpadapter.NewHandler(svc).Register(mux)
	return mux
}

func bulkRequest(t *testing.T, body string, headers map[string]string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/attendance/bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func teacherHeaders() map[string]string {
	return map[string]string{
		auth.HeaderUserID:      "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
		auth.HeaderTenant:      bulkTenant,
		auth.HeaderRole:        "teacher",
		auth.HeaderPermissions: application.PermMark,
		tenancy.HeaderTenantID: bulkTenant,
	}
}

const validBulkBody = `{
	"date": "2025-09-01",
	"academic_year_id": "` + bulkAY + `",
	"class_id": "` + bulkClass + `",
	"subject_id": "` + bulkSubject + `",
	"records": [
		{"student_id": "` + bulkStu1 + `", "status": "present"},
		{"student_id": "` + bulkStu2 + `", "status": "absent", "remark": "sick note"}
	]
}`

func TestHandler_BulkMark_Created(t *testing.T) {
	mux := newBulkMux(enabledGates())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, bulkRequest(t, validBulkBody, teacherHeaders()))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []struct {
			ID             string  `json:"id"`
			TenantID       string  `json:"tenant_id"`
			StudentID      string  `json:"student_id"`
			AcademicYearID string  `json:"academic_year_id"`
			ClassID        *string `json:"class_id"`
			SubjectID      *string `json:"subject_id"`
			Date           string  `json:"date"`
			Status         string  `json:"status"`
			MarkedBy       string  `json:"marked_by"`
		} `json:"data"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 2 {
		t.Fatalf("expected 2 records, got %d", len(body.Data))
	}
	if body.NextCursor != nil {
		t.Fatalf("expected null next_cursor, got %v", *body.NextCursor)
	}
	for i, row := range body.Data {
		if row.TenantID != bulkTenant || row.AcademicYearID != bulkAY || row.Date != "2025-09-01" {
			t.Fatalf("record %d scope mismatch: %+v", i, row)
		}
		if row.ClassID == nil || *row.ClassID != bulkClass {
			t.Fatalf("record %d missing class_id: %+v", i, row)
		}
		if row.MarkedBy != "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee" {
			t.Fatalf("record %d marked_by should be the actor: %q", i, row.MarkedBy)
		}
	}
}

func TestHandler_BulkMark_InvalidJSON(t *testing.T) {
	mux := newBulkMux(enabledGates())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, bulkRequest(t, `{"date":`, teacherHeaders()))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestHandler_BulkMark_RowValidationError(t *testing.T) {
	mux := newBulkMux(enabledGates())
	body := `{
		"date": "2025-09-01",
		"academic_year_id": "` + bulkAY + `",
		"records": [
			{"student_id": "` + bulkStu1 + `", "status": "present"},
			{"student_id": "` + bulkStu2 + `", "status": "unknown"}
		]
	}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, bulkRequest(t, body, teacherHeaders()))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	var errBody struct {
		Code    string         `json:"error"`
		Details map[string]any `json:"details"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errBody.Code != "validation_error" {
		t.Fatalf("expected validation_error, got %q", errBody.Code)
	}
	rows, ok := errBody.Details["rows"].(map[string]any)
	if !ok {
		t.Fatalf("expected details.rows in error response, got %v", errBody.Details)
	}
	if _, ok := rows["records[1].status"]; !ok {
		t.Fatalf("expected records[1].status row error, got %v", rows)
	}
}

func TestHandler_BulkMark_PermissionDenied(t *testing.T) {
	mux := newBulkMux(enabledGates())
	headers := teacherHeaders()
	headers[auth.HeaderPermissions] = application.PermRead
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, bulkRequest(t, validBulkBody, headers))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestHandler_BulkMark_FeatureDisabled(t *testing.T) {
	gates := flags.NewStaticSnapshot()
	gates.Set(bulkTenant, application.FeatureAttendance, false)
	mux := newBulkMux(gates)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, bulkRequest(t, validBulkBody, teacherHeaders()))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	var errBody struct {
		Code string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errBody); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errBody.Code != "feature_disabled" {
		t.Fatalf("expected feature_disabled, got %q", errBody.Code)
	}
}

func TestHandler_BulkMark_MissingTenant(t *testing.T) {
	mux := newBulkMux(enabledGates())
	headers := teacherHeaders()
	delete(headers, tenancy.HeaderTenantID)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, bulkRequest(t, validBulkBody, headers))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
