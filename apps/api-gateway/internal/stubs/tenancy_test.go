package stubs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func tenantRequest(t *testing.T) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://api.auraedu.com/api/v1/students", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Tenant-ID", "unverified-school")
	return request
}

func TestProductionResolverRejectsUnverifiedTenantHeader(t *testing.T) {
	resolver := &TenantResolver{}

	if _, err := resolver.Resolve(context.Background(), tenantRequest(t)); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("error=%v, want tenant not found", err)
	}
}

func TestDevelopmentResolverMayUseExplicitUnverifiedFixtureMode(t *testing.T) {
	resolver := &TenantResolver{AllowUnverifiedHeader: true}

	tenant, err := resolver.Resolve(context.Background(), tenantRequest(t))
	if err != nil || tenant.ID != "unverified-school" {
		t.Fatalf("tenant=%+v error=%v", tenant, err)
	}
}

func TestProductionResolverFailsClosedWhenTenantServiceIsUnavailable(t *testing.T) {
	resolver := &TenantResolver{
		TenantServiceURL: "http://tenant-service:8082",
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dependency unavailable")
		})},
	}

	if _, err := resolver.Resolve(context.Background(), tenantRequest(t)); !errors.Is(err, ErrTenantUnavailable) {
		t.Fatalf("error=%v, want tenant unavailable", err)
	}
}

func TestProductionResolverDoesNotConvertNotFoundIntoTenant(t *testing.T) {
	resolver := &TenantResolver{
		TenantServiceURL: "http://tenant-service:8082",
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{"error":"not_found"}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	if _, err := resolver.Resolve(context.Background(), tenantRequest(t)); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("error=%v, want tenant not found", err)
	}
}

func TestTenantLookupNeverForwardsClientActorHeaders(t *testing.T) {
	resolver := &TenantResolver{
		TenantServiceURL: "http://tenant-service:8082",
		Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			for _, header := range []string{"X-Actor-User", "X-Actor-Tenant", "X-Actor-Role", "X-Actor-Permissions"} {
				if value := request.Header.Get(header); value != "" {
					t.Fatalf("client-forged internal header %s reached Tenant Service: %q", header, value)
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"tenant_code":"verified-school","name":"Verified School"}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	request := tenantRequest(t)
	request.Header.Set("X-Actor-User", "attacker")
	request.Header.Set("X-Actor-Tenant", "victim-school")
	request.Header.Set("X-Actor-Role", "platform_super_admin")
	request.Header.Set("X-Actor-Permissions", "*")

	tenant, err := resolver.Resolve(context.Background(), request)
	if err != nil || tenant.ID != "verified-school" {
		t.Fatalf("tenant=%+v error=%v", tenant, err)
	}
}
