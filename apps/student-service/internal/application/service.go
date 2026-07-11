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

// CreateGuardianRequest is the input for creating a guardian.
type CreateGuardianRequest struct {
	FirstName   string
	LastName    string
	Relationship string
	Phone       *string
	Email       *string
}

// UpdateGuardianRequest is the input for patching a guardian.
type UpdateGuardianRequest struct {
	FirstName   *string
	LastName    *string
	Relationship *string
	Phone       *string
	Email       *string
}

// LinkGuardianRequest links a guardian to a student.
type LinkGuardianRequest struct {
	GuardianID   string
	Relationship *string
	IsPrimary    bool
}

// ImportStudentRow is one row from a bulk-import CSV/JSON payload.
type ImportStudentRow struct {
	FirstName           string
	LastName            string
	DateOfBirth         *string
	Gender              *string
	Relationship        *string
	GuardianFirstName   *string
	GuardianLastName    *string
	GuardianPhone       *string
	GuardianEmail       *string
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
	_ = s.pub.Publish(ctx, "guardian.created.v1", nil, map[string]any{"guardian_id": g.ID, "tenant_id": g.TenantID})
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
	_ = s.pub.Publish(ctx, "guardian.updated.v1", nil, map[string]any{"guardian_id": g.ID, "changed_fields": changed})
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
	_ = s.pub.Publish(ctx, "guardian.deleted.v1", nil, map[string]any{"guardian_id": id})
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
	_ = s.pub.Publish(ctx, "guardian.linked.v1", nil, map[string]any{
		"student_id":  link.StudentID,
		"guardian_id": link.GuardianID,
	})
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
	_ = s.pub.Publish(ctx, "guardian.unlinked.v1", nil, map[string]any{
		"student_id":  studentID,
		"guardian_id": guardianID,
	})
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

func (s *Service) importRow(ctx context.Context, tenantID string, row ImportStudentRow, result *ImportResult, guardianByEmail map[string]*domain.Guardian) error {
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
	_ = s.pub.Publish(ctx, "student.created.v1", student, nil)

	// Create/link guardian if any guardian field is present.
	if row.GuardianFirstName != nil || row.GuardianLastName != nil || row.GuardianEmail != nil || row.GuardianPhone != nil {
		gf := stringPtr("")
		if row.GuardianFirstName != nil {
			gf = row.GuardianFirstName
		}
		gl := stringPtr("")
		if row.GuardianLastName != nil {
			gl = row.GuardianLastName
		}
		rel := "parent"
		if row.Relationship != nil && *row.Relationship != "" {
			rel = *row.Relationship
		}

		var guardian *domain.Guardian
		if row.GuardianEmail != nil && *row.GuardianEmail != "" {
			if existing, ok := guardianByEmail[*row.GuardianEmail]; ok {
				guardian = existing
			}
		}

		if guardian == nil {
			if gf == nil || gl == nil || *gf == "" || *gl == "" {
				return domain.ErrValidation
			}
			guardian, err = domain.NewGuardian(tenantID, *gf, *gl, rel)
			if err != nil {
				return err
			}
			guardian.Phone = row.GuardianPhone
			guardian.Email = row.GuardianEmail
			if err := guardian.Validate(); err != nil {
				return err
			}
			if err := s.repo.CreateGuardian(ctx, tenantID, guardian); err != nil {
				return err
			}
			result.GuardiansCreated++
			_ = s.pub.Publish(ctx, "guardian.created.v1", nil, map[string]any{"guardian_id": guardian.ID, "tenant_id": guardian.TenantID})
			if guardian.Email != nil && *guardian.Email != "" {
				guardianByEmail[*guardian.Email] = guardian
			}
		}

		link, err := domain.NewStudentGuardian(tenantID, student.ID, guardian.ID, row.Relationship, false)
		if err != nil {
			return err
		}
		if err := s.repo.LinkGuardianToStudent(ctx, tenantID, link); err != nil {
			return err
		}
		result.LinksCreated++
		_ = s.pub.Publish(ctx, "guardian.linked.v1", nil, map[string]any{
			"student_id":  student.ID,
			"guardian_id": guardian.ID,
		})
	}
	return nil
}

func stringPtr(s string) *string { return &s }

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
