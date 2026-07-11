// Package http provides the HTTP adapter for the audit service.
package http

import (
	"net/http"

	"github.com/auraedu/platform/httpx"
)

// Handler exposes the minimal HTTP surface for the audit service. For the
// base story this is only liveness/readiness probes; CRUD endpoints are out of
// scope (AURA-23.1).
type Handler struct {
	health *httpx.HealthState
}

// NewHandler creates the HTTP adapter.
func NewHandler(health *httpx.HealthState) *Handler {
	return &Handler{health: health}
}

// Register mounts /health and /ready onto mux.
func (h *Handler) Register(mux *http.ServeMux) {
	h.health.Register(mux)
}
