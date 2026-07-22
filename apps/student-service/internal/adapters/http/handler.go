// Package http exposes the student service REST API.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	mux.HandleFunc("POST /api/v1/students/import", h.importStudents)
	// Literal segments win over {student_id} in Go 1.22+ ServeMux, so /students/me
	// resolves the caller's own record (AURA-10.12).
	mux.HandleFunc("GET /api/v1/students/me", h.getMyStudent)
	mux.HandleFunc("GET /api/v1/students/{student_id}", h.get)
	mux.HandleFunc("PATCH /api/v1/students/{student_id}", h.update)
	mux.HandleFunc("DELETE /api/v1/students/{student_id}", h.delete)
	mux.HandleFunc("GET /api/v1/students/{student_id}/enrollments", h.listEnrollments)
	mux.HandleFunc("POST /api/v1/students/{student_id}/enrollments", h.createEnrollment)

	// Student ↔ Guardian links
	mux.HandleFunc("GET /api/v1/students/{student_id}/guardians", h.listStudentGuardians)
	mux.HandleFunc("POST /api/v1/students/{student_id}/guardians", h.linkGuardian)
	mux.HandleFunc("DELETE /api/v1/students/{student_id}/guardians/{guardian_id}", h.unlinkGuardian)

	// Guardians
	mux.HandleFunc("POST /api/v1/guardians", h.createGuardian)
	mux.HandleFunc("GET /api/v1/guardians/me/children", h.getMyGuardianChildren)
	mux.HandleFunc("GET /api/v1/guardians/{guardian_id}", h.getGuardian)
	mux.HandleFunc("PATCH /api/v1/guardians/{guardian_id}", h.updateGuardian)
	mux.HandleFunc("DELETE /api/v1/guardians/{guardian_id}", h.deleteGuardian)
}

func (h *Handler) listEnrollments(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	enrollments, next, err := h.svc.ListEnrollments(ctx, actor, r.PathValue("student_id"), parseLimit(r.URL.Query().Get("limit")), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": enrollments, "next_cursor": nullIfEmpty(next)})
}

func (h *Handler) createEnrollment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body struct {
		ClassID        string `json:"class_id"`
		AcademicYearID string `json:"academic_year_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	enrollment, err := h.svc.CreateEnrollment(
		ctx,
		actor,
		r.PathValue("student_id"),
		application.CreateEnrollmentRequest{ClassID: body.ClassID, AcademicYearID: body.AcademicYearID},
	)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, enrollment)
}

func (h *Handler) RegisterInternal(mux *http.ServeMux, token string) {
	mux.HandleFunc("GET /internal/v1/learner-scope", func(w http.ResponseWriter, r *http.Request) {
		provided, expected := r.Header.Get("Authorization"), "Bearer "+token
		if token == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			httpx.Unauthorized(w, r, "valid service credentials are required")
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get(tenancy.HeaderTenantID))
		ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{TenantID: tenantID})
		scope, err := h.svc.ResolveLearnerScope(ctx, tenantID, r.URL.Query().Get("user_id"), r.URL.Query().Get("role"))
		if err != nil {
			h.writeErr(w, r, err)
			return
		}
		httpx.RespondJSON(w, r, http.StatusOK, scope)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	// Optional roster filter (?class_id=<uuid>); the use case validates its format.
	var classID *string
	if v := strings.TrimSpace(r.URL.Query().Get("class_id")); v != "" {
		classID = &v
	}
	students, nextCursor, err := h.svc.List(ctx, actor, classID, limit, cursor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": students, "next_cursor": nullIfEmpty(nextCursor)})
}

type createBody struct {
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	DateOfBirth    *string `json:"date_of_birth"`
	Gender         *string `json:"gender"`
	ClassID        *string `json:"class_id"`
	AcademicYearID *string `json:"academic_year_id"`
	UserID         *string `json:"user_id"`
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
		FirstName:      body.FirstName,
		LastName:       body.LastName,
		DateOfBirth:    body.DateOfBirth,
		Gender:         body.Gender,
		ClassID:        body.ClassID,
		AcademicYearID: body.AcademicYearID,
		UserID:         body.UserID,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, student)
}

func (h *Handler) importStudents(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"file": "missing or invalid multipart file"})
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Default().ErrorContext(ctx, "failed to close uploaded file", "err", cerr)
		}
	}()

	rows, parseErr := parseStudentCSV(file)
	if parseErr != nil {
		h.writeErr(w, r, parseErr)
		return
	}
	result, err := h.svc.ImportStudents(ctx, actor, rows)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, result)
}

func parseStudentCSV(r io.Reader) ([]application.ImportStudentRow, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, domain.ErrValidation
	}
	if len(records) == 0 {
		return nil, domain.ErrValidation
	}

	header := make(map[string]int)
	for i, h := range records[0] {
		header[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"first_name", "last_name"}
	for _, key := range required {
		if _, ok := header[key]; !ok {
			return nil, domain.ErrValidation
		}
	}

	var rows []application.ImportStudentRow
	for i, record := range records[1:] {
		row, err := csvRowToImportRow(header, record)
		if err != nil {
			return nil, err
		}
		// csvRowToImportRow currently returns error per row; we could enrich later.
		_ = i
		rows = append(rows, row)
	}
	return rows, nil
}

func csvRowToImportRow(header map[string]int, record []string) (application.ImportStudentRow, error) {
	get := func(key string) *string {
		idx, ok := header[key]
		if !ok || idx >= len(record) {
			return nil
		}
		v := strings.TrimSpace(record[idx])
		if v == "" {
			return nil
		}
		return &v
	}

	fn := get("first_name")
	ln := get("last_name")
	if fn == nil || ln == nil {
		return application.ImportStudentRow{}, domain.ErrValidation
	}
	return application.ImportStudentRow{
		FirstName:         *fn,
		LastName:          *ln,
		DateOfBirth:       get("date_of_birth"),
		Gender:            get("gender"),
		Relationship:      get("relationship"),
		GuardianFirstName: get("guardian_first_name"),
		GuardianLastName:  get("guardian_last_name"),
		GuardianPhone:     get("guardian_phone"),
		GuardianEmail:     get("guardian_email"),
		UserID:            get("user_id"),
		GuardianUserID:    get("guardian_user_id"),
	}, nil
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

// getMyStudent resolves the caller's own student record from the actor's user id
// (AURA-10.12); 404 when the identity user has no linked student.
func (h *Handler) getMyStudent(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	student, err := h.svc.GetMyStudent(ctx, actor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, student)
}

type updateBody struct {
	FirstName *string         `json:"first_name"`
	LastName  *string         `json:"last_name"`
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
	userID, err := nullableStringPatch(body.UserID)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"user_id": "must be a UUID string or null"})
		return
	}
	student, err := h.svc.Update(ctx, actor, r.PathValue("student_id"), application.UpdateStudentRequest{
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Status:    body.Status,
		UserID:    userID,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, student)
}

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
	UserID       *string `json:"user_id"`
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
		UserID:       body.UserID,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, g)
}

// getMyGuardianChildren resolves the caller's guardian record from the actor's user
// id and returns it with the linked students (AURA-10.12); 404 when the identity
// user has no linked guardian.
func (h *Handler) getMyGuardianChildren(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	result, err := h.svc.GetMyGuardianChildren(ctx, actor)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, result)
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
	limit := parseLimit(r.URL.Query().Get("limit"))
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
	ctx = auth.WithActor(ctx, actor)
	return ctx, actor, true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "student")
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondJSON(w, r, http.StatusConflict, map[string]any{"error": "conflict", "message": err.Error()})
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureStudentManagement)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, domain.ErrUnavailable):
		httpx.RespondJSON(w, r, http.StatusServiceUnavailable, httpx.Error{Code: httpx.ErrInternal, Message: "teacher class scope is unavailable"})
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
