// Package observ provides shared observability primitives: structured logging
// with PII redaction, health handlers, metrics and tracing utilities.
package observ

import (
	"context"
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

func redactReplacer(_ []string, a slog.Attr) slog.Attr {
	key := a.Key
	for _, p := range piiPatterns {
		if strings.EqualFold(key, p.key) || p.re.MatchString(key) {
			return slog.String(key, "[REDACTED]")
		}
	}
	if s, ok := a.Value.Any().(string); ok {
		if strings.HasPrefix(s, "eyJ") && len(s) > 20 {
			return slog.String(key, s[:8]+"...[REDACTED]")
		}
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

// DefaultLogger returns a logger writing INFO-level JSON to stdout.
func DefaultLogger() *slog.Logger {
	return NewLogger(slog.LevelInfo, os.Stdout)
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
