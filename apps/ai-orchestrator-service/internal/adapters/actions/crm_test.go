package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

func TestCRMExecutorStoresOnlyPIIFreeAssignmentEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"11111111-1111-4111-8111-111111111111","owner_user_id":"22222222-2222-4222-8222-222222222222","first_name":"Ama","last_name":"Mensah","email":"ama@example.com","phone":"+233200000000"}`))
	}))
	defer server.Close()
	executor := NewCRMExecutor(server.URL)
	result, err := executor.Execute(context.Background(), domain.ActionProposal{Action: domain.ActionCRMAssignLead, TargetID: "11111111-1111-4111-8111-111111111111", TenantID: "school-one", PayloadHash: strings.Repeat("a", 64), Payload: json.RawMessage(`{"owner_user_id":"22222222-2222-4222-8222-222222222222"}`)}, auth.Actor{UserID: "admin", Role: "school_admin", Permissions: []string{"crm.lead.assign"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	value := string(result.Body)
	if strings.Contains(value, "Ama") || strings.Contains(value, "email") || strings.Contains(value, "phone") || !strings.Contains(value, "lead_id") || !strings.Contains(value, "owner_user_id") {
		t.Fatalf("unsafe action evidence: %s", value)
	}
}

func TestCRMExecutorRedactsDownstreamErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"email":"ama@example.com","detail":"bad lead"}`))
	}))
	defer server.Close()
	result, err := NewCRMExecutor(server.URL).Execute(context.Background(), domain.ActionProposal{Action: domain.ActionCRMAssignLead, TargetID: "11111111-1111-4111-8111-111111111111", TenantID: "school-one", PayloadHash: strings.Repeat("b", 64), Payload: json.RawMessage(`{"owner_user_id":"22222222-2222-4222-8222-222222222222"}`)}, auth.Actor{})
	if err == nil || strings.Contains(string(result.Body), "email") || !strings.Contains(string(result.Body), "response_redacted") {
		t.Fatalf("result=%s err=%v", result.Body, err)
	}
}

func TestCRMExecutorNormalizesRenderHostport(t *testing.T) {
	executor := NewCRMExecutor("crm-service:10000/")
	if executor.baseURL != "http://crm-service:10000" {
		t.Fatalf("baseURL=%q", executor.baseURL)
	}
}
