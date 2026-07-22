// Package application implements the staff service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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

// FeatureStaffManagement is the feature flag key from contracts/features/features.yaml.
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

func (s *Service) commitLifecycle(ctx context.Context, tenantID string, staff *domain.Staff, mutation, eventType string, meta map[string]any) error {
	if repo, ok := s.repo.(ports.LifecycleRepository); ok {
		return repo.CommitStaffLifecycle(ctx, tenantID, staff, mutation, eventType, ports.StaffEventData(staff, meta))
	}
	var err error
	switch mutation {
	case ports.StaffMutationCreate:
		err = s.repo.Create(ctx, tenantID, staff)
	case ports.StaffMutationUpdate:
		err = s.repo.Update(ctx, tenantID, staff)
	case ports.StaffMutationDelete:
		err = s.repo.Delete(ctx, tenantID, staff.ID)
	}
	if err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, eventType, staff, meta); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish staff lifecycle event", "event_type", eventType, "err", err)
	}
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
	UserID    *string
}

// UpdateStaffRequest is the input for patching a staff record.
type UpdateStaffRequest struct {
	FirstName *string
	LastName  *string
	StaffType *string
	Email     *string
	Status    *string
	UserID    *string
}

// CreateAssignmentRequest links a teacher to a class and optional subject.
type CreateAssignmentRequest struct {
	ClassID   string
	SubjectID *string
	Role      *string
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
	if req.UserID != nil {
		staff.UserID = req.UserID
	}
	if err := staff.Validate(); err != nil {
		return nil, err
	}
	if err := s.commitLifecycle(ctx, tenantID, staff, ports.StaffMutationCreate, "staff.created.v1", nil); err != nil {
		return nil, err
	}
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
	changed, err := staff.ApplyUpdate(req.FirstName, req.LastName, req.StaffType, req.Email, req.Status, req.UserID)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return staff, nil
	}
	if err := staff.Validate(); err != nil {
		return nil, err
	}
	if err := s.commitLifecycle(ctx, tenantID, staff, ports.StaffMutationUpdate, "staff.updated.v1", map[string]any{"changed_fields": changed}); err != nil {
		return nil, err
	}
	return staff, nil
}

// ResolveTeacherScope maps an identity user to an active teacher staff record.
// It is exposed only through the authenticated internal service boundary.
func (s *Service) ResolveTeacherScope(ctx context.Context, tenantID, userID string) (string, error) {
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if userID == "" {
		return "", domain.ErrValidation
	}
	staff, err := s.repo.GetByUserID(ctx, tenantID, userID)
	if err != nil {
		return "", err
	}
	if staff.StaffType != string(domain.StaffTypeTeacher) || !staff.IsActive() {
		return "", domain.ErrForbidden
	}
	return staff.ID, nil
}

// ResolveTeacherAssignments returns the explicit class and subject scope for an
// already authenticated active teacher identity.
func (s *Service) ResolveTeacherAssignments(ctx context.Context, tenantID, staffID string) ([]string, []string, error) {
	repo, ok := s.repo.(ports.AssignmentRepository)
	if !ok {
		return nil, nil, fmt.Errorf("staff: assignment repository unavailable")
	}
	classIDs, err := repo.ListAssignmentClassIDs(ctx, tenantID, staffID)
	if err != nil {
		return nil, nil, err
	}
	subjectIDs, err := repo.ListAssignmentSubjectIDs(ctx, tenantID, staffID)
	if err != nil {
		return nil, nil, err
	}
	return classIDs, subjectIDs, nil
}

// CreateAssignment persists a teacher scope assignment and its durable event.
func (s *Service) CreateAssignment(ctx context.Context, actor auth.Actor, staffID string, req CreateAssignmentRequest) (*domain.Assignment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	staff, err := s.repo.GetByID(ctx, tenantID, staffID)
	if err != nil {
		return nil, err
	}
	if staff.StaffType != string(domain.StaffTypeTeacher) || !staff.IsActive() {
		return nil, domain.ErrForbidden
	}
	repo, ok := s.repo.(ports.AssignmentRepository)
	if !ok {
		return nil, fmt.Errorf("staff: assignment repository unavailable")
	}
	assignment, err := domain.NewAssignment(tenantID, staffID, req.ClassID, req.SubjectID, req.Role)
	if err != nil {
		return nil, err
	}
	if err := repo.CreateAssignment(ctx, tenantID, assignment, ports.AssignmentEventData(assignment)); err != nil {
		return nil, err
	}
	return assignment, nil
}

// ListAssignments returns assignment scope for one tenant-owned staff record.
func (s *Service) ListAssignments(ctx context.Context, actor auth.Actor, staffID string, limit int, cursor string) ([]*domain.Assignment, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	if _, err := s.repo.GetByID(ctx, tenantID, staffID); err != nil {
		return nil, "", err
	}
	repo, ok := s.repo.(ports.AssignmentRepository)
	if !ok {
		return nil, "", fmt.Errorf("staff: assignment repository unavailable")
	}
	return repo.ListAssignments(ctx, tenantID, staffID, normalizeLimit(limit), cursor)
}

// DeleteAssignment removes one assignment owned by the selected staff member.
func (s *Service) DeleteAssignment(ctx context.Context, actor auth.Actor, staffID, assignmentID string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return err
	}
	repo, ok := s.repo.(ports.AssignmentRepository)
	if !ok {
		return fmt.Errorf("staff: assignment repository unavailable")
	}
	return repo.DeleteAssignment(ctx, tenantID, staffID, assignmentID)
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
	return s.commitLifecycle(ctx, tenantID, staff, ports.StaffMutationDelete, "staff.deleted.v1", nil)
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
