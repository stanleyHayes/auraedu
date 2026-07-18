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
	return &warnOnceFallback{fallback: fallback, log: log}
}

type warnOnceFallback struct {
	fallback Gate
	log      *slog.Logger
	once     sync.Once
}

func (g *warnOnceFallback) IsEnabled(ctx context.Context, tenantID, key string) bool {
	g.once.Do(func() {
		g.log.Warn("flags: tenant-service lookup failed; serving static registry fallback",
			"tenant", tenantID, "feature", key)
	})
	return g.fallback.IsEnabled(ctx, tenantID, key)
}
