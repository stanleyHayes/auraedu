package application

import (
	"context"
	"strings"
	"time"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

// Assignment use cases (AURA-14.9). Assignments are assessments with
// type='assignment'; every use case is gated on the `assignments` feature flag.

// CreateAssignmentRequest is the input for creating an assignment.
type CreateAssignmentRequest struct {
	AcademicYearID string
	SubjectID      string
	Title          string
	Instructions   string
	MaxScore       int
	DueDate        *time.Time
	ClassIDs       []string
}

// UpdateAssignmentRequest is the input for patching an assignment.
type UpdateAssignmentRequest struct {
	Title        *string
	Instructions *string
	MaxScore     *int
	DueDate      *time.Time
	ClassIDs     []string
}

// CreateAssignment validates and persists a new assignment for the actor's tenant.
func (s *Service) CreateAssignment(ctx context.Context, actor auth.Actor, req CreateAssignmentRequest) (*domain.Assignment, error) {
	tenantID, err := s.requireFeature(ctx, actor, PermManage, FeatureAssignments)
	if err != nil {
		return nil, err
	}
	assignment, err := domain.NewAssignment(tenantID, req.AcademicYearID, req.SubjectID, req.Title, req.Instructions, req.MaxScore, req.DueDate, req.ClassIDs)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateAssessment(ctx, tenantID, assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

// ListAssignments returns a tenant-scoped page of assignments, optionally filtered.
func (s *Service) ListAssignments(ctx context.Context, actor auth.Actor, filter ports.AssignmentListFilter) ([]*domain.Assessment, string, error) {
	tenantID, err := s.requireFeature(ctx, actor, PermRead, FeatureAssignments)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	if scopedRole(actor.Role) {
		if s.scope == nil {
			return nil, "", domain.ErrUnavailable
		}
		scope, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, strings.ToLower(strings.TrimSpace(actor.Role)))
		if err != nil {
			return nil, "", err
		}
		filter.ClassIDs = scope.ClassIDs
		if strings.EqualFold(strings.TrimSpace(actor.Role), "student") || strings.EqualFold(strings.TrimSpace(actor.Role), "parent") {
			filter.Status = string(domain.StatusPublished)
		}
	}
	return s.repo.ListAssignments(ctx, tenantID, filter)
}

// GetAssignment returns a single assignment if the actor may read the tenant's data.
func (s *Service) GetAssignment(ctx context.Context, actor auth.Actor, id string) (*domain.Assignment, error) {
	tenantID, err := s.requireFeature(ctx, actor, PermRead, FeatureAssignments)
	if err != nil {
		return nil, err
	}
	assignment, err := s.getAssignment(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if scopedRole(actor.Role) { //nolint:nestif // Role and class scope must be evaluated together.
		if s.scope == nil {
			return nil, domain.ErrUnavailable
		}
		scope, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, strings.ToLower(strings.TrimSpace(actor.Role)))
		if err != nil {
			return nil, err
		}
		allowed := false
		for _, assigned := range scope.ClassIDs {
			for _, target := range assignment.ClassIDs {
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
		if (strings.EqualFold(actor.Role, "student") || strings.EqualFold(actor.Role, "parent")) && assignment.Status != string(domain.StatusPublished) {
			return nil, domain.ErrNotFound
		}
	}
	return assignment, nil
}

// UpdateAssignment patches an assignment.
func (s *Service) UpdateAssignment(ctx context.Context, actor auth.Actor, id string, req UpdateAssignmentRequest) (*domain.Assignment, error) {
	tenantID, err := s.requireFeature(ctx, actor, PermManage, FeatureAssignments)
	if err != nil {
		return nil, err
	}
	assignment, err := s.getAssignment(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := assignment.ApplyAssignmentUpdate(req.Title, req.Instructions, req.MaxScore, req.DueDate, req.ClassIDs)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return assignment, nil
	}
	if err := assignment.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAssessment(ctx, tenantID, assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

// DeleteAssignment removes an assignment.
func (s *Service) DeleteAssignment(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireFeature(ctx, actor, PermManage, FeatureAssignments)
	if err != nil {
		return err
	}
	if _, err := s.getAssignment(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.DeleteAssessment(ctx, tenantID, id)
}

// PublishAssignment transitions a draft assignment to published and emits
// assignment.published.v1 (contract contracts/events/assignment.published.v1.json).
func (s *Service) PublishAssignment(ctx context.Context, actor auth.Actor, id string) (*domain.Assignment, error) {
	tenantID, err := s.requireFeature(ctx, actor, PermManage, FeatureAssignments)
	if err != nil {
		return nil, err
	}
	assignment, err := s.getAssignment(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := assignment.Publish(); err != nil {
		return nil, err
	}
	events := []ports.LifecycleEvent{{EventType: "assignment.published.v1", Payload: ports.AssignmentEventData(assignment, nil)}}
	if err := s.commitLifecycle(
		ctx,
		tenantID,
		ports.LifecycleMutation{Kind: ports.AssessmentMutationUpdate, Assessment: assignment},
		events,
		func() error { return s.repo.UpdateAssessment(ctx, tenantID, assignment) },
		func() error { return s.pub.PublishAssignment(ctx, "assignment.published.v1", assignment, nil) },
	); err != nil {
		return nil, err
	}
	return assignment, nil
}

// getAssignment fetches an assessment and enforces that it is an assignment.
func (s *Service) getAssignment(ctx context.Context, tenantID, id string) (*domain.Assessment, error) {
	assessment, err := s.repo.GetAssessmentByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if !assessment.IsAssignment() {
		return nil, domain.ErrNotFound
	}
	return assessment, nil
}
