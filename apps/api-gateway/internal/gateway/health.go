// Package gateway implements the api-gateway reverse proxy, middleware, and route registry.
package gateway

import (
	"encoding/json"
	"net/http"
)

type HealthState struct {
	Service string
	Version string
	checks  []readinessCheck
}

type readinessCheck struct {
	name string
	fn   func() error
}

func NewHealth(service, version string) *HealthState {
	return &HealthState{Service: service, Version: version}
}

func (h *HealthState) AddReadinessCheck(name string, fn func() error) {
	h.checks = append(h.checks, readinessCheck{name: name, fn: fn})
}

func (h *HealthState) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeHealthJSON(w, http.StatusOK, map[string]string{
			"status": "ok", "service": h.Service, "version": h.Version,
		})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		for _, c := range h.checks {
			if err := c.fn(); err != nil {
				writeHealthJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "not_ready", "service": h.Service, "check": c.name,
				})
				return
			}
		}
		writeHealthJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": h.Service})
	})
}

func writeHealthJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func registerHealth(mux *http.ServeMux, service, version string) {
	NewHealth(service, version).Register(mux)
}
