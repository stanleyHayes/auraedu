package gateway

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestProxyStripsPrefixAndForwardsHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/123" {
			t.Errorf("upstream path: got %q, want %q", r.URL.Path, "/123")
		}
		w.Header().Set("X-Upstream", "ok")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "pong")
	}))
	defer upstream.Close()

	reg := ServiceRegistry{{Prefix: "/api/v1/students", Target: upstream.URL}}
	proxy, err := NewReverseProxy(reg, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students/123", nil)
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

func TestProxyReturns404ForUnknownRoute(t *testing.T) {
	proxy, err := NewReverseProxy(ServiceRegistry{}, slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("new proxy: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	rr := httptest.NewRecorder()
	proxy.Handler(ServiceRegistry{}).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}
