package observ

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHandlerExposesGoldenSignalsWithCanonicalRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/students/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := HTTPHandler("student-service", mux)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/students/student-secret-id", nil,
	))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`auraedu_http_requests_total{method="GET",route="GET /api/v1/students/{id}",service="student-service",status="202"} 1`,
		`auraedu_http_request_duration_seconds_count{method="GET",route="GET /api/v1/students/{id}",service="student-service"} 1`,
		`auraedu_http_requests_in_flight{service="student-service"} 0`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics missing %q:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "student-secret-id") {
		t.Fatal("raw path identifier leaked into metrics")
	}
}

func TestHTTPHandlerDoesNotCountMetricsScrapes(t *testing.T) {
	handler := HTTPHandler("test-service", http.NotFoundHandler())
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/metrics", nil,
	))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	if strings.Contains(recorder.Body.String(), `route="unmatched"`) {
		t.Fatal("metrics scrape counted as an application request")
	}
}

func TestHTTPHandlerProtectsMetricsWhenTokenConfigured(t *testing.T) {
	t.Setenv("METRICS_BEARER_TOKEN", "metrics-test-secret")
	handler := HTTPHandler("test-service", http.NotFoundHandler())
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}
	authorizedRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer metrics-test-secret")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK || !strings.Contains(authorized.Body.String(), "go_info") {
		t.Fatalf("authorized status=%d body=%s", authorized.Code, authorized.Body.String())
	}
}
