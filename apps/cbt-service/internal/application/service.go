// Package application implements the CBT use cases and RBAC policy.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/cbt-service/internal/domain"
	"github.com/auraedu/cbt-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead   = "cbt.read"
	PermAuthor = "cbt.author"
	PermTake   = "cbt.take"
	PermGrade  = "cbt.grade"
)

// FeatureCBTExams is the feature flag key for CBT exams.
const FeatureCBTExams = "cbt_exams"

// Service holds the CBT use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo  ports.Repository
	pub   ports.EventPublisher
	gates flags.Gate
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

type noopPublisher struct{}

func (noopPublisher) PublishQuestionBank(context.Context, string, *domain.QuestionBank, map[string]any) error {
	return nil
}
func (noopPublisher) PublishExamSession(context.Context, string, *domain.ExamSession, map[string]any) error {
	return nil
}
func (noopPublisher) PublishSubmission(context.Context, string, *domain.Submission, map[string]any) error {
	return nil
}

// NewService constructs the application service.
func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}, gates: flags.NewStaticSnapshot()}
	for _, o := range opts {
		o(s)
	}
	return s
}

// --- Question bank requests. ---

// CreateQuestionRequest is the input for creating a question.
type CreateQuestionRequest struct {
	AcademicYearID string
	SubjectID      string
	QuestionText   string
	QuestionType   string
	Options        []string
	CorrectAnswer  string
	Marks          int
}

// UpdateQuestionRequest is the input for patching a question.
type UpdateQuestionRequest struct {
	QuestionText  *string
	QuestionType  *string
	Options       []string
	CorrectAnswer *string
	Marks         *int
	Status        *string
}

// CreateQuestion validates and persists a new QuestionBank for the actor's tenant.
func (s *Service) CreateQuestion(ctx context.Context, actor auth.Actor, req CreateQuestionRequest) (*domain.QuestionBank, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermAuthor)
	if err != nil {
		return nil, err
	}
	q, err := domain.NewQuestionBank(tenantID, req.AcademicYearID, req.SubjectID, req.QuestionText, req.QuestionType, req.CorrectAnswer, req.Marks, req.Options)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateQuestion(ctx, tenantID, q); err != nil {
		return nil, err
	}
	if err := s.pub.PublishQuestionBank(ctx, "cbt.question_created.v1", q, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish question created event", "err", err)
	}
	return q, nil
}

// ListQuestions returns a tenant-scoped page of questions, optionally filtered.
func (s *Service) ListQuestions(ctx context.Context, actor auth.Actor, filter ports.QuestionListFilter) ([]*domain.QuestionBank, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListQuestions(ctx, tenantID, filter)
}

// GetQuestion returns a single question if the actor may read the tenant's data.
func (s *Service) GetQuestion(ctx context.Context, actor auth.Actor, id string) (*domain.QuestionBank, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetQuestionByID(ctx, tenantID, id)
}

// UpdateQuestion patches a question.
func (s *Service) UpdateQuestion(ctx context.Context, actor auth.Actor, id string, req UpdateQuestionRequest) (*domain.QuestionBank, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermAuthor)
	if err != nil {
		return nil, err
	}
	q, err := s.repo.GetQuestionByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := q.ApplyUpdate(req.QuestionText, req.QuestionType, req.CorrectAnswer, req.Marks, req.Options, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return q, nil
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateQuestion(ctx, tenantID, q); err != nil {
		return nil, err
	}
	if err := s.pub.PublishQuestionBank(ctx, "cbt.question_updated.v1", q, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish question updated event", "err", err)
	}
	return q, nil
}

// DeleteQuestion removes a question.
func (s *Service) DeleteQuestion(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermAuthor)
	if err != nil {
		return err
	}
	q, err := s.repo.GetQuestionByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteQuestion(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.PublishQuestionBank(ctx, "cbt.question_deleted.v1", q, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish question deleted event", "err", err)
	}
	return nil
}

// --- Exam session requests. ---

// CreateExamSessionRequest is the input for creating an exam session.
type CreateExamSessionRequest struct {
	Title           string
	AcademicYearID  string
	SubjectID       string
	QuestionIDs     []string
	DurationMinutes int
	StartAt         *time.Time
	EndAt           *time.Time
}

// UpdateExamSessionRequest is the input for patching an exam session.
type UpdateExamSessionRequest struct {
	Title           *string
	QuestionIDs     []string
	DurationMinutes *int
	StartAt         *time.Time
	EndAt           *time.Time
	Status          *string
}

// CreateExamSession validates and persists a new ExamSession for the actor's tenant.
func (s *Service) CreateExamSession(ctx context.Context, actor auth.Actor, req CreateExamSessionRequest) (*domain.ExamSession, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermAuthor)
	if err != nil {
		return nil, err
	}
	if err := s.ensureQuestionsExist(ctx, tenantID, req.QuestionIDs); err != nil {
		return nil, err
	}
	e, err := domain.NewExamSession(tenantID, req.Title, req.AcademicYearID, req.SubjectID, req.QuestionIDs, req.DurationMinutes, req.StartAt, req.EndAt)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateExamSession(ctx, tenantID, e); err != nil {
		return nil, err
	}
	if err := s.pub.PublishExamSession(ctx, "cbt.exam_created.v1", e, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish exam created event", "err", err)
	}
	return e, nil
}

// ListExamSessions returns a tenant-scoped page of exam sessions, optionally filtered.
func (s *Service) ListExamSessions(ctx context.Context, actor auth.Actor, filter ports.ExamSessionListFilter) ([]*domain.ExamSession, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListExamSessions(ctx, tenantID, filter)
}

// GetExamSession returns a single exam session if the actor may read the tenant's data.
func (s *Service) GetExamSession(ctx context.Context, actor auth.Actor, id string) (*domain.ExamSession, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetExamSessionByID(ctx, tenantID, id)
}

// UpdateExamSession patches an exam session.
func (s *Service) UpdateExamSession(ctx context.Context, actor auth.Actor, id string, req UpdateExamSessionRequest) (*domain.ExamSession, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermAuthor)
	if err != nil {
		return nil, err
	}
	e, err := s.repo.GetExamSessionByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if req.QuestionIDs != nil {
		if err := s.ensureQuestionsExist(ctx, tenantID, req.QuestionIDs); err != nil {
			return nil, err
		}
	}
	changed, err := e.ApplyUpdate(req.Title, req.QuestionIDs, req.DurationMinutes, req.StartAt, req.EndAt, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return e, nil
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateExamSession(ctx, tenantID, e); err != nil {
		return nil, err
	}
	if err := s.pub.PublishExamSession(ctx, "cbt.exam_updated.v1", e, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish exam updated event", "err", err)
	}
	return e, nil
}

// DeleteExamSession removes an exam session.
func (s *Service) DeleteExamSession(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermAuthor)
	if err != nil {
		return err
	}
	e, err := s.repo.GetExamSessionByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteExamSession(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.PublishExamSession(ctx, "cbt.exam_deleted.v1", e, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish exam deleted event", "err", err)
	}
	return nil
}

// --- Submission requests. ---

// StartSubmission creates a submission in the in_progress state if the exam
// session is active and the student has no existing submission.
func (s *Service) StartSubmission(ctx context.Context, actor auth.Actor, examSessionID, studentID string) (*domain.Submission, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermTake)
	if err != nil {
		return nil, err
	}
	if actor.UserID != studentID && !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	e, err := s.repo.GetExamSessionByID(ctx, tenantID, examSessionID)
	if err != nil {
		return nil, err
	}
	if !e.IsActive(time.Now().UTC()) {
		return nil, fmt.Errorf("%w: exam session is not active", domain.ErrValidation)
	}
	existing, err := s.repo.GetSubmissionByExamAndStudent(ctx, tenantID, examSessionID, studentID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: student already has an active submission for this exam", domain.ErrConflict)
	}
	sub, err := domain.NewSubmission(tenantID, examSessionID, studentID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateSubmission(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// SubmitAnswers stores the final answers and transitions the submission to submitted.
func (s *Service) SubmitAnswers(ctx context.Context, actor auth.Actor, submissionID string, answers map[string]string) (*domain.Submission, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermTake)
	if err != nil {
		return nil, err
	}
	sub, err := s.repo.GetSubmissionByID(ctx, tenantID, submissionID)
	if err != nil {
		return nil, err
	}
	if actor.UserID != sub.StudentID && !actor.PlatformAdmin {
		return nil, domain.ErrForbidden
	}
	if domain.SubmissionStatus(sub.Status) != domain.SubmissionStatusInProgress {
		return nil, fmt.Errorf("%w: submission is not in_progress", domain.ErrValidation)
	}
	if err := sub.Submit(answers); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSubmission(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	if err := s.pub.PublishSubmission(ctx, "cbt.exam_submitted.v1", sub, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish exam submitted event", "err", err)
	}
	return sub, nil
}

// GradeSubmission auto-grades a submitted exam by comparing answers to question
// bank correct answers, computes score and max_score, transitions to graded,
// and emits cbt.graded.v1.
func (s *Service) GradeSubmission(ctx context.Context, actor auth.Actor, submissionID string) (*domain.Submission, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermGrade)
	if err != nil {
		return nil, err
	}
	sub, err := s.repo.GetSubmissionByID(ctx, tenantID, submissionID)
	if err != nil {
		return nil, err
	}
	if domain.SubmissionStatus(sub.Status) != domain.SubmissionStatusSubmitted {
		return nil, fmt.Errorf("%w: submission must be submitted before grading", domain.ErrValidation)
	}
	e, err := s.repo.GetExamSessionByID(ctx, tenantID, sub.ExamSessionID)
	if err != nil {
		return nil, err
	}

	score, maxScore, err := s.computeScore(ctx, tenantID, e, sub)
	if err != nil {
		return nil, err
	}
	if err := sub.Grade(score, maxScore); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSubmission(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	if err := s.pub.PublishSubmission(ctx, "cbt.graded.v1", sub, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish graded event", "err", err)
	}
	return sub, nil
}

// ListSubmissions returns a tenant-scoped page of submissions, optionally filtered.
func (s *Service) ListSubmissions(ctx context.Context, actor auth.Actor, filter ports.SubmissionListFilter) ([]*domain.Submission, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListSubmissions(ctx, tenantID, filter)
}

// GetSubmission returns a single submission if the actor may read the tenant's data.
func (s *Service) GetSubmission(ctx context.Context, actor auth.Actor, id string) (*domain.Submission, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetSubmissionByID(ctx, tenantID, id)
}

// --- Helpers. ---

func (s *Service) requireAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	if !actor.Authenticated() {
		return "", domain.ErrForbidden
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if !actor.CanAccessTenant(tenantID) {
		return "", domain.ErrForbidden
	}
	if !actor.Has(perm) {
		return "", domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureCBTExams) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureCBTExams)
	}
	return tenantID, nil
}

func (s *Service) ensureQuestionsExist(ctx context.Context, tenantID string, questionIDs []string) error {
	for _, qid := range questionIDs {
		if _, err := s.repo.GetQuestionByID(ctx, tenantID, qid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) computeScore(ctx context.Context, tenantID string, e *domain.ExamSession, sub *domain.Submission) (int, int, error) {
	maxScore := 0
	score := 0
	for _, qid := range e.QuestionIDs {
		q, err := s.repo.GetQuestionByID(ctx, tenantID, qid)
		if err != nil {
			return 0, 0, fmt.Errorf("load question %s: %w", qid, err)
		}
		maxScore += q.Marks
		given, ok := sub.Answers[qid]
		if !ok {
			continue
		}
		if answersEqual(q.QuestionType, given, q.CorrectAnswer) {
			score += q.Marks
		}
	}
	return score, maxScore, nil
}

func answersEqual(questionType, given, correct string) bool {
	if questionType == string(domain.TypeShortAnswer) {
		return strings.EqualFold(strings.TrimSpace(given), strings.TrimSpace(correct))
	}
	return strings.TrimSpace(given) == strings.TrimSpace(correct)
}

func normalizeLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
