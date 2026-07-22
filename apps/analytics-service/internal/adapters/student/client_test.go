package student

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestClientResolvesAuthenticatedTeacherRoster(t *testing.T) {
	client := NewClient("http://student.internal", "secret")
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get(tenancy.HeaderTenantID) != "school-a" {
			t.Fatalf("missing internal authentication headers")
		}
		if r.URL.Query().Get("user_id") != "teacher-a" || r.URL.Query().Get("role") != "teacher" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"student_ids":["student-a","student-b"],"class_ids":["class-a"]}`)),
		}, nil
	})}

	scope, err := client.Resolve(context.Background(), "school-a", "teacher-a", "teacher")
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.StudentIDs) != 2 || scope.StudentIDs[0] != "student-a" || scope.StudentIDs[1] != "student-b" {
		t.Fatalf("unexpected scope: %+v", scope)
	}
}

func TestClientFailsClosedWhenUnconfigured(t *testing.T) {
	_, err := NewClient("", "").Resolve(context.Background(), "school-a", "teacher-a", "teacher")
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}
