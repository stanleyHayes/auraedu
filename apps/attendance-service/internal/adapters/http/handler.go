package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/attendance-service/internal/application"
	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the attendance use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/attendance", h.list)
	mux.HandleFunc("POST /api/v1/attendance", h.create)
	mux.HandleFunc("GET /api/v1/attendance/{attendance_id}", h.get)
	mux.HandleFunc("PATCH /api/v1/attendance/{attendance_id}", h.update)
	mux.HandleFunc("DELETE /api/v1/attendance/{attendance_id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := ports.ListFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		StudentID:      r.URL.Query().Get("student_id"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		Date:           r.URL.Query().Get("date"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.List(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createBody struct {
	StudentID      string  `json:"student_id"`
	AcademicYearID string  `json:"academic_year_id"`
	Date           string  `json:"date"`
	Status         string  `json:"status"`
	Reason         *string `json:"reason"`
	MarkedBy       string  `json:"marked_by"`
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
	record, err := h.svc.Create(ctx, actor, application.CreateAttendanceRequest{
		StudentID:      body.StudentID,
		AcademicYearID: body.AcademicYearID,
		Date:           body.Date,
		Status:         body.Status,
		Reason:         body.Reason,
		MarkedBy:       body.MarkedBy,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.Get(ctx, actor, r.PathValue("attendance_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

type updateBody struct {
	Status   *string `json:"status"`
	Reason   *string `json:"reason"`
	MarkedBy *string `json:"marked_by"`
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
	record, err := h.svc.Update(ctx, actor, r.PathValue("attendance_id"), application.UpdateAttendanceRequest{
		Status:   body.Status,
		Reason:   body.Reason,
		MarkedBy: body.MarkedBy,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx, actor, r.PathValue("attendance_id")); err != nil {
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
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "attendance record")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureAttendance)
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
