// Package http provides the HTTP adapter for the audit service.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/audit-service/internal/application"
	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

// Handler exposes the audit service HTTP surface: liveness/readiness probes
// plus the read-only audit log query API (AURA-23.2). No business logic here
// (agent_plan §5); tenant scoping and RBAC live in the application layer.
type Handler struct {
	health *httpx.HealthState
	query  *application.Query
}

// NewHandler creates the HTTP adapter.
func NewHandler(health *httpx.HealthState, query *application.Query) *Handler {
	return &Handler{health: health, query: query}
}

// Register mounts the service routes onto mux.
//
// GET /api/v1/audit-logs is the contract path (contracts/openapi/audit.v1.yaml).
// GET /api/v1/audit/logs is the gateway-prefixed alias the superadmin web app
// calls; both match the gateway's /api/v1/audit route prefix, which preserves
// the full path downstream.
func (h *Handler) Register(mux *http.ServeMux) {
	h.health.Register(mux)
	mux.HandleFunc("GET /api/v1/audit-logs", h.listAuditLogs)
	mux.HandleFunc("GET /api/v1/audit/logs", h.listAuditLogs)
}

// auditLogDTO is the contract representation of an audit log (audit.v1.yaml
// AuditLog schema); the domain aggregate carries sink-only fields that are
// not exposed over the query API.
type auditLogDTO struct {
	ID           uuid.UUID       `json:"id"`
	TenantID     string          `json:"tenant_id"`
	EventType    string          `json:"event_type"`
	ActorID      *string         `json:"actor_id"`
	ResourceType *string         `json:"resource_type"`
	ResourceID   *string         `json:"resource_id"`
	Metadata     json.RawMessage `json:"metadata"`
	OccurredAt   time.Time       `json:"occurred_at"`
}

func toDTO(log *domain.AuditLog) auditLogDTO {
	return auditLogDTO{
		ID:           log.ID,
		TenantID:     log.TenantID,
		EventType:    log.EventType,
		ActorID:      nullIfEmpty(log.ActorID),
		ResourceType: nullIfEmpty(log.ResourceType),
		ResourceID:   nullIfEmpty(log.ResourceID),
		Metadata:     log.Payload,
		OccurredAt:   log.Timestamp,
	}
}

func (h *Handler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx, actor := h.context(r)
	logs, nextCursor, err := h.query.ListAuditLogs(ctx, actor,
		parseLimit(r.URL.Query().Get("limit")),
		r.URL.Query().Get("cursor"),
	)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}

	data := make([]auditLogDTO, 0, len(logs))
	for _, log := range logs {
		data = append(data, toDTO(log))
	}
	var next any
	if nextCursor != "" {
		next = nextCursor
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": data, "next_cursor": next})
}

func (h *Handler) context(r *http.Request) (context.Context, auth.Actor) {
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
	return ctx, actor
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func nullIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func parseLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}
