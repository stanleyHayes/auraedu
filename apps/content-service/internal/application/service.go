// Package application coordinates content workflows and authorization.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/google/uuid"
)

const FeatureContentAI = "growth_content_ai"

const (
	PermGenerate = "content.generate"
	PermReview   = "content.review"
)

type Service struct {
	repo      ports.Repository
	generator ports.Generator
	gate      flags.Gate
	now       func() time.Time
}

type Option func(*Service)

func WithFeatureGate(gate flags.Gate) Option { return func(s *Service) { s.gate = gate } }
func WithClock(now func() time.Time) Option  { return func(s *Service) { s.now = now } }

func NewService(repo ports.Repository, generator ports.Generator, options ...Option) *Service {
	s := &Service{repo: repo, generator: generator, now: time.Now}
	for _, option := range options {
		option(s)
	}
	return s
}

type BrandProfileInput struct {
	ToneOfVoice                     string
	ApprovedTerms, ProhibitedClaims []string
	RequiredDisclaimers             []string
	Locale                          string
	ExpectedVersion                 int
}

type GenerateInput struct {
	IdempotencyKey, ContentType, Title, Brief, Audience, Locale string
	CampaignID                                                  *string
	KeyMessages                                                 []string
	Facts                                                       []domain.Fact
	ExpiresAt                                                   *time.Time
}

type ReviseInput struct {
	Content, ChangeNote string
	ExpectedVersion     int
	ExpiresAt           *time.Time
}

func (s *Service) allowed(ctx context.Context, actor auth.Actor, permission string) bool {
	return actor.TenantID != "" && actor.UserID != "" && actor.Has(permission) && (s.gate == nil || s.gate.IsEnabled(ctx, actor.TenantID, FeatureContentAI))
}

func (s *Service) GetBrandProfile(ctx context.Context, actor auth.Actor) (domain.BrandProfile, error) {
	if !s.allowed(ctx, actor, PermGenerate) && !s.allowed(ctx, actor, PermReview) {
		return domain.BrandProfile{}, domain.ErrForbidden
	}
	return s.repo.GetBrandProfile(ctx, actor.TenantID)
}

func (s *Service) UpsertBrandProfile(ctx context.Context, actor auth.Actor, input BrandProfileInput) (domain.BrandProfile, error) {
	if !s.allowed(ctx, actor, PermReview) {
		return domain.BrandProfile{}, domain.ErrForbidden
	}
	profile, err := domain.NewBrandProfile(domain.BrandProfileInput{TenantID: actor.TenantID, ToneOfVoice: input.ToneOfVoice,
		ApprovedTerms: input.ApprovedTerms, ProhibitedClaims: input.ProhibitedClaims, RequiredDisclaimers: input.RequiredDisclaimers,
		Locale: input.Locale, UpdatedBy: actor.UserID, ExpectedVersion: input.ExpectedVersion}, s.now())
	if err != nil {
		return domain.BrandProfile{}, err
	}
	if err := s.repo.UpsertBrandProfile(ctx, profile, input.ExpectedVersion); err != nil {
		return domain.BrandProfile{}, err
	}
	return profile, nil
}

func (s *Service) Generate(ctx context.Context, actor auth.Actor, input GenerateInput) (domain.Draft, error) {
	if !s.allowed(ctx, actor, PermGenerate) {
		return domain.Draft{}, domain.ErrForbidden
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 16 || len(input.IdempotencyKey) > 128 {
		return domain.Draft{}, domain.ErrValidation
	}
	requestBytes, err := json.Marshal(input)
	if err != nil {
		return domain.Draft{}, domain.ErrValidation
	}
	requestHash, keyHash := digest(string(requestBytes)), digest(strings.TrimSpace(input.IdempotencyKey))
	if replay, storedHash, found, err := s.repo.FindReplay(ctx, actor.TenantID, keyHash, requestHash); err != nil {
		return domain.Draft{}, err
	} else if found {
		if storedHash != requestHash {
			return domain.Draft{}, domain.ErrConflict
		}
		return replay, nil
	}
	profile, err := s.repo.GetBrandProfile(ctx, actor.TenantID)
	if err != nil {
		return domain.Draft{}, err
	}
	// Validate the entire governed request before incurring provider cost. The
	// placeholder copy is never persisted or sent to the provider.
	if _, err := domain.NewDraft(domain.DraftInput{TenantID: actor.TenantID, CampaignID: input.CampaignID, ContentType: input.ContentType,
		Title: input.Title, Brief: input.Brief, Audience: input.Audience, Locale: input.Locale, KeyMessages: input.KeyMessages,
		Facts: input.Facts, Content: "validation-only", Generator: "validation-only", BrandProfileVersion: profile.Version,
		CreatedBy: actor.UserID, ExpiresAt: input.ExpiresAt}, profile, s.now()); err != nil {
		return domain.Draft{}, err
	}
	generated, err := s.generator.Generate(ctx, ports.GenerateInput{ContentType: input.ContentType, Title: input.Title, Brief: input.Brief,
		Audience: input.Audience, Locale: input.Locale, KeyMessages: input.KeyMessages, Facts: input.Facts, Profile: profile})
	if err != nil || strings.TrimSpace(generated.Content) == "" || strings.TrimSpace(generated.Generator) == "" {
		return domain.Draft{}, domain.ErrUnavailable
	}
	draft, err := domain.NewDraft(domain.DraftInput{TenantID: actor.TenantID, CampaignID: input.CampaignID, ContentType: input.ContentType,
		Title: input.Title, Brief: input.Brief, Audience: input.Audience, Locale: input.Locale, KeyMessages: input.KeyMessages,
		Facts: input.Facts, Content: generated.Content, Generator: generated.Generator, BrandProfileVersion: profile.Version,
		CreatedBy: actor.UserID, ExpiresAt: input.ExpiresAt}, profile, s.now())
	if err != nil {
		return domain.Draft{}, err
	}
	version := draft.Snapshot("Generated from approved brief and facts")
	if err := s.repo.CreateDraftWithEvent(
		ctx, draft, version, keyHash, requestHash, ports.DraftGeneratedEventData(draft),
	); err != nil {
		return domain.Draft{}, err
	}
	return draft, nil
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (domain.Draft, []domain.Version, error) {
	if !s.allowed(ctx, actor, PermGenerate) && !s.allowed(ctx, actor, PermReview) {
		return domain.Draft{}, nil, domain.ErrForbidden
	}
	return s.repo.GetDraft(ctx, actor.TenantID, id)
}

func (s *Service) List(ctx context.Context, actor auth.Actor, filter ports.ListFilter) ([]domain.Draft, error) {
	if !s.allowed(ctx, actor, PermGenerate) && !s.allowed(ctx, actor, PermReview) {
		return nil, domain.ErrForbidden
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 25
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.ContentType = strings.TrimSpace(filter.ContentType)
	filter.CampaignID = strings.TrimSpace(filter.CampaignID)
	if (filter.Status != "" && !domain.IsValidStatus(filter.Status)) ||
		(filter.ContentType != "" && !domain.IsValidContentType(filter.ContentType)) {
		return nil, domain.ErrValidation
	}
	if filter.CampaignID != "" {
		if _, err := uuid.Parse(filter.CampaignID); err != nil {
			return nil, domain.ErrValidation
		}
	}
	return s.repo.ListDrafts(ctx, actor.TenantID, filter)
}

func (s *Service) Revise(ctx context.Context, actor auth.Actor, id string, input ReviseInput) (domain.Draft, error) {
	if !s.allowed(ctx, actor, PermGenerate) {
		return domain.Draft{}, domain.ErrForbidden
	}
	draft, _, err := s.repo.GetDraft(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Draft{}, err
	}
	profile, err := s.repo.GetBrandProfile(ctx, actor.TenantID)
	if err != nil {
		return domain.Draft{}, err
	}
	expected, expectedUpdatedAt := draft.Version, draft.UpdatedAt
	if err := draft.Revise(actor.UserID, input.Content, input.ChangeNote, input.ExpectedVersion, input.ExpiresAt, profile, s.now()); err != nil {
		return domain.Draft{}, err
	}
	version := draft.Snapshot(input.ChangeNote)
	if err := s.repo.UpdateDraftWithVersionAndEvent(ctx, draft, expected, expectedUpdatedAt, &version, "", nil); err != nil {
		return domain.Draft{}, err
	}
	return draft, nil
}

func (s *Service) Submit(ctx context.Context, actor auth.Actor, id string, expectedVersion int) (domain.Draft, error) {
	if !s.allowed(ctx, actor, PermGenerate) {
		return domain.Draft{}, domain.ErrForbidden
	}
	return s.transition(ctx, actor, id, func(d *domain.Draft) error { return d.Submit(actor.UserID, expectedVersion, s.now()) })
}

func (s *Service) Approve(ctx context.Context, actor auth.Actor, id, note string, expectedVersion int) (domain.Draft, error) {
	if !s.allowed(ctx, actor, PermReview) {
		return domain.Draft{}, domain.ErrForbidden
	}
	return s.transition(ctx, actor, id, func(d *domain.Draft) error { return d.Review(actor.UserID, note, expectedVersion, true, s.now()) })
}

func (s *Service) Reject(ctx context.Context, actor auth.Actor, id, note string, expectedVersion int) (domain.Draft, error) {
	if !s.allowed(ctx, actor, PermReview) {
		return domain.Draft{}, domain.ErrForbidden
	}
	return s.transition(ctx, actor, id, func(d *domain.Draft) error { return d.Review(actor.UserID, note, expectedVersion, false, s.now()) })
}

func (s *Service) transition(ctx context.Context, actor auth.Actor, id string, change func(*domain.Draft) error) (domain.Draft, error) {
	draft, _, err := s.repo.GetDraft(ctx, actor.TenantID, id)
	if err != nil {
		return domain.Draft{}, err
	}
	previousStatus, previousVersion, previousUpdatedAt := draft.Status, draft.Version, draft.UpdatedAt
	if err := change(&draft); err != nil {
		return domain.Draft{}, err
	}
	payload := ports.StatusChangedEventData(draft, previousStatus, actor.UserID)
	if err := s.repo.UpdateDraftWithVersionAndEvent(
		ctx, draft, previousVersion, previousUpdatedAt, nil, "content.status_changed.v1", payload,
	); err != nil {
		return domain.Draft{}, err
	}
	return draft, nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
