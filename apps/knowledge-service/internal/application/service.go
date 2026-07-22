// Package application coordinates governed knowledge workflows.
package application

import (
	"context"
	"strings"
	"time"

	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/knowledge-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

const (
	PermRead         = "knowledge.read"
	PermManage       = "knowledge.manage"
	PermApprove      = "knowledge.approve"
	FeatureKnowledge = "growth_website_chat"
)

type Service struct {
	repo ports.Repository
	pub  ports.EventPublisher
	now  func() time.Time
	gate flags.Gate
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, string, string, map[string]any) error { return nil }

type Option func(*Service)

func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }
func WithClock(now func() time.Time) Option         { return func(s *Service) { s.now = now } }
func WithFeatureGate(gate flags.Gate) Option        { return func(s *Service) { s.gate = gate } }

func NewService(repo ports.Repository, options ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}, now: time.Now}
	for _, option := range options {
		option(s)
	}
	return s
}

type CreateInput struct {
	SourceType, Title, Owner, Content, Confidentiality, Locale string
	EffectiveAt                                                time.Time
	ExpiresAt                                                  *time.Time
	Programme, Campus, Intake                                  *string
}

func (s *Service) Create(ctx context.Context, actor auth.Actor, input CreateInput) (domain.Source, error) {
	if !actor.Has(PermManage) || strings.TrimSpace(actor.TenantID) == "" {
		return domain.Source{}, domain.ErrForbidden
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, actor.TenantID, FeatureKnowledge) {
		return domain.Source{}, domain.ErrForbidden
	}
	source, err := domain.NewSource(domain.CreateInput{TenantID: actor.TenantID, SourceType: input.SourceType,
		Title: input.Title, Owner: input.Owner, Content: input.Content, Confidentiality: input.Confidentiality,
		Locale:      input.Locale,
		EffectiveAt: input.EffectiveAt, ExpiresAt: input.ExpiresAt, Programme: input.Programme, Campus: input.Campus, Intake: input.Intake}, s.now())
	if err != nil {
		return domain.Source{}, err
	}
	if err := s.repo.Create(ctx, source); err != nil {
		return domain.Source{}, err
	}
	return source, nil
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (domain.Source, error) {
	if (!actor.Has(PermRead) && !actor.Has(PermManage) && !actor.Has(PermApprove)) || actor.TenantID == "" {
		return domain.Source{}, domain.ErrForbidden
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, actor.TenantID, FeatureKnowledge) {
		return domain.Source{}, domain.ErrForbidden
	}
	return s.repo.Get(ctx, actor.TenantID, id)
}

func (s *Service) List(ctx context.Context, actor auth.Actor, status domain.Status, limit int) ([]domain.Source, error) {
	if (!actor.Has(PermRead) && !actor.Has(PermManage) && !actor.Has(PermApprove)) || actor.TenantID == "" {
		return nil, domain.ErrForbidden
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, actor.TenantID, FeatureKnowledge) {
		return nil, domain.ErrForbidden
	}
	if status != "" && status != domain.StatusDraft && status != domain.StatusApproved && status != domain.StatusRetired {
		return nil, domain.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.repo.List(ctx, actor.TenantID, status, limit)
}

func (s *Service) Approve(ctx context.Context, actor auth.Actor, id, note string) (domain.Source, error) {
	note = strings.TrimSpace(note)
	if !actor.Has(PermApprove) || actor.TenantID == "" {
		return domain.Source{}, domain.ErrForbidden
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, actor.TenantID, FeatureKnowledge) {
		return domain.Source{}, domain.ErrForbidden
	}
	if actor.UserID == "" || len(note) < 3 || len(note) > 500 {
		return domain.Source{}, domain.ErrValidation
	}
	now := s.now().UTC()
	if transactional, ok := s.repo.(ports.TransactionalApprovalRepository); ok {
		return transactional.ApproveWithEvent(ctx, actor.TenantID, id, actor.UserID, note, now, "knowledge.source_approved.v1")
	}
	source, err := s.repo.Approve(ctx, actor.TenantID, id, actor.UserID, note, now)
	if err != nil {
		return domain.Source{}, err
	}
	if err := s.pub.Publish(ctx, "knowledge.source_approved.v1", source.TenantID, ports.ApprovalEventData(source)); err != nil {
		return domain.Source{}, err
	}
	return source, nil
}

func (s *Service) Retire(ctx context.Context, actor auth.Actor, id string) (domain.Source, error) {
	if !actor.Has(PermApprove) || actor.TenantID == "" {
		return domain.Source{}, domain.ErrForbidden
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, actor.TenantID, FeatureKnowledge) {
		return domain.Source{}, domain.ErrForbidden
	}
	return s.repo.Retire(ctx, actor.TenantID, id, s.now().UTC())
}

func (s *Service) SearchApproved(ctx context.Context, tenantID, query, locale string, limit int, asOf time.Time) ([]domain.SearchResult, error) {
	tenantID, query = strings.TrimSpace(tenantID), strings.TrimSpace(query)
	locale, localeOK := domain.NormalizeLocale(locale)
	if tenantID == "" || len(query) < 2 || len(query) > 500 || !localeOK {
		return nil, domain.ErrValidation
	}
	if s.gate != nil && !s.gate.IsEnabled(ctx, tenantID, FeatureKnowledge) {
		return nil, domain.ErrForbidden
	}
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	if asOf.IsZero() {
		asOf = s.now().UTC()
	}
	return s.repo.Search(ctx, tenantID, query, locale, limit, asOf)
}
