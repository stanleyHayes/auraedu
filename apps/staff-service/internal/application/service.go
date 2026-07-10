package application

import (
	"context"

	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
)

// Service holds the staff use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo ports.Repository
}

func NewService(repo ports.Repository) *Service { return &Service{repo: repo} }

// Create validates and persists a new Staff for the given tenant.
func (s *Service) Create(ctx context.Context, tenantID, id, name string) (*domain.Staff, error) {
	e, err := domain.NewStaff(id, tenantID, name)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, e); err != nil {
		return nil, err
	}
	return e, nil
}
