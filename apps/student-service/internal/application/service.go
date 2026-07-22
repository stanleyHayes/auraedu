// Package application implements the student service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/student-service/internal/domain"
	"github.com/auraedu/student-service/internal/ports"
	"github.com/google/uuid"
)

// RBAC permission keys (contracts/permissions/permissions.yaml).
const (
	PermRead   = "students.read"
	PermCreate = "students.create"
	PermUpdate = "students.update"
	PermDelete = "students.delete"
)

// FeatureStudentManagement is the feature flag key from contracts/features/features.yaml.
const FeatureStudentManagement = "student_management"

// Service holds the student use cases. Tenant scope + RBAC + feature-flag checks belong
// here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo    ports.Repository
	pub     ports.EventPublisher
	gates   flags.Gate
	classes ports.TeacherClassScopeResolver
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

func WithTeacherClassScopeResolver(r ports.TeacherClassScopeResolver) Option {
	return func(s *Service) { s.classes = r }
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, string, *domain.Student, map[string]any) error {
	return nil
}

func (s *Service) commitLifecycle(
	ctx context.Context,
	tenantID string,
	mutation ports.LifecycleMutation,
	eventType string,
	student *domain.Student,
	meta map[string]any,
) error {
	if repo, ok := s.repo.(ports.LifecycleRepository); ok {
		return repo.CommitStudentLifecycle(ctx, tenantID, mutation, eventType, ports.StudentEventData(student, meta))
	}
	var err error
	switch mutation.Kind {
	case ports.MutationStudentCreate:
		err = s.repo.Create(ctx, tenantID, mutation.Student)
	case ports.MutationStudentUpdate:
		err = s.repo.Update(ctx, tenantID, mutation.Student)
	case ports.MutationStudentDelete:
		err = s.repo.Delete(ctx, tenantID, mutation.Student.ID)
	case ports.MutationEnrollmentCreate:
		err = s.repo.CreateEnrollment(ctx, tenantID, mutation.Enrollment)
	case ports.MutationGuardianCreate:
		err = s.repo.CreateGuardian(ctx, tenantID, mutation.Guardian)
	case ports.MutationGuardianUpdate:
		err = s.repo.UpdateGuardian(ctx, tenantID, mutation.Guardian)
	case ports.MutationGuardianDelete:
		err = s.repo.DeleteGuardian(ctx, tenantID, mutation.Guardian.ID)
	case ports.MutationGuardianLink:
		err = s.repo.LinkGuardianToStudent(ctx, tenantID, mutation.Link)
	case ports.MutationGuardianUnlink:
		err = s.repo.UnlinkGuardianFromStudent(ctx, tenantID, mutation.StudentID, mutation.GuardianID)
	}
	if err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, eventType, student, meta); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish student lifecycle event", "event_type", eventType, "err", err)
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

// CreateStudentRequest is the input for creating a student.
type CreateStudentRequest struct {
	FirstName   string
	LastName    string
	DateOfBirth *string
	Gender      *string
	// ClassID and AcademicYearID are the optional class assignment. They are persisted
	// on the student record (since AURA-10.11) and also carried on the
	// student.enrolled.v1 event (contracts/events/student.enrolled.v1.json).
	ClassID        *string
	AcademicYearID *string
	// UserID optionally links the record to an identity-service user (AURA-10.12).
	UserID *string
}

// UpdateStudentRequest is the input for patching a student.
type UpdateStudentRequest struct {
	FirstName *string
	LastName  *string
	Status    *string
	UserID    *string
}

type CreateEnrollmentRequest struct {
	ClassID        string
	AcademicYearID string
}

// CreateGuardianRequest is the input for creating a guardian.
type CreateGuardianRequest struct {
	FirstName    string
	LastName     string
	Relationship string
	Phone        *string
	Email        *string
	// UserID optionally links the record to an identity-service user (AURA-10.12).
	UserID *string
}

// UpdateGuardianRequest is the input for patching a guardian.
type UpdateGuardianRequest struct {
	FirstName    *string
	LastName     *string
	Relationship *string
	Phone        *string
	Email        *string
}

// LinkGuardianRequest links a guardian to a student.
type LinkGuardianRequest struct {
	GuardianID   string
	Relationship *string
	IsPrimary    bool
}

// ImportStudentRow is one row from a bulk-import CSV/JSON payload.
type ImportStudentRow struct {
	FirstName         string
	LastName          string
	DateOfBirth       *string
	Gender            *string
	Relationship      *string
	GuardianFirstName *string
	GuardianLastName  *string
	GuardianPhone     *string
	GuardianEmail     *string
	// UserID and GuardianUserID optionally link the created records to
	// identity-service users (AURA-10.12).
	UserID         *string
	GuardianUserID *string
}

// ImportError describes a single row that failed.
type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportResult summarizes a bulk import.
type ImportResult struct {
	StudentsCreated  int           `json:"students_created"`
	GuardiansCreated int           `json:"guardians_created"`
	LinksCreated     int           `json:"links_created"`
	Errors           []ImportError `json:"errors"`
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
	student.ClassID = normalizeOptionalUUID(req.ClassID)
	student.AcademicYearID = normalizeOptionalUUID(req.AcademicYearID)
	student.UserID = normalizeOptionalUUID(req.UserID)
	if err := student.Validate(); err != nil {
		return nil, err
	}
	eventType, eventMeta := studentCreationEvent(student, req.ClassID, req.AcademicYearID)
	mutation := ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: student}
	if err := s.commitLifecycle(ctx, tenantID, mutation, eventType, student, eventMeta); err != nil {
		return nil, err
	}
	return student, nil
}

// studentCreationEvent distinguishes creating a student record from completing
// an academic-year class enrollment. Publishing student.enrolled without both
// enrollment identifiers would describe a fact that has not occurred.
func studentCreationEvent(student *domain.Student, classID, academicYearID *string) (string, map[string]any) {
	if classID == nil || academicYearID == nil || strings.TrimSpace(*classID) == "" || strings.TrimSpace(*academicYearID) == "" {
		return "student.created.v1", nil
	}
	return "student.enrolled.v1", enrolledMeta(student, classID, academicYearID)
}

// enrolledMeta builds the data payload for student.enrolled.v1. student_id is
// added by the publisher; class/year are omitted only for unassigned imports.
func enrolledMeta(student *domain.Student, classID, academicYearID *string) map[string]any {
	meta := map[string]any{"enrollment_date": student.CreatedAt.UTC().Format(time.DateOnly)}
	if classID != nil && *classID != "" {
		meta["class_id"] = *classID
	}
	if academicYearID != nil && *academicYearID != "" {
		meta["academic_year_id"] = *academicYearID
	}
	return meta
}

// CreateEnrollment records one academic-year class assignment and atomically
// updates the student's current roster projection.
func (s *Service) CreateEnrollment(ctx context.Context, actor auth.Actor, studentID string, req CreateEnrollmentRequest) (*domain.Enrollment, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	student, err := s.repo.GetByID(ctx, tenantID, studentID)
	if err != nil {
		return nil, err
	}
	enrollment, err := domain.NewEnrollment(tenantID, studentID, req.ClassID, req.AcademicYearID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	student.ClassID = &enrollment.ClassID
	student.AcademicYearID = &enrollment.AcademicYearID
	mutation := ports.LifecycleMutation{Kind: ports.MutationEnrollmentCreate, Enrollment: enrollment}
	if err := s.commitLifecycle(ctx, tenantID, mutation, "student.enrolled.v1", student, map[string]any{
		"class_id": enrollment.ClassID, "academic_year_id": enrollment.AcademicYearID,
		"enrollment_date": enrollment.EnrolledAt.Format(time.DateOnly),
	}); err != nil {
		return nil, err
	}
	return enrollment, nil
}

func (s *Service) ListEnrollments(ctx context.Context, actor auth.Actor, studentID string, limit int, cursor string) ([]*domain.Enrollment, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	// Reuse the student read path so teacher class scope is enforced.
	if _, err := s.Get(ctx, actor, studentID); err != nil {
		return nil, "", err
	}
	return s.repo.ListEnrollments(ctx, tenantID, studentID, normalizeLimit(limit), cursor)
}

// List returns a tenant-scoped page of students. When classID is non-nil the page is
// restricted to students assigned to that class (roster view, AURA-10.11); an empty
// string is treated as unset. A non-UUID classID is a validation error.
func (s *Service) List(ctx context.Context, actor auth.Actor, classID *string, limit int, cursor string) ([]*domain.Student, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	classID = normalizeOptionalUUID(classID)
	if err := validateOptionalUUID(classID); err != nil {
		return nil, "", err
	}
	if isTeacher(actor) {
		return s.listTeacherStudents(ctx, tenantID, actor.UserID, classID, limit, cursor)
	}
	return s.repo.List(ctx, tenantID, classID, normalizeLimit(limit), cursor)
}

func validateOptionalUUID(value *string) error {
	if value == nil {
		return nil
	}
	if _, err := uuid.Parse(*value); err != nil {
		return domain.ErrValidation
	}
	return nil
}

func isTeacher(actor auth.Actor) bool {
	return strings.EqualFold(strings.TrimSpace(actor.Role), "teacher")
}

func (s *Service) listTeacherStudents(
	ctx context.Context,
	tenantID string,
	userID string,
	classID *string,
	limit int,
	cursor string,
) ([]*domain.Student, string, error) {
	classIDs, err := s.teacherClassIDs(ctx, tenantID, userID)
	if err != nil {
		return nil, "", err
	}
	if classID != nil {
		if !containsString(classIDs, *classID) {
			return nil, "", domain.ErrNotFound
		}
		return s.repo.List(ctx, tenantID, classID, normalizeLimit(limit), cursor)
	}
	studentIDs, err := s.repo.ListStudentIDsByClassIDs(ctx, tenantID, classIDs)
	if err != nil {
		return nil, "", err
	}
	return s.loadStudentIDPage(ctx, tenantID, studentIDs, normalizeLimit(limit), cursor)
}

func (s *Service) teacherClassIDs(ctx context.Context, tenantID, userID string) ([]string, error) {
	if s.classes == nil {
		return nil, domain.ErrUnavailable
	}
	return s.classes.ResolveTeacherClasses(ctx, tenantID, userID)
}

func (s *Service) loadStudentIDPage(
	ctx context.Context,
	tenantID string,
	studentIDs []string,
	limit int,
	cursor string,
) ([]*domain.Student, string, error) {
	start := indexAfter(studentIDs, cursor)
	end := min(start+limit, len(studentIDs))
	students := make([]*domain.Student, 0, end-start)
	for _, id := range studentIDs[start:end] {
		student, err := s.repo.GetByID(ctx, tenantID, id)
		if err != nil {
			return nil, "", err
		}
		students = append(students, student)
	}
	if end < len(studentIDs) && len(students) > 0 {
		return students, students[len(students)-1].ID, nil
	}
	return students, "", nil
}

func indexAfter(values []string, cursor string) int {
	if cursor == "" {
		return 0
	}
	for index, value := range values {
		if value == cursor {
			return index + 1
		}
	}
	return 0
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

// Get returns a single student if the actor may read the tenant's data.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*domain.Student, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if isTeacher(actor) {
		classIDs, err := s.teacherClassIDs(ctx, tenantID, actor.UserID)
		if err != nil {
			return nil, err
		}
		studentIDs, err := s.repo.ListStudentIDsByClassIDs(ctx, tenantID, classIDs)
		if err != nil {
			return nil, err
		}
		if !containsString(studentIDs, id) {
			return nil, domain.ErrNotFound
		}
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// GetMyStudent returns the student record linked to the actor's identity user
// (AURA-10.12), or domain.ErrNotFound when the user has no linked student.
func (s *Service) GetMyStudent(ctx context.Context, actor auth.Actor) (*domain.Student, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetStudentByUserID(ctx, tenantID, actor.UserID)
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
	changed, err := student.ApplyUpdate(req.FirstName, req.LastName, req.Status, req.UserID)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return student, nil
	}
	if err := student.Validate(); err != nil {
		return nil, err
	}
	mutation := ports.LifecycleMutation{Kind: ports.MutationStudentUpdate, Student: student}
	if err := s.commitLifecycle(
		ctx, tenantID, mutation, "student.updated.v1", student, map[string]any{"changed_fields": changed},
	); err != nil {
		return nil, err
	}
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
	return s.commitLifecycle(ctx, tenantID, ports.LifecycleMutation{Kind: ports.MutationStudentDelete, Student: student}, "student.deleted.v1", student, nil)
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

// normalizeOptionalUUID maps a nil or empty optional UUID pointer to nil so empty
// values are never persisted or passed to the database as "".
func normalizeOptionalUUID(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	return v
}

// --- Guardians ---

// CreateGuardian validates and persists a new Guardian for the actor's tenant.
func (s *Service) CreateGuardian(ctx context.Context, actor auth.Actor, req CreateGuardianRequest) (*domain.Guardian, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}
	g, err := domain.NewGuardian(tenantID, req.FirstName, req.LastName, req.Relationship)
	if err != nil {
		return nil, err
	}
	g.Phone = req.Phone
	g.Email = req.Email
	g.UserID = normalizeOptionalUUID(req.UserID)
	if err := g.Validate(); err != nil {
		return nil, err
	}
	mutation := ports.LifecycleMutation{Kind: ports.MutationGuardianCreate, Guardian: g}
	meta := map[string]any{"guardian_id": g.ID, "tenant_id": g.TenantID}
	if err := s.commitLifecycle(ctx, tenantID, mutation, "guardian.created.v1", nil, meta); err != nil {
		return nil, err
	}
	return g, nil
}

// GetGuardian returns a single guardian.
func (s *Service) GetGuardian(ctx context.Context, actor auth.Actor, id string) (*domain.Guardian, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetGuardianByID(ctx, tenantID, id)
}

// GuardianChildren bundles the caller's guardian record with its linked students
// (the response shape of GET /guardians/me/children, AURA-10.12).
type GuardianChildren struct {
	Guardian *domain.Guardian  `json:"guardian"`
	Students []*domain.Student `json:"students"`
}

// ResolvedLearnerScope is the private service boundary used by record-owning
// services to enforce parent/student scopes without copying Student data.
type ResolvedLearnerScope struct {
	StudentIDs []string `json:"student_ids"`
	ClassIDs   []string `json:"class_ids"`
}

func (s *Service) ResolveLearnerScope(ctx context.Context, tenantID, userID, role string) (ResolvedLearnerScope, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" {
		return ResolvedLearnerScope{}, domain.ErrMissingTenant
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "student":
		student, err := s.repo.GetStudentByUserID(ctx, tenantID, userID)
		if err != nil {
			return ResolvedLearnerScope{}, err
		}
		classes := []string{}
		if student.ClassID != nil {
			classes = append(classes, *student.ClassID)
		}
		return ResolvedLearnerScope{StudentIDs: []string{student.ID}, ClassIDs: classes}, nil
	case "parent":
		guardian, err := s.repo.GetGuardianByUserID(ctx, tenantID, userID)
		if err != nil {
			return ResolvedLearnerScope{}, err
		}
		students, err := s.repo.ListStudentsByGuardian(ctx, tenantID, guardian.ID)
		if err != nil {
			return ResolvedLearnerScope{}, err
		}
		ids := make([]string, 0, len(students))
		classSet := map[string]struct{}{}
		for _, student := range students {
			ids = append(ids, student.ID)
			if student.ClassID != nil {
				classSet[*student.ClassID] = struct{}{}
			}
		}
		classIDs := make([]string, 0, len(classSet))
		for id := range classSet {
			classIDs = append(classIDs, id)
		}
		sort.Strings(classIDs)
		return ResolvedLearnerScope{StudentIDs: ids, ClassIDs: classIDs}, nil
	case "teacher":
		if s.classes == nil {
			return ResolvedLearnerScope{}, domain.ErrUnavailable
		}
		classIDs, err := s.classes.ResolveTeacherClasses(ctx, tenantID, userID)
		if err != nil {
			return ResolvedLearnerScope{}, err
		}
		studentIDs, err := s.repo.ListStudentIDsByClassIDs(ctx, tenantID, classIDs)
		if err != nil {
			return ResolvedLearnerScope{}, err
		}
		return ResolvedLearnerScope{StudentIDs: studentIDs, ClassIDs: classIDs}, nil
	default:
		return ResolvedLearnerScope{}, domain.ErrForbidden
	}
}

// GetMyGuardianChildren returns the guardian record linked to the actor's identity
// user plus the students linked to that guardian, or domain.ErrNotFound when the
// user has no linked guardian.
func (s *Service) GetMyGuardianChildren(ctx context.Context, actor auth.Actor) (*GuardianChildren, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	g, err := s.repo.GetGuardianByUserID(ctx, tenantID, actor.UserID)
	if err != nil {
		return nil, err
	}
	students, err := s.repo.ListStudentsByGuardian(ctx, tenantID, g.ID)
	if err != nil {
		return nil, err
	}
	if students == nil {
		students = []*domain.Student{}
	}
	return &GuardianChildren{Guardian: g, Students: students}, nil
}

// ListStudentGuardians returns the guardians linked to a student.
func (s *Service) ListStudentGuardians(ctx context.Context, actor auth.Actor, studentID string, limit int, cursor string) ([]*domain.Guardian, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	// Verify the student belongs to the tenant (will return not-found otherwise).
	if _, err := s.repo.GetByID(ctx, tenantID, studentID); err != nil {
		return nil, "", err
	}
	return s.repo.ListGuardiansByStudent(ctx, tenantID, studentID, normalizeLimit(limit), cursor)
}

// UpdateGuardian patches a guardian record.
func (s *Service) UpdateGuardian(ctx context.Context, actor auth.Actor, id string, req UpdateGuardianRequest) (*domain.Guardian, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	g, err := s.repo.GetGuardianByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := g.ApplyUpdate(req.FirstName, req.LastName, req.Relationship, req.Phone, req.Email)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return g, nil
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	mutation := ports.LifecycleMutation{Kind: ports.MutationGuardianUpdate, Guardian: g}
	if err := s.commitLifecycle(ctx, tenantID, mutation, "guardian.updated.v1", nil, map[string]any{
		"guardian_id":    g.ID,
		"tenant_id":      tenantID,
		"changed_fields": changed,
	}); err != nil {
		return nil, err
	}
	return g, nil
}

// DeleteGuardian removes a guardian and all its student links.
func (s *Service) DeleteGuardian(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermDelete)
	if err != nil {
		return err
	}
	g, err := s.repo.GetGuardianByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.commitLifecycle(ctx, tenantID, ports.LifecycleMutation{Kind: ports.MutationGuardianDelete, Guardian: g}, "guardian.deleted.v1", nil, map[string]any{
		"guardian_id": id,
		"tenant_id":   tenantID,
	})
}

// LinkGuardian links an existing guardian to a student.
func (s *Service) LinkGuardian(ctx context.Context, actor auth.Actor, studentID string, req LinkGuardianRequest) (*domain.StudentGuardian, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return nil, err
	}
	// Verify both resources belong to the tenant.
	if _, err := s.repo.GetByID(ctx, tenantID, studentID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetGuardianByID(ctx, tenantID, req.GuardianID); err != nil {
		return nil, err
	}
	link, err := domain.NewStudentGuardian(tenantID, studentID, req.GuardianID, req.Relationship, req.IsPrimary)
	if err != nil {
		return nil, err
	}
	if err := s.commitLifecycle(ctx, tenantID, ports.LifecycleMutation{Kind: ports.MutationGuardianLink, Link: link}, "guardian.linked.v1", nil, map[string]any{
		"student_id":  link.StudentID,
		"guardian_id": link.GuardianID,
		"tenant_id":   tenantID,
	}); err != nil {
		return nil, err
	}
	return link, nil
}

// UnlinkGuardian removes the link between a student and a guardian.
func (s *Service) UnlinkGuardian(ctx context.Context, actor auth.Actor, studentID, guardianID string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return err
	}
	mutation := ports.LifecycleMutation{
		Kind: ports.MutationGuardianUnlink, StudentID: studentID, GuardianID: guardianID,
	}
	return s.commitLifecycle(ctx, tenantID, mutation, "guardian.unlinked.v1", nil, map[string]any{
		"student_id":  studentID,
		"guardian_id": guardianID,
		"tenant_id":   tenantID,
	})
}

// ImportStudents bulk-creates students and optionally their guardians from a parsed slice.
// Rows that fail validation or persistence are collected as errors; successful rows commit.
func (s *Service) ImportStudents(ctx context.Context, actor auth.Actor, rows []ImportStudentRow) (*ImportResult, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermCreate)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{Errors: []ImportError{}}
	// Reuse an existing guardian within the batch when email matches.
	guardianByEmail := make(map[string]*domain.Guardian)

	for i, row := range rows {
		rowNum := i + 1
		if err := s.importRow(ctx, tenantID, row, result, guardianByEmail); err != nil {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Message: err.Error()})
		}
	}
	return result, nil
}

func (s *Service) importRow(
	ctx context.Context,
	tenantID string,
	row ImportStudentRow,
	result *ImportResult,
	guardianByEmail map[string]*domain.Guardian,
) error {
	student, err := domain.NewStudent(tenantID, row.FirstName, row.LastName)
	if err != nil {
		return err
	}
	student.DateOfBirth = row.DateOfBirth
	student.Gender = row.Gender
	student.UserID = normalizeOptionalUUID(row.UserID)
	if err := student.Validate(); err != nil {
		return err
	}
	mutation := ports.LifecycleMutation{Kind: ports.MutationStudentCreate, Student: student}
	if err := s.commitLifecycle(ctx, tenantID, mutation, "student.created.v1", student, nil); err != nil {
		return err
	}
	result.StudentsCreated++

	if !hasGuardianData(row) {
		return nil
	}

	guardian, err := s.findOrCreateGuardian(ctx, tenantID, row, result, guardianByEmail)
	if err != nil {
		return err
	}

	link, err := domain.NewStudentGuardian(tenantID, student.ID, guardian.ID, row.Relationship, false)
	if err != nil {
		return err
	}
	if err := s.commitLifecycle(ctx, tenantID, ports.LifecycleMutation{Kind: ports.MutationGuardianLink, Link: link}, "guardian.linked.v1", nil, map[string]any{
		"student_id":  student.ID,
		"guardian_id": guardian.ID,
		"tenant_id":   tenantID,
	}); err != nil {
		return err
	}
	result.LinksCreated++
	return nil
}

func hasGuardianData(row ImportStudentRow) bool {
	return row.GuardianFirstName != nil || row.GuardianLastName != nil ||
		row.GuardianEmail != nil || row.GuardianPhone != nil
}

func (s *Service) findOrCreateGuardian(
	ctx context.Context,
	tenantID string,
	row ImportStudentRow,
	result *ImportResult,
	guardianByEmail map[string]*domain.Guardian,
) (*domain.Guardian, error) {
	if row.GuardianEmail != nil && *row.GuardianEmail != "" {
		if existing, ok := guardianByEmail[*row.GuardianEmail]; ok {
			return existing, nil
		}
	}
	guardian, err := s.newGuardianFromRow(ctx, tenantID, row)
	if err != nil {
		return nil, err
	}
	mutation := ports.LifecycleMutation{Kind: ports.MutationGuardianCreate, Guardian: guardian}
	meta := map[string]any{"guardian_id": guardian.ID, "tenant_id": guardian.TenantID}
	if err := s.commitLifecycle(ctx, tenantID, mutation, "guardian.created.v1", nil, meta); err != nil {
		return nil, err
	}
	result.GuardiansCreated++
	if guardian.Email != nil && *guardian.Email != "" {
		guardianByEmail[*guardian.Email] = guardian
	}
	return guardian, nil
}

func (s *Service) newGuardianFromRow(_ context.Context, tenantID string, row ImportStudentRow) (*domain.Guardian, error) {
	gf, gl, ok := guardianNames(row)
	if !ok {
		return nil, domain.ErrValidation
	}
	rel := defaultRelationship(row.Relationship)
	guardian, err := domain.NewGuardian(tenantID, gf, gl, rel)
	if err != nil {
		return nil, err
	}
	guardian.Phone = row.GuardianPhone
	guardian.Email = row.GuardianEmail
	guardian.UserID = normalizeOptionalUUID(row.GuardianUserID)
	if err := guardian.Validate(); err != nil {
		return nil, err
	}
	return guardian, nil
}

func guardianNames(row ImportStudentRow) (first, last string, ok bool) {
	if row.GuardianFirstName == nil || row.GuardianLastName == nil {
		return "", "", false
	}
	first = strings.TrimSpace(*row.GuardianFirstName)
	last = strings.TrimSpace(*row.GuardianLastName)
	return first, last, first != "" && last != ""
}

func defaultRelationship(rel *string) string {
	if rel != nil && strings.TrimSpace(*rel) != "" {
		return strings.TrimSpace(*rel)
	}
	return "parent"
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
