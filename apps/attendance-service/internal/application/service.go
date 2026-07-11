// Package application implements the attendance use cases and RBAC policy.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead = "attendance.read"
	PermMark = "attendance.mark"
)

// FeatureAttendance is the feature flag key for attendance management.
const FeatureAttendance = "attendance"

// Service holds the attendance use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
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

func (noopPublisher) Publish(context.Context, string, *domain.AttendanceRecord, map[string]any) error {
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

// CreateAttendanceRequest is the input for marking attendance.
type CreateAttendanceRequest struct {
	StudentID      string
	AcademicYearID string
	Date           string
	Status         string
	Reason         *string
	MarkedBy       string
}

// UpdateAttendanceRequest is the input for patching an attendance record.
type UpdateAttendanceRequest struct {
	Status   *string
	Reason   *string
	MarkedBy *string
}

// Create validates and persists a new AttendanceRecord for the actor's tenant.
func (s *Service) Create(ctx context.Context, actor auth.Actor, req CreateAttendanceRequest) (*domain.AttendanceRecord, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermMark)
	if err != nil {
		return nil, err
	}
	record, err := domain.NewAttendanceRecord(tenantID, req.StudentID, req.AcademicYearID, req.Date, req.Status, req.MarkedBy, req.Reason)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, record); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "attendance.marked.v1", record, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish attendance marked event", "err", err)
	}
	return record, nil
}

// List returns a tenant-scoped page of attendance records, optionally filtered.
func (s *Service) List(ctx context.Context, actor auth.Actor, filter ports.ListFilter) ([]*domain.AttendanceRecord, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.List(ctx, tenantID, filter)
}

// Get returns a single attendance record if the actor may read the tenant's data.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*domain.AttendanceRecord, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// Update patches an attendance record.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, req UpdateAttendanceRequest) (*domain.AttendanceRecord, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermMark)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := record.ApplyUpdate(req.Status, req.Reason, req.MarkedBy)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return record, nil
	}
	if err := record.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tenantID, record); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "attendance.updated.v1", record, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish attendance updated event", "err", err)
	}
	return record, nil
}

// Delete removes an attendance record.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermMark)
	if err != nil {
		return err
	}
	record, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "attendance.deleted.v1", record, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish attendance deleted event", "err", err)
	}
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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureAttendance) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureAttendance)
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
