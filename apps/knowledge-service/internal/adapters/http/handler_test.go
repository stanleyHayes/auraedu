package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auraedu/knowledge-service/internal/adapters/memory"
	"github.com/auraedu/knowledge-service/internal/application"
	"github.com/auraedu/platform/auth"
)

func TestInternalSearchRequiresServiceCredentialAndReturnsCitations(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	svc := application.NewService(memory.New(), application.WithClock(func() time.Time { return now }))
	mux := http.NewServeMux()
	handler := NewHandler(svc)
	handler.Register(mux)
	handler.RegisterInternal(mux, "service-secret")

	create := map[string]any{"source_type": "programme", "title": "Computer Science Programme", "owner": "Admissions",
		"content":      "The Computer Science programme requires the official application form and published entry qualifications.",
		"effective_at": now.Add(-time.Hour), "confidentiality": "public"}
	created := performJSON(t, mux, "/api/v1/knowledge/sources", create, map[string]string{
		auth.HeaderUserID: "manager", auth.HeaderTenant: "school-one", auth.HeaderPermissions: application.PermManage,
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var source struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &source); err != nil {
		t.Fatalf("decode source: %v", err)
	}
	approved := performJSON(t, mux, "/api/v1/knowledge/sources/"+source.ID+"/approve", map[string]string{"review_note": "Verified programme handbook"}, map[string]string{
		auth.HeaderUserID: "reviewer", auth.HeaderTenant: "school-one", auth.HeaderPermissions: application.PermApprove,
	})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", approved.Code, approved.Body.String())
	}
	unauthorized := performJSON(t, mux, "/internal/v1/knowledge/search", map[string]string{"query": "Computer Science"}, map[string]string{"X-Tenant-Code": "school-one"})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized search status=%d", unauthorized.Code)
	}
	searched := performJSON(t, mux, "/internal/v1/knowledge/search", map[string]string{"query": "Computer Science"}, map[string]string{
		"X-Tenant-Code": "school-one", "Authorization": "Bearer service-secret",
	})
	if searched.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", searched.Code, searched.Body.String())
	}
	var response struct {
		Results []struct {
			SourceID string `json:"source_id"`
			Title    string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal(searched.Body.Bytes(), &response); err != nil || len(response.Results) != 1 || response.Results[0].SourceID != source.ID || response.Results[0].Title == "" {
		t.Fatalf("citation response=%+v err=%v", response, err)
	}
}

func performJSON(t *testing.T, handler http.Handler, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}
