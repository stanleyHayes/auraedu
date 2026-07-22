package staff

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
)

func TestResolveTeacherAuthenticatesAndForwardsTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get(tenancy.HeaderTenantID) != "school-one" || r.URL.Query().Get("user_id") != "user-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"staff_id":"staff-1","class_ids":["class-1"],"subject_ids":[]}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	staffID, err := NewClient(server.URL, "secret").ResolveTeacher(context.Background(), "school-one", "user-1")
	if err != nil || staffID != "staff-1" {
		t.Fatalf("staff_id=%q err=%v", staffID, err)
	}
}

func TestResolveTeacherAssignmentsReturnsExplicitClassScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprint(w, `{"staff_id":"staff-1","class_ids":["class-1","class-2"],"subject_ids":[]}`); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()
	staffID, classIDs, err := NewClient(server.URL, "secret").ResolveTeacherAssignments(context.Background(), "school-one", "user-1")
	if err != nil || staffID != "staff-1" || len(classIDs) != 2 {
		t.Fatalf("staff_id=%q class_ids=%v err=%v", staffID, classIDs, err)
	}
}

func TestResolveTeacherFailsClosed(t *testing.T) {
	if _, err := NewClient("", "").ResolveTeacher(context.Background(), "school-one", "user-1"); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("unconfigured=%v", err)
	}
}

func TestResolveTeacherAssignmentsRejectsOversizedDependencyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", (1<<20)+1))
	}))
	defer server.Close()

	_, _, err := NewClient(server.URL, "secret").ResolveTeacherAssignments(context.Background(), "school-one", "user-1")
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("oversized response error=%v", err)
	}
}
