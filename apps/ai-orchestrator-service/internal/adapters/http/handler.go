// Package http exposes the AI orchestrator HTTP transport.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
)

type Handler struct{ svc *application.Service }

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/public/assistant/messages", h.ask)
}

func (h *Handler) ask(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		SessionID *string `json:"session_id"`
		Question  string  `json:"question"`
		Locale    string  `json:"locale"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "request failed validation")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "request body must contain one JSON object")
		return
	}
	sessionID := ""
	if body.SessionID != nil {
		sessionID = *body.SessionID
	}
	response, err := h.svc.Ask(r.Context(), application.AskInput{TenantID: tenantCode(r),
		IdempotencyKey: r.Header.Get("Idempotency-Key"), SessionID: sessionID, Question: body.Question, Locale: body.Locale})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func tenantCode(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Tenant-Code")); value != "" {
		return value
	}
	return strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusNotFound, "not_found", "assistant is not available")
	case errors.Is(err, domain.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", err.Error())
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
