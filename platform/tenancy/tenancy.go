// Package tenancy extracts, propagates and enforces tenant context across HTTP
// requests, JWT claims and CloudEvents. Every downstream service imports this
// package to guarantee that all reads/writes are scoped to a single school.
package tenancy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/auraedu/platform/auth"
)

const (
	HeaderTenantID  = "X-Tenant-Id"
	HeaderRequestID = "X-Request-Id"
	HeaderActorUser = "X-Actor-User"
)

var (
	ErrMissingTenant  = errors.New("tenancy: tenant_id is required")
	ErrTenantMismatch = errors.New("tenancy: actor cannot access tenant")
)

type TenantContext struct {
	TenantID  string
	RequestID string
	ActorID   string
	ActorRole string
}

type ctxKey struct{}

var tenantCtxKey ctxKey

func WithContext(parent context.Context, tc TenantContext) context.Context {
	return context.WithValue(parent, tenantCtxKey, tc)
}

func FromContext(ctx context.Context) (TenantContext, bool) {
	v, ok := ctx.Value(tenantCtxKey).(TenantContext)
	return v, ok
}

func MustFromContext(ctx context.Context) TenantContext {
	v, ok := FromContext(ctx)
	if !ok {
		panic("tenancy: no tenant context in context")
	}
	return v
}

func TenantID(ctx context.Context) string {
	if v, ok := FromContext(ctx); ok {
		return v.TenantID
	}
	return ""
}

func RequestID(ctx context.Context) string {
	if v, ok := FromContext(ctx); ok {
		return v.RequestID
	}
	return ""
}

func FromRequest(r *http.Request, jwtKey []byte) (TenantContext, error) {
	if tenant := strings.TrimSpace(r.Header.Get(HeaderTenantID)); tenant != "" {
		return tenantContextFromHeaders(r), nil
	}
	if token, ok := bearerToken(r); ok && len(jwtKey) > 0 {
		claims, err := auth.Verify(token, jwtKey, time.Now())
		if err == nil && claims.TenantID != "" {
			return TenantContext{
				TenantID:  claims.TenantID,
				RequestID: r.Header.Get(HeaderRequestID),
				ActorID:   claims.Subject,
				ActorRole: claims.Role,
			}, nil
		}
	}
	return TenantContext{}, ErrMissingTenant
}

func tenantContextFromHeaders(r *http.Request) TenantContext {
	return TenantContext{
		TenantID:  strings.TrimSpace(r.Header.Get(HeaderTenantID)),
		RequestID: strings.TrimSpace(r.Header.Get(HeaderRequestID)),
		ActorID:   strings.TrimSpace(r.Header.Get(HeaderActorUser)),
		ActorRole: strings.TrimSpace(r.Header.Get(auth.HeaderRole)),
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):], true
	}
	return "", false
}

func Middleware(jwtKey []byte, enforce bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, err := FromRequest(r, jwtKey)
			if err != nil {
				if enforce {
					http.Error(w, `{"error":"tenant_mismatch","message":"tenant context required"}`, http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), tc)))
		})
	}
}

func CacheKey(tenantID, key string) string {
	if tenantID == "" {
		return key
	}
	return fmt.Sprintf("tenant:%s:%s", tenantID, key)
}

func FilePath(tenantID, subpath string) string {
	if tenantID == "" {
		return path.Clean("/" + subpath)
	}
	return path.Clean("/" + path.Join(tenantID, subpath))
}

func ValidateAccess(actor auth.Actor, tenantID string) error {
	if actor.PlatformAdmin {
		return nil
	}
	if actor.Authenticated() && actor.TenantID == tenantID {
		return nil
	}
	return ErrTenantMismatch
}
