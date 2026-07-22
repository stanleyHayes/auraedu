package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	feeshttp "github.com/auraedu/fees-service/internal/adapters/http"
	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
)

func TestInternalInvoiceAccessRequiresTokenAndReturnsOwnedSubset(t *testing.T) {
	own := newInvoice(t, "student-own")
	other := newInvoice(t, "student-other")
	repo := &scopedInvoiceRepo{invoices: map[string]*domain.Invoice{own.ID: own, other.ID: other}}
	svc := application.NewService(fakeFeeStructureRepo{}, repo, application.WithLearnerScopeResolver(learnerResolver{ids: []string{own.StudentID}}))
	mux := http.NewServeMux()
	feeshttp.NewHandler(svc).RegisterInternal(mux, "service-secret")
	payload, err := json.Marshal(map[string]any{"user_id": "parent-1", "role": "parent", "invoice_ids": []string{own.ID, other.ID}})
	if err != nil {
		t.Fatal(err)
	}

	unauthorized := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/v1/invoice-access", bytes.NewReader(payload))
	unauthorized.Header.Set(tenancy.HeaderTenantID, feesTenant)
	unauthorizedResult := httptest.NewRecorder()
	mux.ServeHTTP(unauthorizedResult, unauthorized)
	if unauthorizedResult.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without service token, got %d", unauthorizedResult.Code)
	}

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/v1/invoice-access", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer service-secret")
	request.Header.Set(tenancy.HeaderTenantID, feesTenant)
	result := httptest.NewRecorder()
	mux.ServeHTTP(result, request)
	if result.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", result.Code, result.Body.String())
	}
	var body struct {
		InvoiceIDs []string `json:"invoice_ids"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.InvoiceIDs) != 1 || body.InvoiceIDs[0] != own.ID {
		t.Fatalf("expected owned subset, got %v", body.InvoiceIDs)
	}
}
