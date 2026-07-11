// Package http is the website-service HTTP adapter.
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
	"github.com/auraedu/website-service/internal/application"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
)

// Handler adapts HTTP to the website use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/website/pages", h.listPages)
	mux.HandleFunc("POST /api/v1/website/pages", h.createPage)
	mux.HandleFunc("GET /api/v1/website/pages/{page_id}", h.getPage)
	mux.HandleFunc("PATCH /api/v1/website/pages/{page_id}", h.updatePage)
	mux.HandleFunc("DELETE /api/v1/website/pages/{page_id}", h.deletePage)
	mux.HandleFunc("GET /api/v1/website/pages/by-slug/{slug}", h.getPageBySlug)

	mux.HandleFunc("GET /api/v1/website/pages/{page_id}/sections", h.listSections)
	mux.HandleFunc("POST /api/v1/website/pages/{page_id}/sections", h.createSection)
	mux.HandleFunc("GET /api/v1/website/sections/{section_id}", h.getSection)
	mux.HandleFunc("PATCH /api/v1/website/sections/{section_id}", h.updateSection)
	mux.HandleFunc("DELETE /api/v1/website/sections/{section_id}", h.deleteSection)
}

// Pages.

func (h *Handler) listPages(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := parseLimit(r)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"limit": "invalid integer"})
		return
	}
	cursor := r.URL.Query().Get("cursor")
	filter := ports.PageFilter{}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = &v
	}
	if v := r.URL.Query().Get("layout"); v != "" {
		filter.Layout = &v
	}
	pages, nextCursor, err := h.svc.ListPages(ctx, actor, limit, cursor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": pages, "next_cursor": nullIfEmpty(nextCursor)})
}

type createPageBody struct {
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	MetaDescription *string `json:"meta_description"`
	Layout          *string `json:"layout"`
	Status          *string `json:"status"`
}

func (h *Handler) createPage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createPageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	page, err := h.svc.CreatePage(ctx, actor, application.CreatePageRequest{
		Slug:            body.Slug,
		Title:           body.Title,
		MetaDescription: body.MetaDescription,
		Layout:          body.Layout,
		Status:          body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, page)
}

func (h *Handler) getPage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	page, err := h.svc.GetPage(ctx, actor, r.PathValue("page_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, page)
}

func (h *Handler) getPageBySlug(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	page, err := h.svc.GetPageBySlug(ctx, actor, r.PathValue("slug"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, page)
}

type updatePageBody struct {
	Slug            *string `json:"slug"`
	Title           *string `json:"title"`
	MetaDescription *string `json:"meta_description"`
	Layout          *string `json:"layout"`
	Status          *string `json:"status"`
}

func (h *Handler) updatePage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updatePageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	page, err := h.svc.UpdatePage(ctx, actor, r.PathValue("page_id"), application.UpdatePageRequest{
		Slug:            body.Slug,
		Title:           body.Title,
		MetaDescription: body.MetaDescription,
		Layout:          body.Layout,
		Status:          body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, page)
}

func (h *Handler) deletePage(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeletePage(ctx, actor, r.PathValue("page_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Sections.

func (h *Handler) listSections(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := parseLimit(r)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"limit": "invalid integer"})
		return
	}
	cursor := r.URL.Query().Get("cursor")
	filter := ports.SectionFilter{}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = &v
	}
	if v := r.URL.Query().Get("type"); v != "" {
		filter.Type = &v
	}
	sections, nextCursor, err := h.svc.ListSections(ctx, actor, r.PathValue("page_id"), limit, cursor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": sections, "next_cursor": nullIfEmpty(nextCursor)})
}

type createSectionBody struct {
	Type      string         `json:"type"`
	Content   domain.Content `json:"content"`
	SortOrder int            `json:"sort_order"`
	Status    *string        `json:"status"`
}

func (h *Handler) createSection(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createSectionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	section, err := h.svc.CreateSection(ctx, actor, application.CreateSectionRequest{
		PageID:    r.PathValue("page_id"),
		Type:      domain.SectionType(body.Type),
		Content:   body.Content,
		SortOrder: body.SortOrder,
		Status:    body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, section)
}

func (h *Handler) getSection(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	section, err := h.svc.GetSection(ctx, actor, r.PathValue("section_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, section)
}

type updateSectionBody struct {
	Type      *domain.SectionType `json:"type"`
	Content   *domain.Content     `json:"content"`
	SortOrder *int                `json:"sort_order"`
	Status    *string             `json:"status"`
}

func (h *Handler) updateSection(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateSectionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	section, err := h.svc.UpdateSection(ctx, actor, r.PathValue("section_id"), application.UpdateSectionRequest{
		Type:      body.Type,
		Content:   body.Content,
		SortOrder: body.SortOrder,
		Status:    body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, section)
}

func (h *Handler) deleteSection(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteSection(ctx, actor, r.PathValue("section_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Context and errors.

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
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "resource")
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondError(w, r, httpx.Error{Code: httpx.ErrValidation, Message: "resource conflict"})
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeaturePublicWebsite)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func parseLimit(r *http.Request) (int, error) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
