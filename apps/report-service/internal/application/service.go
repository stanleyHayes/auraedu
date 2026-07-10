package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	repo            ports.Repository
	pub             ports.EventPublisher
	pdfGen          ports.PDFGenerator
	gates           flags.Gate
	reportOutputDir string
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithPDFGenerator sets the PDF generator.
func WithPDFGenerator(g ports.PDFGenerator) Option { return func(s *Service) { s.pdfGen = g } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithReportOutputDir sets the local directory for generated PDFs.
func WithReportOutputDir(dir string) Option {
	return func(s *Service) {
		if dir != "" {
			s.reportOutputDir = dir
		}
	}
}

type noopPublisher struct{}

func (noopPublisher) PublishReportTemplate(context.Context, string, *domain.ReportTemplate, map[string]any) error {
	return nil
}
func (noopPublisher) PublishReportCard(context.Context, string, *domain.ReportCard, map[string]any) error {
	return nil
}

type noopPDFGenerator struct{}

func (noopPDFGenerator) GenerateReportCard(context.Context, *domain.ReportCard) ([]byte, error) {
	return nil, errors.New("pdf generator not configured")
}

// NewService constructs the application service.
func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{
		repo:            repo,
		pub:             noopPublisher{},
		pdfGen:          noopPDFGenerator{},
		gates:           flags.NewStaticSnapshot(),
		reportOutputDir: DefaultReportOutputDir,
	}
	for _, o := range opts {
		o(s)
	}
	return s
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
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	t, err := domain.NewReportTemplate(tenantID, req.Name, req.AcademicYearID, req.BodyTemplate)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateReportTemplate(ctx, tenantID, t); err != nil {
		return nil, err
	}
	_ = s.pub.PublishReportTemplate(ctx, "report.created.v1", t, nil)
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
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
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
	if err := s.repo.UpdateReportTemplate(ctx, tenantID, t); err != nil {
		return nil, err
	}
	_ = s.pub.PublishReportTemplate(ctx, "report.updated.v1", t, map[string]any{"changed_fields": changed})
	return t, nil
}

// DeleteReportTemplate removes a report template.
func (s *Service) DeleteReportTemplate(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return err
	}
	t, err := s.repo.GetReportTemplateByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteReportTemplate(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.PublishReportTemplate(ctx, "report.deleted.v1", t, nil)
	return nil
}

// --- Report card requests. ---

// CreateReportCardRequest is the input for creating a report card.
type CreateReportCardRequest struct {
	StudentID      string
	AcademicYearID string
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
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	c, err := domain.NewReportCard(tenantID, req.StudentID, req.AcademicYearID, req.TemplateID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateReportCard(ctx, tenantID, c); err != nil {
		return nil, err
	}
	_ = s.pub.PublishReportCard(ctx, "report.created.v1", c, nil)
	return c, nil
}

// ListReportCards returns a tenant-scoped page of report cards, optionally filtered.
func (s *Service) ListReportCards(ctx context.Context, actor auth.Actor, filter ports.ReportCardListFilter) ([]*domain.ReportCard, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.ListReportCards(ctx, tenantID, filter)
}

// GetReportCard returns a single report card if the actor may read the tenant's data.
func (s *Service) GetReportCard(ctx context.Context, actor auth.Actor, id string) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetReportCardByID(ctx, tenantID, id)
}

// UpdateReportCard patches a report card.
func (s *Service) UpdateReportCard(ctx context.Context, actor auth.Actor, id string, req UpdateReportCardRequest) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
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
	if err := s.repo.UpdateReportCard(ctx, tenantID, c); err != nil {
		return nil, err
	}
	_ = s.pub.PublishReportCard(ctx, "report.updated.v1", c, map[string]any{"changed_fields": changed})
	return c, nil
}

// DeleteReportCard removes a report card.
func (s *Service) DeleteReportCard(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return err
	}
	c, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteReportCard(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.PublishReportCard(ctx, "report.deleted.v1", c, nil)
	return nil
}

// GenerateReportCard generates a PDF for the report card, stores it, and publishes it.
func (s *Service) GenerateReportCard(ctx context.Context, actor auth.Actor, id string) (*domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermPublish)
	if err != nil {
		return nil, err
	}
	c, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if domain.ReportCardStatus(c.Status) == domain.ReportCardStatusGenerating {
		return nil, fmt.Errorf("%w: report card is already being generated", domain.ErrConflict)
	}

	c.SetGenerating()
	if err := s.repo.UpdateReportCard(ctx, tenantID, c); err != nil {
		return nil, err
	}

	pdfBytes, err := s.pdfGen.GenerateReportCard(ctx, c)
	if err != nil {
		s.rollbackToDraft(ctx, tenantID, c)
		return nil, fmt.Errorf("report: generate pdf: %w", err)
	}

	pdfPath := s.reportCardPath(c)
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0o755); err != nil {
		s.rollbackToDraft(ctx, tenantID, c)
		return nil, fmt.Errorf("report: create output dir: %w", err)
	}
	if err := os.WriteFile(pdfPath, pdfBytes, 0o644); err != nil {
		s.rollbackToDraft(ctx, tenantID, c)
		return nil, fmt.Errorf("report: write pdf: %w", err)
	}

	c.SetPublished(pdfPath)
	if err := s.repo.UpdateReportCard(ctx, tenantID, c); err != nil {
		return nil, err
	}

	_ = s.pub.PublishReportCard(ctx, "report.published.v1", c, nil)
	return c, nil
}

func (s *Service) rollbackToDraft(ctx context.Context, tenantID string, c *domain.ReportCard) {
	status := string(domain.ReportCardStatusDraft)
	_, _ = c.ApplyUpdate(nil, nil, nil, &status)
	_ = s.repo.UpdateReportCard(ctx, tenantID, c)
}

func (s *Service) reportCardPath(c *domain.ReportCard) string {
	return filepath.Join(s.reportOutputDir, c.TenantID, c.ID+".pdf")
}

// DownloadReportCardPath returns the absolute path to a published report card PDF.
func (s *Service) DownloadReportCardPath(ctx context.Context, actor auth.Actor, id string) (string, *domain.ReportCard, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return "", nil, err
	}
	c, err := s.repo.GetReportCardByID(ctx, tenantID, id)
	if err != nil {
		return "", nil, err
	}
	if domain.ReportCardStatus(c.Status) != domain.ReportCardStatusPublished || c.PDFPath == nil {
		return "", c, domain.ErrNotFound
	}
	return *c.PDFPath, c, nil
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
