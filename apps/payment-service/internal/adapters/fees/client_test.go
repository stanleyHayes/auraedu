package fees

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/platform/tenancy"
)

func TestClientResolvePropagatesServiceAndLearnerScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/invoice-access" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer service-secret" || r.Header.Get(tenancy.HeaderTenantID) != "tenant-a" {
			t.Errorf("missing private auth headers")
		}
		var body struct {
			UserID     string   `json:"user_id"`
			Role       string   `json:"role"`
			InvoiceIDs []string `json:"invoice_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.UserID != "parent-1" || body.Role != "parent" || len(body.InvoiceIDs) != 2 {
			t.Errorf("unexpected request: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"invoice_ids": []string{"invoice-owned"}})
	}))
	defer server.Close()

	ids, err := NewClient(server.URL, "service-secret").Resolve(context.Background(), "tenant-a", "parent-1", "parent", []string{"invoice-owned", "invoice-other"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != "invoice-owned" {
		t.Fatalf("unexpected authorized invoices: %v", ids)
	}
}
