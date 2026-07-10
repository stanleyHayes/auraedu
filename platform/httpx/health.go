// Package httpx provides the shared HTTP server surface for every AuraEDU Go
// service: health/readiness probes, a canonical error envelope, and middleware.
// Stdlib-only by design so services stay lean and start fast (agent_plan §7).
package httpx

import (
	"encoding/json"
	"net/http"
)

// HealthState reports a service's liveness/readiness. Readiness callbacks
// (DB ping, NATS ping, …) are registered per service and evaluated on /ready.
type HealthState struct {
	Service string
	Version string
	checks  []func() error
}

// NewHealth creates a HealthState for a named service.
func NewHealth(service, version string) *HealthState {
	return &HealthState{Service: service, Version: version}
}

// AddReadinessCheck registers a dependency probe evaluated on GET /ready.
func (h *HealthState) AddReadinessCheck(fn func() error) {
	h.checks = append(h.checks, fn)
}

// Register wires GET /health (liveness) and GET /ready (readiness) onto mux.
func (h *HealthState) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok", "service": h.Service, "version": h.Version,
		})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		for _, c := range h.checks {
			if err := c(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "not_ready", "service": h.Service, "error": err.Error(),
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
