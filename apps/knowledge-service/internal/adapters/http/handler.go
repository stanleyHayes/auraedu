// Package http exposes the knowledge service HTTP API.
package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/knowledge-service/internal/application"
	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/knowledge/sources", h.create)
	mux.HandleFunc("GET /api/v1/knowledge/sources", h.list)
	mux.HandleFunc("GET /api/v1/knowledge/sources/{source_id}", h.get)
	mux.HandleFunc("POST /api/v1/knowledge/sources/{source_id}/approve", h.approve)
	mux.HandleFunc("POST /api/v1/knowledge/sources/{source_id}/retire", h.retire)
}

func (h *Handler) RegisterInternal(mux *http.ServeMux, token string) {
	mux.HandleFunc("POST /internal/v1/knowledge/search", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid service credentials are required")
			return
		}
		var body struct {
			Query  string    `json:"query"`
			Locale string    `json:"locale"`
			Limit  int       `json:"limit"`
			AsOf   time.Time `json:"as_of"`
		}
		if !decode(w, r, &body) {
			return
		}
		results, err := h.svc.SearchApproved(r.Context(), tenantCode(r), body.Query, body.Locale, body.Limit, body.AsOf)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
	})
}

type createBody struct {
	SourceType      string     `json:"source_type"`
	Title           string     `json:"title"`
	Owner           string     `json:"owner"`
	Content         string     `json:"content"`
	EffectiveAt     time.Time  `json:"effective_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Programme       *string    `json:"programme"`
	Campus          *string    `json:"campus"`
	Intake          *string    `json:"intake"`
	Confidentiality string     `json:"confidentiality"`
	Locale          string     `json:"locale"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body createBody
	if !decode(w, r, &body) {
		return
	}
	source, err := h.svc.Create(r.Context(), requestActor(r), application.CreateInput{SourceType: body.SourceType,
		Title: body.Title, Owner: body.Owner, Content: body.Content, EffectiveAt: body.EffectiveAt,
		ExpiresAt: body.ExpiresAt, Programme: body.Programme, Campus: body.Campus, Intake: body.Intake,
		Confidentiality: body.Confidentiality, Locale: body.Locale})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, source)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeDomainError(w, domain.ErrValidation)
			return
		}
		limit = parsed
	}
	items, err := h.svc.List(r.Context(), requestActor(r), domain.Status(r.URL.Query().Get("status")), limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	source, err := h.svc.Get(r.Context(), requestActor(r), r.PathValue("source_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ReviewNote string `json:"review_note"`
	}
	if !decode(w, r, &body) {
		return
	}
	source, err := h.svc.Approve(r.Context(), requestActor(r), r.PathValue("source_id"), body.ReviewNote)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (h *Handler) retire(w http.ResponseWriter, r *http.Request) {
	source, err := h.svc.Retire(r.Context(), requestActor(r), r.PathValue("source_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func requestActor(r *http.Request) auth.Actor {
	actor := auth.FromHeaders(r.Header)
	if actor.TenantID == "" {
		actor.TenantID = tenantCode(r)
	}
	return actor
}

func tenantCode(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Tenant-Code")); value != "" {
		return value
	}
	return strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
}

func authorized(r *http.Request, token string) bool {
	expected, provided := []byte("Bearer "+token), []byte(r.Header.Get("Authorization"))
	return token != "" && len(expected) == len(provided) && subtle.ConstantTimeCompare(expected, provided) == 1
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "request failed validation")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "request body must contain one JSON object")
		return false
	}
	return true
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "request could not be completed")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
