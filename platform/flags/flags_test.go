package flags

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
