package flags

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRuntimeGateDevelopmentUsesRegistryWithoutLiveService(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	fallback := NewStaticSnapshot()
	fallback.Set("upshs", "fees", true)

	gate := NewRuntimeGate("", fallback, nil)
	if !gate.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("expected development registry fallback")
	}
}

func TestRuntimeGateProductionFailsClosedWithoutLiveService(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	fallback := NewStaticSnapshot()
	fallback.Set("upshs", "fees", true)

	gate := NewRuntimeGate("", fallback, log)
	if gate.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("production must not grant a registry-default entitlement")
	}
	if !strings.Contains(logs.String(), "failing closed") {
		t.Fatalf("expected fail-closed warning, got %q", logs.String())
	}
}

func TestRuntimeGateProductionUsesLiveEntitlementButNotOutageFallback(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tenant_code":"upshs","features":[{"feature_key":"fees","is_enabled":true}]}`))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	fallback := NewStaticSnapshot()
	fallback.Set("upshs", "fees", true)
	gate := NewRuntimeGate(server.URL, fallback, nil)
	if !gate.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("expected live production entitlement")
	}
	if gate.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("production outage must fail closed instead of using registry defaults")
	}
}

func TestRuntimeGateDevelopmentUsesRegistryDuringOutage(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	fallback := NewStaticSnapshot()
	fallback.Set("upshs", "fees", true)
	gate := NewRuntimeGate(server.URL, fallback, nil)
	if !gate.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("expected development registry fallback during outage")
	}
}
