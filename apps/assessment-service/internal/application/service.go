// Package application holds the assessment-service use cases. Tenant scope, RBAC,
// feature-flag checks and event publishing belong here (agent_plan §5).
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead         = "assessments.read"
	PermManage       = "assessments.manage"
	PermRecordScores = "assessments.record_scores"
)

// Feature flag keys.
const (
	FeatureAssessments = "assessments"
	FeatureAssignments = "assignments"
)

// Service holds the assessment use cases. Tenant scope + RBAC + feature-flag checks
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

func (noopPublisher) PublishAssessment(context.Context, string, *domain.Assessment, map[string]any) error {
	return nil
}
func (noopPublisher) PublishAssignment(context.Context, string, *domain.Assessment, map[string]any) error {
	return nil
}
func (noopPublisher) PublishScore(context.Context, string, *domain.Score, map[string]any) error {
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

// --- Assessment requests. ---

// CreateAssessmentRequest is the input for creating an assessment.
type CreateAssessmentRequest struct {
	AcademicYearID string
	SubjectID      string
	Type           string
	Title          string
	Description    string
	MaxScore       int
	DueDate        *time.Time
}

// UpdateAssessmentRequest is the input for patching an assessment.
type UpdateAssessmentRequest struct {
	Title       *string
	Description *string
	Type        *string
	MaxScore    *int
	DueDate     *time.Time
	Status      *string
}

// CreateAssessment validates and persists a new Assessment for the actor's tenant.
func (s *Service) CreateAssessment(ctx context.Context, actor auth.Actor, req CreateAssessmentRequest) (*domain.Assessment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	assessment, err := domain.NewAssessment(tenantID, req.AcademicYearID, req.SubjectID, req.Type, req.Title, req.Description, req.MaxScore, req.DueDate)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateAssessment(ctx, tenantID, assessment); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the assessment is persisted.
	_ = s.pub.PublishAssessment(ctx, "assessment.created.v1", assessment, nil)
	return assessment, nil
}

// ListAssessments returns a tenant-scoped page of assessments, optionally filtered.
func (s *Service) ListAssessments(ctx context.Context, actor auth.Actor, filter ports.AssessmentListFilter) ([]*domain.Assessment, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListAssessments(ctx, tenantID, filter)
}

// GetAssessment returns a single assessment if the actor may read the tenant's data.
func (s *Service) GetAssessment(ctx context.Context, actor auth.Actor, id string) (*domain.Assessment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetAssessmentByID(ctx, tenantID, id)
}

// UpdateAssessment patches an assessment.
func (s *Service) UpdateAssessment(ctx context.Context, actor auth.Actor, id string, req UpdateAssessmentRequest) (*domain.Assessment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	prevStatus := assessment.Status
	changed, err := assessment.ApplyUpdate(req.Title, req.Description, req.Type, req.MaxScore, req.DueDate, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return assessment, nil
	}
	if err := assessment.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAssessment(ctx, tenantID, assessment); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the assessment is updated.
	_ = s.pub.PublishAssessment(ctx, "assessment.updated.v1", assessment, map[string]any{"changed_fields": changed})
	if prevStatus != string(domain.StatusPublished) && assessment.Status == string(domain.StatusPublished) {
		//nolint:errcheck // Event publishing is best-effort after the assessment is updated.
		_ = s.pub.PublishAssessment(ctx, "assessment.published.v1", assessment, nil)
	}
	return assessment, nil
}

// DeleteAssessment removes an assessment.
func (s *Service) DeleteAssessment(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteAssessment(ctx, tenantID, id); err != nil {
		return err
	}
	//nolint:errcheck // Event publishing is best-effort after the assessment is deleted.
	_ = s.pub.PublishAssessment(ctx, "assessment.deleted.v1", assessment, nil)
	return nil
}

// --- Score requests. ---

// CreateScoreRequest is the input for recording a score.
type CreateScoreRequest struct {
	AssessmentID string
	StudentID    string
	Score        int
	RecordedBy   string
	Notes        string
}

// UpdateScoreRequest is the input for patching a score.
type UpdateScoreRequest struct {
	Score *int
	Notes *string
}

// CreateScore validates and persists a new Score for the actor's tenant.
func (s *Service) CreateScore(ctx context.Context, actor auth.Actor, req CreateScoreRequest) (*domain.Score, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRecordScores)
	if err != nil {
		return nil, err
	}
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, req.AssessmentID)
	if err != nil {
		return nil, err
	}
	score, err := domain.NewScore(tenantID, req.AssessmentID, req.StudentID, req.Score, req.RecordedBy, req.Notes, assessment.MaxScore)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateScore(ctx, tenantID, score); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the score is recorded.
	_ = s.pub.PublishScore(ctx, "assessment.score_recorded.v1", score, map[string]any{"assessment_id": assessment.ID})
	return score, nil
}

// ListScores returns a tenant-scoped page of scores for an assessment.
func (s *Service) ListScores(ctx context.Context, actor auth.Actor, assessmentID string, filter ports.ScoreListFilter) ([]*domain.Score, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListScores(ctx, tenantID, assessmentID, filter)
}

// GetScore returns a single score if the actor may read the tenant's data.
func (s *Service) GetScore(ctx context.Context, actor auth.Actor, assessmentID, scoreID string) (*domain.Score, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetScoreByID(ctx, tenantID, assessmentID, scoreID)
}

// UpdateScore patches a score.
func (s *Service) UpdateScore(ctx context.Context, actor auth.Actor, assessmentID, scoreID string, req UpdateScoreRequest) (*domain.Score, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRecordScores)
	if err != nil {
		return nil, err
	}
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, assessmentID)
	if err != nil {
		return nil, err
	}
	score, err := s.repo.GetScoreByID(ctx, tenantID, assessmentID, scoreID)
	if err != nil {
		return nil, err
	}
	changed, err := score.ApplyUpdate(req.Score, req.Notes, assessment.MaxScore)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return score, nil
	}
	if err := score.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateScore(ctx, tenantID, score); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the score is updated.
	_ = s.pub.PublishScore(ctx, "assessment.score_updated.v1", score, map[string]any{"assessment_id": assessment.ID, "changed_fields": changed})
	return score, nil
}

// DeleteScore removes a score.
func (s *Service) DeleteScore(ctx context.Context, actor auth.Actor, assessmentID, scoreID string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermRecordScores)
	if err != nil {
		return err
	}
	score, err := s.repo.GetScoreByID(ctx, tenantID, assessmentID, scoreID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteScore(ctx, tenantID, assessmentID, scoreID); err != nil {
		return err
	}
	//nolint:errcheck // Event publishing is best-effort after the score is deleted.
	_ = s.pub.PublishScore(ctx, "assessment.score_deleted.v1", score, map[string]any{"assessment_id": assessmentID})
	return nil
}

func (s *Service) requireAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	return s.requireFeature(ctx, actor, perm, FeatureAssessments)
}

// requireFeature enforces authentication, tenant scope, RBAC and the given
// feature flag for the actor's tenant.
func (s *Service) requireFeature(ctx context.Context, actor auth.Actor, perm, feature string) (string, error) {
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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, feature) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, feature)
	}
	return tenantID, nil
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
