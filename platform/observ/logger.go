// Package observ provides shared observability primitives: structured logging
// with PII redaction, health handlers, metrics and tracing utilities.
package observ

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

//nolint:gochecknoglobals // Compiled regex table for PII redaction; package scope avoids recomputation on every log attribute.
var piiPatterns = []struct {
	key string
	re  *regexp.Regexp
}{
	{"password", regexp.MustCompile(`(?i)password|passwd|pwd`)},
	{"token", regexp.MustCompile(`(?i)token|authorization|jwt|api_key|apikey`)},
	{"email", regexp.MustCompile(`(?i)e?mail`)},
	{"phone", regexp.MustCompile(`(?i)phone|mobile|whatsapp`)},
	{"address", regexp.MustCompile(`(?i)address|residence|location`)},
	{"ssn", regexp.MustCompile(`(?i)ssn|social_security|national_id`)},
}

//nolint:gochecknoglobals // Compiled once because every emitted attribute is inspected.
var sensitiveValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`),
	regexp.MustCompile(`(?i)(bearer\s+|password\s*[=:]|access[_-]?token\s*[=:]|refresh[_-]?token\s*[=:]|api[_-]?key\s*[=:])\S+`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,}(?:\.[A-Za-z0-9_-]{5,})?\b`),
	regexp.MustCompile(`(?i)(?:\+?\d[\d ()-]{8,}\d)`),
}

func containsSensitiveValue(value string) bool {
	for _, pattern := range sensitiveValuePatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func redactReplacer(_ []string, a slog.Attr) slog.Attr {
	key := a.Key
	for _, p := range piiPatterns {
		if strings.EqualFold(key, p.key) || p.re.MatchString(key) {
			return slog.String(key, "[REDACTED]")
		}
	}
	value := fmt.Sprint(a.Value.Any())
	if containsSensitiveValue(value) {
		return slog.String(key, "[REDACTED]")
	}
	return a
}

// NewLogger returns a JSON slog.Logger with PII redaction configured.
func NewLogger(level slog.Leveler, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactReplacer,
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// DefaultLogger installs and returns an INFO-level redacting JSON logger.
// Installing it ensures packages that use slog.Default also inherit the same
// runtime PII protection as the service entrypoint's explicit logger.
func DefaultLogger() *slog.Logger {
	logger := NewLogger(slog.LevelInfo, os.Stdout)
	slog.SetDefault(logger)
	return logger
}

// LoggerFromContext returns the logger stored in ctx, or DefaultLogger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return DefaultLogger()
}

// WithLogger stores logger in parent and returns the derived context.
func WithLogger(parent context.Context, l *slog.Logger) context.Context {
	return context.WithValue(parent, loggerKey{}, l)
}

type loggerKey struct{}
