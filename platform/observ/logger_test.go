package observ

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLoggerRedactsPII(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(slog.LevelInfo, &buf)
	l.Info("user action", "email", "alice@example.com", "password", "secret123", "token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test")

	out := buf.String()
	if strings.Contains(out, "alice@example.com") {
		t.Fatal("email should be redacted")
	}
	if strings.Contains(out, "secret123") {
		t.Fatal("password should be redacted")
	}
	if strings.Contains(out, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test") {
		t.Fatal("JWT should be redacted")
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatal("expected [REDACTED] in output")
	}
}

func TestLoggerFromContext(t *testing.T) {
	ctx := context.Background()
	defaultLogger := LoggerFromContext(ctx)
	if defaultLogger == nil {
		t.Fatal("expected default logger")
	}

	custom := DefaultLogger()
	ctx = WithLogger(ctx, custom)
	if LoggerFromContext(ctx) != custom {
		t.Fatal("expected custom logger from context")
	}
}
