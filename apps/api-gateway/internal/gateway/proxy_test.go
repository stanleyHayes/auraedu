package gateway

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestProxyPreservesPathAndForwardsHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/students/123" {
			t.Errorf("upstream path: got %q, want %q", r.URL.Path, "/api/v1/students/123")
		}
		if got := r.Header.Get("X-Actor-User"); got != "u1" {
			t.Errorf("actor user: got %q, want u1", got)
		}
		if got := r.Header.Get("X-Actor-Tenant"); got != "upshs" {
			t.Errorf("actor tenant: got %q, want upshs", got)
		}
		if got := r.Header.Get("X-Actor-Role"); got != "teacher" {
			t.Errorf("actor role: got %q, want teacher", got)
		}
		if got := r.Header.Get("X-Actor-Permissions"); got != "" {
			t.Errorf("client-forged permissions leaked upstream: %q", got)
		}
		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, "pong"); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer upstream.Close()

	reg := ServiceRegistry{{Prefix: "/api/v1/students", Target: upstream.URL}}
	proxy, err := NewReverseProxy(reg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/students/123", nil)
	req.Header.Set("X-Actor-User", "attacker")
	req.Header.Set("X-Actor-Tenant", "aboom")
	req.Header.Set("X-Actor-Role", "platform_super_admin")
	req.Header.Set("X-Actor-Permissions", "*")
	req = req.WithContext(WithRequestID(req.Context(), "rid-1"))
	req = req.WithContext(WithTenantID(req.Context(), "upshs"))
	req = req.WithContext(WithActor(req.Context(), ActorContext{UserID: "u1", Role: "teacher"}))

	rr := httptest.NewRecorder()
	proxy.Handler(reg).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "pong" {
		t.Fatalf("body: got %q, want pong", rr.Body.String())
	}
	if rr.Header().Get("X-Upstream") != "ok" {
		t.Fatal("expected upstream response header")
	}
}

func TestProxyStripsForgedActorHeadersOnPublicRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, name := range []string{"X-Actor-User", "X-Actor-Tenant", "X-Actor-Role", "X-Actor-Permissions"} {
			if got := r.Header.Get(name); got != "" {
				t.Errorf("%s leaked to public upstream: %q", name, got)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	reg := ServiceRegistry{{Prefix: "/api/v1/public", Target: upstream.URL, Public: true, TenantOptional: true}}
	proxy, err := NewReverseProxy(reg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/public/intake", nil)
	req.Header.Set("X-Actor-User", "attacker")
	req.Header.Set("X-Actor-Tenant", "victim-school")
	req.Header.Set("X-Actor-Role", "platform_super_admin")
	req.Header.Set("X-Actor-Permissions", "*")
	rr := httptest.NewRecorder()
	proxy.Handler(reg).ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestProxyReturns404ForUnknownRoute(t *testing.T) {
	proxy, err := NewReverseProxy(ServiceRegistry{}, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/unknown", nil)
	rr := httptest.NewRecorder()
	proxy.Handler(ServiceRegistry{}).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}
