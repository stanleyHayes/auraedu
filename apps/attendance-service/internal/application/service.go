// Package application implements the attendance use cases and RBAC policy.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/auraedu/attendance-service/internal/domain"
	"github.com/auraedu/attendance-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
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

// BulkMarkRow is one student's mark within a BulkMarkRequest.
type BulkMarkRow struct {
	StudentID string
	Status    string
	Remark    *string
}

// BulkMarkRequest is the input for marking a whole class at once (AURA-13.9;
// markAttendanceBulk in contracts/openapi/attendance.v1.yaml).
type BulkMarkRequest struct {
	AcademicYearID string
	Date           string
	ClassID        *string
	SubjectID      *string
	Records        []BulkMarkRow
}

// RowValidationError reports every invalid row of a bulk request at once. It unwraps to
// domain.ErrValidation so the HTTP adapter maps it to a 422 validation_error.
type RowValidationError struct {
	Rows map[string]string
}

func (e *RowValidationError) Error() string {
	keys := make([]string, 0, len(e.Rows))
	for k := range e.Rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e.Rows[k])
	}
	return fmt.Sprintf("%s: %s", domain.ErrValidation, strings.Join(parts, "; "))
}

// Unwrap supports errors.Is(err, domain.ErrValidation).
func (e *RowValidationError) Unwrap() error { return domain.ErrValidation }

// BulkMark validates and persists one attendance mark per student for a class+date.
// Following the contract (201 AttendanceRecordList or 422 ValidationError) the request
// is all-or-nothing: every row is validated up front with all failures reported together,
// and nothing is persisted when any row is invalid. Persistence is an idempotent upsert
// on (tenant_id, student_id, academic_year_id, date) — the uniqueness rule enforced by
// the migrations — so retrying a bulk mark converges instead of duplicating. The actor is
// recorded as marked_by and one attendance.marked.v1 event is emitted per student.
func (s *Service) BulkMark(ctx context.Context, actor auth.Actor, req BulkMarkRequest) ([]*domain.AttendanceRecord, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermMark)
	if err != nil {
		return nil, err
	}
	if err := validateBulkMark(req); err != nil {
		return nil, err
	}
	classID := nilIfBlank(req.ClassID)
	subjectID := nilIfBlank(req.SubjectID)
	records := make([]*domain.AttendanceRecord, 0, len(req.Records))
	for _, row := range req.Records {
		record, err := domain.NewAttendanceRecord(tenantID, row.StudentID, req.AcademicYearID, req.Date, row.Status, actor.UserID, row.Remark)
		if err != nil {
			return nil, err
		}
		record.ClassID = classID
		record.SubjectID = subjectID
		records = append(records, record)
	}
	if err := s.repo.UpsertMany(ctx, tenantID, records); err != nil {
		return nil, err
	}
	for _, record := range records {
		if err := s.pub.Publish(ctx, "attendance.marked.v1", record, nil); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish attendance marked event", "err", err, "attendance_id", record.ID)
		}
	}
	return records, nil
}

func validateBulkMark(req BulkMarkRequest) error {
	rows := map[string]string{}
	if strings.TrimSpace(req.AcademicYearID) == "" {
		rows["academic_year_id"] = "academic_year_id is required"
	} else if _, err := uuid.Parse(req.AcademicYearID); err != nil {
		rows["academic_year_id"] = "academic_year_id must be a valid UUID"
	}
	date, err := domain.NewDate(req.Date)
	if err != nil || date.IsEmpty() {
		rows["date"] = "date must be a valid date (YYYY-MM-DD)"
	}
	if classID := nilIfBlank(req.ClassID); classID != nil {
		if _, err := uuid.Parse(*classID); err != nil {
			rows["class_id"] = "class_id must be a valid UUID"
		}
	}
	if subjectID := nilIfBlank(req.SubjectID); subjectID != nil {
		if _, err := uuid.Parse(*subjectID); err != nil {
			rows["subject_id"] = "subject_id must be a valid UUID"
		}
	}
	if len(req.Records) == 0 {
		rows["records"] = "records must contain at least one entry"
	}
	seen := make(map[string]int, len(req.Records))
	for i, row := range req.Records {
		prefix := fmt.Sprintf("records[%d]", i)
		studentID := strings.TrimSpace(row.StudentID)
		if studentID == "" {
			rows[prefix+".student_id"] = "student_id is required"
		} else if _, err := uuid.Parse(studentID); err != nil {
			rows[prefix+".student_id"] = "student_id must be a valid UUID"
		} else if prev, dup := seen[studentID]; dup {
			rows[prefix+".student_id"] = fmt.Sprintf("duplicate of records[%d]", prev)
		} else {
			seen[studentID] = i
		}
		switch domain.Status(strings.TrimSpace(row.Status)) {
		case domain.StatusPresent, domain.StatusAbsent, domain.StatusLate, domain.StatusExcused:
		default:
			rows[prefix+".status"] = "status must be present, absent, late or excused"
		}
	}
	if len(rows) > 0 {
		return &RowValidationError{Rows: rows}
	}
	return nil
}

func nilIfBlank(v *string) *string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return v
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
