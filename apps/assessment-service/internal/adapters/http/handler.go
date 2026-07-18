// Package http adapts HTTP requests to the assessment-service application layer.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/auraedu/assessment-service/internal/application"
	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/httpx"
	"github.com/auraedu/platform/tenancy"
)

// Handler adapts HTTP to the assessment use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

// NewHandler creates the HTTP adapter.
func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/assessments", h.listAssessments)
	mux.HandleFunc("POST /api/v1/assessments", h.createAssessment)
	mux.HandleFunc("GET /api/v1/assessments/{assessment_id}", h.getAssessment)
	mux.HandleFunc("PATCH /api/v1/assessments/{assessment_id}", h.updateAssessment)
	mux.HandleFunc("DELETE /api/v1/assessments/{assessment_id}", h.deleteAssessment)

	mux.HandleFunc("GET /api/v1/assessments/{assessment_id}/scores", h.listScores)
	mux.HandleFunc("POST /api/v1/assessments/{assessment_id}/scores", h.createScore)
	mux.HandleFunc("GET /api/v1/assessments/{assessment_id}/scores/{score_id}", h.getScore)
	mux.HandleFunc("PATCH /api/v1/assessments/{assessment_id}/scores/{score_id}", h.updateScore)
	mux.HandleFunc("DELETE /api/v1/assessments/{assessment_id}/scores/{score_id}", h.deleteScore)

	mux.HandleFunc("GET /api/v1/assignments", h.listAssignments)
	mux.HandleFunc("POST /api/v1/assignments", h.createAssignment)
	mux.HandleFunc("GET /api/v1/assignments/{assignment_id}", h.getAssignment)
	mux.HandleFunc("PATCH /api/v1/assignments/{assignment_id}", h.updateAssignment)
	mux.HandleFunc("DELETE /api/v1/assignments/{assignment_id}", h.deleteAssignment)
	mux.HandleFunc("POST /api/v1/assignments/{assignment_id}/publish", h.publishAssignment)

	mux.HandleFunc("GET /api/v1/gradebook", h.getGradebook)
}

// --- Assessments. ---

func (h *Handler) listAssessments(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := 25
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}
	filter := ports.AssessmentListFilter{
		Limit:          limit,
		Cursor:         r.URL.Query().Get("cursor"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		SubjectID:      r.URL.Query().Get("subject_id"),
		Type:           r.URL.Query().Get("type"),
		Status:         r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListAssessments(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createAssessmentBody struct {
	AcademicYearID string  `json:"academic_year_id"`
	SubjectID      string  `json:"subject_id"`
	Type           string  `json:"type"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	MaxScore       int     `json:"max_score"`
	DueDate        *string `json:"due_date"`
}

func (h *Handler) createAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createAssessmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	dueDate, err := parseOptionalTime(body.DueDate)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"due_date": "must be a valid RFC3339 timestamp"})
		return
	}
	assessment, err := h.svc.CreateAssessment(ctx, actor, application.CreateAssessmentRequest{
		AcademicYearID: body.AcademicYearID,
		SubjectID:      body.SubjectID,
		Type:           body.Type,
		Title:          body.Title,
		Description:    body.Description,
		MaxScore:       body.MaxScore,
		DueDate:        dueDate,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, assessment)
}

func (h *Handler) getAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	assessment, err := h.svc.GetAssessment(ctx, actor, r.PathValue("assessment_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, assessment)
}

type updateAssessmentBody struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Type        *string `json:"type"`
	MaxScore    *int    `json:"max_score"`
	DueDate     *string `json:"due_date"`
	Status      *string `json:"status"`
}

func (h *Handler) updateAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateAssessmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	dueDate, err := parseOptionalTime(body.DueDate)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"due_date": "must be a valid RFC3339 timestamp"})
		return
	}
	assessment, err := h.svc.UpdateAssessment(ctx, actor, r.PathValue("assessment_id"), application.UpdateAssessmentRequest{
		Title:       body.Title,
		Description: body.Description,
		Type:        body.Type,
		MaxScore:    body.MaxScore,
		DueDate:     dueDate,
		Status:      body.Status,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, assessment)
}

func (h *Handler) deleteAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAssessment(ctx, actor, r.PathValue("assessment_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Scores. ---

func (h *Handler) listScores(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := 25
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}
	filter := ports.ScoreListFilter{
		Limit:     limit,
		Cursor:    r.URL.Query().Get("cursor"),
		StudentID: r.URL.Query().Get("student_id"),
	}
	records, nextCursor, err := h.svc.ListScores(ctx, actor, r.PathValue("assessment_id"), filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": records, "next_cursor": nullIfEmpty(nextCursor)})
}

type createScoreBody struct {
	StudentID  string `json:"student_id"`
	Score      int    `json:"score"`
	RecordedBy string `json:"recorded_by"`
	Notes      string `json:"notes"`
}

func (h *Handler) createScore(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createScoreBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	score, err := h.svc.CreateScore(ctx, actor, application.CreateScoreRequest{
		AssessmentID: r.PathValue("assessment_id"),
		StudentID:    body.StudentID,
		Score:        body.Score,
		RecordedBy:   body.RecordedBy,
		Notes:        body.Notes,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, score)
}

func (h *Handler) getScore(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	score, err := h.svc.GetScore(ctx, actor, r.PathValue("assessment_id"), r.PathValue("score_id"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, score)
}

type updateScoreBody struct {
	Score *int    `json:"score"`
	Notes *string `json:"notes"`
}

func (h *Handler) updateScore(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateScoreBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	score, err := h.svc.UpdateScore(ctx, actor, r.PathValue("assessment_id"), r.PathValue("score_id"), application.UpdateScoreRequest{
		Score: body.Score,
		Notes: body.Notes,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, score)
}

func (h *Handler) deleteScore(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteScore(ctx, actor, r.PathValue("assessment_id"), r.PathValue("score_id")); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Assignments. ---

// assignmentResponse is the contract shape for the assignments API
// (contracts/openapi/assessment.v1.yaml Assignment). instructions maps to the
// assessment description.
type assignmentResponse struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Title          string     `json:"title"`
	Instructions   *string    `json:"instructions"`
	SubjectID      string     `json:"subject_id"`
	AcademicYearID string     `json:"academic_year_id"`
	ClassIDs       []string   `json:"class_ids"`
	DueDate        *time.Time `json:"due_date"`
	MaxScore       int        `json:"max_score"`
	Status         string     `json:"status"`
	PublishedAt    *time.Time `json:"published_at"`
}

func toAssignmentResponse(a *domain.Assessment) assignmentResponse {
	classIDs := a.ClassIDs
	if classIDs == nil {
		classIDs = []string{}
	}
	return assignmentResponse{
		ID:             a.ID,
		TenantID:       a.TenantID,
		Title:          a.Title,
		Instructions:   a.Description,
		SubjectID:      a.SubjectID,
		AcademicYearID: a.AcademicYearID,
		ClassIDs:       classIDs,
		DueDate:        a.DueDate,
		MaxScore:       a.MaxScore,
		Status:         a.Status,
		PublishedAt:    a.PublishedAt,
	}
}

func (h *Handler) listAssignments(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	limit := 25
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}
	filter := ports.AssignmentListFilter{
		Limit:     limit,
		Cursor:    r.URL.Query().Get("cursor"),
		SubjectID: r.URL.Query().Get("subject_id"),
		ClassID:   r.URL.Query().Get("class_id"),
		StudentID: r.URL.Query().Get("student_id"),
		Status:    r.URL.Query().Get("status"),
	}
	records, nextCursor, err := h.svc.ListAssignments(ctx, actor, filter)
	if err != nil {
		h.writeErrFeature(w, r, err, application.FeatureAssignments)
		return
	}
	out := make([]assignmentResponse, 0, len(records))
	for _, rec := range records {
		out = append(out, toAssignmentResponse(rec))
	}
	httpx.RespondJSON(w, r, http.StatusOK, map[string]any{"data": out, "next_cursor": nullIfEmpty(nextCursor)})
}

type createAssignmentBody struct {
	AcademicYearID string   `json:"academic_year_id"`
	SubjectID      string   `json:"subject_id"`
	Title          string   `json:"title"`
	Instructions   string   `json:"instructions"`
	MaxScore       int      `json:"max_score"`
	DueDate        *string  `json:"due_date"`
	ClassIDs       []string `json:"class_ids"`
}

func (h *Handler) createAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body createAssignmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	dueDate, err := parseOptionalTime(body.DueDate)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"due_date": "must be a valid RFC3339 timestamp"})
		return
	}
	assignment, err := h.svc.CreateAssignment(ctx, actor, application.CreateAssignmentRequest{
		AcademicYearID: body.AcademicYearID,
		SubjectID:      body.SubjectID,
		Title:          body.Title,
		Instructions:   body.Instructions,
		MaxScore:       body.MaxScore,
		DueDate:        dueDate,
		ClassIDs:       body.ClassIDs,
	})
	if err != nil {
		h.writeErrFeature(w, r, err, application.FeatureAssignments)
		return
	}
	httpx.RespondJSON(w, r, http.StatusCreated, toAssignmentResponse(assignment))
}

func (h *Handler) getAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	assignment, err := h.svc.GetAssignment(ctx, actor, r.PathValue("assignment_id"))
	if err != nil {
		h.writeErrFeature(w, r, err, application.FeatureAssignments)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, toAssignmentResponse(assignment))
}

type updateAssignmentBody struct {
	Title        *string  `json:"title"`
	Instructions *string  `json:"instructions"`
	MaxScore     *int     `json:"max_score"`
	DueDate      *string  `json:"due_date"`
	ClassIDs     []string `json:"class_ids"`
}

func (h *Handler) updateAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	var body updateAssignmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.ValidationError(w, r, map[string]any{"body": "invalid JSON"})
		return
	}
	dueDate, err := parseOptionalTime(body.DueDate)
	if err != nil {
		httpx.ValidationError(w, r, map[string]any{"due_date": "must be a valid RFC3339 timestamp"})
		return
	}
	assignment, err := h.svc.UpdateAssignment(ctx, actor, r.PathValue("assignment_id"), application.UpdateAssignmentRequest{
		Title:        body.Title,
		Instructions: body.Instructions,
		MaxScore:     body.MaxScore,
		DueDate:      dueDate,
		ClassIDs:     body.ClassIDs,
	})
	if err != nil {
		h.writeErrFeature(w, r, err, application.FeatureAssignments)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, toAssignmentResponse(assignment))
}

func (h *Handler) deleteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAssignment(ctx, actor, r.PathValue("assignment_id")); err != nil {
		h.writeErrFeature(w, r, err, application.FeatureAssignments)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) publishAssignment(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	assignment, err := h.svc.PublishAssignment(ctx, actor, r.PathValue("assignment_id"))
	if err != nil {
		h.writeErrFeature(w, r, err, application.FeatureAssignments)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, toAssignmentResponse(assignment))
}

// --- Gradebook. ---

func (h *Handler) getGradebook(w http.ResponseWriter, r *http.Request) {
	ctx, actor, ok := h.context(r)
	if !ok {
		return
	}
	filter := ports.GradebookFilter{
		StudentID:      r.URL.Query().Get("student_id"),
		ClassID:        r.URL.Query().Get("class_id"),
		AcademicYearID: r.URL.Query().Get("academic_year_id"),
		SubjectID:      r.URL.Query().Get("subject_id"),
	}
	book, err := h.svc.GetGradebook(ctx, actor, filter)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.RespondJSON(w, r, http.StatusOK, book)
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
	h.writeErrFeature(w, r, err, application.FeatureAssessments)
}

func (h *Handler) writeErrFeature(w http.ResponseWriter, r *http.Request, err error, feature string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.ValidationError(w, r, map[string]any{"detail": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		httpx.NotFound(w, r, "assessment or score")
	case errors.Is(err, flags.ErrFeatureDisabled):
		httpx.FeatureDisabled(w, r, feature)
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
