// Package stubs provides local stand-ins for platform dependencies until they land.
package stubs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/auraedu/platform/httpx"
)

type TenantResolver struct {
	ByHost                map[string]string
	BySubdomain           map[string]string
	TenantServiceURL      string
	SubdomainBaseDomain   string
	Client                *http.Client
	AllowUnverifiedHeader bool
}

type Tenant struct {
	ID   string
	Name string
}

var (
	ErrTenantRequired    = errors.New("tenant context is required")
	ErrTenantNotFound    = errors.New("tenant not found")
	ErrTenantUnavailable = errors.New("tenant resolution unavailable")
)

func (r *TenantResolver) Resolve(ctx context.Context, req *http.Request) (Tenant, error) {
	host := requestHost(req)
	if key := tenantHeader(req); key != "" {
		return r.resolveHeader(ctx, key)
	}

	if t := r.lookupStatic(host); t.ID != "" {
		return t, nil
	}

	subdomain := tenantSubdomain(host, r.SubdomainBaseDomain)
	if subdomain != "" {
		if t := r.lookupStatic(subdomain); t.ID != "" {
			return t, nil
		}
	}

	if r.TenantServiceURL != "" {
		t, err := r.resolveFromService(ctx, host, subdomain)
		if err == nil {
			return t, nil
		}
		if errors.Is(err, ErrTenantNotFound) {
			return Tenant{}, err
		}
		return Tenant{}, errors.Join(ErrTenantUnavailable, err)
	}

	return Tenant{}, ErrTenantRequired
}

func requestHost(req *http.Request) string {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if index := strings.Index(host, ":"); index >= 0 {
		host = host[:index]
	}
	return strings.ToLower(host)
}

func tenantHeader(req *http.Request) string {
	value := strings.TrimSpace(req.Header.Get("X-Tenant-ID"))
	if value == "" {
		value = strings.TrimSpace(req.Header.Get("X-Tenant-Code"))
	}
	return strings.ToLower(value)
}

func tenantSubdomain(host, configuredBase string) string {
	baseDomain := strings.ToLower(strings.Trim(strings.TrimSpace(configuredBase), "."))
	if baseDomain == "" || !strings.HasSuffix(host, "."+baseDomain) {
		return ""
	}
	prefix := strings.TrimSuffix(host, "."+baseDomain)
	if prefix == "" || strings.Contains(prefix, ".") {
		return ""
	}
	return prefix
}

func (r *TenantResolver) resolveHeader(ctx context.Context, key string) (Tenant, error) {
	if tenant := r.lookupStatic(key); tenant.ID != "" {
		return tenant, nil
	}
	if r.TenantServiceURL != "" {
		tenant, err := r.resolveFromService(ctx, "", key)
		if err == nil || errors.Is(err, ErrTenantNotFound) {
			return tenant, err
		}
		if !r.AllowUnverifiedHeader {
			return Tenant{}, errors.Join(ErrTenantUnavailable, err)
		}
	}
	if r.AllowUnverifiedHeader {
		return Tenant{ID: key}, nil
	}
	return Tenant{}, ErrTenantNotFound
}

// ResolveDomain performs an exact custom-hostname lookup. It deliberately does
// not fall back to the first DNS label, because an attacker-controlled domain
// such as school.attacker.example must never inherit the "school" tenant.
func (r *TenantResolver) ResolveDomain(ctx context.Context, hostname string) (Tenant, error) {
	hostname = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname), "."))
	if hostname == "" {
		return Tenant{}, ErrTenantRequired
	}
	if r.ByHost != nil {
		if id, ok := r.ByHost[hostname]; ok {
			return Tenant{ID: id}, nil
		}
	}
	if r.TenantServiceURL == "" {
		return Tenant{}, ErrTenantNotFound
	}
	return r.resolveFromService(ctx, hostname, "")
}

func (r *TenantResolver) lookupStatic(key string) Tenant {
	key = strings.ToLower(key)
	if r.ByHost != nil {
		if id, ok := r.ByHost[key]; ok {
			return Tenant{ID: id}
		}
	}
	if r.BySubdomain != nil {
		if id, ok := r.BySubdomain[key]; ok {
			return Tenant{ID: id}
		}
	}
	return Tenant{}
}

func (r *TenantResolver) resolveFromService(ctx context.Context, domain, subdomain string) (Tenant, error) {
	base, err := url.Parse(r.TenantServiceURL)
	if err != nil {
		return Tenant{}, fmt.Errorf("invalid tenant service URL: %w", err)
	}
	u := base.JoinPath("api", "v1", "tenants", "resolve")
	q := u.Query()
	if domain != "" {
		q.Set("domain", domain)
	}
	if subdomain != "" {
		q.Set("subdomain", subdomain)
	}
	u.RawQuery = q.Encode()

	out, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Tenant{}, err
	}

	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(out)
	if err != nil {
		return Tenant{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // response body close errors are safe to ignore

	switch resp.StatusCode {
	case http.StatusOK:
		var tr tenantResponse
		if err := httpx.DecodeJSONResponse(resp.Body, &tr); err != nil {
			return Tenant{}, err
		}
		return Tenant{ID: tr.TenantCode, Name: tr.Name}, nil
	case http.StatusNotFound:
		return Tenant{}, ErrTenantNotFound
	default:
		return Tenant{}, fmt.Errorf("tenant service returned status %d", resp.StatusCode)
	}
}

type tenantResponse struct {
	TenantCode string `json:"tenant_code"`
	Name       string `json:"name"`
}
