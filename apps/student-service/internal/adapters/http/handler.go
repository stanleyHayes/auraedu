package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/application"
	"github.com/auraedu/student-service/internal/domain"
)

// Handler adapts HTTP to the student use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	// Students
	mux.HandleFunc("GET /api/v1/students", h.list)
	mux.HandleFunc("POST /api/v1/students", h.create)
	mux.HandleFunc("GET /api/v1/students/{student_id}", h.get)
	mux.HandleFunc("PATCH /api/v1/students/{student_id}", h.update)
	mux.HandleFunc("DELETE /api/v1/students/{student_id}", h.delete)

	// Student ↔ Guardian links
	mux.HandleFunc("GET /api/v1/students/{student_id}/guardians", h.listStudentGuardians)
	mux.HandleFunc("POST /api/v1/students/{student_id}/guardians", h.linkGuardian)
	mux.HandleFunc("DELETE /api/v1/students/{student_id}/guardians/{guardian_id}", h.unlinkGuardian)

	// Guardians
	mux.HandleFunc("POST /api/v1/guardians", h.createGuardian)
	mux.HandleFunc("GET /api/v1/guardians/{guardian_id}", h.getGuardian)
	mux.HandleFunc("PATCH /api/v1/guardians/{guardian_id}", h.updateGuardian)
	mux.HandleFunc("DELETE /api/v1/guardians/{guardian_id}", h.deleteGuardian)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	students, nextCursor, err := h.svc.List(ctx, actor, limit, cursor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": students, "next_cursor": nullIfEmpty(nextCursor)})
}

type createBody struct {
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	DateOfBirth *string `json:"date_of_birth"`
	Gender      *string `json:"gender"`
	ClassID     *string `json:"class_id"`
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
	student, err := h.svc.Create(ctx, actor, application.CreateStudentRequest{
		FirstName:   body.FirstName,
		LastName:    body.LastName,
		DateOfBirth: body.DateOfBirth,
		Gender:      body.Gender,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	_ = body.ClassID
	httpx.RespondJSON(w, r, http.StatusCreated, student)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	student, err := h.svc.Get(ctx, actor, r.PathValue("student_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, student)
}

type updateBody struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Status    *string `json:"status"`
	ClassID   *string `json:"class_id"`
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
	_ = body.ClassID
	student, err := h.svc.Update(ctx, actor, r.PathValue("student_id"), application.UpdateStudentRequest{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Status:    body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, student)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx, actor, r.PathValue("student_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Guardian handlers ---

type createGuardianBody struct {
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Relationship string  `json:"relationship"`
	Phone        *string `json:"phone"`
	Email        *string `json:"email"`
}

func (h *Handler) createGuardian(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createGuardianBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	g, err := h.svc.CreateGuardian(ctx, actor, application.CreateGuardianRequest{
		FirstName:    body.FirstName,
		LastName:     body.LastName,
		Relationship: body.Relationship,
		Phone:        body.Phone,
		Email:        body.Email,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, g)
}

func (h *Handler) getGuardian(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	g, err := h.svc.GetGuardian(ctx, actor, r.PathValue("guardian_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, g)
}

type updateGuardianBody struct {
	FirstName    *string `json:"first_name"`
	LastName     *string `json:"last_name"`
	Relationship *string `json:"relationship"`
	Phone        *string `json:"phone"`
	Email        *string `json:"email"`
}

func (h *Handler) updateGuardian(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateGuardianBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	g, err := h.svc.UpdateGuardian(ctx, actor, r.PathValue("guardian_id"), application.UpdateGuardianRequest{
		FirstName:    body.FirstName,
		LastName:     body.LastName,
		Relationship: body.Relationship,
		Phone:        body.Phone,
		Email:        body.Email,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, g)
}

func (h *Handler) deleteGuardian(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteGuardian(ctx, actor, r.PathValue("guardian_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listStudentGuardians(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	guardians, nextCursor, err := h.svc.ListStudentGuardians(ctx, actor, r.PathValue("student_id"), limit, cursor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": guardians, "next_cursor": nullIfEmpty(nextCursor)})
}

type linkGuardianBody struct {
	GuardianID   string  `json:"guardian_id"`
	Relationship *string `json:"relationship"`
	IsPrimary    bool    `json:"is_primary"`
}

func (h *Handler) linkGuardian(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body linkGuardianBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	link, err := h.svc.LinkGuardian(ctx, actor, r.PathValue("student_id"), application.LinkGuardianRequest{
		GuardianID:   body.GuardianID,
		Relationship: body.Relationship,
		IsPrimary:    body.IsPrimary,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, link)
}

func (h *Handler) unlinkGuardian(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.UnlinkGuardian(ctx, actor, r.PathValue("student_id"), r.PathValue("guardian_id")); err != nil {
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
		httpx.NotFound(w, r, "student")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureStudentManagement)
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
