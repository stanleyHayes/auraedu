package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/auraedu/assessment-service/internal/domain"
	"github.com/auraedu/assessment-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

// Gradebook use case (AURA-14.10). Read-only: aggregates recorded scores
// joined to their assessments; no persistence of its own. Gated on the
// `assessments` feature flag with the assessments.read permission.

// GetGradebook computes a per-student or per-class gradebook summary.
// At least one of StudentID or ClassID must be set on the filter.
func (s *Service) GetGradebook(ctx context.Context, actor auth.Actor, filter ports.GradebookFilter) (*domain.Gradebook, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if filter.StudentID == "" && filter.ClassID == "" {
		return nil, fmt.Errorf("%w: student_id or class_id is required", domain.ErrValidation)
	}
	if scopedRole(actor.Role) { //nolint:nestif // Learner scope requires nested authorization checks.
		if s.scope == nil {
			return nil, domain.ErrUnavailable
		}
		role := strings.ToLower(strings.TrimSpace(actor.Role))
		scope, scopeErr := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, role)
		if scopeErr != nil {
			return nil, scopeErr
		}
		if role == "student" || role == "parent" {
			if filter.ClassID != "" || !contains(scope.StudentIDs, filter.StudentID) {
				return nil, domain.ErrNotFound
			}
		} else {
			if filter.StudentID != "" && !contains(scope.StudentIDs, filter.StudentID) {
				return nil, domain.ErrNotFound
			}
			if filter.ClassID != "" && !contains(scope.ClassIDs, filter.ClassID) {
				return nil, domain.ErrNotFound
			}
		}
	}
	rows, err := s.repo.GradebookScores(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}
	book := domain.AggregateGrades(rows)
	book.StudentID = filter.StudentID
	book.ClassID = filter.ClassID
	book.AcademicYearID = filter.AcademicYearID
	return &book, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
