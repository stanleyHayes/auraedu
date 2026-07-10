package observ

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

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

func redactReplacer(groups []string, a slog.Attr) slog.Attr {
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

func DefaultLogger() *slog.Logger {
	return NewLogger(slog.LevelInfo, os.Stdout)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return DefaultLogger()
}

func WithLogger(parent context.Context, l *slog.Logger) context.Context {
	return context.WithValue(parent, loggerKey{}, l)
}

type loggerKey struct{}
