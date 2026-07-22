// Package http exposes the market-intelligence HTTP API.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/application"
	"github.com/auraedu/market-intelligence-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type Handler struct{ svc *application.Service }

func NewHandler(s *application.Service) *Handler { return &Handler{svc: s} }
func (h *Handler) Register(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/intelligence/sources", h.listSources)
	m.HandleFunc("POST /api/v1/intelligence/sources", h.createSource)
	m.HandleFunc("POST /api/v1/intelligence/sources/{id}/review", h.reviewSource)
	m.HandleFunc("GET /api/v1/intelligence/observations", h.listObservations)
	m.HandleFunc("POST /api/v1/intelligence/observations", h.createObservation)
	m.HandleFunc("POST /api/v1/intelligence/observations/{id}/review", h.reviewObservation)
	m.HandleFunc("POST /api/v1/intelligence/observations/{id}/resolve", h.resolveObservation)
	m.HandleFunc("GET /api/v1/intelligence/alert-rule", h.getAlertRule)
	m.HandleFunc("PUT /api/v1/intelligence/alert-rule", h.updateAlertRule)
	m.HandleFunc("GET /api/v1/intelligence/alerts", h.listAlerts)
	m.HandleFunc("POST /api/v1/intelligence/alerts/{id}/acknowledge", h.acknowledgeAlert)
	m.HandleFunc("GET /api/v1/intelligence/competitor-summaries", h.listSummaries)
	m.HandleFunc("POST /api/v1/intelligence/competitor-summaries", h.generateSummary)
	m.HandleFunc("POST /api/v1/intelligence/competitor-summaries/{id}/review", h.reviewSummary)
}

func (h *Handler) generateSummary(w http.ResponseWriter, r *http.Request) {
	var b struct {
		PeriodFrom time.Time `json:"period_from"`
		PeriodTo   time.Time `json:"period_to"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.GenerateSummary(r.Context(), auth.FromHeaders(r.Header), b.PeriodFrom, b.PeriodTo)
	respond(w, http.StatusCreated, v, e)
}
func (h *Handler) listSummaries(w http.ResponseWriter, r *http.Request) {
	n, e := queryLimit(r)
	if e != nil {
		writeErr(w, e)
		return
	}
	v, e := h.svc.ListSummaries(r.Context(), auth.FromHeaders(r.Header), domain.Status(r.URL.Query().Get("status")), n)
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": v})
}
func (h *Handler) reviewSummary(w http.ResponseWriter, r *http.Request) {
	var b reviewBody
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.ReviewSummary(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.Decision, b.ReviewNote)
	respond(w, http.StatusOK, v, e)
}

func (h *Handler) getAlertRule(w http.ResponseWriter, r *http.Request) {
	v, e := h.svc.GetAlertRule(r.Context(), auth.FromHeaders(r.Header))
	respond(w, http.StatusOK, v, e)
}
func (h *Handler) updateAlertRule(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Threshold  int `json:"threshold"`
		WindowDays int `json:"window_days"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.UpdateAlertRule(r.Context(), auth.FromHeaders(r.Header), b.Threshold, b.WindowDays)
	respond(w, http.StatusOK, v, e)
}
func (h *Handler) listAlerts(w http.ResponseWriter, r *http.Request) {
	n, e := queryLimit(r)
	if e != nil {
		writeErr(w, e)
		return
	}
	v, e := h.svc.ListAlerts(r.Context(), auth.FromHeaders(r.Header), r.URL.Query().Get("status"), n)
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": v})
}
func (h *Handler) acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	var b struct {
		AcknowledgementNote string `json:"acknowledgement_note"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.AcknowledgeAlert(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.AcknowledgementNote)
	respond(w, http.StatusOK, v, e)
}
func queryLimit(r *http.Request) (int, error) {
	if v := r.URL.Query().Get("limit"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			return 0, domain.ErrValidation
		}
		return n, nil
	}
	return 50, nil
}
func (h *Handler) createSource(w http.ResponseWriter, r *http.Request) {
	var b application.CreateSourceInput
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.CreateSource(r.Context(), auth.FromHeaders(r.Header), b)
	respond(w, http.StatusCreated, v, e)
}
func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	n, e := queryLimit(r)
	if e != nil {
		writeErr(w, e)
		return
	}
	v, e := h.svc.ListSources(r.Context(), auth.FromHeaders(r.Header), domain.Kind(r.URL.Query().Get("kind")), n)
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": v})
}

type reviewBody struct {
	Decision   string `json:"decision"`
	ReviewNote string `json:"review_note"`
}

func (h *Handler) reviewSource(w http.ResponseWriter, r *http.Request) {
	var b reviewBody
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.ReviewSource(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.Decision, b.ReviewNote)
	respond(w, http.StatusOK, v, e)
}
func (h *Handler) createObservation(w http.ResponseWriter, r *http.Request) {
	var b application.CreateObservationInput
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.CreateObservation(r.Context(), auth.FromHeaders(r.Header), b)
	respond(w, http.StatusCreated, v, e)
}
func (h *Handler) listObservations(w http.ResponseWriter, r *http.Request) {
	n, e := queryLimit(r)
	if e != nil {
		writeErr(w, e)
		return
	}
	v, e := h.svc.ListObservations(r.Context(), auth.FromHeaders(r.Header), domain.Kind(r.URL.Query().Get("kind")), domain.Status(r.URL.Query().Get("status")), n)
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"data": v})
}
func (h *Handler) reviewObservation(w http.ResponseWriter, r *http.Request) {
	var b reviewBody
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.ReviewObservation(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.Decision, b.ReviewNote)
	respond(w, http.StatusOK, v, e)
}
func (h *Handler) resolveObservation(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ResolutionNote string `json:"resolution_note"`
	}
	if !decode(w, r, &b) {
		return
	}
	v, e := h.svc.ResolveObservation(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), b.ResolutionNote)
	respond(w, http.StatusOK, v, e)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		writeErr(w, domain.ErrValidation)
		return false
	}
	if e := d.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		writeErr(w, domain.ErrValidation)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any, e error) {
	if e != nil {
		writeErr(w, e)
		return
	}
	jsonOut(w, status, v)
}
func writeErr(w http.ResponseWriter, e error) {
	status, code := 500, "internal"
	switch {
	case errors.Is(e, domain.ErrValidation):
		status, code = 422, "validation_error"
	case errors.Is(e, domain.ErrForbidden):
		status, code = 403, "forbidden"
	case errors.Is(e, domain.ErrNotFound):
		status, code = 404, "not_found"
	case errors.Is(e, domain.ErrConflict):
		status, code = 409, "conflict"
	}
	jsonOut(w, status, map[string]string{"code": code, "message": e.Error()})
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
