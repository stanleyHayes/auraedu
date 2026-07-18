// Package application holds the academic-service use cases. Tenant scope, RBAC,
// feature-flag checks and event publishing belong here (agent_plan §5).
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

// RBAC permission keys (contracts/permissions/permissions.yaml).
const (
	PermRead   = "academic.read"
	PermManage = "academic.manage"
)

// FeatureAcademicManagement is the feature flag key for academic management.
const FeatureAcademicManagement = "academic_management"

// Service holds the academic use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	yearRepo    ports.AcademicYearRepository
	termRepo    ports.TermRepository
	classRepo   ports.ClassRepository
	subjectRepo ports.SubjectRepository
	pub         ports.EventPublisher
	gates       flags.Gate
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

type noopPublisher struct{}

func (noopPublisher) PublishYear(context.Context, string, *domain.AcademicYear, map[string]any) error {
	return nil
}

func (noopPublisher) PublishTerm(context.Context, string, *domain.Term, map[string]any) error {
	return nil
}

func (noopPublisher) PublishClass(context.Context, string, *domain.Class, map[string]any) error {
	return nil
}

func (noopPublisher) PublishSubject(context.Context, string, *domain.Subject, map[string]any) error {
	return nil
}

// NewService constructs the application service.
func NewService(
	yearRepo ports.AcademicYearRepository,
	termRepo ports.TermRepository,
	classRepo ports.ClassRepository,
	subjectRepo ports.SubjectRepository,
	opts ...Option,
) *Service {
	s := &Service{
		yearRepo:    yearRepo,
		termRepo:    termRepo,
		classRepo:   classRepo,
		subjectRepo: subjectRepo,
		pub:         noopPublisher{},
		gates:       flags.NewStaticSnapshot(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ---- Academic years ---------------------------------------------------------

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

// CreateAcademicYear validates and persists a new AcademicYear for the actor's tenant.
func (s *Service) CreateAcademicYear(ctx context.Context, actor auth.Actor, req CreateAcademicYearRequest) (*domain.AcademicYear, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
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
	if err := s.yearRepo.Create(ctx, tenantID, year); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the academic year is persisted.
	_ = s.pub.PublishYear(ctx, "academic.year_created.v1", year, nil)
	return year, nil
}

// ListAcademicYears returns a tenant-scoped page of academic years.
func (s *Service) ListAcademicYears(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.AcademicYear, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.yearRepo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// GetAcademicYear returns a single academic year if the actor may read the tenant's data.
func (s *Service) GetAcademicYear(ctx context.Context, actor auth.Actor, id string) (*domain.AcademicYear, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.yearRepo.GetByID(ctx, tenantID, id)
}

// UpdateAcademicYear patches an academic year record.
func (s *Service) UpdateAcademicYear(ctx context.Context, actor auth.Actor, id string, req UpdateAcademicYearRequest) (*domain.AcademicYear, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	year, err := s.yearRepo.GetByID(ctx, tenantID, id)
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
	if err := s.yearRepo.Update(ctx, tenantID, year); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the academic year is updated.
	_ = s.pub.PublishYear(ctx, "academic.year_updated.v1", year, map[string]any{"changed_fields": changed})
	return year, nil
}

// DeleteAcademicYear removes an academic year record.
func (s *Service) DeleteAcademicYear(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	year, err := s.yearRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.yearRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	//nolint:errcheck // Event publishing is best-effort after the academic year is deleted.
	_ = s.pub.PublishYear(ctx, "academic.year_deleted.v1", year, nil)
	return nil
}

// ---- Terms ------------------------------------------------------------------

// CreateTermRequest is the input for creating a term.
type CreateTermRequest struct {
	AcademicYearID string
	Name           string
	StartDate      string
	EndDate        string
}

// UpdateTermRequest is the input for patching a term.
type UpdateTermRequest struct {
	Name      *string
	StartDate *string
	EndDate   *string
}

// CreateTerm validates and persists a new Term within an academic year of the actor's tenant.
func (s *Service) CreateTerm(ctx context.Context, actor auth.Actor, req CreateTermRequest) (*domain.Term, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if err := s.requireAcademicYear(ctx, tenantID, req.AcademicYearID); err != nil {
		return nil, err
	}
	term, err := domain.NewTerm(tenantID, req.AcademicYearID, req.Name, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if err := s.termRepo.Create(ctx, tenantID, term); err != nil {
		return nil, err
	}
	// NOTE: no academic.term_created event contract exists under contracts/events/, so
	// none is emitted (created events are emitted only where a contract exists; term
	// updated/deleted follow the uncontracted years precedent).
	return term, nil
}

// ListTerms returns a tenant-scoped page of terms.
func (s *Service) ListTerms(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.Term, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.termRepo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// GetTerm returns a single term if the actor may read the tenant's data.
func (s *Service) GetTerm(ctx context.Context, actor auth.Actor, id string) (*domain.Term, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.termRepo.GetByID(ctx, tenantID, id)
}

// UpdateTerm patches a term record.
func (s *Service) UpdateTerm(ctx context.Context, actor auth.Actor, id string, req UpdateTermRequest) (*domain.Term, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	term, err := s.termRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := term.ApplyUpdate(req.Name, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return term, nil
	}
	if err := term.Validate(); err != nil {
		return nil, err
	}
	if err := s.termRepo.Update(ctx, tenantID, term); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the term is updated.
	_ = s.pub.PublishTerm(ctx, "academic.term_updated.v1", term, map[string]any{"changed_fields": changed})
	return term, nil
}

// DeleteTerm removes a term record.
func (s *Service) DeleteTerm(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	term, err := s.termRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.termRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	//nolint:errcheck // Event publishing is best-effort after the term is deleted.
	_ = s.pub.PublishTerm(ctx, "academic.term_deleted.v1", term, nil)
	return nil
}

// ---- Classes ----------------------------------------------------------------

// CreateClassRequest is the input for creating a class.
type CreateClassRequest struct {
	Name           string
	AcademicYearID string
	ClassTeacherID *string
	Capacity       *int
}

// UpdateClassRequest is the input for patching a class.
type UpdateClassRequest struct {
	Name           *string
	ClassTeacherID *string
	Capacity       *int
}

// CreateClass validates and persists a new Class within an academic year of the actor's tenant.
func (s *Service) CreateClass(ctx context.Context, actor auth.Actor, req CreateClassRequest) (*domain.Class, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if err := s.requireAcademicYear(ctx, tenantID, req.AcademicYearID); err != nil {
		return nil, err
	}
	class, err := domain.NewClass(tenantID, req.AcademicYearID, req.Name, req.ClassTeacherID, req.Capacity)
	if err != nil {
		return nil, err
	}
	if err := s.classRepo.Create(ctx, tenantID, class); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the class is persisted.
	_ = s.pub.PublishClass(ctx, "academic.class_created.v1", class, nil)
	return class, nil
}

// ListClasses returns a tenant-scoped page of classes.
func (s *Service) ListClasses(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.Class, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.classRepo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// GetClass returns a single class if the actor may read the tenant's data.
func (s *Service) GetClass(ctx context.Context, actor auth.Actor, id string) (*domain.Class, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.classRepo.GetByID(ctx, tenantID, id)
}

// UpdateClass patches a class record.
func (s *Service) UpdateClass(ctx context.Context, actor auth.Actor, id string, req UpdateClassRequest) (*domain.Class, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	class, err := s.classRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := class.ApplyUpdate(req.Name, req.ClassTeacherID, req.Capacity)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return class, nil
	}
	if err := class.Validate(); err != nil {
		return nil, err
	}
	if err := s.classRepo.Update(ctx, tenantID, class); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the class is updated.
	_ = s.pub.PublishClass(ctx, "academic.class_updated.v1", class, map[string]any{"changed_fields": changed})
	return class, nil
}

// DeleteClass removes a class record.
func (s *Service) DeleteClass(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	class, err := s.classRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.classRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	//nolint:errcheck // Event publishing is best-effort after the class is deleted.
	_ = s.pub.PublishClass(ctx, "academic.class_deleted.v1", class, nil)
	return nil
}

// ---- Subjects ---------------------------------------------------------------

// CreateSubjectRequest is the input for creating a subject.
type CreateSubjectRequest struct {
	Name        string
	Code        *string
	Description *string
}

// UpdateSubjectRequest is the input for patching a subject.
type UpdateSubjectRequest struct {
	Name        *string
	Code        *string
	Description *string
}

// CreateSubject validates and persists a new Subject for the actor's tenant.
func (s *Service) CreateSubject(ctx context.Context, actor auth.Actor, req CreateSubjectRequest) (*domain.Subject, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	subject, err := domain.NewSubject(tenantID, req.Name, req.Code, req.Description)
	if err != nil {
		return nil, err
	}
	if err := s.subjectRepo.Create(ctx, tenantID, subject); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the subject is persisted.
	_ = s.pub.PublishSubject(ctx, "academic.subject_created.v1", subject, nil)
	return subject, nil
}

// ListSubjects returns a tenant-scoped page of subjects.
func (s *Service) ListSubjects(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.Subject, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.subjectRepo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

// GetSubject returns a single subject if the actor may read the tenant's data.
func (s *Service) GetSubject(ctx context.Context, actor auth.Actor, id string) (*domain.Subject, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.subjectRepo.GetByID(ctx, tenantID, id)
}

// UpdateSubject patches a subject record.
func (s *Service) UpdateSubject(ctx context.Context, actor auth.Actor, id string, req UpdateSubjectRequest) (*domain.Subject, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	subject, err := s.subjectRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := subject.ApplyUpdate(req.Name, req.Code, req.Description)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return subject, nil
	}
	if err := subject.Validate(); err != nil {
		return nil, err
	}
	if err := s.subjectRepo.Update(ctx, tenantID, subject); err != nil {
		return nil, err
	}
	//nolint:errcheck // Event publishing is best-effort after the subject is updated.
	_ = s.pub.PublishSubject(ctx, "academic.subject_updated.v1", subject, map[string]any{"changed_fields": changed})
	return subject, nil
}

// DeleteSubject removes a subject record.
func (s *Service) DeleteSubject(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	subject, err := s.subjectRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.subjectRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	//nolint:errcheck // Event publishing is best-effort after the subject is deleted.
	_ = s.pub.PublishSubject(ctx, "academic.subject_deleted.v1", subject, nil)
	return nil
}

// ---- shared helpers ---------------------------------------------------------

// requireAcademicYear enforces that academicYearID references an academic year within
// the actor's tenant, so terms/classes can never point across tenant boundaries.
func (s *Service) requireAcademicYear(ctx context.Context, tenantID, academicYearID string) error {
	if academicYearID == "" {
		return fmt.Errorf("%w: academic_year_id is required", domain.ErrValidation)
	}
	if _, err := s.yearRepo.GetByID(ctx, tenantID, academicYearID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: academic_year_id does not reference an academic year in this tenant", domain.ErrValidation)
		}
		return err
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
