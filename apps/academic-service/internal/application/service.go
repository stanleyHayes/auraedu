package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead   = "academic.read"
	PermCreate = "academic.create"
	PermUpdate = "academic.update"
	PermDelete = "academic.delete"
)

// Feature flag key for academic management.
const FeatureAcademicManagement = "academic_management"

// Service holds the academic use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
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

func (noopPublisher) Publish(context.Context, string, *domain.AcademicYear, map[string]any) error {
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

// CreateAcademicYearRequest is the input for creating an academic year.
type CreateAcademicYearRequest struct {
	Name      string
	Code      *string
	StartDate string
	EndDate   string
	IsCurrent bool
}

// UpdateAcademicYearRequest is the input for patching an academic year.
type UpdateAcademicYearRequest struct {
	Name      *string
	Code      *string
	StartDate *string
	EndDate   *string
	Status    *string
	IsCurrent *bool
}

// Create validates and persists a new AcademicYear for the actor's tenant.
func (s *Service) Create(ctx context.Context, actor auth.Actor, req CreateAcademicYearRequest) (*domain.AcademicYear, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	code := ""
	if req.Code != nil {
		code = *req.Code
	}
	year, err := domain.NewAcademicYear(tenantID, req.Name, code, req.StartDate, req.EndDate, req.IsCurrent)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, year); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "academic.year_created.v1", year, nil)
	return year, nil
}

// List returns a tenant-scoped page of academic years.
func (s *Service) List(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.AcademicYear, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.repo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// Get returns a single academic year if the actor may read the tenant's data.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*domain.AcademicYear, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// Update patches an academic year record.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, req UpdateAcademicYearRequest) (*domain.AcademicYear, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	year, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := year.ApplyUpdate(req.Name, req.Code, req.StartDate, req.EndDate, req.Status, req.IsCurrent)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return year, nil
	}
	if err := year.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tenantID, year); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "academic.year_updated.v1", year, map[string]any{"changed_fields": changed})
	return year, nil
}

// Delete removes an academic year record.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermDelete)
	if err != nil {
		return err
	}
	year, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.Publish(ctx, "academic.year_deleted.v1", year, nil)
	return nil
}

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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureAcademicManagement) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureAcademicManagement)
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
