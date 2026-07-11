// Package stubs provides local stand-ins for platform dependencies until they land.
package stubs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/auraedu/platform/auth"
)

type TenantResolver struct {
	ByHost           map[string]string
	BySubdomain      map[string]string
	TenantServiceURL string
	Client           *http.Client
}

type Tenant struct {
	ID   string
	Name string
}

var (
	ErrTenantRequired = errors.New("tenant context is required")
	ErrTenantNotFound = errors.New("tenant not found")
)

func (r *TenantResolver) Resolve(ctx context.Context, req *http.Request) (Tenant, error) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)

	if header := strings.TrimSpace(req.Header.Get("X-Tenant-ID")); header != "" {
		key := strings.ToLower(header)
		if t := r.lookupStatic(key); t.ID != "" {
			return t, nil
		}
		if r.TenantServiceURL != "" {
			if t, err := r.resolveFromService(ctx, req, "", key); err == nil {
				return t, nil
			}
		}
		return Tenant{ID: key}, nil
	}

	if t := r.lookupStatic(host); t.ID != "" {
		return t, nil
	}

	parts := strings.Split(host, ".")
	var subdomain string
	if len(parts) >= 2 {
		subdomain = parts[0]
		if t := r.lookupStatic(subdomain); t.ID != "" {
			return t, nil
		}
	}

	if r.TenantServiceURL != "" {
		if t, err := r.resolveFromService(ctx, req, host, subdomain); err == nil {
			return t, nil
		}
	}

	return Tenant{}, ErrTenantRequired
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

func (r *TenantResolver) resolveFromService(ctx context.Context, req *http.Request, domain, subdomain string) (Tenant, error) {
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

	for _, h := range []string{auth.HeaderUserID, auth.HeaderTenant, auth.HeaderRole, auth.HeaderPermissions} {
		if v := req.Header.Get(h); v != "" {
			out.Header.Set(h, v)
		}
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
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
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
