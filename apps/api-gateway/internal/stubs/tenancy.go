// Package stubs provides local stand-ins for platform dependencies until they land.
package stubs

import (
	"context"
	"errors"
	"net/http"
	"strings"
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
	if header := strings.TrimSpace(req.Header.Get("X-Tenant-ID")); header != "" {
		if t := r.lookupStatic(header); t.ID != "" {
			return t, nil
		}
		if r.TenantServiceURL != "" {
			return r.resolveFromService(ctx, header)
		}
		return Tenant{ID: header}, nil
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}

	if t := r.lookupStatic(host); t.ID != "" {
		return t, nil
	}

	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		sub := strings.ToLower(parts[0])
		if t := r.lookupStatic(sub); t.ID != "" {
			return t, nil
		}
		if r.TenantServiceURL != "" {
			if t, err := r.resolveFromService(ctx, sub); err == nil {
				return t, nil
			}
		}
	}

	if r.TenantServiceURL != "" {
		if t, err := r.resolveFromService(ctx, host); err == nil {
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

func (r *TenantResolver) resolveFromService(ctx context.Context, identifier string) (Tenant, error) {
	_ = ctx
	_ = identifier
	return Tenant{}, ErrTenantNotFound
}
