// Package application holds the assessment-service use cases. Tenant scope, RBAC,
// feature-flag checks and event publishing belong here (agent_plan §5).
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	scope ports.LearnerScopeResolver
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }
func WithLearnerScopeResolver(r ports.LearnerScopeResolver) Option {
	return func(s *Service) { s.scope = r }
}

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

func (s *Service) commitLifecycle(
	ctx context.Context,
	tenantID string,
	mutation ports.LifecycleMutation,
	events []ports.LifecycleEvent,
	fallback func() error,
	publish func() error,
) error {
	if repo, ok := s.repo.(ports.LifecycleRepository); ok {
		return repo.CommitAssessmentLifecycle(ctx, tenantID, mutation, events)
	}
	if err := fallback(); err != nil {
		return err
	}
	return publish()
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
	events := []ports.LifecycleEvent{{EventType: "assessment.created.v1", Payload: ports.AssessmentEventData(assessment, nil)}}
	if err := s.commitLifecycle(
		ctx,
		tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationCreate, Assessment: assessment},
		events,
		func() error { return s.repo.CreateAssessment(ctx, tenantID, assessment) },
		func() error { return s.pub.PublishAssessment(ctx, "assessment.created.v1", assessment, nil) },
	); err != nil {
		return nil, err
	}
	return assessment, nil
}

// ListAssessments returns a tenant-scoped page of assessments, optionally filtered.
func (s *Service) ListAssessments(ctx context.Context, actor auth.Actor, filter ports.AssessmentListFilter) ([]*domain.Assessment, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	if scopedRole(actor.Role) {
		if s.scope == nil {
			return nil, "", domain.ErrUnavailable
		}
		role := strings.ToLower(strings.TrimSpace(actor.Role))
		scope, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, role)
		if err != nil {
			return nil, "", err
		}
		filter.ClassIDs = scope.ClassIDs
		if role == "student" || role == "parent" {
			filter.Status = string(domain.StatusPublished)
		}
	}
	return s.repo.ListAssessments(ctx, tenantID, filter)
}

// GetAssessment returns a single assessment if the actor may read the tenant's data.
func (s *Service) GetAssessment(ctx context.Context, actor auth.Actor, id string) (*domain.Assessment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if scopedRole(actor.Role) { //nolint:nestif // Role and assignment scope must be evaluated together.
		if s.scope == nil {
			return nil, domain.ErrUnavailable
		}
		role := strings.ToLower(strings.TrimSpace(actor.Role))
		scope, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, role)
		if err != nil {
			return nil, err
		}
		allowed := false
		for _, assigned := range scope.ClassIDs {
			for _, target := range assessment.ClassIDs {
				if assigned == target {
					allowed = true
					break
				}
			}
			if allowed {
				break
			}
		}
		if !allowed {
			return nil, domain.ErrNotFound
		}
		if (role == "student" || role == "parent") && assessment.Status != string(domain.StatusPublished) {
			return nil, domain.ErrNotFound
		}
	}
	return assessment, nil
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
	meta := map[string]any{"changed_fields": changed}
	events := []ports.LifecycleEvent{{EventType: "assessment.updated.v1", Payload: ports.AssessmentEventData(assessment, meta)}}
	if prevStatus != string(domain.StatusPublished) && assessment.Status == string(domain.StatusPublished) {
		events = append(events, ports.LifecycleEvent{EventType: "assessment.published.v1", Payload: ports.AssessmentEventData(assessment, nil)})
	}
	if err := s.commitLifecycle(
		ctx,
		tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationUpdate, Assessment: assessment},
		events,
		func() error { return s.repo.UpdateAssessment(ctx, tenantID, assessment) },
		func() error {
			if err := s.pub.PublishAssessment(ctx, "assessment.updated.v1", assessment, meta); err != nil {
				return err
			}
			if len(events) > 1 {
				return s.pub.PublishAssessment(ctx, "assessment.published.v1", assessment, nil)
			}
			return nil
		},
	); err != nil {
		return nil, err
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
	events := []ports.LifecycleEvent{{EventType: "assessment.deleted.v1", Payload: ports.AssessmentEventData(assessment, nil)}}
	return s.commitLifecycle(
		ctx,
		tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationDelete, Assessment: assessment},
		events,
		func() error { return s.repo.DeleteAssessment(ctx, tenantID, id) },
		func() error { return s.pub.PublishAssessment(ctx, "assessment.deleted.v1", assessment, nil) },
	)
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
	if err := s.authorizeTeacherStudent(ctx, actor, req.StudentID); err != nil {
		return nil, err
	}
	score, err := domain.NewScore(tenantID, req.AssessmentID, req.StudentID, req.Score, req.RecordedBy, req.Notes, assessment.MaxScore)
	if err != nil {
		return nil, err
	}
	meta := scoreEventMeta(assessment, score, nil)
	events := []ports.LifecycleEvent{{EventType: "assessment.score_recorded.v1", Payload: ports.ScoreEventData(score, meta)}}
	if err := s.commitLifecycle(
		ctx, tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationScoreCreate, Score: score},
		events,
		func() error { return s.repo.CreateScore(ctx, tenantID, score) },
		func() error { return s.pub.PublishScore(ctx, "assessment.score_recorded.v1", score, meta) },
	); err != nil {
		return nil, err
	}
	return score, nil
}

// ListScores returns a tenant-scoped page of scores for an assessment.
func (s *Service) ListScores(ctx context.Context, actor auth.Actor, assessmentID string, filter ports.ScoreListFilter) ([]*domain.Score, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	filter, err = s.applyScoreScope(ctx, actor, filter)
	if err != nil {
		return nil, "", err
	}
	return s.repo.ListScores(ctx, tenantID, assessmentID, filter)
}

// GetScore returns a single score if the actor may read the tenant's data.
func (s *Service) GetScore(ctx context.Context, actor auth.Actor, assessmentID, scoreID string) (*domain.Score, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	score, err := s.repo.GetScoreByID(ctx, tenantID, assessmentID, scoreID)
	if err != nil {
		return nil, err
	}
	if scopedRole(actor.Role) {
		filter, scopeErr := s.applyScoreScope(ctx, actor, ports.ScoreListFilter{StudentID: score.StudentID})
		if scopeErr != nil {
			return nil, scopeErr
		}
		if len(filter.StudentIDs) == 0 {
			return nil, domain.ErrNotFound
		}
	}
	return score, nil
}

func scopedRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "parent" || role == "student" || role == "teacher"
}
func (s *Service) applyScoreScope(ctx context.Context, actor auth.Actor, filter ports.ScoreListFilter) (ports.ScoreListFilter, error) {
	if !scopedRole(actor.Role) {
		return filter, nil
	}
	if s.scope == nil {
		return filter, domain.ErrUnavailable
	}
	scope, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, strings.ToLower(strings.TrimSpace(actor.Role)))
	if err != nil {
		return filter, err
	}
	if filter.StudentID != "" {
		for _, id := range scope.StudentIDs {
			if id == filter.StudentID {
				filter.StudentIDs = []string{id}
				filter.StudentID = ""
				return filter, nil
			}
		}
		return filter, domain.ErrNotFound
	}
	filter.StudentIDs = scope.StudentIDs
	return filter, nil
}

func (s *Service) authorizeTeacherStudent(ctx context.Context, actor auth.Actor, studentID string) error {
	if strings.ToLower(strings.TrimSpace(actor.Role)) != "teacher" {
		return nil
	}
	if s.scope == nil {
		return domain.ErrUnavailable
	}
	scope, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, "teacher")
	if err != nil {
		return err
	}
	for _, id := range scope.StudentIDs {
		if id == studentID {
			return nil
		}
	}
	return domain.ErrNotFound
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
	if err := s.authorizeTeacherStudent(ctx, actor, score.StudentID); err != nil {
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
	meta := scoreEventMeta(assessment, score, map[string]any{"changed_fields": changed})
	events := []ports.LifecycleEvent{{EventType: "assessment.score_updated.v1", Payload: ports.ScoreEventData(score, meta)}}
	if err := s.commitLifecycle(
		ctx, tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationScoreUpdate, Score: score},
		events,
		func() error { return s.repo.UpdateScore(ctx, tenantID, score) },
		func() error { return s.pub.PublishScore(ctx, "assessment.score_updated.v1", score, meta) },
	); err != nil {
		return nil, err
	}
	return score, nil
}

// scoreEventMeta carries the assessment context consumers need without
// coupling them to the Assessment Service database. Keep this payload aligned
// with contracts/events/assessment.score_recorded.v1.json.
func scoreEventMeta(assessment *domain.Assessment, score *domain.Score, extra map[string]any) map[string]any {
	meta := map[string]any{
		"assessment_id":    assessment.ID,
		"subject_id":       assessment.SubjectID,
		"academic_year_id": assessment.AcademicYearID,
		"max_score":        assessment.MaxScore,
		"recorded_at":      score.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":       score.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if len(assessment.ClassIDs) > 0 {
		meta["class_ids"] = append([]string(nil), assessment.ClassIDs...)
	}
	for key, value := range extra {
		meta[key] = value
	}
	return meta
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
	if err := s.authorizeTeacherStudent(ctx, actor, score.StudentID); err != nil {
		return err
	}
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, assessmentID)
	if err != nil {
		return err
	}
	meta := scoreEventMeta(assessment, score, map[string]any{"deleted_at": time.Now().UTC().Format(time.RFC3339)})
	events := []ports.LifecycleEvent{{EventType: "assessment.score_deleted.v1", Payload: ports.ScoreEventData(score, meta)}}
	return s.commitLifecycle(
		ctx, tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationScoreDelete, Score: score},
		events,
		func() error { return s.repo.DeleteScore(ctx, tenantID, assessmentID, scoreID) },
		func() error { return s.pub.PublishScore(ctx, "assessment.score_deleted.v1", score, meta) },
	)
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
