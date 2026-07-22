// Package gateway implements the api-gateway reverse proxy, middleware, and route registry.
package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/api-gateway/internal/stubs"
	"github.com/auraedu/platform/auth"
)

type TenantResolver interface {
	Resolve(ctx context.Context, req *http.Request) (stubs.Tenant, error)
	ResolveDomain(ctx context.Context, hostname string) (stubs.Tenant, error)
}

type FeatureFlagClient interface {
	IsEnabled(ctx context.Context, tenantID, feature string) bool
}

type Builder struct {
	Log          *slog.Logger
	Config       *Config
	Registry     ServiceRegistry
	Proxy        *ReverseProxy
	Tenant       TenantResolver
	Flags        FeatureFlagClient
	RateLimiter  RateLimiter
	Health       *HealthState
	Dependencies http.Handler
	Service      string
	Version      string
	SkipAuthFunc func(r *http.Request) bool
}

func (b *Builder) Build() http.Handler {
	mux := http.NewServeMux()
	if b.Service == "" {
		b.Service = "api-gateway"
	}
	if b.Health == nil {
		b.Health = NewHealth(b.Service, b.Version)
	}
	b.Health.Register(mux)
	if b.Dependencies != nil {
		// Platform health is gateway-owned, authenticated and tenant-optional. It
		// intentionally bypasses tenant/feature middleware because a platform
		// operator must still see outages when Tenant Service is unavailable.
		platformHealth := b.accessLog(b.securityHeaders(b.cors(b.requestID(b.auth(b.Dependencies)))))
		mux.Handle("/api/v1/platform/health", platformHealth)
	}

	api := b.chain(b.Proxy.Handler(b.Registry))
	mux.Handle("/api/v1/", api)
	mux.Handle("/api/v1", http.RedirectHandler("/api/v1/", http.StatusMovedPermanently))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			writeJSONError(w, http.StatusNotFound, "not_found", "unknown path")
			return
		}
		writeJSONError(w, http.StatusOK, "ok", "AuraEDU API Gateway")
	})

	return mux
}

func (b *Builder) chain(next http.Handler) http.Handler {
	// Order matters: access log wraps everything; response security policy is
	// applied before CORS handles preflight;
	// request-id is generated before auth/tenant so all logs carry it; auth and
	// tenant resolve the caller; feature-flag and rate-limit are edge gates
	// immediately before the upstream proxy.
	return b.accessLog(b.securityHeaders(b.cors(b.requestID(b.auth(b.tenant(b.permission(b.featureFlag(b.rateLimit(next)))))))))
}

func (b *Builder) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if b.Config.Environment == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Builder) skipAuth(r *http.Request) bool {
	if b.SkipAuthFunc != nil {
		return b.SkipAuthFunc(r)
	}
	path := r.URL.Path
	if (path == "/api/v1/website" || strings.HasPrefix(path, "/api/v1/website/")) && r.Method == http.MethodGet {
		return true
	}
	if rt, ok := b.Registry.Match(path); ok && rt.Public {
		return true
	}
	return false
}

func (b *Builder) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = generateRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

func generateRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (b *Builder) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := b.allowRequestOrigin(r.Context(), origin)
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		if len(b.Config.CORSMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(b.Config.CORSMethods, ", "))
		}
		if len(b.Config.CORSHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(b.Config.CORSHeaders, ", "))
		}
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id,X-Tenant-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Builder) allowOrigin(origin string) bool {
	return corsOriginAllowed(origin, b.Config.CORSOrigins)
}

func (b *Builder) allowRequestOrigin(ctx context.Context, origin string) bool {
	if b.allowOrigin(origin) {
		return true
	}
	parsed, err := url.Parse(origin)
	validOrigin := err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" &&
		parsed.Port() == "" && parsed.User == nil && parsed.Path == "" &&
		parsed.RawQuery == "" && parsed.Fragment == "" && b.Tenant != nil
	if !validOrigin {
		return false
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tenant, err := b.Tenant.ResolveDomain(lookupCtx, parsed.Hostname())
	return err == nil && tenant.ID != ""
}

func (b *Builder) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, hasToken := bearerToken(r)
		if b.skipAuth(r) {
			if hasToken {
				actorCtx, err := b.actorContext(r, token)
				if err != nil {
					writeJSONError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
					return
				}
				next.ServeHTTP(w, r.WithContext(actorCtx))
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if !hasToken {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid authorization header")
			return
		}

		actorCtx, err := b.actorContext(r, token)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired access token")
			return
		}
		next.ServeHTTP(w, r.WithContext(actorCtx))
	})
}

func (b *Builder) actorContext(r *http.Request, token string) (context.Context, error) {
	claims, err := auth.Verify(token, b.Config.SigningKey, time.Now())
	if err != nil {
		return nil, err
	}
	actor := claims.Actor()
	actorCtx := WithActor(r.Context(), ActorContext{
		UserID:      actor.UserID,
		TenantID:    actor.TenantID,
		Role:        actor.Role,
		Permissions: strings.Join(actor.Permissions, ", "),
		Platform:    actor.PlatformAdmin,
	})
	return auth.WithActor(actorCtx, actor), nil
}

func (b *Builder) permission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt, ok := b.Registry.Match(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		permission := requiredPermission(rt, r.Method)
		if permission == "" {
			next.ServeHTTP(w, r)
			return
		}

		actor := ActorFrom(r.Context())
		if actor.IsEmpty() {
			writeJSONError(w, http.StatusForbidden, "permission_required", "route requires an authenticated actor")
			return
		}
		if !actor.HasPermission(permission) {
			writeJSONError(w, http.StatusForbidden, "permission_denied", "actor lacks permission for this route")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requiredPermission(rt Route, method string) string {
	if len(rt.Permissions) > 0 {
		if p, ok := rt.Permissions[method]; ok {
			return p
		}
		return ""
	}
	return rt.Permission
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) || len(h) <= len(prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}

func (b *Builder) tenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route, ok := b.Registry.Match(r.URL.Path); ok && route.TenantOptional {
			next.ServeHTTP(w, r)
			return
		}
		if b.Tenant == nil {
			writeJSONError(w, http.StatusInternalServerError, "tenant_resolver_unavailable", "tenant resolution is not configured")
			return
		}

		tenant, err := b.Tenant.Resolve(r.Context(), r)
		if err != nil {
			if errors.Is(err, stubs.ErrTenantUnavailable) {
				writeJSONError(w, http.StatusServiceUnavailable, "tenant_resolver_unavailable", "tenant resolution is temporarily unavailable")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "tenant_required", "could not resolve tenant context")
			return
		}

		if tenant.ID == "" {
			writeJSONError(w, http.StatusBadRequest, "tenant_required", "tenant identifier is empty")
			return
		}

		actor := ActorFrom(r.Context())
		if !actor.IsEmpty() && !actor.Platform && actor.UserID != "" {
			if actor.TenantID == "" || actor.TenantID != tenant.ID {
				writeJSONError(w, http.StatusForbidden, "tenant_mismatch", "actor tenant does not match request tenant")
				return
			}
		}

		w.Header().Set("X-Tenant-ID", tenant.ID)
		next.ServeHTTP(w, r.WithContext(WithTenantID(r.Context(), tenant.ID)))
	})
}

func (b *Builder) featureFlag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt, ok := b.Registry.Match(r.URL.Path)
		if !ok || rt.FeatureKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		// A feature-owned route must never become available because the flag
		// dependency was omitted or failed to initialize. Treat an absent gate as
		// disabled; the live client itself already fails closed to its snapshot.
		if b.Flags == nil {
			writeJSONErrorWithDetails(w, http.StatusForbidden, "feature_disabled", "this feature is not enabled for the tenant", map[string]any{"feature": rt.FeatureKey})
			return
		}

		tenantID := TenantIDFrom(r.Context())
		if tenantID == "" {
			writeJSONError(w, http.StatusBadRequest, "tenant_required", "tenant context missing for feature check")
			return
		}

		if !b.Flags.IsEnabled(r.Context(), tenantID, rt.FeatureKey) {
			writeJSONErrorWithDetails(w, http.StatusForbidden, "feature_disabled", "this feature is not enabled for the tenant", map[string]any{"feature": rt.FeatureKey})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Builder) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.RateLimiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		route, routeFound := b.Registry.Match(r.URL.Path)
		tenantID := TenantIDFrom(r.Context())
		if tenantID == "" {
			if !routeFound || !route.TenantOptional {
				next.ServeHTTP(w, r)
				return
			}
			tenantID = "public:" + b.clientAddress(r)
		} else if routeFound && route.Public {
			// Public tenant routes must be isolated by both school and client. A
			// single abusive visitor must not exhaust the whole school's bucket.
			tenantID = tenantID + ":public:" + b.clientAddress(r)
		}

		ratePath := r.URL.Path
		if routeFound {
			// Dynamic IDs and one-time credentials must never enter Redis keys.
			// The canonical registry prefix also groups a route family correctly.
			ratePath = route.Prefix
		}
		key := fmt.Sprintf("rate:%s:%s %s", tenantID, r.Method, ratePath)
		allowed, err := b.RateLimiter.Allow(r.Context(), key)
		if err != nil {
			b.Log.Error("rate limiter error", "err", err, "request_id", RequestIDFrom(r.Context()))
			if routeFound && route.Public {
				w.Header().Set("Retry-After", strconv.Itoa(int(b.Config.RateLimitWindow.Seconds())))
				writeJSONError(w, http.StatusServiceUnavailable, "rate_limiter_unavailable", "public request protection is temporarily unavailable")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(b.Config.RateLimitWindow.Seconds())))
			writeJSONError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (b *Builder) clientAddress(r *http.Request) string {
	if b.Config.TrustedProxy == "render" {
		if cloudflareIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); net.ParseIP(cloudflareIP) != nil {
			return cloudflareIP
		}
	} else if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	address := r.RemoteAddr
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	if address == "" {
		return "unknown"
	}
	return address
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(p []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(p)
	rr.size += n
	return n, err
}

func (b *Builder) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := newResponseRecorder(w)
		next.ServeHTTP(rr, r)

		rid := rr.Header().Get("X-Request-Id")
		if rid == "" {
			rid = RequestIDFrom(r.Context())
		}
		actor := ActorFrom(r.Context())
		logPath := r.URL.Path
		if route, ok := b.Registry.Match(r.URL.Path); ok {
			// Keep record IDs and one-time credentials out of durable access logs.
			logPath = route.Prefix
		}
		b.Log.Info("access",
			"method", r.Method,
			"path", logPath,
			"status", rr.status,
			"size", rr.size,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", rid,
			"tenant_id", TenantIDFrom(r.Context()),
			"user_id", actor.UserID,
			"remote_addr", r.RemoteAddr,
			"cf_ray", boundedHeader(r.Header.Get("CF-Ray"), 128),
			"user_agent", r.UserAgent(),
		)
	})
}

func boundedHeader(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
