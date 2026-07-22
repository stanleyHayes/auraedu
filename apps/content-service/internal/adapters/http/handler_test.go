package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/content-service/internal/adapters/memory"
	"github.com/auraedu/content-service/internal/application"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

type generator struct{}

func (generator) Generate(context.Context, ports.GenerateInput) (ports.GenerateOutput, error) {
	return ports.GenerateOutput{Content: "Meet our teachers on 30 August. Terms apply", Generator: "test:model"}, nil
}

func request(t *testing.T, mux http.Handler, method, target, body string, user, permissions string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.HeaderUserID, user)
	req.Header.Set(auth.HeaderTenant, "school-a")
	req.Header.Set(auth.HeaderPermissions, permissions)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	return recorder
}

func TestContentHTTPWorkflowAndNoPublishSurface(t *testing.T) {
	service := application.NewService(memory.NewRepository(), generator{})
	mux := http.NewServeMux()
	NewHandler(service).Register(mux)
	profile := `{"tone_of_voice":"Warm, factual and encouraging","approved_terms":[],"prohibited_claims":["guaranteed admission"],"required_disclaimers":["Terms apply"],"locale":"en-GH","expected_version":0}`
	if response := request(t, mux, http.MethodPut, "/api/v1/content/brand-profile", profile, "reviewer", application.PermReview); response.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", response.Code, response.Body.String())
	}
	generateBody := `{"content_type":"social_post","title":"Open day","brief":"Create an invitation for our next open day.","audience":"Prospective families","locale":"en-GH","key_messages":["Meet our teachers"],"facts":[{"label":"Date","value":"30 August"}]}`
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/content/generate",
		bytes.NewBufferString(generateBody),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "content-http-0001")
	req.Header.Set(auth.HeaderUserID, "author")
	req.Header.Set(auth.HeaderTenant, "school-a")
	req.Header.Set(auth.HeaderPermissions, application.PermGenerate)
	generated := httptest.NewRecorder()
	mux.ServeHTTP(generated, req)
	if generated.Code != http.StatusCreated {
		t.Fatalf("generate status=%d body=%s", generated.Code, generated.Body.String())
	}
	var draft struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(generated.Body).Decode(&draft); err != nil {
		t.Fatal(err)
	}
	if draft.ID == "" || draft.Version != 1 || draft.Status != "draft" {
		t.Fatalf("unexpected draft %#v", draft)
	}
	if response := request(t, mux, http.MethodPost, "/api/v1/content/"+draft.ID+"/publish", `{}`, "reviewer", "content.publish"); response.Code != http.StatusNotFound {
		t.Fatalf("MVP must expose no publish endpoint, got %d", response.Code)
	}
}

func TestContentHTTPRejectsUnknownFields(t *testing.T) {
	service := application.NewService(memory.NewRepository(), generator{})
	mux := http.NewServeMux()
	NewHandler(service).Register(mux)
	response := request(t, mux, http.MethodPut, "/api/v1/content/brand-profile", `{"tone_of_voice":"Warm","locale":"en-GH","expected_version":0,"bypass_review":true}`, "reviewer", application.PermReview)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field status=%d body=%s", response.Code, response.Body.String())
	}
}
