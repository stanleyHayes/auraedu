// Package httpx provides the shared HTTP server surface for every AuraEDU Go
// service: health/readiness probes, a canonical error envelope, and middleware.
// Stdlib-only by design so services stay lean and start fast (agent_plan §7).
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// readinessCheck is a named dependency probe (e.g. "postgres", "nats").
type readinessCheck struct {
	name string
	fn   func() error
}

// HealthState reports a service's liveness/readiness. Readiness callbacks
// (DB ping, NATS ping, …) are registered per service and evaluated on /ready.
type HealthState struct {
	Service string
	Version string
	log     *slog.Logger
	checks  []readinessCheck
}

// NewHealth creates a HealthState for a named service.
func NewHealth(service, version string) *HealthState {
	return &HealthState{Service: service, Version: version, log: slog.Default()}
}

// WithLogger sets the logger used to record readiness failures server-side.
func (h *HealthState) WithLogger(l *slog.Logger) *HealthState {
	if l != nil {
		h.log = l
	}
	return h
}

// AddReadinessCheck registers a named dependency probe evaluated on GET /ready.
func (h *HealthState) AddReadinessCheck(name string, fn func() error) {
	h.checks = append(h.checks, readinessCheck{name: name, fn: fn})
}

// Register wires GET /health (liveness) and GET /ready (readiness) onto mux.
// Readiness failures are logged server-side; the response never echoes internal
// error detail (only the safe check name) to avoid information disclosure.
func (h *HealthState) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok", "service": h.Service, "version": h.Version,
		})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		for _, c := range h.checks {
			if err := c.fn(); err != nil {
				h.log.Error("readiness check failed", "service", h.Service, "check", c.name, "err", err)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "not_ready", "service": h.Service, "check": c.name,
				})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": h.Service})
	})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
