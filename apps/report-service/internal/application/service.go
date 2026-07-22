// Package application implements the report service use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/report-service/internal/domain"
	"github.com/auraedu/report-service/internal/ports"
)

// RBAC permission keys.
const (
	PermRead    = "reports.read"
	PermPublish = "reports.publish"
)

// FeatureReportCards is the feature flag key for report cards.
const FeatureReportCards = "report_cards"

// DefaultReportOutputDir is the default local directory for generated PDFs.
const DefaultReportOutputDir = "/tmp/auraedu-reports"

// Service holds the report use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	repo    ports.Repository
	pub     ports.EventPublisher
	pdfGen  ports.PDFGenerator
	storage ports.ReportStorage
	gates   flags.Gate
	scope   ports.LearnerScopeResolver
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithPDFGenerator sets the PDF generator.
func WithPDFGenerator(g ports.PDFGenerator) Option { return func(s *Service) { s.pdfGen = g } }

// WithStorage sets the generated-report object storage adapter.
func WithStorage(storage ports.ReportStorage) Option { return func(s *Service) { s.storage = storage } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithLearnerScopeResolver configures identity-to-student ownership resolution.
func WithLearnerScopeResolver(r ports.LearnerScopeResolver) Option {
	return func(s *Service) { s.scope = r }
}

type noopPublisher struct{}

func (noopPublisher) PublishReportTemplate(context.Context, string, *domain.ReportTemplate, map[string]any) error {
	return nil
}
func (noopPublisher) PublishReportCard(context.Context, string, *domain.ReportCard, map[string]any) error {
	return nil
}

type noopPDFGenerator struct{}

func (noopPDFGenerator) GenerateReportCard(context.Context, *domain.ReportCardDocument) ([]byte, error) {
	return nil, errors.New("pdf generator not configured")
}

type noopReportStorage struct{}

func (noopReportStorage) Save(context.Context, string, string, []byte) (string, error) {
	return "", errors.New("report storage not configured")
}
func (noopReportStorage) Open(context.Context, string, string) (io.ReadCloser, error) {
	return nil, errors.New("report storage not configured")
}

// NewService constructs the application service.
func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{
		repo:    repo,
		pub:     noopPublisher{},
		pdfGen:  noopPDFGenerator{},
		storage: noopReportStorage{},
		gates:   flags.NewStaticSnapshot(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// RequestReportCardGeneration atomically moves a report card to GENERATING and
// writes a durable queue record. The HTTP request returns immediately; a worker
// owns rendering, storage and the final PUBLISHED transition.
func (s *Service) RequestReportCardGeneration(ctx context.Context, actor auth.Actor, id string) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return nil, err
	}
	return s.repo.EnqueueReportGeneration(ctx, tenantID, id)
}

// ProcessNextGeneration leases and processes one durable PDF job. The boolean
// is false when no job is ready. Failures are persisted with bounded retry;
// callers may log the returned error without losing queue state.
func (s *Service) ProcessNextGeneration(ctx context.Context, lease time.Duration, maxAttempts int) (bool, error) {
	job, err := s.repo.ClaimReportGeneration(ctx, lease)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	jobCtx := tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: job.TenantID})

	processErr := func() error {
		card, err := s.repo.GetReportCardByID(jobCtx, job.TenantID, job.ReportCardID)
		if err != nil {
			return err
		}
		doc, err := s.buildReportCardDocument(jobCtx, job.TenantID, card)
		if err != nil {
			return err
		}
		pdfBytes, err := s.pdfGen.GenerateReportCard(jobCtx, doc)
		if err != nil {
			return fmt.Errorf("report: generate pdf: %w", err)
		}
		storagePath, err := s.storage.Save(jobCtx, job.TenantID, card.ID+".pdf", pdfBytes)
		if err != nil {
			return fmt.Errorf("report: store pdf: %w", err)
		}
		_, err = s.repo.CompleteReportGeneration(jobCtx, job, storagePath)
		if err != nil {
			return err
		}
		return nil
	}()
	if processErr == nil {
		return true, nil
	}
	terminal, retryErr := s.repo.RetryReportGeneration(jobCtx, job, processErr.Error(), maxAttempts)
	if retryErr != nil {
		return true, errors.Join(processErr, retryErr)
	}
	if terminal {
		return true, fmt.Errorf("report generation permanently failed after %d attempts: %w", job.Attempts, processErr)
	}
	return true, processErr
}

// --- Report template requests. ---

// CreateReportTemplateRequest is the input for creating a report template.
type CreateReportTemplateRequest struct {
	Name           string
	AcademicYearID string
	BodyTemplate   string
}

// UpdateReportTemplateRequest is the input for patching a report template.
type UpdateReportTemplateRequest struct {
	Name           *string
	AcademicYearID *string
	BodyTemplate   *string
	Status         *string
}

// CreateReportTemplate validates and persists a new ReportTemplate for the actor's tenant.
func (s *Service) CreateReportTemplate(ctx context.Context, actor auth.Actor, req CreateReportTemplateRequest) (*domain.ReportTemplate, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return nil, err
	}
	t, err := domain.NewReportTemplate(tenantID, req.Name, req.AcademicYearID, req.BodyTemplate)
	if err != nil {
		return nil, err
	}
	if err := s.persistReportTemplateLifecycle(ctx, tenantID, t, ports.ReportMutationCreate, "report.created.v1", nil); err != nil {
		return nil, err
	}
	return t, nil
}

// ListReportTemplates returns a tenant-scoped page of report templates, optionally filtered.
func (s *Service) ListReportTemplates(ctx context.Context, actor auth.Actor, filter ports.ReportTemplateListFilter) ([]*domain.ReportTemplate, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListReportTemplates(ctx, tenantID, filter)
}

// GetReportTemplate returns a single report template if the actor may read the tenant's data.
func (s *Service) GetReportTemplate(ctx context.Context, actor auth.Actor, id string) (*domain.ReportTemplate, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetReportTemplateByID(ctx, tenantID, id)
}

// UpdateReportTemplate patches a report template.
func (s *Service) UpdateReportTemplate(ctx context.Context, actor auth.Actor, id string, req UpdateReportTemplateRequest) (*domain.ReportTemplate, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetReportTemplateByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := t.ApplyUpdate(req.Name, req.AcademicYearID, req.BodyTemplate, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return t, nil
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.persistReportTemplateLifecycle(
		ctx,
		tenantID,
		t,
		ports.ReportMutationUpdate,
		"report.updated.v1",
		map[string]any{"changed_fields": changed},
	); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteReportTemplate removes a report template.
func (s *Service) DeleteReportTemplate(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return err
	}
	t, err := s.repo.GetReportTemplateByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.persistReportTemplateLifecycle(ctx, tenantID, t, ports.ReportMutationDelete, "report.deleted.v1", nil)
}

// --- Report card requests. ---

// CreateReportCardRequest is the input for creating a report card.
type CreateReportCardRequest struct {
	StudentID      string
	AcademicYearID string
	TermID         string
	TemplateID     string
}

// UpdateReportCardRequest is the input for patching a report card.
type UpdateReportCardRequest struct {
	StudentID      *string
	AcademicYearID *string
	TemplateID     *string
	Status         *string
}

// CreateReportCard validates and persists a new ReportCard for the actor's tenant.
func (s *Service) CreateReportCard(ctx context.Context, actor auth.Actor, req CreateReportCardRequest) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return nil, err
	}
	c, err := domain.NewReportCard(tenantID, req.StudentID, req.AcademicYearID, req.TemplateID)
	if err != nil {
		return nil, err
	}
	c.TermID = strings.TrimSpace(req.TermID)
	if err := s.persistReportCardLifecycle(ctx, tenantID, c, ports.ReportMutationCreate, "report.created.v1", nil); err != nil {
		return nil, err
	}
	return c, nil
}

// ListReportCards returns a tenant-scoped page of report cards, optionally filtered.
func (s *Service) ListReportCards(ctx context.Context, actor auth.Actor, filter ports.ReportCardListFilter) ([]*domain.ReportCard, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	filter, err = s.applyReportScope(ctx, actor, filter)
	if err != nil {
		return nil, "", err
	}
	return s.repo.ListReportCards(ctx, tenantID, filter)
}

// GetReportCard returns a single report card if the actor may read the tenant's data.
func (s *Service) GetReportCard(ctx context.Context, actor auth.Actor, id string) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	c, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeReportCard(ctx, actor, c); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateReportCard patches a report card.
func (s *Service) UpdateReportCard(ctx context.Context, actor auth.Actor, id string, req UpdateReportCardRequest) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return nil, err
	}
	c, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := c.ApplyUpdate(req.StudentID, req.AcademicYearID, req.TemplateID, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return c, nil
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := s.persistReportCardLifecycle(
		ctx,
		tenantID,
		c,
		ports.ReportMutationUpdate,
		"report.updated.v1",
		map[string]any{"changed_fields": changed},
	); err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteReportCard removes a report card.
func (s *Service) DeleteReportCard(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return err
	}
	c, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.persistReportCardLifecycle(ctx, tenantID, c, ports.ReportMutationDelete, "report.deleted.v1", nil)
}

func (s *Service) persistReportTemplateLifecycle(
	ctx context.Context,
	tenantID string,
	template *domain.ReportTemplate,
	mutation string,
	eventType string,
	meta map[string]any,
) error {
	if durable, ok := s.repo.(ports.LifecycleRepository); ok {
		return durable.CommitReportTemplateLifecycle(
			ctx,
			tenantID,
			template,
			mutation,
			eventType,
			ports.ReportTemplateEventData(template, meta),
		)
	}
	var err error
	switch mutation {
	case ports.ReportMutationCreate:
		err = s.repo.CreateReportTemplate(ctx, tenantID, template)
	case ports.ReportMutationUpdate:
		err = s.repo.UpdateReportTemplate(ctx, tenantID, template)
	case ports.ReportMutationDelete:
		err = s.repo.DeleteReportTemplate(ctx, tenantID, template.ID)
	default:
		return fmt.Errorf("report: unsupported template lifecycle mutation %q", mutation)
	}
	if err != nil {
		return err
	}
	if err := s.pub.PublishReportTemplate(ctx, eventType, template, meta); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish report template lifecycle event", "event_type", eventType, "err", err)
	}
	return nil
}

func (s *Service) persistReportCardLifecycle(
	ctx context.Context,
	tenantID string,
	card *domain.ReportCard,
	mutation string,
	eventType string,
	meta map[string]any,
) error {
	if durable, ok := s.repo.(ports.LifecycleRepository); ok {
		return durable.CommitReportCardLifecycle(
			ctx,
			tenantID,
			card,
			mutation,
			eventType,
			ports.ReportCardEventData(eventType, card, meta),
		)
	}
	var err error
	switch mutation {
	case ports.ReportMutationCreate:
		err = s.repo.CreateReportCard(ctx, tenantID, card)
	case ports.ReportMutationUpdate:
		err = s.repo.UpdateReportCard(ctx, tenantID, card)
	case ports.ReportMutationDelete:
		err = s.repo.DeleteReportCard(ctx, tenantID, card.ID)
	default:
		return fmt.Errorf("report: unsupported card lifecycle mutation %q", mutation)
	}
	if err != nil {
		return err
	}
	if err := s.pub.PublishReportCard(ctx, eventType, card, meta); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish report card lifecycle event", "event_type", eventType, "err", err)
	}
	return nil
}

// buildReportCardDocument assembles the render model for the PDF generator:
// the card, its template (when assigned), the materialized score entries
// aggregated per subject and the attendance summary.
func (s *Service) buildReportCardDocument(ctx context.Context, tenantID string, c *domain.ReportCard) (*domain.ReportCardDocument, error) {
	doc := &domain.ReportCardDocument{Card: c, GeneratedAt: time.Now().UTC()}

	if c.TemplateID != "" {
		tmpl, err := s.repo.GetReportTemplateByID(ctx, tenantID, c.TemplateID)
		switch {
		case err == nil:
			doc.Template = tmpl
		case errors.Is(err, domain.ErrNotFound):
			// The template was deleted after assignment; render without it.
			slog.Default().WarnContext(ctx, "report card template not found; rendering without template",
				"report_card_id", c.ID, "template_id", c.TemplateID)
		default:
			return nil, fmt.Errorf("report: load template: %w", err)
		}
	}

	scores, err := s.repo.ListScoreEntries(ctx, tenantID, c.ID)
	if err != nil {
		return nil, err
	}
	doc.Scores = domain.AggregateScores(scores)

	attendance, err := s.repo.ListAttendanceEntries(ctx, tenantID, c.ID)
	if err != nil {
		return nil, err
	}
	doc.Attendance = domain.SummarizeAttendance(attendance)
	return doc, nil
}

// DownloadReportCard opens an authorized published PDF from the configured
// storage backend. The caller owns closing the returned reader.
func (s *Service) DownloadReportCard(ctx context.Context, actor auth.Actor, id string) (io.ReadCloser, *domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, nil, err
	}
	card, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.authorizeReportCard(ctx, actor, card); err != nil {
		return nil, nil, err
	}
	if domain.ReportCardStatus(card.Status) != domain.ReportCardStatusPublished || card.PDFPath == nil {
		return nil, card, domain.ErrNotFound
	}
	reader, err := s.storage.Open(ctx, tenantID, *card.PDFPath)
	if err != nil {
		return nil, card, fmt.Errorf("report: open published pdf: %w", err)
	}
	return reader, card, nil
}

// GetTranscript assembles the immutable academic record from published and
// archived report cards. Learners and parents may request only owned students.
func (s *Service) GetTranscript(ctx context.Context, actor auth.Actor, studentID string) (*domain.Transcript, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return nil, fmt.Errorf("%w: student_id is required", domain.ErrValidation)
	}
	if reportScopedRole(actor.Role) {
		filter, scopeErr := s.applyReportScope(ctx, actor, ports.ReportCardListFilter{StudentID: studentID})
		if scopeErr != nil || len(filter.StudentIDs) == 0 {
			if scopeErr != nil {
				return nil, scopeErr
			}
			return nil, domain.ErrNotFound
		}
	}
	cards, err := s.repo.ListTranscriptReportCards(ctx, tenantID, studentID)
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return nil, domain.ErrNotFound
	}
	transcript := &domain.Transcript{TenantID: tenantID, StudentID: studentID, GeneratedAt: time.Now().UTC()}
	for _, card := range cards {
		scores, err := s.repo.ListScoreEntries(ctx, tenantID, card.ID)
		if err != nil {
			return nil, err
		}
		attendance, err := s.repo.ListAttendanceEntries(ctx, tenantID, card.ID)
		if err != nil {
			return nil, err
		}
		publishedAt := card.UpdatedAt
		if card.GeneratedAt != nil {
			publishedAt = *card.GeneratedAt
		}
		transcript.Entries = append(transcript.Entries, domain.TranscriptEntry{
			ReportCardID: card.ID, AcademicYearID: card.AcademicYearID, TermID: card.TermID,
			PublishedAt: publishedAt, Scores: domain.AggregateScores(scores), Attendance: domain.SummarizeAttendance(attendance),
		})
	}
	return transcript, nil
}

func reportFamilyRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "parent" || role == "student"
}

func reportScopedRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return reportFamilyRole(role) || role == "teacher"
}

func (s *Service) applyReportScope(ctx context.Context, actor auth.Actor, filter ports.ReportCardListFilter) (ports.ReportCardListFilter, error) {
	if !reportScopedRole(actor.Role) {
		return filter, nil
	}
	if s.scope == nil {
		return filter, domain.ErrUnavailable
	}
	ids, err := s.scope.Resolve(ctx, actor.TenantID, actor.UserID, strings.ToLower(strings.TrimSpace(actor.Role)))
	if err != nil {
		return filter, err
	}
	if filter.StudentID != "" {
		for _, id := range ids {
			if id == filter.StudentID {
				filter.StudentIDs = []string{id}
				filter.StudentID = ""
				if reportFamilyRole(actor.Role) {
					filter.Status = string(domain.ReportCardStatusPublished)
				}
				return filter, nil
			}
		}
		return filter, domain.ErrNotFound
	}
	filter.StudentIDs = ids
	if reportFamilyRole(actor.Role) {
		filter.Status = string(domain.ReportCardStatusPublished)
	}
	return filter, nil
}

func (s *Service) authorizeReportCard(ctx context.Context, actor auth.Actor, c *domain.ReportCard) error {
	if !reportScopedRole(actor.Role) {
		return nil
	}
	filter, err := s.applyReportScope(ctx, actor, ports.ReportCardListFilter{StudentID: c.StudentID})
	if err != nil {
		return err
	}
	if len(filter.StudentIDs) == 0 || (reportFamilyRole(actor.Role) && c.Status != string(domain.ReportCardStatusPublished)) {
		return domain.ErrNotFound
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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureReportCards) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureReportCards)
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
