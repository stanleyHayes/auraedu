// Package application implements the student service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	// ClassID and AcademicYearID are the optional class assignment. They are persisted
	// on the student record (since AURA-10.11) and also carried on the
	// student.enrolled.v1 event (contracts/events/student.enrolled.v1.json).
	ClassID        *string
	AcademicYearID *string
}

// UpdateStudentRequest is the input for patching a student.
type UpdateStudentRequest struct {
	FirstName *string
	LastName  *string
	Status    *string
}

// CreateGuardianRequest is the input for creating a guardian.
type CreateGuardianRequest struct {
	FirstName    string
	LastName     string
	Relationship string
	Phone        *string
	Email        *string
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
	if err := student.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, tenantID, student); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "student.enrolled.v1", student, enrolledMeta(student, req.ClassID, req.AcademicYearID)); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish student.enrolled event", "err", err)
	}
	return student, nil
}

// enrolledMeta builds the data payload for student.enrolled.v1
// (contracts/events/student.enrolled.v1.json). student_id is added by the publisher;
// enrollment_date is always known; class_id/academic_year_id are included only when the
// create flow actually has them (the schema marks them required, but enrollment records
// are not persisted yet — see contracts/openapi student.v1.yaml CreateEnrollment).
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

// List returns a tenant-scoped page of students. When classID is non-nil the page is
// restricted to students assigned to that class (roster view, AURA-10.11); an empty
// string is treated as unset. A non-UUID classID is a validation error.
func (s *Service) List(ctx context.Context, actor auth.Actor, classID *string, limit int, cursor string) ([]*domain.Student, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	classID = normalizeOptionalUUID(classID)
	if classID != nil {
		if _, err := uuid.Parse(*classID); err != nil {
			return nil, "", domain.ErrValidation
		}
	}
	return s.repo.List(ctx, tenantID, classID, normalizeLimit(limit), cursor)
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
	if err := s.pub.Publish(ctx, "student.updated.v1", student, map[string]any{"changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish student.updated event", "err", err)
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
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "student.deleted.v1", student, nil); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish student.deleted event", "err", err)
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
	if err := g.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateGuardian(ctx, tenantID, g); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "guardian.created.v1", nil, map[string]any{"guardian_id": g.ID, "tenant_id": g.TenantID}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.created event", "err", err)
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
	if err := s.repo.UpdateGuardian(ctx, tenantID, g); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "guardian.updated.v1", nil, map[string]any{"guardian_id": g.ID, "changed_fields": changed}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.updated event", "err", err)
	}
	return g, nil
}

// DeleteGuardian removes a guardian and all its student links.
func (s *Service) DeleteGuardian(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermDelete)
	if err != nil {
		return err
	}
	if _, err := s.repo.GetGuardianByID(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.DeleteGuardian(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "guardian.deleted.v1", nil, map[string]any{"guardian_id": id}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.deleted event", "err", err)
	}
	return nil
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
	if err := s.repo.LinkGuardianToStudent(ctx, tenantID, link); err != nil {
		return nil, err
	}
	if err := s.pub.Publish(ctx, "guardian.linked.v1", nil, map[string]any{
		"student_id":  link.StudentID,
		"guardian_id": link.GuardianID,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.linked event", "err", err)
	}
	return link, nil
}

// UnlinkGuardian removes the link between a student and a guardian.
func (s *Service) UnlinkGuardian(ctx context.Context, actor auth.Actor, studentID, guardianID string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermUpdate)
	if err != nil {
		return err
	}
	if err := s.repo.UnlinkGuardianFromStudent(ctx, tenantID, studentID, guardianID); err != nil {
		return err
	}
	if err := s.pub.Publish(ctx, "guardian.unlinked.v1", nil, map[string]any{
		"student_id":  studentID,
		"guardian_id": guardianID,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.unlinked event", "err", err)
	}
	return nil
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
	if err := student.Validate(); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, tenantID, student); err != nil {
		return err
	}
	result.StudentsCreated++
	if err := s.pub.Publish(ctx, "student.enrolled.v1", student, enrolledMeta(student, nil, nil)); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish student.enrolled event", "err", err)
	}

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
	if err := s.repo.LinkGuardianToStudent(ctx, tenantID, link); err != nil {
		return err
	}
	result.LinksCreated++
	if err := s.pub.Publish(ctx, "guardian.linked.v1", nil, map[string]any{
		"student_id":  student.ID,
		"guardian_id": guardian.ID,
	}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.linked event", "err", err)
	}
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
	if err := s.repo.CreateGuardian(ctx, tenantID, guardian); err != nil {
		return nil, err
	}
	result.GuardiansCreated++
	if err := s.pub.Publish(ctx, "guardian.created.v1", nil, map[string]any{"guardian_id": guardian.ID, "tenant_id": guardian.TenantID}); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish guardian.created event", "err", err)
	}
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
