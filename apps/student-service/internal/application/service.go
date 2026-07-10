package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
)

// RBAC permission keys (contracts/permissions/permissions.yaml).
const (
	PermRead   = "students.read"
	PermCreate = "students.create"
	PermUpdate = "students.update"
	PermDelete = "students.delete"
)

// Feature flag key from contracts/features/features.yaml.
const FeatureStudentManagement = "student_management"

// Service holds the student use cases. Tenant scope + RBAC + feature-flag checks belong
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

func (noopPublisher) Publish(context.Context, string, *domain.Student, map[string]any) error {
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

// CreateStudentRequest is the input for creating a student.
type CreateStudentRequest struct {
	FirstName   string
	LastName    string
	DateOfBirth *string
	Gender      *string
}

// UpdateStudentRequest is the input for patching a student.
type UpdateStudentRequest struct {
	FirstName *string
	LastName  *string
	Status    *string
}

// Create validates and persists a new Student for the actor's tenant.
func (s *Service) Create(ctx context.Context, actor auth.Actor, req CreateStudentRequest) (*domain.Student, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	student, err := domain.NewStudent(tenantID, req.FirstName, req.LastName)
	if err != nil {
		return nil, err
	}
	if req.DateOfBirth != nil {
		student.DateOfBirth = req.DateOfBirth
	}
	if req.Gender != nil {
		student.Gender = req.Gender
	}
	if err := student.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, student); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "student.created.v1", student, nil)
	return student, nil
}

// List returns a tenant-scoped page of students.
func (s *Service) List(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.Student, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.repo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// Get returns a single student if the actor may read the tenant's data.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*domain.Student, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// Update patches a student record.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, req UpdateStudentRequest) (*domain.Student, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	student, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := student.ApplyUpdate(req.FirstName, req.LastName, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return student, nil
	}
	if err := student.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tenantID, student); err != nil {
		return nil, err
	}
	_ = s.pub.Publish(ctx, "student.updated.v1", student, map[string]any{"changed_fields": changed})
	return student, nil
}

// Delete removes a student record.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermDelete)
	if err != nil {
		return err
	}
	student, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.Publish(ctx, "student.deleted.v1", student, nil)
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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureStudentManagement) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureStudentManagement)
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
