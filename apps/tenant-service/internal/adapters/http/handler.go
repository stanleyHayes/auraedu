// Package http adapts HTTP to the Tenant Service use cases (no business logic here).
// Routes align with contracts/openapi/tenant.v1.yaml. Until the gateway injects tenant
// context (EP-03), /features resolves the tenant from the X-Tenant-Code header or ?tenant=.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tenants", h.listTenants)
	mux.HandleFunc("GET /api/v1/tenants/{code}", h.getTenant)
	mux.HandleFunc("GET /api/v1/tenants/{code}/branding", h.branding)
	mux.HandleFunc("GET /api/v1/features", h.features)
	mux.HandleFunc("PUT /api/v1/features/{key}", h.setFeature)
}

func tenantCode(r *http.Request) string {
	if c := r.Header.Get("X-Tenant-Code"); c != "" {
		return c
	}
	return r.URL.Query().Get("tenant")
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.svc.ListTenants(auth.FromHeaders(r.Header))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tenants, "next_cursor": nil})
}

func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTenant(auth.FromHeaders(r.Header), r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) branding(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Branding(r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) features(w http.ResponseWriter, r *http.Request) {
	code := tenantCode(r)
	fs, err := h.svc.Features(auth.FromHeaders(r.Header), code)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_code": code, "features": fs})
}

type setFeatureBody struct {
	Tenant  string `json:"tenant_code"`
	Enabled bool   `json:"is_enabled"`
}

func (h *Handler) setFeature(w http.ResponseWriter, r *http.Request) {
	var body setFeatureBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "invalid request body"))
		return
	}
	code := body.Tenant
	if code == "" {
		code = tenantCode(r)
	}
	f, err := h.svc.SetFeature(auth.FromHeaders(r.Header), code, r.PathValue("key"), body.Enabled)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// --- helpers (canonical error envelope, agent_plan §6.3) ---

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func errEnv(code, msg string) map[string]string {
	return map[string]string{"code": code, "message": msg}
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		writeJSON(w, http.StatusForbidden, errEnv("forbidden", "not permitted for this actor or tenant"))
	case errors.Is(err, domain.ErrEntitlement):
		writeJSON(w, http.StatusForbidden, errEnv("plan_required", "the tenant's plan does not include this feature"))
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errEnv("not_found", "tenant not found"))
	case errors.Is(err, domain.ErrNoTenant):
		writeJSON(w, http.StatusBadRequest, errEnv("tenant_mismatch", "tenant context required (X-Tenant-Code header or ?tenant=)"))
	case errors.Is(err, domain.ErrValidation):
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "unknown feature key"))
	default:
		writeJSON(w, http.StatusInternalServerError, errEnv("internal", "unexpected error"))
	}
}
