package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func response(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{}`)),
	}
}

func platformRequest(method string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), method, "/api/v1/platform/health", nil)
	return request.WithContext(WithActor(request.Context(), ActorContext{
		UserID:   "platform-operator",
		Role:     auth.RolePlatformSuperAdmin,
		Platform: true,
	}))
}

func TestDependencyHealthReportsConcurrentSafeStatuses(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "healthy.internal":
			return response(http.StatusOK), nil
		case "degraded.internal":
			return response(http.StatusServiceUnavailable), nil
		case "slow.internal":
			<-request.Context().Done()
			return nil, request.Context().Err()
		default:
			return nil, errors.New("private connection detail must not escape")
		}
	})}
	handler := NewDependencyHealthHandler([]Dependency{
		{Service: "slow", URL: "http://slow.internal", Path: "/health"},
		{Service: "healthy", URL: "http://healthy.internal", Path: "/ready"},
		{Service: "degraded", URL: "http://degraded.internal", Path: "/ready"},
	}, client, 5*time.Millisecond)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, platformRequest(http.MethodGet))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("Cache-Control=%q", cache)
	}
	var report DependencyHealthReport
	if err := json.Unmarshal(recorder.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Status != "degraded" || len(report.Checks) != 3 {
		t.Fatalf("report=%+v", report)
	}
	if report.Checks[0].Service != "degraded" || report.Checks[0].Status != "degraded" || report.Checks[0].Detail != "Service Unavailable" {
		t.Fatalf("degraded check=%+v", report.Checks[0])
	}
	if report.Checks[1].Service != "healthy" || report.Checks[1].Status != "healthy" {
		t.Fatalf("healthy check=%+v", report.Checks[1])
	}
	if report.Checks[2].Service != "slow" || report.Checks[2].Status != "unreachable" || report.Checks[2].Detail != "timeout" {
		t.Fatalf("slow check=%+v", report.Checks[2])
	}
	if strings.Contains(recorder.Body.String(), ".internal") || strings.Contains(recorder.Body.String(), "private connection detail") {
		t.Fatalf("report leaked private transport detail: %s", recorder.Body.String())
	}
}

func TestDependencyHealthRequiresPlatformAdministrator(t *testing.T) {
	handler := NewDependencyHealthHandler(nil, nil, 0)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/platform/health", nil)
	request = request.WithContext(WithActor(request.Context(), ActorContext{UserID: "school-admin", Role: "school_admin"}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "platform_admin_required") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPlatformHealthRouteAuthenticatesWithoutTenantResolution(t *testing.T) {
	builder := testBuilder()
	builder.Dependencies = NewDependencyHealthHandler([]Dependency{{Service: "student-service", URL: "http://student.internal", Path: "/ready"}}, &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(http.StatusOK), nil }),
	}, time.Second)
	handler := builder.Build()

	platformToken := signTestToken(builder, auth.Claims{Subject: "platform-operator", Role: auth.RolePlatformSuperAdmin})
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/platform/health", nil)
	request.Header.Set("Authorization", "Bearer "+platformToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"healthy"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	schoolToken := signTestToken(builder, auth.Claims{Subject: "school-admin", TenantID: "upshs", Role: "school_admin"})
	request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/platform/health", nil)
	request.Header.Set("Authorization", "Bearer "+schoolToken)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("school actor status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDefaultDependenciesAreUniqueAndConfigured(t *testing.T) {
	seen := make(map[string]bool)
	dependencies := DefaultDependencies()
	if len(dependencies) < 20 {
		t.Fatalf("dependency inventory unexpectedly small: %d", len(dependencies))
	}
	for _, dependency := range dependencies {
		if dependency.Service == "" || dependency.Path == "" {
			t.Fatalf("incomplete dependency: %+v", dependency)
		}
		if seen[dependency.Service] {
			t.Fatalf("duplicate dependency %q", dependency.Service)
		}
		seen[dependency.Service] = true
		parsed, err := url.Parse(dependency.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			t.Fatalf("invalid URL for %s: %q (%v)", dependency.Service, dependency.URL, err)
		}
	}
}

func TestAggregateDependencyStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		checks []DependencyCheck
		want   string
	}{
		{name: "empty", want: "down"},
		{name: "healthy", checks: []DependencyCheck{{Status: "healthy"}}, want: "healthy"},
		{name: "mixed", checks: []DependencyCheck{{Status: "healthy"}, {Status: "unreachable"}}, want: "degraded"},
		{name: "responding but degraded", checks: []DependencyCheck{{Status: "degraded"}}, want: "degraded"},
		{name: "all unreachable", checks: []DependencyCheck{{Status: "unreachable"}, {Status: "unreachable"}}, want: "down"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := aggregateDependencyStatus(test.checks); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
