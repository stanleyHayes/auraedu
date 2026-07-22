// Package application implements Growth CRM use cases and access gates.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

const (
	FeatureGrowthCRM   = "growth_crm"
	FeatureLeadScoring = "growth_lead_scoring"
	PermRead           = "crm.lead.read"
	PermUpdate         = "crm.lead.update"
	PermAssign         = "crm.lead.assign"
	PermInteraction    = "crm.interaction.create"
)

type Service struct {
	repo         ports.Repository
	feedbackRepo ports.FeedbackRepository
	callbackRepo ports.CallbackRepository
	pub          ports.EventPublisher
	gates        flags.Gate
}

type Option func(*Service)

func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }
func WithFeatureGate(gate flags.Gate) Option        { return func(s *Service) { s.gates = gate } }
func WithFeedbackRepository(repo ports.FeedbackRepository) Option {
	return func(s *Service) { s.feedbackRepo = repo }
}
func WithCallbackRepository(repo ports.CallbackRepository) Option {
	return func(s *Service) { s.callbackRepo = repo }
}

func NewService(repo ports.Repository, options ...Option) *Service {
	s := &Service{repo: repo, pub: noopPublisher{}, gates: flags.NewStaticSnapshot()}
	for _, option := range options {
		option(s)
	}
	return s
}

type noopPublisher struct{}

func (noopPublisher) LeadCreated(context.Context, *domain.Lead) error               { return nil }
func (noopPublisher) InteractionCreated(context.Context, *domain.Interaction) error { return nil }
func (noopPublisher) FeedbackSubmitted(context.Context, *domain.Feedback) error     { return nil }
func (noopPublisher) CallbackRequested(context.Context, *domain.CallbackRequest) error {
	return nil
}
func (noopPublisher) LeadScored(context.Context, string, string, domain.LeadScore) error { return nil }

type CaptureRequest struct {
	FirstName, LastName string
	Email, Phone        *string
	InstitutionID       *string
	ProgrammeIDs        []string
	IntakeID            *string
	Source              string
	CampaignID          *string
	Message             *string
	Consent             domain.Consent
}

type ScheduleCallbackRequest struct {
	FirstName, LastName string
	Email, Phone        *string
	PreferredAt         time.Time
	Timezone, Locale    string
	Message             *string
	Consent             domain.Consent
}

func (s *Service) ScheduleCallback(ctx context.Context, tenantID, idempotencyKey string, request ScheduleCallbackRequest) (ports.CallbackResult, error) {
	tenantID, idempotencyKey = strings.TrimSpace(tenantID), strings.TrimSpace(idempotencyKey)
	if tenantID == "" {
		return ports.CallbackResult{}, domain.ErrMissingTenant
	}
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 || s.callbackRepo == nil || request.Phone == nil || !request.Consent.Voice {
		return ports.CallbackResult{}, domain.ErrValidation
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureGrowthCRM) {
		return ports.CallbackResult{}, fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureGrowthCRM)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return ports.CallbackResult{}, err
	}
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
	keyHash, requestHash := hash(idempotencyKey), hash(string(payload))
	if replay, found, err := s.callbackRepo.FindCallbackReplay(ctx, tenantID, keyHash, requestHash); err != nil {
		return ports.CallbackResult{}, err
	} else if found {
		return replay, nil
	}
	leadResult, err := s.Capture(ctx, tenantID, hash(idempotencyKey+":lead"), CaptureRequest{
		FirstName: request.FirstName, LastName: request.LastName, Email: request.Email, Phone: request.Phone,
		Source: "website_assistant_callback", Message: request.Message, Consent: request.Consent,
	})
	if err != nil {
		return ports.CallbackResult{}, err
	}
	callback, err := domain.NewCallbackRequest(tenantID, leadResult.Lead.ID, request.PreferredAt, request.Timezone, request.Locale, time.Now())
	if err != nil {
		return ports.CallbackResult{}, err
	}
	result, err := s.callbackRepo.ScheduleCallback(ctx, callback, keyHash, requestHash)
	if err == nil && !result.Replay {
		recordAsyncError("publish callback request", s.pub.CallbackRequested(ctx, result.Callback))
	}
	return result, err
}

func (s *Service) ListCallbacks(ctx context.Context, actor auth.Actor, status domain.CallbackStatus, limit int) ([]*domain.CallbackRequest, error) {
	tenantID, err := s.require(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if !validCallbackFilter(status) {
		return nil, domain.ErrValidation
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if s.callbackRepo == nil {
		return nil, domain.ErrValidation
	}
	return s.callbackRepo.ListCallbacks(ctx, tenantID, status, limit)
}

type SubmitFeedbackRequest struct {
	InteractionID *string
	AIRunID       *string
	FeedbackType  string
	Rating        *int
	Comment       *string
}

func (s *Service) SubmitFeedback(ctx context.Context, tenantID, idempotencyKey string, request SubmitFeedbackRequest) (ports.FeedbackResult, error) {
	tenantID, idempotencyKey = strings.TrimSpace(tenantID), strings.TrimSpace(idempotencyKey)
	if tenantID == "" {
		return ports.FeedbackResult{}, domain.ErrMissingTenant
	}
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 || s.feedbackRepo == nil {
		return ports.FeedbackResult{}, domain.ErrValidation
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureGrowthCRM) {
		return ports.FeedbackResult{}, fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureGrowthCRM)
	}
	feedback, err := domain.NewFeedback(tenantID, request.InteractionID, request.AIRunID, request.FeedbackType, request.Rating, request.Comment)
	if err != nil {
		return ports.FeedbackResult{}, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return ports.FeedbackResult{}, err
	}
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
	result, err := s.feedbackRepo.SubmitFeedback(ctx, feedback, hash(idempotencyKey), hash(string(payload)))
	if err == nil && !result.Replay {
		recordAsyncError("publish feedback submission", s.pub.FeedbackSubmitted(ctx, result.Feedback))
	}
	return result, err
}

func (s *Service) Capture(ctx context.Context, tenantID, idempotencyKey string, request CaptureRequest) (ports.CaptureResult, error) {
	tenantID, idempotencyKey = strings.TrimSpace(tenantID), strings.TrimSpace(idempotencyKey)
	if tenantID == "" {
		return ports.CaptureResult{}, domain.ErrMissingTenant
	}
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return ports.CaptureResult{}, domain.ErrValidation
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureGrowthCRM) {
		return ports.CaptureResult{}, fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureGrowthCRM)
	}
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
	lead, err := domain.NewLead(tenantID, request.FirstName, request.LastName, request.Email, request.Phone, request.Source, request.Consent)
	if err != nil {
		return ports.CaptureResult{}, err
	}
	lead.InstitutionID, lead.PreferredIntakeID, lead.CampaignID = request.InstitutionID, request.IntakeID, request.CampaignID
	if request.ProgrammeIDs != nil {
		lead.PreferredProgrammeIDs = append([]string{}, request.ProgrammeIDs...)
	}
	var initial *domain.Interaction
	if request.Message != nil && strings.TrimSpace(*request.Message) != "" {
		initial, err = domain.NewInteraction(tenantID, lead.ID, "website", "inbound", "prospect", *request.Message, nil, nil)
		if err != nil {
			return ports.CaptureResult{}, err
		}
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return ports.CaptureResult{}, fmt.Errorf("crm: hash capture request: %w", err)
	}
	result, err := s.repo.Capture(ctx, lead, hash(idempotencyKey), hash(string(payload)), initial)
	if err != nil {
		return ports.CaptureResult{}, err
	}
	if result.Created && !result.Replay {
		recordAsyncError("publish lead creation", s.pub.LeadCreated(ctx, result.Lead))
	}
	if initial != nil && !result.Replay {
		initial.LeadID = result.Lead.ID
		recordAsyncError("publish initial lead interaction", s.pub.InteractionCreated(ctx, initial))
	}
	if !result.Replay && s.gates != nil && s.gates.IsEnabled(ctx, tenantID, FeatureLeadScoring) {
		_, scoreErr := s.scoreLead(ctx, tenantID, result.Lead.ID, "lead_capture")
		recordAsyncError("score captured lead", scoreErr)
	}
	return result, nil
}

func (s *Service) GetLead(ctx context.Context, actor auth.Actor, leadID string) (*domain.Lead, error) {
	tenantID, err := s.require(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.repo.GetLead(ctx, tenantID, leadID)
}

type WelcomeRecipient struct {
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	FirstName        string `json:"first_name"`
	Eligible         bool   `json:"eligible"`
	EmailEligible    bool   `json:"email_eligible"`
	SMSEligible      bool   `json:"sms_eligible"`
	WhatsAppEligible bool   `json:"whatsapp_eligible"`
}

// ResolveWelcomeRecipient is intentionally exposed only through the protected
// internal transport. Contact data never enters the event payload.
func (s *Service) ResolveWelcomeRecipient(ctx context.Context, tenantID, leadID string) (WelcomeRecipient, error) {
	if strings.TrimSpace(tenantID) == "" {
		return WelcomeRecipient{}, domain.ErrMissingTenant
	}
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
	lead, err := s.repo.GetLead(ctx, tenantID, leadID)
	if err != nil {
		return WelcomeRecipient{}, err
	}
	recipient := WelcomeRecipient{FirstName: lead.FirstName}
	if lead.Email != nil {
		recipient.Email = strings.TrimSpace(*lead.Email)
		recipient.EmailEligible = lead.Consent.Email && recipient.Email != ""
		recipient.Eligible = recipient.EmailEligible
	}
	if lead.Phone != nil {
		recipient.Phone = strings.TrimSpace(*lead.Phone)
		recipient.SMSEligible = lead.Consent.SMS && recipient.Phone != ""
		recipient.WhatsAppEligible = lead.Consent.WhatsApp && recipient.Phone != ""
	}
	return recipient, nil
}

func (s *Service) ListLeads(ctx context.Context, actor auth.Actor, limit int, cursor string, filter ports.LeadFilter) ([]*domain.Lead, string, error) {
	tenantID, err := s.require(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListLeads(ctx, tenantID, limit, cursor, filter)
}

type UpdateLeadRequest struct {
	Stage        *domain.LeadStage
	OwnerUserID  *string
	ProgrammeIDs *[]string
}

func (s *Service) UpdateLead(ctx context.Context, actor auth.Actor, leadID string, request UpdateLeadRequest) (*domain.Lead, error) {
	permission := PermUpdate
	if request.OwnerUserID != nil {
		permission = PermAssign
	}
	tenantID, err := s.require(ctx, actor, permission)
	if err != nil {
		return nil, err
	}
	lead, err := s.repo.GetLead(ctx, tenantID, leadID)
	if err != nil {
		return nil, err
	}
	if request.Stage != nil && lead.SetStage(*request.Stage) != nil {
		return nil, domain.ErrValidation
	}
	if request.OwnerUserID != nil {
		lead.OwnerUserID = request.OwnerUserID
	}
	if request.ProgrammeIDs != nil {
		lead.PreferredProgrammeIDs = *request.ProgrammeIDs
	}
	lead.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateLead(ctx, tenantID, lead); err != nil {
		return nil, err
	}
	if s.gates != nil && s.gates.IsEnabled(ctx, tenantID, FeatureLeadScoring) {
		_, scoreErr := s.scoreLead(ctx, tenantID, lead.ID, "lead_updated")
		recordAsyncError("score updated lead", scoreErr)
	}
	return lead, nil
}

func (s *Service) RescoreLead(ctx context.Context, actor auth.Actor, leadID string) (domain.LeadScore, error) {
	tenantID, err := s.require(ctx, actor, PermUpdate)
	if err != nil {
		return domain.LeadScore{}, err
	}
	if s.gates == nil || !s.gates.IsEnabled(ctx, tenantID, FeatureLeadScoring) {
		return domain.LeadScore{}, fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureLeadScoring)
	}
	return s.scoreLead(ctx, tenantID, leadID, actor.UserID)
}

func (s *Service) scoreLead(ctx context.Context, tenantID, leadID, triggeredBy string) (domain.LeadScore, error) {
	lead, err := s.repo.GetLead(ctx, tenantID, leadID)
	if err != nil {
		return domain.LeadScore{}, err
	}
	evidence, err := s.repo.GetScoringEvidence(ctx, tenantID, leadID)
	if err != nil {
		return domain.LeadScore{}, err
	}
	score := domain.ScoreLead(*lead, evidence, time.Now().UTC())
	changed, err := s.repo.SaveLeadScore(ctx, tenantID, leadID, triggeredBy, score)
	if err != nil {
		return domain.LeadScore{}, err
	}
	if changed {
		recordAsyncError("publish lead score", s.pub.LeadScored(ctx, tenantID, leadID, score))
	}
	return score, nil
}

func (s *Service) CreateInteraction(
	ctx context.Context,
	actor auth.Actor,
	leadID, channel, direction, summary string,
	occurredAt *time.Time,
) (*domain.Interaction, error) {
	tenantID, err := s.require(ctx, actor, PermInteraction)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetLead(ctx, tenantID, leadID); err != nil {
		return nil, err
	}
	interaction, err := domain.NewInteraction(tenantID, leadID, channel, direction, "staff", summary, &actor.UserID, occurredAt)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateInteraction(ctx, tenantID, interaction); err != nil {
		return nil, err
	}
	recordAsyncError("publish lead interaction", s.pub.InteractionCreated(ctx, interaction))
	if s.gates != nil && s.gates.IsEnabled(ctx, tenantID, FeatureLeadScoring) {
		_, scoreErr := s.scoreLead(ctx, tenantID, leadID, "interaction_created")
		recordAsyncError("score lead interaction", scoreErr)
	}
	return interaction, nil
}

func (s *Service) ListInteractions(ctx context.Context, actor auth.Actor, leadID string, limit int, cursor string) ([]*domain.Interaction, string, error) {
	tenantID, err := s.require(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	if _, err := s.repo.GetLead(ctx, tenantID, leadID); err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListInteractions(ctx, tenantID, leadID, limit, cursor)
}

func (s *Service) require(ctx context.Context, actor auth.Actor, permission string) (string, error) {
	if !actor.Authenticated() {
		return "", domain.ErrUnauthorized
	}
	tenantID := tenancy.TenantID(ctx)
	if tenantID == "" {
		return "", domain.ErrMissingTenant
	}
	if !actor.CanAccessTenant(tenantID) || !actor.Has(permission) {
		return "", domain.ErrForbidden
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureGrowthCRM) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureGrowthCRM)
	}
	return tenantID, nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func recordAsyncError(operation string, err error) {
	if err != nil {
		slog.Error("CRM asynchronous operation failed", "operation", operation, "err", err)
	}
}

func validCallbackFilter(status domain.CallbackStatus) bool {
	switch status {
	case "", domain.CallbackRequested, domain.CallbackConfirmed, domain.CallbackCompleted, domain.CallbackCancelled:
		return true
	default:
		return false
	}
}
