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
	mux.HandleFunc("POST /api/v1/tenants", h.createTenant)
	mux.HandleFunc("GET /api/v1/tenants/resolve", h.resolveTenant)
	mux.HandleFunc("GET /api/v1/tenants/{code}", h.getTenant)
	mux.HandleFunc("PATCH /api/v1/tenants/{code}", h.updateTenant)
	mux.HandleFunc("DELETE /api/v1/tenants/{code}", h.deleteTenant)
	mux.HandleFunc("GET /api/v1/tenants/{code}/branding", h.branding)
	mux.HandleFunc("GET /api/v1/features", h.features)
	mux.HandleFunc("PUT /api/v1/features/{key}", h.setFeature)
	mux.HandleFunc("POST /api/v1/super-admin/features/{key}/override", h.overrideFeature)
}

func tenantCode(r *http.Request) string {
	if c := r.Header.Get("X-Tenant-Code"); c != "" {
		return c
	}
	return r.URL.Query().Get("tenant")
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.svc.ListTenants(r.Context(), auth.FromHeaders(r.Header))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tenants, "next_cursor": nil})
}

type createTenantBody struct {
	Code     string          `json:"tenant_code"`
	Name     string          `json:"name"`
	Short    string          `json:"short"`
	Status   string          `json:"status"`
	Plan     string          `json:"plan"`
	Branding domain.Branding `json:"branding"`
}

func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var body createTenantBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "invalid request body"))
		return
	}
	t := domain.Tenant{
		Code:     body.Code,
		Name:     body.Name,
		Short:    body.Short,
		Status:   body.Status,
		Plan:     body.Plan,
		Branding: body.Branding,
	}
	created, err := h.svc.CreateTenant(r.Context(), auth.FromHeaders(r.Header), t)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) resolveTenant(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	subdomain := r.URL.Query().Get("subdomain")
	if domain == "" && subdomain == "" {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "?domain= or ?subdomain= is required"))
		return
	}
	t, err := h.svc.ResolveTenant(r.Context(), domain, subdomain)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_code": t.Code,
		"name":        t.Name,
		"status":      t.Status,
	})
}

func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTenant(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) updateTenant(w http.ResponseWriter, r *http.Request) {
	var upd domain.TenantUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "invalid request body"))
		return
	}
	t, err := h.svc.UpdateTenant(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"), upd)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) deleteTenant(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteTenant(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) branding(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Branding(r.Context(), r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) features(w http.ResponseWriter, r *http.Request) {
	code := tenantCode(r)
	fs, err := h.svc.Features(r.Context(), auth.FromHeaders(r.Header), code)
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
	f, err := h.svc.SetFeature(r.Context(), auth.FromHeaders(r.Header), code, r.PathValue("key"), body.Enabled)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

type overrideFeatureBody struct {
	Tenant  string `json:"tenant_code"`
	Enabled bool   `json:"is_enabled"`
	Reason  string `json:"reason"`
}

func (h *Handler) overrideFeature(w http.ResponseWriter, r *http.Request) {
	var body overrideFeatureBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "invalid request body"))
		return
	}
	if body.Tenant == "" {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "tenant_code is required"))
		return
	}
	f, err := h.svc.OverrideFeature(r.Context(), auth.FromHeaders(r.Header), body.Tenant, r.PathValue("key"), body.Enabled, body.Reason)
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
