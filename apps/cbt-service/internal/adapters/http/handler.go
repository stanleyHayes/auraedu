// Package http provides the HTTP adapter for the CBT service.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/cbt-service/internal/application"
	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the CBT use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/cbt/questions", h.listQuestions)
	mux.HandleFunc("POST /api/v1/cbt/questions", h.createQuestion)
	mux.HandleFunc("GET /api/v1/cbt/questions/{question_id}", h.getQuestion)
	mux.HandleFunc("PATCH /api/v1/cbt/questions/{question_id}", h.updateQuestion)
	mux.HandleFunc("DELETE /api/v1/cbt/questions/{question_id}", h.deleteQuestion)

	mux.HandleFunc("GET /api/v1/cbt/exams", h.listExams)
	mux.HandleFunc("POST /api/v1/cbt/exams", h.createExam)
	mux.HandleFunc("GET /api/v1/cbt/exams/{exam_id}", h.getExam)
	mux.HandleFunc("PATCH /api/v1/cbt/exams/{exam_id}", h.updateExam)
	mux.HandleFunc("DELETE /api/v1/cbt/exams/{exam_id}", h.deleteExam)
	mux.HandleFunc("POST /api/v1/cbt/exams/{exam_id}/start", h.startSubmission)

	mux.HandleFunc("GET /api/v1/cbt/submissions", h.listSubmissions)
	mux.HandleFunc("GET /api/v1/cbt/submissions/{submission_id}", h.getSubmission)
	mux.HandleFunc("POST /api/v1/cbt/submissions/{submission_id}/submit", h.submitAnswers)
	mux.HandleFunc("POST /api/v1/cbt/submissions/{submission_id}/grade", h.gradeSubmission)
}

// --- Questions. ---

func (h *Handler) listQuestions(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	filter := ports.QuestionListFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		SubjectID:      r.URL.Query().Get("subject_id"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListQuestions(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createQuestionBody struct {
	AcademicYearID string   `json:"academic_year_id"`
	SubjectID      string   `json:"subject_id"`
	QuestionText   string   `json:"question_text"`
	QuestionType   string   `json:"question_type"`
	Options        []string `json:"options"`
	CorrectAnswer  string   `json:"correct_answer"`
	Marks          int      `json:"marks"`
}

func (h *Handler) createQuestion(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	q, err := h.svc.CreateQuestion(ctx, actor, application.CreateQuestionRequest{
		AcademicYearID: body.AcademicYearID,
		SubjectID:      body.SubjectID,
		QuestionText:   body.QuestionText,
		QuestionType:   body.QuestionType,
		Options:        body.Options,
		CorrectAnswer:  body.CorrectAnswer,
		Marks:          body.Marks,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, q)
}

func (h *Handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	q, err := h.svc.GetQuestion(ctx, actor, r.PathValue("question_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, q)
}

type updateQuestionBody struct {
	QuestionText  *string  `json:"question_text,omitempty"`
	QuestionType  *string  `json:"question_type,omitempty"`
	Options       []string `json:"options,omitempty"`
	CorrectAnswer *string  `json:"correct_answer,omitempty"`
	Marks         *int     `json:"marks,omitempty"`
	Status        *string  `json:"status,omitempty"`
}

func (h *Handler) updateQuestion(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	q, err := h.svc.UpdateQuestion(ctx, actor, r.PathValue("question_id"), application.UpdateQuestionRequest{
		QuestionText:  body.QuestionText,
		QuestionType:  body.QuestionType,
		Options:       body.Options,
		CorrectAnswer: body.CorrectAnswer,
		Marks:         body.Marks,
		Status:        body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, q)
}

func (h *Handler) deleteQuestion(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteQuestion(ctx, actor, r.PathValue("question_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Exam sessions. ---

func (h *Handler) listExams(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	filter := ports.ExamSessionListFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		SubjectID:      r.URL.Query().Get("subject_id"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListExamSessions(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createExamBody struct {
	Title           string   `json:"title"`
	AcademicYearID  string   `json:"academic_year_id"`
	SubjectID       string   `json:"subject_id"`
	QuestionIDs     []string `json:"question_ids"`
	DurationMinutes int      `json:"duration_minutes"`
	StartAt         *string  `json:"start_at,omitempty"`
	EndAt           *string  `json:"end_at,omitempty"`
}

func (h *Handler) createExam(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createExamBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	startAt, err := parseOptionalTime(body.StartAt)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"start_at": "must be a valid RFC3339 timestamp"})
		return
	}
	endAt, err := parseOptionalTime(body.EndAt)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"end_at": "must be a valid RFC3339 timestamp"})
		return
	}
	e, err := h.svc.CreateExamSession(ctx, actor, application.CreateExamSessionRequest{
		Title:           body.Title,
		AcademicYearID:  body.AcademicYearID,
		SubjectID:       body.SubjectID,
		QuestionIDs:     body.QuestionIDs,
		DurationMinutes: body.DurationMinutes,
		StartAt:         startAt,
		EndAt:           endAt,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, e)
}

func (h *Handler) getExam(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	e, err := h.svc.GetExamSession(ctx, actor, r.PathValue("exam_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, e)
}

type updateExamBody struct {
	Title           *string  `json:"title,omitempty"`
	QuestionIDs     []string `json:"question_ids,omitempty"`
	DurationMinutes *int     `json:"duration_minutes,omitempty"`
	StartAt         *string  `json:"start_at,omitempty"`
	EndAt           *string  `json:"end_at,omitempty"`
	Status          *string  `json:"status,omitempty"`
}

func (h *Handler) updateExam(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateExamBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	startAt, err := parseOptionalTime(body.StartAt)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"start_at": "must be a valid RFC3339 timestamp"})
		return
	}
	endAt, err := parseOptionalTime(body.EndAt)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"end_at": "must be a valid RFC3339 timestamp"})
		return
	}
	e, err := h.svc.UpdateExamSession(ctx, actor, r.PathValue("exam_id"), application.UpdateExamSessionRequest{
		Title:           body.Title,
		QuestionIDs:     body.QuestionIDs,
		DurationMinutes: body.DurationMinutes,
		StartAt:         startAt,
		EndAt:           endAt,
		Status:          body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, e)
}

func (h *Handler) deleteExam(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteExamSession(ctx, actor, r.PathValue("exam_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Submissions. ---

type startSubmissionBody struct {
	StudentID string `json:"student_id"`
}

func (h *Handler) startSubmission(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body startSubmissionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	sub, err := h.svc.StartSubmission(ctx, actor, r.PathValue("exam_id"), body.StudentID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, sub)
}

func (h *Handler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	filter := ports.SubmissionListFilter{
		Limit:         limit,
		Cursor:        r.URL.Query().Get("cursor"),
		ExamSessionID: r.URL.Query().Get("exam_session_id"),
		StudentID:     r.URL.Query().Get("student_id"),
		Status:        r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListSubmissions(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

func (h *Handler) getSubmission(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	sub, err := h.svc.GetSubmission(ctx, actor, r.PathValue("submission_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, sub)
}

type submitAnswersBody struct {
	Answers map[string]string `json:"answers"`
}

func (h *Handler) submitAnswers(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body submitAnswersBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	sub, err := h.svc.SubmitAnswers(ctx, actor, r.PathValue("submission_id"), body.Answers)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, sub)
}

func (h *Handler) gradeSubmission(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	sub, err := h.svc.GradeSubmission(ctx, actor, r.PathValue("submission_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, sub)
}

// --- Helpers. ---

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
		httpx.NotFound(w, r, "resource")
	case errors.Is(err, domain.ErrConflict):
		httpx.RespondJSON(w, r, http.StatusConflict, map[string]any{"error": "conflict", "message": err.Error()})
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, application.FeatureCBTExams)
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

func parseOptionalTime(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	if *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
