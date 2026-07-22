package student

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
)

func TestResolveLearnerScopeAuthenticatesAndForwardsTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get(tenancy.HeaderTenantID) != "school-one" || r.URL.Query().Get("role") != "parent" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"student_ids":["student-1","student-2"]}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	scope, err := NewClient(server.URL, "secret").Resolve(context.Background(), "school-one", "parent-1", "parent")
	if err != nil || len(scope.StudentIDs) != 2 {
		t.Fatalf("scope=%v err=%v", scope, err)
	}
}

func TestResolveLearnerScopeFailsClosed(t *testing.T) {
	if _, err := NewClient("", "").Resolve(context.Background(), "school-one", "parent-1", "parent"); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unconfigured=%v", err)
	}
}
