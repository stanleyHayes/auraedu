// Package http adapts HTTP to the Tenant Service use cases (no business logic here).
// Routes align with contracts/openapi/tenant.v1.yaml. Until the gateway injects tenant
// context (EP-03), /features resolves the tenant from the X-Tenant-Code header or ?tenant=.
package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/tenant-service/internal/application"
	"github.com/auraedu/tenant-service/internal/domain"
)

type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/public/onboarding-requests", h.submitOnboarding)
	mux.HandleFunc("GET /api/v1/super-admin/onboarding-requests", h.listOnboarding)
	mux.HandleFunc("POST /api/v1/super-admin/onboarding-requests/{request_id}/approve", h.approveOnboarding)
	mux.HandleFunc("POST /api/v1/super-admin/onboarding-requests/{request_id}/reject", h.rejectOnboarding)
	mux.HandleFunc("GET /api/v1/tenants", h.listTenants)
	mux.HandleFunc("POST /api/v1/tenants", h.createTenant)
	mux.HandleFunc("GET /api/v1/tenants/resolve", h.resolveTenant)
	mux.HandleFunc("GET /api/v1/tenants/{code}", h.getTenant)
	mux.HandleFunc("PATCH /api/v1/tenants/{code}", h.updateTenant)
	mux.HandleFunc("DELETE /api/v1/tenants/{code}", h.deleteTenant)
	mux.HandleFunc("GET /api/v1/tenants/{code}/branding", h.branding)
	mux.HandleFunc("GET /api/v1/tenants/{code}/settings", h.settings)
	mux.HandleFunc("PATCH /api/v1/tenants/{code}/settings", h.updateSettings)
	mux.HandleFunc("POST /api/v1/tenants/{code}/custom-domain", h.requestCustomDomain)
	mux.HandleFunc("GET /api/v1/tenants/{code}/custom-domain", h.getCustomDomain)
	mux.HandleFunc("POST /api/v1/tenants/{code}/custom-domain/verify", h.verifyCustomDomain)
	mux.HandleFunc("POST /api/v1/super-admin/tenants/{code}/custom-domain/activate", h.activateCustomDomain)
	mux.HandleFunc("POST /api/v1/super-admin/tenants/{code}/custom-domain/deactivate", h.deactivateCustomDomain)
	mux.HandleFunc("GET /api/v1/features", h.features)
	mux.HandleFunc("PUT /api/v1/features/{key}", h.setFeature)
	mux.HandleFunc("POST /api/v1/super-admin/features/{key}/override", h.overrideFeature)
}

func (h *Handler) RegisterInternal(mux *http.ServeMux, token string) {
	mux.HandleFunc("GET /internal/v1/onboarding-requests/{request_id}/administrator", func(w http.ResponseWriter, r *http.Request) {
		if !internalAuthorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, errEnv("unauthorized", "valid service credentials are required"))
			return
		}
		request, err := h.svc.ResolveOnboardingAdministrator(r.Context(), r.PathValue("request_id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"tenant_code": *request.TenantCode, "administrator_name": request.AdministratorName, "email": request.Email,
		})
	})
	mux.HandleFunc("POST /internal/v1/tenants/{code}/activate", func(w http.ResponseWriter, r *http.Request) {
		if !internalAuthorized(r, token) {
			writeJSON(w, http.StatusUnauthorized, errEnv("unauthorized", "valid service credentials are required"))
			return
		}
		if err := h.svc.ActivateOnboardingTenant(r.Context(), r.PathValue("code")); err != nil {
			writeErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func internalAuthorized(r *http.Request, token string) bool {
	expected := []byte("Bearer " + token)
	provided := []byte(r.Header.Get("Authorization"))
	return token != "" && len(expected) == len(provided) && subtle.ConstantTimeCompare(expected, provided) == 1
}

type onboardingRequestBody struct {
	SchoolName           string  `json:"school_name"`
	AdministratorName    string  `json:"administrator_name"`
	Email                string  `json:"email"`
	Phone                *string `json:"phone"`
	CountryCode          string  `json:"country_code"`
	Plan                 string  `json:"plan"`
	Priorities           *string `json:"priorities"`
	PrivacyNoticeVersion string  `json:"privacy_notice_version"`
	AcceptedTerms        bool    `json:"accepted_terms"`
	Website              string  `json:"website"`
}

func (h *Handler) submitOnboarding(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body onboardingRequestBody
	if err := decoder.Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
		return
	}
	request, _, err := h.svc.SubmitOnboarding(r.Context(), r.Header.Get("Idempotency-Key"), application.SubmitOnboardingInput{
		SchoolName: body.SchoolName, AdministratorName: body.AdministratorName,
		Email: body.Email, Phone: body.Phone, CountryCode: body.CountryCode,
		Plan: body.Plan, Priorities: body.Priorities,
		PrivacyNoticeVersion: body.PrivacyNoticeVersion,
		AcceptedTerms:        body.AcceptedTerms, Website: body.Website,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"request_id": request.ID, "status": request.Status, "submitted_at": request.SubmittedAt,
	})
}

func (h *Handler) listOnboarding(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "limit is invalid"))
			return
		}
		limit = parsed
	}
	requests, next, err := h.svc.ListOnboarding(r.Context(), auth.FromHeaders(r.Header), limit, r.URL.Query().Get("cursor"), r.URL.Query().Get("status"))
	if err != nil {
		writeErr(w, err)
		return
	}
	var nextCursor any
	if next != "" {
		nextCursor = next
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": requests, "next_cursor": nextCursor})
}

func (h *Handler) approveOnboarding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantCode string `json:"tenant_code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
		return
	}
	request, err := h.svc.ApproveOnboarding(r.Context(), auth.FromHeaders(r.Header), r.PathValue("request_id"), body.TenantCode)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (h *Handler) rejectOnboarding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
		return
	}
	request, err := h.svc.RejectOnboarding(r.Context(), auth.FromHeaders(r.Header), r.PathValue("request_id"), body.Reason)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
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
	writeJSON(w, http.StatusOK, t)
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

func (h *Handler) requestCustomDomain(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Hostname string `json:"hostname"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
		return
	}
	registration, err := h.svc.RequestCustomDomain(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"), body.Hostname)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, registration)
}

func (h *Handler) getCustomDomain(w http.ResponseWriter, r *http.Request) {
	registration, err := h.svc.GetCustomDomain(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, registration)
}

func (h *Handler) verifyCustomDomain(w http.ResponseWriter, r *http.Request) {
	registration, err := h.svc.VerifyCustomDomain(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, registration)
}

func (h *Handler) activateCustomDomain(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		ProviderReference string `json:"provider_reference"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
		return
	}
	registration, err := h.svc.ActivateCustomDomain(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"), body.ProviderReference)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, registration)
}

func (h *Handler) deactivateCustomDomain(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		ProviderReference string `json:"provider_reference"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
		return
	}
	registration, err := h.svc.DeactivateCustomDomain(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"), body.ProviderReference)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, registration)
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

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Settings(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var s domain.Settings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeJSON(w, http.StatusBadRequest, errEnv("validation_error", "invalid request body"))
		return
	}
	updated, err := h.svc.UpdateSettings(r.Context(), auth.FromHeaders(r.Header), r.PathValue("code"), s)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
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
	case errors.Is(err, flags.ErrFeatureDisabled):
		writeJSON(w, http.StatusForbidden, errEnv("feature_disabled", "custom domains are disabled for this tenant"))
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errEnv("not_found", "tenant not found"))
	case errors.Is(err, domain.ErrNoTenant):
		writeJSON(w, http.StatusBadRequest, errEnv("tenant_mismatch", "tenant context required (X-Tenant-Code header or ?tenant=)"))
	case errors.Is(err, domain.ErrValidation):
		writeJSON(w, http.StatusUnprocessableEntity, errEnv("validation_error", "request failed validation"))
	case errors.Is(err, domain.ErrConflict):
		writeJSON(w, http.StatusConflict, errEnv("conflict", "request conflicts with existing state"))
	case errors.Is(err, domain.ErrUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, errEnv("dependency_unavailable", "domain verification is temporarily unavailable"))
	default:
		writeJSON(w, http.StatusInternalServerError, errEnv("internal", "unexpected error"))
	}
}
