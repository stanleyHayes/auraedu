package httpx

import (
	"mime"
	"net/http"
	"strings"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-Id"

const (
	maxStandardRequestBodyBytes  int64 = 1 << 20
	maxMultipartRequestBodyBytes int64 = 40 << 20
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	bounded := RequestBoundaryMiddleware(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(RequestIDHeader))
		if id == "" {
			id = uuid.Must(uuid.NewV7()).String()
			r.Header.Set(RequestIDHeader, id)
		}
		w.Header().Set(RequestIDHeader, id)
		bounded.ServeHTTP(w, r)
	})
}

// RequestBoundaryMiddleware caps inbound bodies before a service handler can
// allocate or decode them. RequestIDMiddleware includes this boundary; the API
// Gateway uses it directly because its own middleware owns request-ID context.
func RequestBoundaryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := requestBodyLimit(r.Header.Get("Content-Type"))
		if r.ContentLength > limit {
			PayloadTooLarge(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

func requestBodyLimit(contentType string) int64 {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		return maxMultipartRequestBodyBytes
	}
	return maxStandardRequestBodyBytes
}

// RequirePermission returns HTTP middleware that allows the request only if the
// gateway-injected actor holds the given permission. Platform super-admins
// implicitly pass. Unauthenticated callers and actors without the permission
// receive a canonical 403 forbidden response.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := auth.FromHeaders(r.Header)
			if !actor.Authenticated() {
				Forbidden(w, r, "authentication required")
				return
			}
			if !actor.Has(perm) {
				Forbidden(w, r, "permission required: "+perm)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

func DefaultCORS() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", RequestIDHeader, tenancy.HeaderTenantID},
		AllowCredentials: false,
		MaxAgeSeconds:    86400,
	}
}

func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	allOrigins := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allOrigins = true
			break
		}
		allowedOrigins[o] = struct{}{}
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	maxAge := cfg.MaxAgeSeconds
	if maxAge == 0 {
		maxAge = 86400
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allOrigins {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allowedOrigins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					if cfg.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", itoa(maxAge))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
