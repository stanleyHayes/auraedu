package tenant

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestActivateUsesAuthenticatedInternalEndpoint(t *testing.T) {
	client := NewClient("http://tenant-service:8082", "service-secret")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/tenants/school-one/activate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer service-secret" {
			t.Fatal("missing service authorization")
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})

	if err := client.Activate(context.Background(), "school-one"); err != nil {
		t.Fatalf("activate: %v", err)
	}
}

func TestActivateFailsClosedWithoutCredentials(t *testing.T) {
	if err := NewClient("http://tenant.invalid", "").Activate(context.Background(), "school-one"); err == nil {
		t.Fatal("activation succeeded without service credentials")
	}
}
