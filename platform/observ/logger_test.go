package observ

import (
	"bytes"
	"context"
	"errors"
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

func TestLoggerRedactsSensitiveValuesInsideMessagesAndErrors(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(slog.LevelInfo, &buf)
	l.Error(
		"delivery failed for sentinel.person@example.test",
		"err",
		errors.New("provider rejected Bearer sentinel-access-token and +233 24 555 0199"),
	)

	out := buf.String()
	for _, secret := range []string{
		"sentinel.person@example.test",
		"sentinel-access-token",
		"+233 24 555 0199",
	} {
		if strings.Contains(out, secret) {
			t.Fatalf("runtime log captured sensitive sentinel %q: %s", secret, out)
		}
	}
	if strings.Count(out, "[REDACTED]") < 2 {
		t.Fatalf("expected message and error redaction, got %s", out)
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

func TestDefaultLoggerInstallsRedactingProcessLogger(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })

	installed := DefaultLogger()
	if slog.Default() != installed {
		t.Fatal("expected DefaultLogger to install the process logger")
	}
}
