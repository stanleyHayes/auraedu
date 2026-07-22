// Package http exposes the content service HTTP API.
//
//nolint:lll // Explicit request-to-command mappings are kept adjacent for auditability.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/content-service/internal/application"
	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

type Handler struct{ service *application.Service }

func NewHandler(service *application.Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/content/brand-profile", h.getBrandProfile)
	mux.HandleFunc("PUT /api/v1/content/brand-profile", h.upsertBrandProfile)
	mux.HandleFunc("POST /api/v1/content/generate", h.generate)
	mux.HandleFunc("GET /api/v1/content", h.list)
	mux.HandleFunc("GET /api/v1/content/{id}", h.get)
	mux.HandleFunc("PATCH /api/v1/content/{id}", h.revise)
	mux.HandleFunc("POST /api/v1/content/{id}/submit-for-review", h.submit)
	mux.HandleFunc("POST /api/v1/content/{id}/approve", h.approve)
	mux.HandleFunc("POST /api/v1/content/{id}/reject", h.reject)
}

type brandProfileBody struct {
	ToneOfVoice         string   `json:"tone_of_voice"`
	ApprovedTerms       []string `json:"approved_terms"`
	ProhibitedClaims    []string `json:"prohibited_claims"`
	RequiredDisclaimers []string `json:"required_disclaimers"`
	Locale              string   `json:"locale"`
	ExpectedVersion     int      `json:"expected_version"`
}

func (h *Handler) getBrandProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.service.GetBrandProfile(r.Context(), auth.FromHeaders(r.Header))
	respond(w, http.StatusOK, profile, err)
}

func (h *Handler) upsertBrandProfile(w http.ResponseWriter, r *http.Request) {
	var body brandProfileBody
	if !decode(w, r, &body) {
		return
	}
	profile, err := h.service.UpsertBrandProfile(r.Context(), auth.FromHeaders(r.Header), application.BrandProfileInput{ToneOfVoice: body.ToneOfVoice, ApprovedTerms: body.ApprovedTerms, ProhibitedClaims: body.ProhibitedClaims, RequiredDisclaimers: body.RequiredDisclaimers, Locale: body.Locale, ExpectedVersion: body.ExpectedVersion})
	respond(w, http.StatusOK, profile, err)
}

type generateBody struct {
	ContentType string        `json:"content_type"`
	Title       string        `json:"title"`
	Brief       string        `json:"brief"`
	Audience    string        `json:"audience"`
	Locale      string        `json:"locale"`
	CampaignID  *string       `json:"campaign_id"`
	KeyMessages []string      `json:"key_messages"`
	Facts       []domain.Fact `json:"facts"`
	ExpiresAt   *time.Time    `json:"expires_at"`
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	var body generateBody
	if !decode(w, r, &body) {
		return
	}
	draft, err := h.service.Generate(r.Context(), auth.FromHeaders(r.Header), application.GenerateInput{IdempotencyKey: r.Header.Get("Idempotency-Key"), ContentType: body.ContentType, Title: body.Title, Brief: body.Brief, Audience: body.Audience, Locale: body.Locale, CampaignID: body.CampaignID, KeyMessages: body.KeyMessages, Facts: body.Facts, ExpiresAt: body.ExpiresAt})
	respond(w, http.StatusCreated, draft, err)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, domain.ErrValidation)
			return
		}
		limit = value
	}
	items, err := h.service.List(r.Context(), auth.FromHeaders(r.Header), ports.ListFilter{Status: r.URL.Query().Get("status"), ContentType: r.URL.Query().Get("content_type"), CampaignID: r.URL.Query().Get("campaign_id"), Limit: limit})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	draft, versions, err := h.service.Get(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": draft, "versions": versions})
}

func (h *Handler) revise(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content         string     `json:"content"`
		ChangeNote      string     `json:"change_note"`
		ExpectedVersion int        `json:"expected_version"`
		ExpiresAt       *time.Time `json:"expires_at"`
	}
	if !decode(w, r, &body) {
		return
	}
	draft, err := h.service.Revise(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), application.ReviseInput{Content: body.Content, ChangeNote: body.ChangeNote, ExpectedVersion: body.ExpectedVersion, ExpiresAt: body.ExpiresAt})
	respond(w, http.StatusOK, draft, err)
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExpectedVersion int `json:"expected_version"`
	}
	if !decode(w, r, &body) {
		return
	}
	draft, err := h.service.Submit(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), body.ExpectedVersion)
	respond(w, http.StatusOK, draft, err)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) { h.review(w, r, true) }
func (h *Handler) reject(w http.ResponseWriter, r *http.Request)  { h.review(w, r, false) }

func (h *Handler) review(w http.ResponseWriter, r *http.Request, approve bool) {
	var body struct {
		ReviewNote      string `json:"review_note"`
		ExpectedVersion int    `json:"expected_version"`
	}
	if !decode(w, r, &body) {
		return
	}
	var draft domain.Draft
	var err error
	if approve {
		draft, err = h.service.Approve(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), body.ReviewNote, body.ExpectedVersion)
	} else {
		draft, err = h.service.Reject(r.Context(), auth.FromHeaders(r.Header), r.PathValue("id"), body.ReviewNote, body.ExpectedVersion)
	}
	respond(w, http.StatusOK, draft, err)
}

func decode(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, domain.ErrValidation)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, domain.ErrValidation)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, body any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, body)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal"
	switch {
	case errors.Is(err, domain.ErrValidation):
		status, code = http.StatusUnprocessableEntity, "validation_error"
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, domain.ErrUnavailable):
		status, code = http.StatusServiceUnavailable, "generator_unavailable"
	}
	writeJSON(w, status, map[string]string{"code": code, "message": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
