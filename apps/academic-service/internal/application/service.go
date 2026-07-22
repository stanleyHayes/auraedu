// Package application holds the academic-service use cases. Tenant scope, RBAC,
// feature-flag checks and event publishing belong here (agent_plan §5).
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
const FeatureTimetable = "timetable"

// Service holds the academic use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	yearRepo    ports.AcademicYearRepository
	termRepo    ports.TermRepository
	classRepo   ports.ClassRepository
	subjectRepo ports.SubjectRepository
	gradingRepo ports.GradingScaleRepository
	pub         ports.EventPublisher
	gates       flags.Gate
	teachers    ports.TeacherIdentityResolver
	timetable   ports.TimetableRepository
	learners    ports.LearnerScopeResolver
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithTeacherIdentityResolver configures the private staff identity boundary.
func WithTeacherIdentityResolver(r ports.TeacherIdentityResolver) Option {
	return func(s *Service) { s.teachers = r }
}

func WithTimetableRepository(r ports.TimetableRepository) Option {
	return func(s *Service) { s.timetable = r }
}
func WithLearnerScopeResolver(r ports.LearnerScopeResolver) Option {
	return func(s *Service) { s.learners = r }
}

// WithGradingScaleRepository enables tenant grading-policy use cases.
func WithGradingScaleRepository(r ports.GradingScaleRepository) Option {
	return func(s *Service) { s.gradingRepo = r }
}

type noopPublisher struct{}

func (noopPublisher) PublishYear(context.Context, string, *domain.AcademicYear, map[string]any) error {
	return nil
}

// ResolveTeacherClassScope returns the classes assigned to an active teacher
// identity. It fails closed when staff-service is unavailable or unconfigured.
func (s *Service) ResolveTeacherClassScope(ctx context.Context, tenantID, userID string) ([]string, error) {
	if tenantID == "" {
		return nil, domain.ErrMissingTenant
	}
	if userID == "" {
		return nil, domain.ErrValidation
	}
	if s.teachers == nil {
		return nil, domain.ErrUnavailable
	}
	var explicit []string
	var staffID string
	var err error
	if resolver, ok := s.teachers.(ports.TeacherAssignmentResolver); ok {
		staffID, explicit, err = resolver.ResolveTeacherAssignments(ctx, tenantID, userID)
	} else {
		staffID, err = s.teachers.ResolveTeacher(ctx, tenantID, userID)
	}
	if err != nil {
		return nil, err
	}
	owned, err := s.classRepo.ListIDsByTeacher(ctx, tenantID, staffID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(owned)+len(explicit))
	result := make([]string, 0, len(owned)+len(explicit))
	for _, id := range append(owned, explicit...) {
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
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

func commitAcademicLifecycle(
	ctx context.Context,
	repo any,
	tenantID string,
	mutation ports.AcademicMutation,
	eventType string,
	payload map[string]any,
	fallback func() error,
	publish func() error,
) error {
	if lifecycle, ok := repo.(ports.LifecycleRepository); ok {
		return lifecycle.CommitAcademicLifecycle(ctx, tenantID, mutation, eventType, payload)
	}
	if err := fallback(); err != nil {
		return err
	}
	return publish()
}

func (s *Service) commitYearLifecycle(
	ctx context.Context,
	tenantID, kind, eventType string,
	year *domain.AcademicYear,
	meta map[string]any,
	fallback func() error,
) error {
	mutation := ports.AcademicMutation{Kind: kind, Year: year}
	return commitAcademicLifecycle(
		ctx, s.yearRepo, tenantID, mutation, eventType, ports.YearEventData(year, meta), fallback,
		func() error { return s.pub.PublishYear(ctx, eventType, year, meta) },
	)
}

func (s *Service) commitTermLifecycle(
	ctx context.Context,
	tenantID, kind, eventType string,
	term *domain.Term,
	meta map[string]any,
	fallback func() error,
) error {
	mutation := ports.AcademicMutation{Kind: kind, Term: term}
	return commitAcademicLifecycle(
		ctx, s.termRepo, tenantID, mutation, eventType, ports.TermEventData(term, meta), fallback,
		func() error { return s.pub.PublishTerm(ctx, eventType, term, meta) },
	)
}

func (s *Service) commitClassLifecycle(
	ctx context.Context,
	tenantID, kind, eventType string,
	class *domain.Class,
	meta map[string]any,
	fallback func() error,
) error {
	mutation := ports.AcademicMutation{Kind: kind, Class: class}
	return commitAcademicLifecycle(
		ctx, s.classRepo, tenantID, mutation, eventType, ports.ClassEventData(class, meta), fallback,
		func() error { return s.pub.PublishClass(ctx, eventType, class, meta) },
	)
}

func (s *Service) commitSubjectLifecycle(
	ctx context.Context,
	tenantID, kind, eventType string,
	subject *domain.Subject,
	meta map[string]any,
	fallback func() error,
) error {
	mutation := ports.AcademicMutation{Kind: kind, Subject: subject}
	return commitAcademicLifecycle(
		ctx, s.subjectRepo, tenantID, mutation, eventType, ports.SubjectEventData(subject, meta), fallback,
		func() error { return s.pub.PublishSubject(ctx, eventType, subject, meta) },
	)
}

// CreateGradingScaleRequest defines a new tenant grading policy.
type CreateGradingScaleRequest struct {
	Name   string
	Ranges []domain.GradeRange
}

// UpdateGradingScaleRequest patches a tenant grading policy.
type UpdateGradingScaleRequest struct {
	Name   *string
	Ranges *[]domain.GradeRange
}

func (s *Service) CreateGradingScale(ctx context.Context, actor auth.Actor, req CreateGradingScaleRequest) (*domain.GradingScale, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if s.gradingRepo == nil {
		return nil, domain.ErrUnavailable
	}
	scale, err := domain.NewGradingScale(tenantID, req.Name, req.Ranges)
	if err != nil {
		return nil, err
	}
	if err := s.gradingRepo.Create(ctx, tenantID, scale); err != nil {
		return nil, err
	}
	return scale, nil
}

func (s *Service) ListGradingScales(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.GradingScale, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	if s.gradingRepo == nil {
		return nil, "", domain.ErrUnavailable
	}
	return s.gradingRepo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

func (s *Service) GetGradingScale(ctx context.Context, actor auth.Actor, id string) (*domain.GradingScale, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if s.gradingRepo == nil {
		return nil, domain.ErrUnavailable
	}
	return s.gradingRepo.GetByID(ctx, tenantID, id)
}

func (s *Service) UpdateGradingScale(ctx context.Context, actor auth.Actor, id string, req UpdateGradingScaleRequest) (*domain.GradingScale, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if s.gradingRepo == nil {
		return nil, domain.ErrUnavailable
	}
	scale, err := s.gradingRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := scale.ApplyUpdate(req.Name, req.Ranges)
	if err != nil {
		return nil, err
	}
	if len(changed) > 0 {
		if err := s.gradingRepo.Update(ctx, tenantID, scale); err != nil {
			return nil, err
		}
	}
	return scale, nil
}

func (s *Service) DeleteGradingScale(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if s.gradingRepo == nil {
		return domain.ErrUnavailable
	}
	return s.gradingRepo.Delete(ctx, tenantID, id)
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
	if err := s.commitYearLifecycle(
		ctx, tenantID, ports.AcademicMutationYearCreate, "academic.year_created.v1", year, nil,
		func() error { return s.yearRepo.Create(ctx, tenantID, year) },
	); err != nil {
		return nil, err
	}
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
	meta := map[string]any{"changed_fields": changed}
	if err := s.commitYearLifecycle(
		ctx, tenantID, ports.AcademicMutationYearUpdate, "academic.year_updated.v1", year, meta,
		func() error { return s.yearRepo.Update(ctx, tenantID, year) },
	); err != nil {
		return nil, err
	}
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
	return s.commitYearLifecycle(
		ctx, tenantID, ports.AcademicMutationYearDelete, "academic.year_deleted.v1", year, nil,
		func() error { return s.yearRepo.Delete(ctx, tenantID, id) },
	)
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
	year, err := s.academicYearForReference(ctx, tenantID, req.AcademicYearID)
	if err != nil {
		return nil, err
	}
	term, err := domain.NewTerm(tenantID, req.AcademicYearID, req.Name, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if err := validateTermWithinYear(term, year); err != nil {
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
	year, err := s.academicYearForReference(ctx, tenantID, term.AcademicYearID)
	if err != nil {
		return nil, err
	}
	if err := validateTermWithinYear(term, year); err != nil {
		return nil, err
	}
	meta := map[string]any{"changed_fields": changed}
	if err := s.commitTermLifecycle(
		ctx, tenantID, ports.AcademicMutationTermUpdate, "academic.term_updated.v1", term, meta,
		func() error { return s.termRepo.Update(ctx, tenantID, term) },
	); err != nil {
		return nil, err
	}
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
	return s.commitTermLifecycle(
		ctx, tenantID, ports.AcademicMutationTermDelete, "academic.term_deleted.v1", term, nil,
		func() error { return s.termRepo.Delete(ctx, tenantID, id) },
	)
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
	if err := s.commitClassLifecycle(
		ctx, tenantID, ports.AcademicMutationClassCreate, "academic.class_created.v1", class, nil,
		func() error { return s.classRepo.Create(ctx, tenantID, class) },
	); err != nil {
		return nil, err
	}
	return class, nil
}

// ListClasses returns a tenant-scoped page of classes.
func (s *Service) ListClasses(ctx context.Context, actor auth.Actor, limit int, cursor string) ([]*domain.Class, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	ids, scoped, err := s.resolveReadableClassScope(ctx, tenantID, actor)
	if err != nil {
		return nil, "", err
	}
	if scoped {
		return s.listScopedClasses(ctx, tenantID, ids, limit, cursor)
	}
	return s.classRepo.List(ctx, tenantID, normalizeLimit(limit), cursor)
}

func (s *Service) listScopedClasses(
	ctx context.Context,
	tenantID string,
	ids []string,
	limit int,
	cursor string,
) ([]*domain.Class, string, error) {
	start := 0
	if cursor != "" {
		for i, id := range ids {
			if id == cursor {
				start = i + 1
				break
			}
		}
	}
	end := min(start+normalizeLimit(limit), len(ids))
	classes := make([]*domain.Class, 0, end-start)
	for _, id := range ids[start:end] {
		class, err := s.classRepo.GetByID(ctx, tenantID, id)
		if err != nil {
			return nil, "", err
		}
		classes = append(classes, class)
	}
	next := ""
	if end < len(ids) && len(classes) > 0 {
		next = classes[len(classes)-1].ID
	}
	return classes, next, nil
}

// GetClass returns a single class if the actor may read the tenant's data.
func (s *Service) GetClass(ctx context.Context, actor auth.Actor, id string) (*domain.Class, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	ids, scoped, err := s.resolveReadableClassScope(ctx, tenantID, actor)
	if err != nil {
		return nil, err
	}
	if scoped {
		allowed := false
		for _, assignedID := range ids {
			if assignedID == id {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, domain.ErrNotFound
		}
	}
	return s.classRepo.GetByID(ctx, tenantID, id)
}

// resolveReadableClassScope returns authoritative class IDs for roles whose
// academic reads must be constrained to their current relationships. It fails
// closed when the relevant identity boundary is unavailable.
func (s *Service) resolveReadableClassScope(ctx context.Context, tenantID string, actor auth.Actor) ([]string, bool, error) {
	role := strings.ToLower(strings.TrimSpace(actor.Role))
	switch role {
	case "teacher":
		ids, err := s.ResolveTeacherClassScope(ctx, tenantID, actor.UserID)
		return ids, true, err
	case "student", "parent":
		if s.learners == nil {
			return nil, true, domain.ErrUnavailable
		}
		ids, err := s.learners.Resolve(ctx, tenantID, actor.UserID, role)
		return ids, true, err
	default:
		return nil, false, nil
	}
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
	meta := map[string]any{"changed_fields": changed}
	if err := s.commitClassLifecycle(
		ctx, tenantID, ports.AcademicMutationClassUpdate, "academic.class_updated.v1", class, meta,
		func() error { return s.classRepo.Update(ctx, tenantID, class) },
	); err != nil {
		return nil, err
	}
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
	return s.commitClassLifecycle(
		ctx, tenantID, ports.AcademicMutationClassDelete, "academic.class_deleted.v1", class, nil,
		func() error { return s.classRepo.Delete(ctx, tenantID, id) },
	)
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
	if err := s.commitSubjectLifecycle(
		ctx, tenantID, ports.AcademicMutationSubjectCreate, "academic.subject_created.v1", subject, nil,
		func() error { return s.subjectRepo.Create(ctx, tenantID, subject) },
	); err != nil {
		return nil, err
	}
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
	meta := map[string]any{"changed_fields": changed}
	if err := s.commitSubjectLifecycle(
		ctx, tenantID, ports.AcademicMutationSubjectUpdate, "academic.subject_updated.v1", subject, meta,
		func() error { return s.subjectRepo.Update(ctx, tenantID, subject) },
	); err != nil {
		return nil, err
	}
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
	return s.commitSubjectLifecycle(
		ctx, tenantID, ports.AcademicMutationSubjectDelete, "academic.subject_deleted.v1", subject, nil,
		func() error { return s.subjectRepo.Delete(ctx, tenantID, id) },
	)
}

// ---- timetable -------------------------------------------------------------

type CreateTimetableRequest struct {
	ClassID, TermID, SubjectID string
	TeacherID                  *string
	Weekday                    int
	StartTime, EndTime         string
	Room                       *string
}
type UpdateTimetableRequest struct {
	TeacherID                        *string
	Weekday                          *int
	StartTime, EndTime, Room, Status *string
}

func (s *Service) CreateTimetableEntry(ctx context.Context, actor auth.Actor, req CreateTimetableRequest) (*domain.TimetableEntry, error) {
	tenantID, err := s.requireTimetableAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if s.timetable == nil {
		return nil, domain.ErrUnavailable
	}
	if _, err := s.classRepo.GetByID(ctx, tenantID, req.ClassID); err != nil {
		return nil, fmt.Errorf("%w: invalid class_id", domain.ErrValidation)
	}
	if _, err := s.termRepo.GetByID(ctx, tenantID, req.TermID); err != nil {
		return nil, fmt.Errorf("%w: invalid term_id", domain.ErrValidation)
	}
	if _, err := s.subjectRepo.GetByID(ctx, tenantID, req.SubjectID); err != nil {
		return nil, fmt.Errorf("%w: invalid subject_id", domain.ErrValidation)
	}
	entry, err := domain.NewTimetableEntry(tenantID, req.ClassID, req.TermID, req.SubjectID, req.TeacherID, req.Weekday, req.StartTime, req.EndTime, req.Room)
	if err != nil {
		return nil, err
	}
	if err := s.timetable.Create(ctx, tenantID, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *Service) ListTimetable(ctx context.Context, actor auth.Actor, filter ports.TimetableFilter) ([]*domain.TimetableEntry, error) {
	tenantID, err := s.requireTimetableAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if s.timetable == nil {
		return nil, domain.ErrUnavailable
	}
	filter.Limit = normalizeLimit(filter.Limit)
	filter, err = s.scopeTimetable(ctx, tenantID, actor, filter)
	if err != nil {
		return nil, err
	}
	return s.timetable.List(ctx, tenantID, filter)
}

func (s *Service) GetTimetableEntry(ctx context.Context, actor auth.Actor, id string) (*domain.TimetableEntry, error) {
	tenantID, err := s.requireTimetableAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if s.timetable == nil {
		return nil, domain.ErrUnavailable
	}
	entry, err := s.timetable.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	filter, err := s.scopeTimetable(ctx, tenantID, actor, ports.TimetableFilter{ClassIDs: []string{entry.ClassID}})
	if err != nil || len(filter.ClassIDs) == 0 {
		return nil, domain.ErrNotFound
	}
	return entry, nil
}

func (s *Service) UpdateTimetableEntry(ctx context.Context, actor auth.Actor, id string, req UpdateTimetableRequest) (*domain.TimetableEntry, error) {
	tenantID, err := s.requireTimetableAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if s.timetable == nil {
		return nil, domain.ErrUnavailable
	}
	entry, err := s.timetable.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := entry.ApplyUpdate(req.TeacherID, req.Weekday, req.StartTime, req.EndTime, req.Room, req.Status); err != nil {
		return nil, err
	}
	if err := s.timetable.Update(ctx, tenantID, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *Service) DeleteTimetableEntry(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireTimetableAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if s.timetable == nil {
		return domain.ErrUnavailable
	}
	return s.timetable.Delete(ctx, tenantID, id)
}

func (s *Service) scopeTimetable(ctx context.Context, tenantID string, actor auth.Actor, filter ports.TimetableFilter) (ports.TimetableFilter, error) {
	role := strings.ToLower(strings.TrimSpace(actor.Role))
	var allowed []string
	var err error
	switch role {
	case "student", "parent":
		if s.learners == nil {
			return filter, domain.ErrUnavailable
		}
		allowed, err = s.learners.Resolve(ctx, tenantID, actor.UserID, role)
	case "teacher":
		allowed, err = s.ResolveTeacherClassScope(ctx, tenantID, actor.UserID)
	default:
		return filter, nil
	}
	if err != nil {
		return filter, err
	}
	if filter.ClassIDs == nil {
		filter.ClassIDs = allowed
	} else {
		filter.ClassIDs = intersectIDs(filter.ClassIDs, allowed)
	}
	filter.Status = "active"
	return filter, nil
}

func intersectIDs(requested, allowed []string) []string {
	set := map[string]struct{}{}
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	var result []string
	for _, id := range requested {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}

// ---- shared helpers ---------------------------------------------------------

// requireAcademicYear enforces that academicYearID references an academic year within
// the actor's tenant, so terms/classes can never point across tenant boundaries.
func (s *Service) requireAcademicYear(ctx context.Context, tenantID, academicYearID string) error {
	_, err := s.academicYearForReference(ctx, tenantID, academicYearID)
	return err
}

func (s *Service) academicYearForReference(ctx context.Context, tenantID, academicYearID string) (*domain.AcademicYear, error) {
	if academicYearID == "" {
		return nil, fmt.Errorf("%w: academic_year_id is required", domain.ErrValidation)
	}
	year, err := s.yearRepo.GetByID(ctx, tenantID, academicYearID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: academic_year_id does not reference an academic year in this tenant", domain.ErrValidation)
		}
		return nil, err
	}
	return year, nil
}

func validateTermWithinYear(term *domain.Term, year *domain.AcademicYear) error {
	if term.StartDate.Before(year.StartDate.Time) || term.EndDate.After(year.EndDate.Time) {
		return fmt.Errorf("%w: term dates must fall within academic year %s to %s", domain.ErrValidation, year.StartDate.String(), year.EndDate.String())
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

func (s *Service) requireTimetableAccess(ctx context.Context, actor auth.Actor, perm string) (string, error) {
	tenantID, err := s.requireAccess(ctx, actor, perm)
	if err != nil {
		return "", err
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureTimetable) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureTimetable)
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
