package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auraedu/admissions-service/internal/adapters/memory"
	"github.com/auraedu/admissions-service/internal/application"
	"github.com/auraedu/admissions-service/internal/domain"
)

func request(mux http.Handler, method, path, body, actor, permissions string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Tenant-Code", "school-one")
	if actor != "" {
		req.Header.Set("X-Actor-User", actor)
		req.Header.Set("X-Actor-Tenant", "school-one")
		req.Header.Set("X-Actor-Role", "school_admin")
		req.Header.Set("X-Actor-Permissions", permissions)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	return recorder
}

func TestCatalogueHTTPToApplicationStart(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	service := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	mux := http.NewServeMux()
	NewHandler(service).Register(mux)
	managerPermissions := application.PermCatalogue + "," + application.PermRead

	created := request(mux, http.MethodPost, "/api/v1/programmes", `{"code":"SCI","name":"General Science","slug":"general-science","summary":"Science programme","description":"Verified programme description"}`, "manager", managerPermissions)
	if created.Code != http.StatusCreated {
		t.Fatalf("create programme status=%d body=%s", created.Code, created.Body.String())
	}
	var programme domain.Programme
	if err := json.Unmarshal(created.Body.Bytes(), &programme); err != nil {
		t.Fatal(err)
	}
	starts, opens, closes := now.Add(60*24*time.Hour).Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339), now.Add(30*24*time.Hour).Format(time.RFC3339)
	intakeResponse := request(mux, http.MethodPost, "/api/v1/programmes/"+programme.ID+"/intakes", fmt.Sprintf(`{"name":"September 2026","starts_at":%q,"application_opens_at":%q,"application_closes_at":%q}`, starts, opens, closes), "manager", managerPermissions)
	if intakeResponse.Code != http.StatusCreated {
		t.Fatalf("create intake status=%d body=%s", intakeResponse.Code, intakeResponse.Body.String())
	}
	var intake domain.Intake
	if err := json.Unmarshal(intakeResponse.Body.Bytes(), &intake); err != nil {
		t.Fatal(err)
	}
	if response := request(mux, http.MethodPatch, "/api/v1/programmes/"+programme.ID, `{"status":"published"}`, "manager", managerPermissions); response.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(mux, http.MethodPatch, "/api/v1/intakes/"+intake.ID, `{"status":"open"}`, "manager", managerPermissions); response.Code != http.StatusOK {
		t.Fatalf("open status=%d body=%s", response.Code, response.Body.String())
	}
	public := request(mux, http.MethodGet, "/api/v1/public/programmes", "", "", "")
	if public.Code != http.StatusOK || !bytes.Contains(public.Body.Bytes(), []byte(programme.ID)) || !bytes.Contains(public.Body.Bytes(), []byte(intake.ID)) {
		t.Fatalf("public status=%d body=%s", public.Code, public.Body.String())
	}
	applicationResponse := request(mux, http.MethodPost, "/api/v1/applications", fmt.Sprintf(`{"programme_id":%q,"intake_id":%q}`, programme.ID, intake.ID), "applicant-1", application.PermCreate)
	if applicationResponse.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", applicationResponse.Code, applicationResponse.Body.String())
	}
}

func TestCatalogueHTTPRejectsMissingTenantAndEmptyPatch(t *testing.T) {
	service := application.NewService(memory.New())
	mux := http.NewServeMux()
	NewHandler(service).Register(mux)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/public/programmes", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing tenant status=%d", recorder.Code)
	}
	response := request(mux, http.MethodPatch, "/api/v1/programmes/does-not-matter", `{}`, "manager", application.PermCatalogue)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty patch status=%d body=%s", response.Code, response.Body.String())
	}
}
