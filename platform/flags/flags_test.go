package flags

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auraedu/platform/auth"
)

func TestStaticSnapshot(t *testing.T) {
	s := NewStaticSnapshot()
	s.Set("upshs", "assessments", true)
	s.Set("aboom-ame-zion-c", "assessments", false)

	ctx := context.Background()
	if !s.IsEnabled(ctx, "upshs", "assessments") {
		t.Fatal("expected assessments enabled for upshs")
	}
	if s.IsEnabled(ctx, "aboom-ame-zion-c", "assessments") {
		t.Fatal("expected assessments disabled for aboom")
	}
	if s.IsEnabled(ctx, "upshs", "cbt_exams") {
		t.Fatal("expected unset feature to be disabled")
	}
}

func TestRequireEnabled(t *testing.T) {
	s := NewStaticSnapshot()
	s.Set("upshs", "billing", true)

	ctx := context.Background()
	if err := RequireEnabled(ctx, s, "upshs", "billing"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RequireEnabled(ctx, s, "upshs", "cbt_exams"); err == nil {
		t.Fatal("expected feature disabled error")
	}
}

func TestTenantServiceClientEnabledAndFallback(t *testing.T) {
	ctx := context.Background()
	fallback := NewStaticSnapshot()
	fallback.Set("upshs", "cbt_exams", true)

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.URL.Path, "/api/v1/features") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("tenant") != "upshs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if requests == 3 && r.Header.Get("X-Actor-User") != "u1" {
			t.Errorf("expected X-Actor-User header u1, got %q", r.Header.Get("X-Actor-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tenant_code":"upshs","features":[{"feature_key":"assessments","is_enabled":true},{"feature_key":"cbt_exams","is_enabled":false}]}`))
	}))
	defer server.Close()

	client := NewTenantServiceClient(server.URL, fallback)
	if !client.IsEnabled(ctx, "upshs", "assessments") {
		t.Fatal("expected assessments enabled from service")
	}
	if client.IsEnabled(ctx, "upshs", "cbt_exams") {
		t.Fatal("expected cbt_exams disabled from service")
	}

	actorCtx := auth.WithActor(ctx, auth.Actor{UserID: "u1", TenantID: "upshs", Role: "teacher", Permissions: []string{"x"}})
	if !client.IsEnabled(actorCtx, "upshs", "assessments") {
		t.Fatal("expected assessments enabled with actor")
	}

	if client.IsEnabled(ctx, "unknown", "cbt_exams") {
		t.Fatal("expected fallback for unknown tenant (404)")
	}

	nilClient := (*TenantServiceClient)(nil)
	if nilClient.IsEnabled(ctx, "upshs", "cbt_exams") {
		t.Fatal("expected nil client to return false")
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "features.yaml")
	content := `version: 1
features:
  - key: billing
    plan_required: core
    defaults: { upshs: on, aboom: on }
  - key: cbt_exams
    plan_required: professional
    defaults: { upshs: on, aboom: off }
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	reg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("load yaml: %v", err)
	}
	if len(reg.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(reg.Features))
	}

	snap := reg.SnapshotFromRegistry()
	ctx := context.Background()
	if !snap.IsEnabled(ctx, "upshs", "cbt_exams") {
		t.Fatal("expected cbt_exams on for upshs")
	}
	if snap.IsEnabled(ctx, "aboom", "cbt_exams") {
		t.Fatal("expected cbt_exams off for aboom")
	}
}
