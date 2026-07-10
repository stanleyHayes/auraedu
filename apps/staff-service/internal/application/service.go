package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/staff-service/internal/domain"
	"github.com/auraedu/staff-service/internal/ports"
)

// RBAC permission keys (contracts/permissions/permissions.yaml).
const (
	PermRead   = "staff.read"
	PermCreate = "staff.create"
	PermUpdate = "staff.update"
	PermDelete = "staff.delete"
)

// Feature flag key from contracts/features/features.yaml.
const FeatureStaffManagement = "staff_management"

// Service holds the staff use cases. Tenant scope + RBAC + feature-flag checks belong
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

func (noopPublisher) Publish(context.Context, string, *domain.Staff, map[string]any) error {
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

// CreateStaffRequest is the input for creating a staff record.
type CreateStaffRequest struct {
	FirstName string
	LastName  string
	StaffType string
	Email     *string
}

// UpdateStaffRequest is the input for patching a staff record.
type UpdateStaffRequest struct {
	FirstName *string
	LastName  *string
	StaffType *string
	Email     *string
	Status    *string
}

// Create validates and persists a new Staff for the actor's tenant.
func (s *Service) Create(ctx context.Context, actor auth.Actor, req CreateStaffRequest) (*domain.Staff, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	staff, err := domain.NewStaff(tenantID, req.FirstName, req.LastName, req.StaffType)
	if err != nil {
		return nil, err
	}
	if req.Email != nil {
		staff.Email = req.Email
	}
	if err := staff.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, staff); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "staff.created.v1", staff, nil)
	return staff, nil
}

// List returns a tenant-scoped page of staff records.
func (s *Service) List(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.Staff, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.repo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// Get returns a single staff record if the actor may read the tenant's data.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*domain.Staff, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// Update patches a staff record.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, req UpdateStaffRequest) (*domain.Staff, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	staff, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := staff.ApplyUpdate(req.FirstName, req.LastName, req.StaffType, req.Email, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return staff, nil
	}
	if err := staff.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tenantID, staff); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "staff.updated.v1", staff, map[string]any{"changed_fields": changed})
	return staff, nil
}

// Delete removes a staff record.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermDelete)
	if err != nil {
		return err
	}
	staff, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.Publish(ctx, "staff.deleted.v1", staff, nil)
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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureStaffManagement) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureStaffManagement)
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
