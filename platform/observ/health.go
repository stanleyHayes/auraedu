package observ

import (
	"context"
	"net/http"
)

type HealthChecker func(ctx context.Context) error

func LiveHandler(service, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"` + service + `","version":"` + version + `"}`))
	}
}

func ReadyHandler(service string, checks []HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		for _, c := range checks {
			if err := c(ctx); err != nil {
				LoggerFromContext(ctx).Error("readiness check failed", "service", service, "err", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"not_ready","service":"` + service + `"}`))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready","service":"` + service + `"}`))
	}
}
