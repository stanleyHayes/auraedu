// Package application coordinates Campaign use cases and policy.
package application

import (
	"context"
	"strings"
	"time"

	"github.com/auraedu/campaign-service/internal/domain"
	"github.com/auraedu/campaign-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
)

const FeatureCampaigns = "growth_campaigns"
const (
	PermRead          = "campaign.read"
	PermCreate        = "campaign.create"
	PermUpdate        = "campaign.update"
	PermApprove       = "campaign.approve"
	PermPublish       = "campaign.publish"
	PermBudgetApprove = "campaign.budget.approve"
)

type Service struct {
	repo ports.Repository
	pub  ports.EventPublisher
	gate flags.Gate
	now  func() time.Time
}
type Option func(*Service)
type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, string, string, map[string]any) error { return nil }
func WithPublisher(pub ports.EventPublisher) Option                                 { return func(s *Service) { s.pub = pub } }
func WithFeatureGate(gate flags.Gate) Option                                        { return func(s *Service) { s.gate = gate } }
func WithClock(now func() time.Time) Option                                         { return func(s *Service) { s.now = now } }
func NewService(repo ports.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type CreateInput struct {
	Name, Objective, Channel, AudienceDefinition, Currency string
	ProgrammeIDs                                           []string
	Budget                                                 float64
	StartAt, EndAt                                         time.Time
}
type UpdateInput struct {
	Name, Objective, AudienceDefinition, Currency *string
	ProgrammeIDs                                  *[]string
	Budget                                        *float64
	StartAt, EndAt                                *time.Time
}

func (s *Service) allowed(ctx context.Context, actor auth.Actor, permission string) bool {
	return actor.TenantID != "" && actor.Has(permission) && (s.gate == nil || s.gate.IsEnabled(ctx, actor.TenantID, FeatureCampaigns))
}
func (s *Service) Create(ctx context.Context, actor auth.Actor, input CreateInput) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermCreate) || actor.UserID == "" {
		return domain.Campaign{}, domain.ErrForbidden
	}
	campaign, err := domain.NewCampaign(domain.CreateInput{
		TenantID: actor.TenantID, Name: input.Name, Objective: input.Objective,
		Channel: input.Channel, AudienceDefinition: input.AudienceDefinition,
		Currency: input.Currency, ProgrammeIDs: input.ProgrammeIDs, Budget: input.Budget,
		StartAt: input.StartAt, EndAt: input.EndAt, OwnerUserID: actor.UserID,
	}, s.now())
	if err != nil {
		return domain.Campaign{}, err
	}
	if err = s.repo.Create(ctx, campaign); err != nil {
		return domain.Campaign{}, err
	}
	return campaign, nil
}
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermRead) {
		return domain.Campaign{}, domain.ErrForbidden
	}
	return s.repo.Get(ctx, actor.TenantID, id)
}
func (s *Service) List(ctx context.Context, actor auth.Actor, status domain.Status, limit int) ([]domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermRead) {
		return nil, domain.ErrForbidden
	}
	if status != "" && !validStatus(status) {
		return nil, domain.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.repo.List(ctx, actor.TenantID, status, limit)
}
func validStatus(status domain.Status) bool {
	switch status {
	case domain.StatusDraft,
		domain.StatusPending,
		domain.StatusApproved,
		domain.StatusScheduled,
		domain.StatusActive,
		domain.StatusPaused,
		domain.StatusCompleted,
		domain.StatusCancelled:
		return true
	}
	return false
}
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, input UpdateInput) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermUpdate) {
		return domain.Campaign{}, domain.ErrForbidden
	}
	campaign, err := s.repo.Get(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Campaign{}, err
	}
	expected := campaign.Status
	if expected != domain.StatusDraft {
		return domain.Campaign{}, domain.ErrConflict
	}
	if input.Name != nil {
		campaign.Name = strings.TrimSpace(*input.Name)
	}
	if input.Objective != nil {
		campaign.Objective = strings.TrimSpace(*input.Objective)
	}
	if input.AudienceDefinition != nil {
		campaign.AudienceDefinition = strings.TrimSpace(*input.AudienceDefinition)
	}
	if input.Currency != nil {
		campaign.Currency = strings.ToUpper(strings.TrimSpace(*input.Currency))
	}
	if input.ProgrammeIDs != nil {
		campaign.ProgrammeIDs = *input.ProgrammeIDs
	}
	if input.Budget != nil {
		campaign.Budget = *input.Budget
	}
	if input.StartAt != nil {
		campaign.StartAt = input.StartAt.UTC()
	}
	if input.EndAt != nil {
		campaign.EndAt = input.EndAt.UTC()
	}
	if err := campaign.ValidateDraft(); err != nil {
		return domain.Campaign{}, err
	}
	campaign.UpdatedAt = s.now().UTC()
	if err := s.repo.Update(ctx, campaign, expected); err != nil {
		return domain.Campaign{}, err
	}
	return campaign, nil
}

func (s *Service) Submit(ctx context.Context, actor auth.Actor, id string) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermUpdate) || actor.UserID == "" {
		return domain.Campaign{}, domain.ErrForbidden
	}
	return s.transition(ctx, actor, id, func(c *domain.Campaign) error { return c.Submit(actor.UserID, s.now()) })
}
func (s *Service) Approve(ctx context.Context, actor auth.Actor, id, note string) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermApprove) || actor.UserID == "" {
		return domain.Campaign{}, domain.ErrForbidden
	}
	campaign, err := s.repo.Get(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Campaign{}, err
	}
	if campaign.Budget > 0 && !actor.Has(PermBudgetApprove) {
		return domain.Campaign{}, domain.ErrForbidden
	}
	return s.transitionCampaign(ctx, campaign, func(c *domain.Campaign) error { return c.Approve(actor.UserID, note, s.now()) })
}
func (s *Service) Publish(ctx context.Context, actor auth.Actor, id string) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermPublish) {
		return domain.Campaign{}, domain.ErrForbidden
	}
	return s.transition(ctx, actor, id, func(c *domain.Campaign) error { return c.Publish(s.now()) })
}
func (s *Service) Pause(ctx context.Context, actor auth.Actor, id string) (domain.Campaign, error) {
	if !s.allowed(ctx, actor, PermPublish) {
		return domain.Campaign{}, domain.ErrForbidden
	}
	return s.transition(ctx, actor, id, func(c *domain.Campaign) error { return c.Pause(s.now()) })
}
func (s *Service) transition(ctx context.Context, actor auth.Actor, id string, change func(*domain.Campaign) error) (domain.Campaign, error) {
	campaign, err := s.repo.Get(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Campaign{}, err
	}
	return s.transitionCampaign(ctx, campaign, change)
}
func (s *Service) transitionCampaign(ctx context.Context, campaign domain.Campaign, change func(*domain.Campaign) error) (domain.Campaign, error) {
	previous := campaign.Status
	if err := change(&campaign); err != nil {
		return domain.Campaign{}, err
	}
	payload := ports.StatusChangedEventData(campaign, previous)
	if transactional, ok := s.repo.(ports.TransactionalRepository); ok {
		if err := transactional.UpdateWithEvent(ctx, campaign, previous, "campaign.status_changed.v1", payload); err != nil {
			return domain.Campaign{}, err
		}
		return campaign, nil
	}
	if err := s.repo.Update(ctx, campaign, previous); err != nil {
		return domain.Campaign{}, err
	}
	if err := s.pub.Publish(ctx, "campaign.status_changed.v1", campaign.TenantID, payload); err != nil {
		return domain.Campaign{}, err
	}
	return campaign, nil
}
