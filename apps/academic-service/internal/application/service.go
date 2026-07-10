package application

import (
	"context"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
)

// Service holds the academic use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo ports.Repository
}

func NewService(repo ports.Repository) *Service { return &Service{repo: repo} }

// Create validates and persists a new Academic for the given tenant.
func (s *Service) Create(ctx context.Context, tenantID, id, name string) (*domain.Academic, error) {
	e, err := domain.NewAcademic(id, tenantID, name)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, e); err != nil {
		return nil, err
	}
	return e, nil
}
