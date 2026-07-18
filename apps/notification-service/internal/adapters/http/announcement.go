package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
)

// registerAnnouncements mounts the announcement routes.
func (h *Handler) registerAnnouncements(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/announcements", h.listAnnouncements)
	mux.HandleFunc("POST /api/v1/announcements", h.createAnnouncement)
	mux.HandleFunc("GET /api/v1/announcements/{announcement_id}", h.getAnnouncement)
	mux.HandleFunc("DELETE /api/v1/announcements/{announcement_id}", h.deleteAnnouncement)
}

// --- Announcement handlers ---

func (h *Handler) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	filter := ports.AnnouncementFilter{
		Limit:    parseLimit(r.URL.Query().Get("limit")),
		Cursor:   r.URL.Query().Get("cursor"),
		Audience: r.URL.Query().Get("audience"),
	}
	records, nextCursor, err := h.svc.ListAnnouncements(ctx, actor, filter)
	if err != nil {
		h.writeAnnouncementErr(w, r, err)
		return
	}
	httpRespondList(w, r, records, nextCursor)
}

type createAnnouncementBody struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Audience string `json:"audience,omitempty"`
}

func (h *Handler) createAnnouncement(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createAnnouncementBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateAnnouncement(ctx, actor, application.CreateAnnouncementRequest{
		Title:    body.Title,
		Body:     body.Body,
		Audience: body.Audience,
	})
	if err != nil {
		h.writeAnnouncementErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getAnnouncement(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetAnnouncement(ctx, actor, r.PathValue("announcement_id"))
	if err != nil {
		h.writeAnnouncementErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAnnouncement(ctx, actor, r.PathValue("announcement_id")); err != nil {
		h.writeAnnouncementErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeAnnouncementErr mirrors Handler.writeErr but reports the announcements
// feature key on feature-disabled errors.
func (h *Handler) writeAnnouncementErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "resource")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureAnnouncements)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}
