package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/api-gateway/internal/stubs"
	"github.com/auraedu/platform/auth"
)

type TenantResolver interface {
	Resolve(ctx context.Context, req *http.Request) (stubs.Tenant, error)
}

type FeatureFlagClient interface {
	IsEnabled(tenantID, feature string) bool
}

type Builder struct {
	Log          *slog.Logger
	Config       *Config
	Registry     ServiceRegistry
	Proxy        *ReverseProxy
	Tenant       TenantResolver
	Flags        FeatureFlagClient
	RateLimiter  RateLimiter
	Service      string
	Version      string
	SkipAuthFunc func(r *http.Request) bool
}

func (b *Builder) Build() http.Handler {
	mux := http.NewServeMux()
	if b.Service == "" {
		b.Service = "api-gateway"
	}
	registerHealth(mux, b.Service, b.Version)

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
	// Order matters: access log wraps everything; CORS handles preflight early;
	// request-id is generated before auth/tenant so all logs carry it; auth and
	// tenant resolve the caller; feature-flag and rate-limit are edge gates
	// immediately before the upstream proxy.
	return b.accessLog(b.cors(b.requestID(b.auth(b.tenant(b.permission(b.featureFlag(b.rateLimit(next))))))))
}

func (b *Builder) skipAuth(r *http.Request) bool {
	if b.SkipAuthFunc != nil {
		return b.SkipAuthFunc(r)
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/identity/login") ||
		strings.HasPrefix(path, "/api/v1/identity/register") ||
		strings.HasPrefix(path, "/api/v1/identity/refresh") ||
		strings.HasPrefix(path, "/api/v1/identity/forgot-password") ||
		(strings.HasPrefix(path, "/api/v1/website/") && r.Method == http.MethodGet) {
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
		allowed := b.allowOrigin(origin)
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
	if origin == "" {
		return false
	}
	for _, o := range b.Config.CORSOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func (b *Builder) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.skipAuth(r) {
			next.ServeHTTP(w, r)
			return
		}

		token, ok := bearerToken(r)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid authorization header")
			return
		}

		claims, err := auth.Verify(token, b.Config.SigningKey, time.Now())
		if err != nil {
			if err == auth.ErrExpiredToken {
				writeJSONError(w, http.StatusUnauthorized, "token_expired", "token expired")
			} else {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
			}
			return
		}

		actor := claims.Actor()
		ctx := WithActor(r.Context(), ActorContext{
			UserID:      actor.UserID,
			Role:        actor.Role,
			Permissions: strings.Join(actor.Permissions, ", "),
			Platform:    actor.PlatformAdmin,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
		if b.Tenant == nil {
			writeJSONError(w, http.StatusInternalServerError, "tenant_resolver_unavailable", "tenant resolution is not configured")
			return
		}

		tenant, err := b.Tenant.Resolve(r.Context(), r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "tenant_required", "could not resolve tenant context")
			return
		}

		if tenant.ID == "" {
			writeJSONError(w, http.StatusBadRequest, "tenant_required", "tenant identifier is empty")
			return
		}

		actor := ActorFrom(r.Context())
		if !actor.IsEmpty() && !actor.Platform && actor.UserID != "" {
			if tenantID := TenantIDFrom(r.Context()); tenantID != "" && tenantID != tenant.ID {
				writeJSONError(w, http.StatusForbidden, "tenant_mismatch", "actor tenant does not match request tenant")
				return
			}
			if actorTenantHeader := r.Header.Get("X-Actor-Tenant"); actorTenantHeader != "" && actorTenantHeader != tenant.ID {
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
		if !ok || rt.FeatureKey == "" || b.Flags == nil {
			next.ServeHTTP(w, r)
			return
		}

		tenantID := TenantIDFrom(r.Context())
		if tenantID == "" {
			writeJSONError(w, http.StatusBadRequest, "tenant_required", "tenant context missing for feature check")
			return
		}

		if !b.Flags.IsEnabled(tenantID, rt.FeatureKey) {
			writeJSONError(w, http.StatusForbidden, "feature_disabled", "this feature is not enabled for the tenant")
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

		tenantID := TenantIDFrom(r.Context())
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := fmt.Sprintf("rate:%s:%s %s", tenantID, r.Method, r.URL.Path)
		allowed, err := b.RateLimiter.Allow(r.Context(), key)
		if err != nil {
			b.Log.Error("rate limiter error", "err", err, "request_id", RequestIDFrom(r.Context()))
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
		b.Log.Info("access",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rr.status,
			"size", rr.size,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", rid,
			"tenant_id", TenantIDFrom(r.Context()),
			"user_id", actor.UserID,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}
