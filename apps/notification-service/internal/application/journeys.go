package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

const FeatureGrowthCRM = "growth_crm"

type CreateJourneyRequest struct {
	Name                  string
	TriggerEvent          string
	Timezone              string
	QuietHoursStartMinute *int
	QuietHoursEndMinute   *int
	FrequencyWindowHours  int
	FrequencyLimit        int
	CancelOnEvents        []string
	Steps                 []domain.JourneyStep
}

func (s *Service) CreateJourney(ctx context.Context, actor auth.Actor, request CreateJourneyRequest) (*domain.Journey, error) {
	tenantID, err := s.requireJourneyAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	journey, err := domain.NewJourney(domain.NewJourneyInput{
		TenantID:              tenantID,
		Name:                  request.Name,
		TriggerEvent:          request.TriggerEvent,
		Timezone:              request.Timezone,
		QuietHoursStartMinute: request.QuietHoursStartMinute,
		QuietHoursEndMinute:   request.QuietHoursEndMinute,
		FrequencyWindowHours:  request.FrequencyWindowHours,
		FrequencyLimit:        request.FrequencyLimit,
		CancelOnEvents:        request.CancelOnEvents,
		Steps:                 request.Steps,
		CreatedBy:             actor.UserID,
	})
	if err != nil {
		return nil, err
	}
	if err := s.validateJourneyTemplates(ctx, tenantID, journey); err != nil {
		return nil, err
	}
	if err := s.journeyRepo.CreateJourney(ctx, tenantID, journey); err != nil {
		return nil, err
	}
	return journey, nil
}

func (s *Service) GetJourney(ctx context.Context, actor auth.Actor, id string) (*domain.Journey, error) {
	tenantID, err := s.requireJourneyAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.journeyRepo.GetJourney(ctx, tenantID, id)
}

func (s *Service) ListJourneys(ctx context.Context, actor auth.Actor, filter ports.JourneyFilter) ([]*domain.Journey, error) {
	tenantID, err := s.requireJourneyAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	if filter.Status != "" {
		switch domain.JourneyStatus(filter.Status) {
		case domain.JourneyStatusDraft, domain.JourneyStatusActive, domain.JourneyStatusPaused, domain.JourneyStatusArchived:
		default:
			return nil, fmt.Errorf("%w: invalid journey status", domain.ErrValidation)
		}
	}
	if filter.TriggerEvent != "" && !domain.IsJourneyEvent(filter.TriggerEvent) {
		return nil, fmt.Errorf("%w: unsupported journey trigger event", domain.ErrValidation)
	}
	return s.journeyRepo.ListJourneys(ctx, tenantID, filter)
}

func (s *Service) ActivateJourney(ctx context.Context, actor auth.Actor, id string) (*domain.Journey, error) {
	tenantID, err := s.requireJourneyAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	journey, err := s.journeyRepo.GetJourney(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if journey.CreatedBy == actor.UserID {
		return nil, fmt.Errorf("%w: journey activation requires an independent reviewer", domain.ErrForbidden)
	}
	if err := s.validateJourneyTemplates(ctx, tenantID, journey); err != nil {
		return nil, err
	}
	if err := s.validateJourneyChannelFeatures(ctx, tenantID, journey); err != nil {
		return nil, err
	}
	if err := journey.Activate(actor.UserID, time.Now()); err != nil {
		return nil, err
	}
	if err := s.journeyRepo.UpdateJourneyStatus(ctx, tenantID, journey, actor.UserID); err != nil {
		return nil, err
	}
	return journey, nil
}

func (s *Service) PauseJourney(ctx context.Context, actor auth.Actor, id string) (*domain.Journey, error) {
	return s.transitionJourney(ctx, actor, id, func(journey *domain.Journey) error { return journey.Pause(time.Now()) })
}

func (s *Service) ArchiveJourney(ctx context.Context, actor auth.Actor, id string) (*domain.Journey, error) {
	return s.transitionJourney(ctx, actor, id, func(journey *domain.Journey) error { return journey.Archive(time.Now()) })
}

func (s *Service) transitionJourney(
	ctx context.Context,
	actor auth.Actor,
	id string,
	transition func(*domain.Journey) error,
) (*domain.Journey, error) {
	tenantID, err := s.requireJourneyAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	journey, err := s.journeyRepo.GetJourney(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := transition(journey); err != nil {
		return nil, err
	}
	if err := s.journeyRepo.UpdateJourneyStatus(ctx, tenantID, journey, actor.UserID); err != nil {
		return nil, err
	}
	return journey, nil
}

func (s *Service) GetJourneyStats(ctx context.Context, actor auth.Actor, id string) (ports.JourneyStats, error) {
	tenantID, err := s.requireJourneyAccess(ctx, actor, PermRead)
	if err != nil {
		return ports.JourneyStats{}, err
	}
	if _, err := s.journeyRepo.GetJourney(ctx, tenantID, id); err != nil {
		return ports.JourneyStats{}, err
	}
	return s.journeyRepo.JourneyStats(ctx, tenantID, id)
}

func (s *Service) requireJourneyAccess(ctx context.Context, actor auth.Actor, permission string) (string, error) {
	if s.journeyRepo == nil {
		return "", domain.ErrUnavailable
	}
	tenantID, err := s.requireAccess(ctx, actor, permission)
	if err != nil {
		return "", err
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureGrowthCRM) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureGrowthCRM)
	}
	return tenantID, nil
}

func (s *Service) validateJourneyTemplates(ctx context.Context, tenantID string, journey *domain.Journey) error {
	for _, step := range journey.Steps {
		template, err := s.templateRepo.GetByID(ctx, tenantID, step.TemplateID)
		if err != nil {
			return fmt.Errorf("%w: journey step %d template is unavailable", domain.ErrValidation, step.Position)
		}
		if template.Status != string(domain.TemplateStatusActive) || template.Channel != step.Channel {
			return fmt.Errorf("%w: journey step %d requires an active matching-channel template", domain.ErrValidation, step.Position)
		}
		if len([]rune(template.SubjectTemplate)) > 200 || len([]rune(template.BodyTemplate)) > 10_000 {
			return fmt.Errorf("%w: journey step %d template exceeds the communication size limit", domain.ErrValidation, step.Position)
		}
		if (step.Channel == string(domain.ChannelSMS) || step.Channel == string(domain.ChannelWhatsApp)) && len([]rune(template.BodyTemplate)) > 1_600 {
			return fmt.Errorf("%w: journey step %d mobile message exceeds 1600 characters", domain.ErrValidation, step.Position)
		}
		if err := domain.ValidateJourneyTemplateVariables(template.SubjectTemplate); err != nil {
			return fmt.Errorf("journey step %d subject: %w", step.Position, err)
		}
		if err := domain.ValidateJourneyTemplateVariables(template.BodyTemplate); err != nil {
			return fmt.Errorf("journey step %d body: %w", step.Position, err)
		}
	}
	return nil
}

func (s *Service) validateJourneyChannelFeatures(ctx context.Context, tenantID string, journey *domain.Journey) error {
	for _, step := range journey.Steps {
		if !s.journeyChannelEnabled(ctx, tenantID, step.Channel) {
			return fmt.Errorf("%w: journey channel %s is disabled", flags.ErrFeatureDisabled, step.Channel)
		}
	}
	return nil
}

func (s *Service) journeyChannelEnabled(ctx context.Context, tenantID, channel string) bool {
	if s.gates == nil {
		return true
	}
	switch domain.NotificationChannel(channel) {
	case domain.ChannelEmail:
		return s.gates.IsEnabled(ctx, tenantID, FeatureEmailNotifications)
	case domain.ChannelSMS:
		return s.gates.IsEnabled(ctx, tenantID, FeatureSMSNotifications)
	case domain.ChannelWhatsApp:
		return s.gates.IsEnabled(ctx, tenantID, FeatureWhatsAppNotifications) &&
			s.gates.IsEnabled(ctx, tenantID, "growth_whatsapp")
	case domain.ChannelInApp:
		return s.gates.IsEnabled(ctx, tenantID, FeatureNotifications)
	default:
		return false
	}
}

// HandleJourneyEvent is a separately idempotent event projection. It is kept
// outside HandleCloudEvent so an existing one-off notification cannot be
// repeated if journey persistence temporarily fails.
func (s *Service) HandleJourneyEvent(ctx context.Context, event tenancy.CloudEvent) error {
	if s.journeyRepo == nil || !domain.IsJourneyEvent(event.Type) {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("notifications: decode journey event: %w", err)
	}
	leadID := strings.TrimSpace(stringField(data, "lead_id"))
	if !validJourneyLeadID(leadID) {
		// Applications that are not linked to a CRM lead cannot be contacted
		// through a prospect journey and are intentionally ignored.
		return nil
	}
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: event.TenantID, ActorRole: "notification_journey_worker"})
	if _, err := s.journeyRepo.CancelJourneysForEvent(ctx, event.TenantID, leadID, event.ID, event.Type); err != nil {
		return err
	}
	if s.gates != nil && (!s.gates.IsEnabled(ctx, event.TenantID, FeatureNotifications) ||
		!s.gates.IsEnabled(ctx, event.TenantID, FeatureGrowthCRM)) {
		return nil
	}
	journeys, err := s.journeyRepo.ListActiveJourneysByTrigger(ctx, event.TenantID, event.Type)
	if err != nil || len(journeys) == 0 {
		return err
	}
	if s.leadResolver == nil {
		return errors.New("notifications: journey lead resolver is not configured")
	}
	recipient, err := s.leadResolver.ResolveWelcomeRecipient(ctx, event.TenantID, leadID)
	if err != nil {
		return err
	}
	contextValues := domain.ExtractJourneyContext(data)
	if firstName := strings.TrimSpace(recipient.FirstName); firstName != "" && len([]rune(firstName)) <= 100 {
		contextValues["first_name"] = firstName
	}
	for _, journey := range journeys {
		if err := s.enrollJourney(ctx, event, leadID, recipient, contextValues, journey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enrollJourney(
	ctx context.Context,
	event tenancy.CloudEvent,
	leadID string,
	recipient ports.LeadWelcomeRecipient,
	contextValues map[string]string,
	journey *domain.Journey,
) error {
	enrollmentID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var messages []*domain.Message
	skipped := 0
	proposed := time.Now().UTC()
	for _, step := range journey.Steps {
		proposed = proposed.Add(time.Duration(step.DelayMinutes) * time.Minute)
		if !step.Matches(contextValues) || !s.journeyChannelEnabled(ctx, event.TenantID, step.Channel) {
			skipped++
			continue
		}
		deliveryAddress, eligible := journeyRecipient(step.Channel, recipient)
		if !eligible {
			skipped++
			continue
		}
		template, err := s.templateRepo.GetByID(ctx, event.TenantID, step.TemplateID)
		if err != nil || template.Status != string(domain.TemplateStatusActive) || template.Channel != step.Channel {
			return fmt.Errorf("notifications: journey %s step %d template unavailable", journey.ID, step.Position)
		}
		subject, err := domain.RenderJourneyTemplate(template.SubjectTemplate, contextValues)
		if err != nil {
			return err
		}
		body, err := domain.RenderJourneyTemplate(template.BodyTemplate, contextValues)
		if err != nil {
			return err
		}
		metadata := map[string]any{
			"journey_id":            journey.ID,
			"journey_version":       journey.Version,
			"journey_enrollment_id": enrollmentID.String(),
			"journey_step_id":       step.ID,
			"journey_step_position": step.Position,
			"trigger_event":         event.Type,
			"trigger_event_id":      event.ID,
			"lead_id":               leadID,
			"consent_verified":      true,
		}
		if deliveryAddress != "" {
			metadata["delivery_address"] = deliveryAddress
		}
		scheduledAt := journey.NextAllowedTime(proposed)
		templateID := template.ID
		message, err := domain.NewMessage(event.TenantID, leadID, step.Channel, subject, body, &templateID, metadata, &scheduledAt)
		if err != nil {
			return err
		}
		messages = append(messages, message)
	}
	_, err = s.journeyRepo.EnrollJourney(ctx, ports.JourneyEnrollment{
		ID: enrollmentID.String(), TenantID: event.TenantID, JourneyID: journey.ID,
		EventID: event.ID, TriggerEvent: event.Type, LeadID: leadID,
		Messages: messages, SkippedSteps: skipped,
	})
	return err
}

func journeyRecipient(channel string, recipient ports.LeadWelcomeRecipient) (string, bool) {
	switch domain.NotificationChannel(channel) {
	case domain.ChannelEmail:
		return strings.ToLower(strings.TrimSpace(recipient.Email)), recipient.EmailEligible && strings.Contains(recipient.Email, "@")
	case domain.ChannelSMS:
		return strings.TrimSpace(recipient.Phone), recipient.SMSEligible && strings.TrimSpace(recipient.Phone) != ""
	case domain.ChannelWhatsApp:
		return strings.TrimSpace(recipient.Phone), recipient.WhatsAppEligible && strings.TrimSpace(recipient.Phone) != ""
	case domain.ChannelInApp:
		return "", true
	default:
		return "", false
	}
}

// prepareJourneyScheduled revalidates policy immediately before provider IO.
// Consent revocation and feature shutdown cancel the message; quiet hours and
// frequency caps defer it without consuming a delivery attempt.
func (s *Service) prepareJourneyScheduled(ctx context.Context, message *domain.Message) (bool, error) {
	journeyID := metadataString(message.Metadata, "journey_id")
	if journeyID == "" || s.journeyRepo == nil {
		return true, nil
	}
	journey, err := s.journeyRepo.GetJourney(ctx, message.TenantID, journeyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, s.cancelJourneyMessage(ctx, message, "journey no longer exists")
		}
		return false, err
	}
	switch domain.JourneyStatus(journey.Status) {
	case domain.JourneyStatusPaused:
		return false, s.deferJourneyMessage(ctx, message, journey.NextAllowedTime(time.Now().Add(time.Hour)))
	case domain.JourneyStatusActive:
	default:
		return false, s.cancelJourneyMessage(ctx, message, "journey is not active")
	}
	if !s.journeyChannelEnabled(ctx, message.TenantID, message.Channel) {
		return false, s.cancelJourneyMessage(ctx, message, "journey channel is disabled")
	}
	now := time.Now().UTC()
	allowed := journey.NextAllowedTime(now)
	if allowed.After(now.Add(time.Second)) {
		return false, s.deferJourneyMessage(ctx, message, allowed)
	}
	scheduledRepo, ok := s.messageRepo.(ports.ScheduledMessageRepository)
	if !ok {
		return false, domain.ErrUnavailable
	}
	if next, err := scheduledRepo.NextJourneyDeliveryAllowedAt(
		ctx, message.TenantID, journey.ID, message.RecipientID,
		time.Duration(journey.FrequencyWindowHours)*time.Hour, journey.FrequencyLimit,
	); err != nil {
		return false, err
	} else if next != nil && next.After(now) {
		return false, s.deferJourneyMessage(ctx, message, journey.NextAllowedTime(*next))
	}
	if message.Channel != string(domain.ChannelInApp) {
		if s.leadResolver == nil {
			return false, errors.New("notifications: journey lead resolver is not configured")
		}
		leadID := metadataString(message.Metadata, "lead_id")
		recipient, err := s.leadResolver.ResolveWelcomeRecipient(ctx, message.TenantID, leadID)
		if err != nil {
			return false, err
		}
		address, eligible := journeyRecipient(message.Channel, recipient)
		if !eligible {
			return false, s.cancelJourneyMessage(ctx, message, "recipient consent is no longer active")
		}
		message.Metadata["delivery_address"] = address
		message.Metadata["consent_verified_at"] = now.Format(time.RFC3339)
		if err := s.messageRepo.Update(ctx, message.TenantID, message); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *Service) deferJourneyMessage(ctx context.Context, message *domain.Message, at time.Time) error {
	at = at.UTC()
	message.ScheduledAt = &at
	message.UpdatedAt = time.Now().UTC()
	return s.messageRepo.Update(ctx, message.TenantID, message)
}

func (s *Service) cancelJourneyMessage(ctx context.Context, message *domain.Message, reason string) error {
	message.MarkCancelled(reason)
	if err := s.messageRepo.Update(ctx, message.TenantID, message); err != nil {
		return err
	}
	return s.finalizeJourneyEnrollment(ctx, message)
}

func (s *Service) finalizeJourneyEnrollment(ctx context.Context, message *domain.Message) error {
	enrollmentID := metadataString(message.Metadata, "journey_enrollment_id")
	if enrollmentID == "" || s.journeyRepo == nil {
		return nil
	}
	return s.journeyRepo.FinalizeJourneyEnrollment(ctx, message.TenantID, enrollmentID)
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func validJourneyLeadID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
