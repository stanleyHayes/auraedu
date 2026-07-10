package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
)

// RBAC permission keys (contracts/permissions/permissions.yaml).
const (
	PermRead   = "website.read"
	PermManage = "website.manage"
)

// Feature flag key from contracts/features/features.yaml.
const FeaturePublicWebsite = "public_website"

// Service holds the website use cases. Tenant scope + RBAC + feature-flag checks
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

func (noopPublisher) PublishPage(context.Context, string, *domain.Page, map[string]any) error {
	return nil
}
func (noopPublisher) PublishSection(context.Context, string, *domain.Section, map[string]any) error {
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

// Page request/response types.

type CreatePageRequest struct {
	Slug            string
	Title           string
	MetaDescription *string
	Layout          *string
	Status          *string
}

type UpdatePageRequest struct {
	Slug            *string
	Title           *string
	MetaDescription *string
	Layout          *string
	Status          *string
}

// Section request/response types.

type CreateSectionRequest struct {
	PageID    string
	Type      domain.SectionType
	Content   domain.Content
	SortOrder int
	Status    *string
}

type UpdateSectionRequest struct {
	Type      *domain.SectionType
	Content   *domain.Content
	SortOrder *int
	Status    *string
}

// Page use cases.

// CreatePage validates and persists a new Page for the actor's tenant.
func (s *Service) CreatePage(ctx context.Context, actor auth.Actor, req CreatePageRequest) (*domain.Page, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	page, err := domain.NewPage(tenantID, req.Slug, req.Title)
	if err != nil {
		return nil, err
	}
	if req.MetaDescription != nil {
		page.MetaDescription = req.MetaDescription
	}
	if req.Layout != nil {
		if _, err := page.ApplyUpdate(nil, nil, nil, nil, req.Layout); err != nil {
			return nil, err
		}
	}
	if req.Status != nil {
		if _, err := page.ApplyUpdate(nil, nil, req.Status, nil, nil); err != nil {
			return nil, err
		}
	}
	if err := page.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreatePage(ctx, tenantID, page); err != nil {
		return nil, err
	}
	_ = s.pub.PublishPage(ctx, "website.page_created.v1", page, nil)
	if page.IsPublished() {
		_ = s.pub.PublishPage(ctx, "website.page_published.v1", page, nil)
	}
	return page, nil
}

// ListPages returns a tenant-scoped page of pages.
func (s *Service) ListPages(ctx context.Context, actor auth.Actor, limit int, cursor string, filter ports.PageFilter) ([]*domain.Page, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.repo.ListPages(ctx, tenantID, normalizeLimit(limit), cursor, filter)
}

// GetPage returns a single page if the actor may read the tenant's data.
func (s *Service) GetPage(ctx context.Context, actor auth.Actor, id string) (*domain.Page, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetPageByID(ctx, tenantID, id)
}

// GetPageBySlug returns a single page by slug.
func (s *Service) GetPageBySlug(ctx context.Context, actor auth.Actor, slug string) (*domain.Page, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetPageBySlug(ctx, tenantID, slug)
}

// UpdatePage patches a page record.
func (s *Service) UpdatePage(ctx context.Context, actor auth.Actor, id string, req UpdatePageRequest) (*domain.Page, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	page, err := s.repo.GetPageByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	wasPublished := page.IsPublished()
	var statusPtr *string
	if req.Status != nil {
		statusPtr = req.Status
	}
	changed, err := page.ApplyUpdate(req.Slug, req.Title, statusPtr, req.MetaDescription, req.Layout)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return page, nil
	}
	if err := page.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdatePage(ctx, tenantID, page); err != nil {
		return nil, err
	}
	eventMeta := map[string]any{"changed_fields": changed}
	_ = s.pub.PublishPage(ctx, "website.page_updated.v1", page, eventMeta)
	if !wasPublished && page.IsPublished() {
		_ = s.pub.PublishPage(ctx, "website.page_published.v1", page, nil)
	}
	return page, nil
}

// DeletePage removes a page record and its sections.
func (s *Service) DeletePage(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	page, err := s.repo.GetPageByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteSectionsByPage(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.repo.DeletePage(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.PublishPage(ctx, "website.page_deleted.v1", page, nil)
	return nil
}

// Section use cases.

// CreateSection validates and persists a new Section for the actor's tenant.
func (s *Service) CreateSection(ctx context.Context, actor auth.Actor, req CreateSectionRequest) (*domain.Section, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	if req.PageID == "" {
		return nil, domain.ErrValidation
	}
	if _, err := s.repo.GetPageByID(ctx, tenantID, req.PageID); err != nil {
		return nil, err
	}
	section, err := domain.NewSection(tenantID, req.PageID, req.Type, req.Content, req.SortOrder)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		if _, err := section.ApplyUpdate(nil, nil, nil, req.Status); err != nil {
			return nil, err
		}
	}
	if err := section.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateSection(ctx, tenantID, section); err != nil {
		return nil, err
	}
	_ = s.pub.PublishSection(ctx, "website.section_created.v1", section, nil)
	return section, nil
}

// ListSections returns a tenant-scoped page of sections for a page.
func (s *Service) ListSections(ctx context.Context, actor auth.Actor, pageID string, limit int, cursor string, filter ports.SectionFilter) ([]*domain.Section, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	return s.repo.ListSections(ctx, tenantID, pageID, normalizeLimit(limit), cursor, filter)
}

// GetSection returns a single section if the actor may read the tenant's data.
func (s *Service) GetSection(ctx context.Context, actor auth.Actor, id string) (*domain.Section, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetSectionByID(ctx, tenantID, id)
}

// UpdateSection patches a section record.
func (s *Service) UpdateSection(ctx context.Context, actor auth.Actor, id string, req UpdateSectionRequest) (*domain.Section, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	section, err := s.repo.GetSectionByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	changed, err := section.ApplyUpdate(req.Type, req.Content, req.SortOrder, req.Status)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return section, nil
	}
	if err := section.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSection(ctx, tenantID, section); err != nil {
		return nil, err
	}
	eventMeta := map[string]any{"changed_fields": changed}
	_ = s.pub.PublishSection(ctx, "website.section_updated.v1", section, eventMeta)
	return section, nil
}

// DeleteSection removes a section record.
func (s *Service) DeleteSection(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	section, err := s.repo.GetSectionByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteSection(ctx, tenantID, id); err != nil {
		return err
	}
	_ = s.pub.PublishSection(ctx, "website.section_deleted.v1", section, nil)
	return nil
}

// Access helpers.

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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeaturePublicWebsite) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeaturePublicWebsite)
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
