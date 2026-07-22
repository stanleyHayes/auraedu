// Package http exposes the staff service REST API.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/staff-service/internal/application"
	"github.com/auraedu/staff-service/internal/domain"
)

// Handler adapts HTTP to the staff use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// RegisterInternal mounts service-to-service routes protected by a shared token.
func (h *Handler) RegisterInternal(mux *http.ServeMux, token string) {
	mux.HandleFunc("GET /internal/v1/teacher-scope", func(w http.ResponseWriter, r *http.Request) {
		provided, expected := r.Header.Get("Authorization"), "Bearer "+token
		if token == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			httpx.Unauthorized(w, r, "valid service credentials are required")
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get(tenancy.HeaderTenantID))
		ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{TenantID: tenantID})
		staffID, err := h.svc.ResolveTeacherScope(ctx, tenantID, strings.TrimSpace(r.URL.Query().Get("user_id")))
		if err != nil {
			h.writeErr(w, r, err)
			return
		}
		classIDs, subjectIDs, err := h.svc.ResolveTeacherAssignments(ctx, tenantID, staffID)
		if err != nil {
			h.writeErr(w, r, err)
			return
		}
		httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"staff_id": staffID, "class_ids": classIDs, "subject_ids": subjectIDs})
	})
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/staff", h.list)
	mux.HandleFunc("POST /api/v1/staff", h.create)
	mux.HandleFunc("GET /api/v1/staff/{staff_id}", h.get)
	mux.HandleFunc("PATCH /api/v1/staff/{staff_id}", h.update)
	mux.HandleFunc("DELETE /api/v1/staff/{staff_id}", h.delete)
	mux.HandleFunc("GET /api/v1/staff/{staff_id}/assignments", h.listAssignments)
	mux.HandleFunc("POST /api/v1/staff/{staff_id}/assignments", h.createAssignment)
	mux.HandleFunc("DELETE /api/v1/staff/{staff_id}/assignments/{assignment_id}", h.deleteAssignment)
}

type createAssignmentBody struct {
	ClassID   string  `json:"class_id"`
	SubjectID *string `json:"subject_id"`
	Role      *string `json:"role"`
}

func (h *Handler) listAssignments(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	rows, next, err := h.svc.ListAssignments(ctx, actor, r.PathValue("staff_id"), parseLimit(r.URL.Query().Get("limit")), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": rows, "next_cursor": nullIfEmpty(next)})
}

func (h *Handler) createAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	var body createAssignmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	assignment, err := h.svc.CreateAssignment(
		ctx,
		actor,
		r.PathValue("staff_id"),
		application.CreateAssignmentRequest{
			ClassID:   body.ClassID,
			SubjectID: body.SubjectID,
			Role:      body.Role,
		},
	)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, assignment)
}

func (h *Handler) deleteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	if err := h.svc.DeleteAssignment(ctx, actor, r.PathValue("staff_id"), r.PathValue("assignment_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	staff, nextCursor, err := h.svc.List(ctx, actor, limit, cursor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": staff, "next_cursor": nullIfEmpty(nextCursor)})
}

type createBody struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	StaffType string  `json:"staff_type"`
	Email     *string `json:"email"`
	UserID    *string `json:"user_id"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	staff, err := h.svc.Create(ctx, actor, application.CreateStaffRequest{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		StaffType: body.StaffType,
		Email:     body.Email,
		UserID:    body.UserID,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, staff)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	staff, err := h.svc.Get(ctx, actor, r.PathValue("staff_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, staff)
}

type updateBody struct {
	FirstName *string         `json:"first_name"`
	LastName  *string         `json:"last_name"`
	StaffType *string         `json:"staff_type"`
	Email     json.RawMessage `json:"email"`
	Status    *string         `json:"status"`
	UserID    json.RawMessage `json:"user_id"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	email, err := nullableStringPatch(body.Email)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"email": "must be a string or null"})
		return
	}
	userID, err := nullableStringPatch(body.UserID)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"user_id": "must be a UUID string or null"})
		return
	}
	staff, err := h.svc.Update(ctx, actor, r.PathValue("staff_id"), application.UpdateStaffRequest{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		StaffType: body.StaffType,
		Email:     email,
		Status:    body.Status,
		UserID:    userID,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, staff)
}

// nullableStringPatch preserves the difference between an omitted property
// (leave unchanged) and JSON null (clear the stored optional value).
func nullableStringPatch(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if string(raw) == "null" {
		empty := ""
		return &empty, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx, actor, r.PathValue("staff_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
		httpx.NotFound(w, r, "staff")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureStaffManagement)
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
