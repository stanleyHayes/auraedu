// Package http adapts HTTP requests to the academic-service application layer.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/auraedu/academic-service/internal/application"
	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the academic use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// RegisterInternal mounts authenticated service-to-service scope routes.
func (h *Handler) RegisterInternal(mux *http.ServeMux, token string) {
	mux.HandleFunc("GET /internal/v1/teacher-class-scope", func(w http.ResponseWriter, r *http.Request) {
		provided, expected := r.Header.Get("Authorization"), "Bearer "+token
		if token == "" || len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			httpx.Unauthorized(w, r, "valid service credentials are required")
			return
		}
		tenantID := strings.TrimSpace(r.Header.Get(tenancy.HeaderTenantID))
		ctx := tenancy.WithContext(r.Context(), tenancy.TenantContext{TenantID: tenantID})
		classIDs, err := h.svc.ResolveTeacherClassScope(ctx, tenantID, strings.TrimSpace(r.URL.Query().Get("user_id")))
		if err != nil {
			h.writeErr(w, r, err, "teacher class scope")
			return
		}
		httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"class_ids": classIDs})
	})
}

// Register mounts the service routes. Paths follow contracts/openapi/academic.v1.yaml;
// get/patch/delete by id mirror the academic-year routes pending contract coverage.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/academic-years", h.listYears)
	mux.HandleFunc("POST /api/v1/academic-years", h.createYear)
	mux.HandleFunc("GET /api/v1/academic-years/{academic_year_id}", h.getYear)
	mux.HandleFunc("PATCH /api/v1/academic-years/{academic_year_id}", h.updateYear)
	mux.HandleFunc("DELETE /api/v1/academic-years/{academic_year_id}", h.deleteYear)

	mux.HandleFunc("GET /api/v1/terms", h.listTerms)
	mux.HandleFunc("POST /api/v1/terms", h.createTerm)
	mux.HandleFunc("GET /api/v1/terms/{term_id}", h.getTerm)
	mux.HandleFunc("PATCH /api/v1/terms/{term_id}", h.updateTerm)
	mux.HandleFunc("DELETE /api/v1/terms/{term_id}", h.deleteTerm)

	mux.HandleFunc("GET /api/v1/classes", h.listClasses)
	mux.HandleFunc("POST /api/v1/classes", h.createClass)
	mux.HandleFunc("GET /api/v1/classes/{class_id}", h.getClass)
	mux.HandleFunc("PATCH /api/v1/classes/{class_id}", h.updateClass)
	mux.HandleFunc("DELETE /api/v1/classes/{class_id}", h.deleteClass)

	mux.HandleFunc("GET /api/v1/subjects", h.listSubjects)
	mux.HandleFunc("POST /api/v1/subjects", h.createSubject)
	mux.HandleFunc("GET /api/v1/subjects/{subject_id}", h.getSubject)
	mux.HandleFunc("PATCH /api/v1/subjects/{subject_id}", h.updateSubject)
	mux.HandleFunc("DELETE /api/v1/subjects/{subject_id}", h.deleteSubject)

	mux.HandleFunc("GET /api/v1/grading-scales", h.listGradingScales)
	mux.HandleFunc("POST /api/v1/grading-scales", h.createGradingScale)
	mux.HandleFunc("GET /api/v1/grading-scales/{grading_scale_id}", h.getGradingScale)
	mux.HandleFunc("PATCH /api/v1/grading-scales/{grading_scale_id}", h.updateGradingScale)
	mux.HandleFunc("DELETE /api/v1/grading-scales/{grading_scale_id}", h.deleteGradingScale)

	mux.HandleFunc("GET /api/v1/timetable", h.listTimetable)
	mux.HandleFunc("POST /api/v1/timetable", h.createTimetable)
	mux.HandleFunc("GET /api/v1/timetable/{entry_id}", h.getTimetable)
	mux.HandleFunc("PATCH /api/v1/timetable/{entry_id}", h.updateTimetable)
	mux.HandleFunc("DELETE /api/v1/timetable/{entry_id}", h.deleteTimetable)
}

// ---- academic years ---------------------------------------------------------

func (h *Handler) listYears(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	years, nextCursor, err := h.svc.ListAcademicYears(ctx, actor, listLimit(r), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err, "academic year")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": years, "next_cursor": nullIfEmpty(nextCursor)})
}

type createYearBody struct {
	Name      string  `json:"name"`
	Code      *string `json:"code"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	IsCurrent bool    `json:"is_current"`
}

func (h *Handler) createYear(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createYearBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	year, err := h.svc.CreateAcademicYear(ctx, actor, application.CreateAcademicYearRequest{
		Name:      body.Name,
		Code:      body.Code,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
		IsCurrent: body.IsCurrent,
	})
	if err != nil {
		h.writeErr(w, r, err, "academic year")
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, year)
}

func (h *Handler) getYear(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	year, err := h.svc.GetAcademicYear(ctx, actor, r.PathValue("academic_year_id"))
	if err != nil {
		h.writeErr(w, r, err, "academic year")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, year)
}

type updateYearBody struct {
	Name      *string `json:"name"`
	Code      *string `json:"code"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
	Status    *string `json:"status"`
	IsCurrent *bool   `json:"is_current"`
}

func (h *Handler) updateYear(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateYearBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	year, err := h.svc.UpdateAcademicYear(ctx, actor, r.PathValue("academic_year_id"), application.UpdateAcademicYearRequest{
		Name:      body.Name,
		Code:      body.Code,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
		Status:    body.Status,
		IsCurrent: body.IsCurrent,
	})
	if err != nil {
		h.writeErr(w, r, err, "academic year")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, year)
}

func (h *Handler) deleteYear(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAcademicYear(ctx, actor, r.PathValue("academic_year_id")); err != nil {
		h.writeErr(w, r, err, "academic year")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- terms ------------------------------------------------------------------

func (h *Handler) listTerms(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	terms, nextCursor, err := h.svc.ListTerms(ctx, actor, listLimit(r), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err, "term")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": terms, "next_cursor": nullIfEmpty(nextCursor)})
}

type createTermBody struct {
	AcademicYearID string `json:"academic_year_id"`
	Name           string `json:"name"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
}

func (h *Handler) createTerm(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createTermBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	term, err := h.svc.CreateTerm(ctx, actor, application.CreateTermRequest{
		AcademicYearID: body.AcademicYearID,
		Name:           body.Name,
		StartDate:      body.StartDate,
		EndDate:        body.EndDate,
	})
	if err != nil {
		h.writeErr(w, r, err, "term")
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, term)
}

func (h *Handler) getTerm(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	term, err := h.svc.GetTerm(ctx, actor, r.PathValue("term_id"))
	if err != nil {
		h.writeErr(w, r, err, "term")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, term)
}

type updateTermBody struct {
	Name      *string `json:"name"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

func (h *Handler) updateTerm(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateTermBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	term, err := h.svc.UpdateTerm(ctx, actor, r.PathValue("term_id"), application.UpdateTermRequest{
		Name:      body.Name,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
	})
	if err != nil {
		h.writeErr(w, r, err, "term")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, term)
}

func (h *Handler) deleteTerm(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteTerm(ctx, actor, r.PathValue("term_id")); err != nil {
		h.writeErr(w, r, err, "term")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- classes ----------------------------------------------------------------

func (h *Handler) listClasses(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	classes, nextCursor, err := h.svc.ListClasses(ctx, actor, listLimit(r), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err, "class")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": classes, "next_cursor": nullIfEmpty(nextCursor)})
}

type createClassBody struct {
	Name           string  `json:"name"`
	AcademicYearID string  `json:"academic_year_id"`
	ClassTeacherID *string `json:"class_teacher_id"`
	Capacity       *int    `json:"capacity"`
}

func (h *Handler) createClass(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createClassBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	class, err := h.svc.CreateClass(ctx, actor, application.CreateClassRequest{
		Name:           body.Name,
		AcademicYearID: body.AcademicYearID,
		ClassTeacherID: body.ClassTeacherID,
		Capacity:       body.Capacity,
	})
	if err != nil {
		h.writeErr(w, r, err, "class")
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, class)
}

func (h *Handler) getClass(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	class, err := h.svc.GetClass(ctx, actor, r.PathValue("class_id"))
	if err != nil {
		h.writeErr(w, r, err, "class")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, class)
}

type updateClassBody struct {
	Name           *string `json:"name"`
	ClassTeacherID *string `json:"class_teacher_id"`
	Capacity       *int    `json:"capacity"`
}

func (h *Handler) updateClass(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateClassBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	class, err := h.svc.UpdateClass(ctx, actor, r.PathValue("class_id"), application.UpdateClassRequest{
		Name:           body.Name,
		ClassTeacherID: body.ClassTeacherID,
		Capacity:       body.Capacity,
	})
	if err != nil {
		h.writeErr(w, r, err, "class")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, class)
}

func (h *Handler) deleteClass(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteClass(ctx, actor, r.PathValue("class_id")); err != nil {
		h.writeErr(w, r, err, "class")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- subjects ---------------------------------------------------------------

func (h *Handler) listSubjects(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	subjects, nextCursor, err := h.svc.ListSubjects(ctx, actor, listLimit(r), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err, "subject")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": subjects, "next_cursor": nullIfEmpty(nextCursor)})
}

type createSubjectBody struct {
	Name        string  `json:"name"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
}

func (h *Handler) createSubject(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createSubjectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	subject, err := h.svc.CreateSubject(ctx, actor, application.CreateSubjectRequest{
		Name:        body.Name,
		Code:        body.Code,
		Description: body.Description,
	})
	if err != nil {
		h.writeErr(w, r, err, "subject")
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, subject)
}

func (h *Handler) getSubject(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	subject, err := h.svc.GetSubject(ctx, actor, r.PathValue("subject_id"))
	if err != nil {
		h.writeErr(w, r, err, "subject")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, subject)
}

type updateSubjectBody struct {
	Name        *string `json:"name"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
}

func (h *Handler) updateSubject(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateSubjectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	subject, err := h.svc.UpdateSubject(ctx, actor, r.PathValue("subject_id"), application.UpdateSubjectRequest{
		Name:        body.Name,
		Code:        body.Code,
		Description: body.Description,
	})
	if err != nil {
		h.writeErr(w, r, err, "subject")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, subject)
}

func (h *Handler) deleteSubject(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteSubject(ctx, actor, r.PathValue("subject_id")); err != nil {
		h.writeErr(w, r, err, "subject")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- grading scales --------------------------------------------------------

type gradingScaleBody struct {
	Name   string              `json:"name"`
	Ranges []domain.GradeRange `json:"ranges"`
}

func (h *Handler) listGradingScales(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	records, nextCursor, err := h.svc.ListGradingScales(ctx, actor, listLimit(r), r.URL.Query().Get("cursor"))
	if err != nil {
		h.writeErr(w, r, err, "grading scale")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

func (h *Handler) createGradingScale(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body gradingScaleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.CreateGradingScale(ctx, actor, application.CreateGradingScaleRequest{Name: body.Name, Ranges: body.Ranges})
	if err != nil {
		h.writeErr(w, r, err, "grading scale")
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, record)
}

func (h *Handler) getGradingScale(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	record, err := h.svc.GetGradingScale(ctx, actor, r.PathValue("grading_scale_id"))
	if err != nil {
		h.writeErr(w, r, err, "grading scale")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) updateGradingScale(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body struct {
		Name   *string              `json:"name"`
		Ranges *[]domain.GradeRange `json:"ranges"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	record, err := h.svc.UpdateGradingScale(
		ctx,
		actor,
		r.PathValue("grading_scale_id"),
		application.UpdateGradingScaleRequest{Name: body.Name, Ranges: body.Ranges},
	)
	if err != nil {
		h.writeErr(w, r, err, "grading scale")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, record)
}

func (h *Handler) deleteGradingScale(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteGradingScale(ctx, actor, r.PathValue("grading_scale_id")); err != nil {
		h.writeErr(w, r, err, "grading scale")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- timetable -------------------------------------------------------------

type timetableBody struct {
	ClassID   string  `json:"class_id"`
	TermID    string  `json:"term_id"`
	SubjectID string  `json:"subject_id"`
	TeacherID *string `json:"teacher_id"`
	Weekday   int     `json:"weekday"`
	StartTime string  `json:"start_time"`
	EndTime   string  `json:"end_time"`
	Room      *string `json:"room"`
}

func (h *Handler) listTimetable(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	filter := ports.TimetableFilter{TermID: r.URL.Query().Get("term_id"), Status: r.URL.Query().Get("status"), Limit: listLimit(r)}
	if id := strings.TrimSpace(r.URL.Query().Get("class_id")); id != "" {
		filter.ClassIDs = []string{id}
	}
	if day, err := strconv.Atoi(r.URL.Query().Get("weekday")); err == nil {
		filter.Weekday = day
	}
	records, err := h.svc.ListTimetable(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err, "timetable entry")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records})
}
func (h *Handler) createTimetable(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	var body timetableBody
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	entry, err := h.svc.CreateTimetableEntry(ctx, actor, application.CreateTimetableRequest{
		ClassID:   body.ClassID,
		TermID:    body.TermID,
		SubjectID: body.SubjectID,
		TeacherID: body.TeacherID,
		Weekday:   body.Weekday,
		StartTime: body.StartTime,
		EndTime:   body.EndTime,
		Room:      body.Room,
	})
	if err != nil {
		h.writeErr(w, r, err, "timetable entry")
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, entry)
}
func (h *Handler) getTimetable(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	entry, err := h.svc.GetTimetableEntry(ctx, actor, r.PathValue("entry_id"))
	if err != nil {
		h.writeErr(w, r, err, "timetable entry")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, entry)
}
func (h *Handler) updateTimetable(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	var body struct {
		TeacherID *string `json:"teacher_id"`
		Weekday   *int    `json:"weekday"`
		StartTime *string `json:"start_time"`
		EndTime   *string `json:"end_time"`
		Room      *string `json:"room"`
		Status    *string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	entry, err := h.svc.UpdateTimetableEntry(
		ctx,
		actor,
		r.PathValue("entry_id"),
		application.UpdateTimetableRequest{
			TeacherID: body.TeacherID,
			Weekday:   body.Weekday,
			StartTime: body.StartTime,
			EndTime:   body.EndTime,
			Room:      body.Room,
			Status:    body.Status,
		},
	)
	if err != nil {
		h.writeErr(w, r, err, "timetable entry")
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, entry)
}
func (h *Handler) deleteTimetable(w http.ResponseWriter, r *http.Request) {
	ctx, actor, _ := h.context(r)
	if err := h.svc.DeleteTimetableEntry(ctx, actor, r.PathValue("entry_id")); err != nil {
		h.writeErr(w, r, err, "timetable entry")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- shared helpers ---------------------------------------------------------

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

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error, resource string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, resource)
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondJSON(w, r, http.StatusConflict, map[string]any{"error": "conflict", "message": "timetable overlaps an existing class or teacher period"})
	case errors.Is(err, flags.ErrFeatureDisabled):
		feature := application.FeatureAcademicManagement
		if strings.Contains(err.Error(), application.FeatureTimetable) {
			feature = application.FeatureTimetable
		}
		httpx.FeatureDisabled(w, r, feature)
	case errors.Is(err, domain.ErrForbidden):
		httpx.Forbidden(w, r, "not permitted for this actor or tenant")
	case errors.Is(err, domain.ErrMissingTenant):
		httpx.TenantMismatch(w, r)
	case errors.Is(err, domain.ErrUnavailable):
		httpx.RespondJSON(w, r, http.StatusServiceUnavailable, httpx.Error{Code: httpx.ErrInternal, Message: "academic scope dependency is unavailable"})
	default:
		httpx.RespondError(w, r, httpx.ErrorFrom(err))
	}
}

func listLimit(r *http.Request) int {
	limit := 25
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}
	return limit
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
