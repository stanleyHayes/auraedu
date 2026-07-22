package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/auraedu/ai-orchestrator-service/internal/application"
	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
)

type ActionHandler struct{ service *application.ActionService }

func NewActionHandler(service *application.ActionService) *ActionHandler {
	return &ActionHandler{service: service}
}

func (h *ActionHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ai/actions", h.propose)
	mux.HandleFunc("GET /api/v1/ai/actions", h.list)
	mux.HandleFunc("GET /api/v1/ai/actions/{action_id}", h.get)
	mux.HandleFunc("POST /api/v1/ai/actions/{action_id}/approve", h.approve)
	mux.HandleFunc("POST /api/v1/ai/actions/{action_id}/reject", h.reject)
	mux.HandleFunc("POST /api/v1/ai/actions/{action_id}/retry", h.retry)
}

func (h *ActionHandler) propose(w http.ResponseWriter, r *http.Request) {
	ctx, actor := actionRequestContext(r)
	var body struct {
		Action   string          `json:"action"`
		TargetID string          `json:"target_id"`
		Payload  json.RawMessage `json:"payload"`
		Reason   string          `json:"reason"`
	}
	if decodeOne(w, r, &body) != nil {
		writeActionError(w, r, domain.ErrValidation)
		return
	}
	action, err := h.service.Propose(ctx, actor, application.ProposeActionInput{
		Action:         body.Action,
		TargetID:       body.TargetID,
		Payload:        body.Payload,
		Reason:         body.Reason,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		writeActionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, action)
}

func (h *ActionHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, actor := actionRequestContext(r)
	limit := 0
	if value := r.URL.Query().Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil {
			writeActionError(w, r, domain.ErrValidation)
			return
		}
	}
	items, err := h.service.List(ctx, actor, r.URL.Query().Get("status"), limit)
	if err != nil {
		writeActionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *ActionHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, actor := actionRequestContext(r)
	action, audit, err := h.service.Get(ctx, actor, r.PathValue("action_id"))
	if err != nil {
		writeActionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "audit": audit})
}

func (h *ActionHandler) review(w http.ResponseWriter, r *http.Request, approve bool) {
	ctx, actor := actionRequestContext(r)
	var body struct {
		Note string `json:"note"`
	}
	if decodeOne(w, r, &body) != nil {
		writeActionError(w, r, domain.ErrValidation)
		return
	}
	action, err := h.service.Review(ctx, actor, r.PathValue("action_id"), body.Note, approve)
	if err != nil {
		writeActionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, action)
}

func (h *ActionHandler) approve(w http.ResponseWriter, r *http.Request) { h.review(w, r, true) }
func (h *ActionHandler) reject(w http.ResponseWriter, r *http.Request)  { h.review(w, r, false) }

func (h *ActionHandler) retry(w http.ResponseWriter, r *http.Request) {
	ctx, actor := actionRequestContext(r)
	action, err := h.service.Retry(ctx, actor, r.PathValue("action_id"))
	if err != nil {
		writeActionError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, action)
}

func actionRequestContext(r *http.Request) (context.Context, auth.Actor) {
	actor := auth.FromHeaders(r.Header)
	tenantID := strings.TrimSpace(r.Header.Get(tenancy.HeaderTenantID))
	if tenantID == "" {
		tenantID = strings.TrimSpace(r.Header.Get("X-Tenant-Code"))
	}
	ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{
		TenantID:  tenantID,
		RequestID: r.Header.Get(tenancy.HeaderRequestID),
		ActorID:   actor.UserID,
		ActorRole: actor.Role,
	})
	return auth.WithActor(ctx, actor), actor
}

func decodeOne(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.ErrValidation
	}
	return nil
}

func writeActionError(w http.ResponseWriter, _ *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "request failed validation")
	case errors.Is(err, domain.ErrProhibited):
		writeError(w, http.StatusForbidden, "action_prohibited", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "action is unavailable or permission is missing")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "action was not found")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", err.Error())
	case errors.Is(err, domain.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "request could not be completed")
	}
}
