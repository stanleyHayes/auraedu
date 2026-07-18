// Package http exposes the report service REST API.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/report-service/internal/application"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

// Handler adapts HTTP to the report use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/report-templates", h.listReportTemplates)
	mux.HandleFunc("POST /api/v1/report-templates", h.createReportTemplate)
	mux.HandleFunc("GET /api/v1/report-templates/{template_id}", h.getReportTemplate)
	mux.HandleFunc("PATCH /api/v1/report-templates/{template_id}", h.updateReportTemplate)
	mux.HandleFunc("DELETE /api/v1/report-templates/{template_id}", h.deleteReportTemplate)

	mux.HandleFunc("GET /api/v1/report-cards", h.listReportCards)
	mux.HandleFunc("POST /api/v1/report-cards", h.createReportCard)
	mux.HandleFunc("GET /api/v1/report-cards/{report_card_id}", h.getReportCard)
	mux.HandleFunc("PATCH /api/v1/report-cards/{report_card_id}", h.updateReportCard)
	mux.HandleFunc("DELETE /api/v1/report-cards/{report_card_id}", h.deleteReportCard)
	mux.HandleFunc("POST /api/v1/report-cards/{report_card_id}/generate", h.generateReportCard)
	mux.HandleFunc("GET /api/v1/report-cards/{report_card_id}/download", h.downloadReportCard)
}

// --- Report templates. ---

func (h *Handler) listReportTemplates(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	filter := ports.ReportTemplateListFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListReportTemplates(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createReportTemplateBody struct {
	Name           string `json:"name"`
	AcademicYearID string `json:"academic_year_id"`
	BodyTemplate   string `json:"body_template"`
}

func (h *Handler) createReportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createReportTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	t, err := h.svc.CreateReportTemplate(ctx, actor, application.CreateReportTemplateRequest{
		Name:           body.Name,
		AcademicYearID: body.AcademicYearID,
		BodyTemplate:   body.BodyTemplate,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, t)
}

func (h *Handler) getReportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	t, err := h.svc.GetReportTemplate(ctx, actor, r.PathValue("template_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, t)
}

type updateReportTemplateBody struct {
	Name           *string `json:"name,omitempty"`
	AcademicYearID *string `json:"academic_year_id,omitempty"`
	BodyTemplate   *string `json:"body_template,omitempty"`
	Status         *string `json:"status,omitempty"`
}

func (h *Handler) updateReportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateReportTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	t, err := h.svc.UpdateReportTemplate(ctx, actor, r.PathValue("template_id"), application.UpdateReportTemplateRequest{
		Name:           body.Name,
		AcademicYearID: body.AcademicYearID,
		BodyTemplate:   body.BodyTemplate,
		Status:         body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, t)
}

func (h *Handler) deleteReportTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteReportTemplate(ctx, actor, r.PathValue("template_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Report cards. ---

func (h *Handler) listReportCards(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	filter := ports.ReportCardListFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		Status:         r.URL.Query().Get("status"),
		StudentID:      r.URL.Query().Get("student_id"),
		TemplateID:     r.URL.Query().Get("template_id"),
	}
	records, nextCursor, err := h.svc.ListReportCards(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createReportCardBody struct {
	StudentID      string `json:"student_id"`
	AcademicYearID string `json:"academic_year_id"`
	TermID         string `json:"term_id"`
	TemplateID     string `json:"template_id"`
}

func (h *Handler) createReportCard(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createReportCardBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	c, err := h.svc.CreateReportCard(ctx, actor, application.CreateReportCardRequest{
		StudentID:      body.StudentID,
		AcademicYearID: body.AcademicYearID,
		TermID:         body.TermID,
		TemplateID:     body.TemplateID,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, c)
}

func (h *Handler) getReportCard(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	c, err := h.svc.GetReportCard(ctx, actor, r.PathValue("report_card_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, c)
}

type updateReportCardBody struct {
	StudentID      *string `json:"student_id,omitempty"`
	AcademicYearID *string `json:"academic_year_id,omitempty"`
	TemplateID     *string `json:"template_id,omitempty"`
	Status         *string `json:"status,omitempty"`
}

func (h *Handler) updateReportCard(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateReportCardBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	c, err := h.svc.UpdateReportCard(ctx, actor, r.PathValue("report_card_id"), application.UpdateReportCardRequest{
		StudentID:      body.StudentID,
		AcademicYearID: body.AcademicYearID,
		TemplateID:     body.TemplateID,
		Status:         body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, c)
}

func (h *Handler) deleteReportCard(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteReportCard(ctx, actor, r.PathValue("report_card_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) generateReportCard(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	c, err := h.svc.GenerateReportCard(ctx, actor, r.PathValue("report_card_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, c)
}

func (h *Handler) downloadReportCard(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	path, _, err := h.svc.DownloadReportCardPath(ctx, actor, r.PathValue("report_card_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	//nolint:gosec // path is generated and confined to the report output directory by the service.
	http.ServeFile(w, r, path)
}

// --- Helpers. ---

func (h *Handler) context(r *http.Request) (context.Context, auth.Actor, bool) {
	actor := auth.FromHeaders(r.Header)
	tenantID := r.Header.Get(tenancy.HeaderTenantID)
	if tenantID == "" {
		tenantID = r.Header.Get("X-Tenant-Code")
	}
	ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{
		TenantID:  tenantID,
		RequestID: r.Header.Get(tenancy.HeaderRequestID),
		ActorID:   actor.UserID,
		ActorRole: actor.Role,
	})
	ctx = auth.WithActor(ctx, actor)
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "report template or report card")
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondError(w, r, httpx.Error{Code: httpx.ErrInternal, Message: err.Error()})
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureReportCards)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
