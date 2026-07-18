package application

import (
	"context"
	"fmt"

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
