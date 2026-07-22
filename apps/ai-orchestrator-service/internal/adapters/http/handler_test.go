package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/adapters/memory"
	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
)

type retriever struct{ results []domain.KnowledgeResult }

func (r retriever) Search(context.Context, string, string, string, int, time.Time) ([]domain.KnowledgeResult, error) {
	return r.results, nil
}

func TestPublicAssistantValidatesAndReturnsGroundedResponse(t *testing.T) {
	svc := application.NewService(memory.New(), retriever{results: []domain.KnowledgeResult{{
		SourceID: "61f9e62e-43ef-475c-a46a-c5de988938b6", Title: "Admissions Guide", Version: 1,
		Passage: "Applications must be submitted through the official applicant portal.", Locale: "en", Score: 0.6,
	}}})
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	payload, err := json.Marshal(map[string]any{"question": "How do I apply?", "session_id": nil, "locale": "en"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/api/v1/public/assistant/messages", bytes.NewReader(payload),
	)
	req.Header.Set("X-Tenant-Code", "school-one")
	req.Header.Set("Idempotency-Key", "assistant-handler-key-0001")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response domain.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || len(response.Citations) != 1 || response.Answer == "" {
		t.Fatalf("response=%+v err=%v", response, err)
	}

	bad := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/v1/public/assistant/messages",
		bytes.NewReader([]byte(`{"question":"ok","unknown":true}`)),
	)
	bad.Header.Set("X-Tenant-Code", "school-one")
	bad.Header.Set("Idempotency-Key", "assistant-handler-key-0002")
	badRecorder := httptest.NewRecorder()
	mux.ServeHTTP(badRecorder, bad)
	if badRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown field status=%d", badRecorder.Code)
	}
}
