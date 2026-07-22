package flags

import (
	"context"
	"log/slog"
	"sync"
)

// WarnOnceFallback wraps the fallback gate of a live client so the first time
// the fallback is consulted — i.e. the live tenant-service lookup failed and
// the static registry snapshot is being served — a single warning is logged
// instead of one per request. The wrapped gate behaves exactly like fallback.
func WarnOnceFallback(fallback Gate, log *slog.Logger) Gate {
	if fallback == nil {
		fallback = NewStaticSnapshot()
	}
	if log == nil {
		log = slog.Default()
	}
	return &warnOnceFallback{
		fallback: fallback,
		log:      log,
		message:  "flags: tenant-service lookup failed; serving static registry fallback",
	}
}

// WarnOnceFailClosed returns a disabled gate and emits one bounded warning when
// production cannot obtain a live entitlement from Tenant Service.
func WarnOnceFailClosed(log *slog.Logger) Gate {
	if log == nil {
		log = slog.Default()
	}
	return &warnOnceFallback{
		fallback: NewStaticSnapshot(),
		log:      log,
		message:  "flags: tenant-service lookup failed; failing closed",
	}
}

type warnOnceFallback struct {
	fallback Gate
	log      *slog.Logger
	message  string
	once     sync.Once
}

func (g *warnOnceFallback) IsEnabled(ctx context.Context, tenantID, key string) bool {
	g.once.Do(func() {
		g.log.Warn(g.message,
			"tenant", tenantID, "feature", key)
	})
	return g.fallback.IsEnabled(ctx, tenantID, key)
}
