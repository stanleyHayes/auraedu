// Package http exposes the Campaign Service HTTP transport.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/campaign-service/internal/application"
	"github.com/auraedu/campaign-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type Handler struct{ svc *application.Service }

func NewHandler(s *application.Service) *Handler { return &Handler{svc: s} }
func (h *Handler) Register(m *http.ServeMux) {
	m.HandleFunc("POST /api/v1/campaigns", h.create)
	m.HandleFunc("GET /api/v1/campaigns", h.list)
	m.HandleFunc("GET /api/v1/campaigns/{id}", h.get)
	m.HandleFunc("PATCH /api/v1/campaigns/{id}", h.update)
	m.HandleFunc("POST /api/v1/campaigns/{id}/submit-for-approval", h.submit)
	m.HandleFunc("POST /api/v1/campaigns/{id}/approve", h.approve)
	m.HandleFunc("POST /api/v1/campaigns/{id}/publish", h.publish)
	m.HandleFunc("POST /api/v1/campaigns/{id}/pause", h.pause)
}

type createBody struct {
	Name               string    `json:"name"`
	Objective          string    `json:"objective"`
	Channel            string    `json:"channel"`
	AudienceDefinition string    `json:"audience_definition"`
	ProgrammeIDs       []string  `json:"programme_ids"`
	Budget             float64   `json:"budget"`
	Currency           string    `json:"currency"`
	StartAt            time.Time `json:"start_at"`
	EndAt              time.Time `json:"end_at"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var b createBody
	if !decode(w, r, &b) {
		return
	}
	c, e := h.svc.Create(r.Context(), auth.FromHeaders(r.Header), application.CreateInput{
		Name: b.Name, Objective: b.Objective, Channel: b.Channel,
		AudienceDefinition: b.AudienceDefinition, ProgrammeIDs: b.ProgrammeIDs,
		Budget: b.Budget, Currency: b.Currency, StartAt: b.StartAt, EndAt: b.EndAt,
	})
	respond(w, http.StatusCreated, c, e)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil {
			writeErr(w, domain.ErrValidation)
			return
		}
		limit = v
	}
	items, e := h.svc.List(r.Context(), auth.FromHeaders(r.Header), domain.Status(r.URL.Query().Get("status")), limit)
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": items})
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	c, e := h.svc.Get(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, c, e)
}

type updateBody struct {
	Name               *string    `json:"name"`
	Objective          *string    `json:"objective"`
	AudienceDefinition *string    `json:"audience_definition"`
	ProgrammeIDs       *[]string  `json:"programme_ids"`
	Budget             *float64   `json:"budget"`
	Currency           *string    `json:"currency"`
	StartAt            *time.Time `json:"start_at"`
	EndAt              *time.Time `json:"end_at"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var b updateBody
	if !decode(w, r, &b) {
		return
	}
	c, e := h.svc.Update(
		r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"),
		application.UpdateInput{
			Name: b.Name, Objective: b.Objective, AudienceDefinition: b.AudienceDefinition,
			ProgrammeIDs: b.ProgrammeIDs, Budget: b.Budget, Currency: b.Currency,
			StartAt: b.StartAt, EndAt: b.EndAt,
		},
	)
	respond(w, http.StatusOK, c, e)
}
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	c, e := h.svc.Submit(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, c, e)
}
func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ReviewNote string `json:"review_note"`
	}
	if !decode(w, r, &b) {
		return
	}
	c, e := h.svc.Approve(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.ReviewNote)
	respond(w, http.StatusOK, c, e)
}
func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	c, e := h.svc.Publish(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, c, e)
}
func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	c, e := h.svc.Pause(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	respond(w, http.StatusOK, c, e)
}
func decode(w http.ResponseWriter, r *http.Request, d any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	x := json.NewDecoder(r.Body)
	x.DisallowUnknownFields()
	if e := x.Decode(d); e != nil {
		writeErr(w, domain.ErrValidation)
		return false
	}
	if e := x.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		writeErr(w, domain.ErrValidation)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, c domain.Campaign, e error) {
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, status, c)
}
func writeErr(w http.ResponseWriter, e error) {
	status, code := http.StatusInternalServerError, "internal"
	switch {
	case errors.Is(e, domain.ErrValidation):
		status, code = http.StatusUnprocessableEntity, "validation_error"
	case errors.Is(e, domain.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(e, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(e, domain.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	}
	jsonOut(w, status, map[string]string{"code": code, "message": e.Error()})
}
func jsonOut(w http.ResponseWriter, status int, b any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(b)
}
