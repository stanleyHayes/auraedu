// Package gateway implements the api-gateway reverse proxy, middleware, and route registry.
package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type ReverseProxy struct {
	log        *slog.Logger
	transports map[string]*httputil.ReverseProxy
}

func NewReverseProxy(registry ServiceRegistry, log *slog.Logger) (*ReverseProxy, error) {
	transports := make(map[string]*httputil.ReverseProxy, len(registry))
	for _, rt := range registry {
		target, err := url.Parse(rt.Target)
		if err != nil {
			return nil, err
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: rewriteForRoute(target),
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
		transports[rt.Prefix] = proxy
	}
	return &ReverseProxy{log: log, transports: transports}, nil
}

type ActorContext struct {
	UserID      string
	TenantID    string
	Role        string
	Permissions string
	Platform    bool
}

func (a ActorContext) IsEmpty() bool { return a.UserID == "" }

func (a ActorContext) HasPermission(permission string) bool {
	if a.IsEmpty() || permission == "" {
		return false
	}
	if a.Platform {
		return true
	}
	for _, p := range strings.Split(a.Permissions, ",") {
		if strings.TrimSpace(p) == permission {
			return true
		}
	}
	return false
}

func rewriteForRoute(target *url.URL) func(*httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.SetXForwarded()

		// Services register the full contract path (e.g., /api/v1/tenants/{code})
		// and expect the gateway to preserve it, not strip the route prefix.
		pr.Out.URL.Path = pr.In.URL.Path
		pr.Out.URL.RawPath = pr.In.URL.RawPath
		pr.Out.URL.RawQuery = pr.In.URL.RawQuery

		pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
		pr.Out.Header.Set("X-Forwarded-Proto", scheme(pr.In))
		// X-Actor-* headers are an internal gateway-to-service trust boundary.
		// Never forward client-supplied values, including on public routes.
		pr.Out.Header.Del("X-Actor-User")
		pr.Out.Header.Del("X-Actor-Tenant")
		pr.Out.Header.Del("X-Actor-Role")
		pr.Out.Header.Del("X-Actor-Permissions")
		if rid := RequestIDFrom(pr.In.Context()); rid != "" {
			pr.Out.Header.Set("X-Request-Id", rid)
		}
		if tenant := TenantIDFrom(pr.In.Context()); tenant != "" {
			pr.Out.Header.Set("X-Tenant-ID", tenant)
			pr.Out.Header.Set("X-Actor-Tenant", tenant)
		}
		if actor := ActorFrom(pr.In.Context()); !actor.IsEmpty() {
			pr.Out.Header.Set("X-Actor-User", actor.UserID)
			pr.Out.Header.Set("X-Actor-Role", actor.Role)
			if actor.Permissions != "" {
				pr.Out.Header.Set("X-Actor-Permissions", actor.Permissions)
			}
		}
	}
}

func (p *ReverseProxy) Handler(registry ServiceRegistry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt, ok := registry.Match(r.URL.Path)
		if !ok {
			writeJSONError(w, http.StatusNotFound, "route_not_found", "no downstream service for path")
			return
		}

		proxy, ok := p.transports[rt.Prefix]
		if !ok {
			writeJSONError(w, http.StatusNotFound, "route_not_found", "proxy not configured")
			return
		}

		proxy.ServeHTTP(w, r)
	})
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}

func writeJSONError(w http.ResponseWriter, code int, errCode, message string) {
	writeJSONErrorWithDetails(w, code, errCode, message, nil)
}

func writeJSONErrorWithDetails(w http.ResponseWriter, code int, errCode, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body := map[string]any{
		"code":    errCode,
		"message": message,
	}
	if details != nil {
		body["details"] = details
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": body,
	})
}
