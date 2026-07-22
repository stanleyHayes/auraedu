package flags

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWarnOnceFallback(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	fallback := NewStaticSnapshot()
	fallback.Set("upshs", "fees", true)

	g := WarnOnceFallback(fallback, log)
	ctx := context.Background()

	if !g.IsEnabled(ctx, "upshs", "fees") {
		t.Fatal("expected wrapped gate to delegate to fallback")
	}
	if g.IsEnabled(ctx, "upshs", "unknown") {
		t.Fatal("expected unknown feature to stay disabled")
	}
	g.IsEnabled(ctx, "upshs", "fees")
	g.IsEnabled(ctx, "upshs", "fees")

	if n := strings.Count(buf.String(), "static registry fallback"); n != 1 {
		t.Fatalf("expected exactly one fallback warning, got %d", n)
	}
}

func TestWarnOnceFallbackNilArgs(t *testing.T) {
	g := WarnOnceFallback(nil, nil)
	if g.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("expected nil fallback to behave like an empty static snapshot")
	}
}

func TestWarnOnceFailClosed(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	g := WarnOnceFailClosed(log)

	if g.IsEnabled(context.Background(), "upshs", "fees") {
		t.Fatal("fail-closed fallback must disable every feature")
	}
	g.IsEnabled(context.Background(), "upshs", "fees")
	if n := strings.Count(buf.String(), "failing closed"); n != 1 {
		t.Fatalf("expected exactly one fail-closed warning, got %d", n)
	}
}
