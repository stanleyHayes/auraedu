package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

// eventRule maps a consumed domain event to the notification it triggers.
type eventRule struct {
	// flagKey gates the side effect (agent_plan §9); the event is acked and
	// skipped when the flag is off for the tenant.
	flagKey string
	// channel is the preferred delivery channel. When the recipient has no
	// enabled subscription for it, the worker falls back to the in-app inbox.
	channel string
	// build derives recipient, subject and body from the event payload. It
	// reports ok=false when the event is not notification-worthy (e.g. a
	// student marked present).
	build func(data map[string]any) (recipient, subject, body string, ok bool)
}

type offerIssuedData struct {
	ApplicationID   string  `json:"application_id"`
	ApplicantUserID string  `json:"applicant_user_id"`
	LeadID          *string `json:"lead_id"`
	OfferExpiresAt  string  `json:"offer_expires_at"`
}

type offerDelivery struct {
	recipientID string
	channel     string
	metadata    map[string]any
}

// ruleForEvent returns the notification policy for a normalized event type.
// Keeping the registry immutable prevents tests or concurrent workers from
// changing production notification behavior at runtime.
func ruleForEvent(eventType string) (eventRule, bool) {
	switch eventType {
	case "intelligence.alert.changed":
		return eventRule{
			flagKey: "growth_reputation_monitor",
			channel: string(domain.ChannelInApp),
			build: func(data map[string]any) (string, string, string, bool) {
				reason := strings.TrimSpace(stringField(data, "reason"))
				if reason == "" {
					return "", "", "", false
				}
				return "", "Reputation threshold alert", reason + ". Review the evidence and assign an owner in the Reputation desk.", true
			},
		}, true
	case "payment.received":
		return eventRule{
			flagKey: FeatureEmailNotifications,
			channel: string(domain.ChannelEmail),
			build: func(data map[string]any) (string, string, string, bool) {
				recipient := firstField(data, "guardian_id", "user_id", "student_id")
				subject := "Payment received"
				body := fmt.Sprintf("A payment of %s was received for invoice %s.",
					amountField(data, "amount"), stringField(data, "invoice_id"))
				return recipient, subject, body, true
			},
		}, true
	case "invoice.created":
		return eventRule{
			flagKey: FeatureEmailNotifications,
			channel: string(domain.ChannelEmail),
			build: func(data map[string]any) (string, string, string, bool) {
				recipient := firstField(data, "guardian_id", "user_id", "student_id")
				subject := "New invoice issued"
				body := fmt.Sprintf("A new invoice for %s has been issued.", amountField(data, "amount_due"))
				if due := stringField(data, "due_date"); due != "" {
					body += fmt.Sprintf(" Payment is due on %s.", due)
				}
				return recipient, subject, body, true
			},
		}, true
	case "attendance.marked":
		return eventRule{
			flagKey: FeatureSMSNotifications,
			channel: string(domain.ChannelSMS),
			build: func(data map[string]any) (string, string, string, bool) {
				// Only guardian-alert-worthy statuses trigger a notification.
				status := stringField(data, "status")
				switch status {
				case "absent", "late":
				default:
					return "", "", "", false
				}
				recipient := firstField(data, "guardian_id", "user_id", "student_id")
				subject := "Attendance alert"
				body := fmt.Sprintf("A student was marked %s on %s.", status, stringField(data, "date"))
				return recipient, subject, body, true
			},
		}, true
	case "assessment.score_recorded":
		return eventRule{
			flagKey: FeatureEmailNotifications,
			channel: string(domain.ChannelEmail),
			build: func(data map[string]any) (string, string, string, bool) {
				recipient := firstField(data, "guardian_id", "user_id", "student_id")
				subject := "New score recorded"
				body := fmt.Sprintf("A score of %s was recorded.", amountField(data, "score"))
				if maximum := amountField(data, "max_score"); maximum != "" {
					body = fmt.Sprintf("A score of %s out of %s was recorded.", amountField(data, "score"), maximum)
				}
				return recipient, subject, body, true
			},
		}, true
	case "report.published":
		return eventRule{
			flagKey: FeatureEmailNotifications,
			channel: string(domain.ChannelEmail),
			build: func(data map[string]any) (string, string, string, bool) {
				recipient := firstField(data, "guardian_id", "user_id", "student_id")
				subject := "Report card published"
				body := "A report card has been published."
				return recipient, subject, body, true
			},
		}, true
	default:
		return eventRule{}, false
	}
}

// HandleCloudEvent applies the notification side effects for one consumed domain
// event: feature-flag gate → idempotency claim → create pending message →
// deliver via the channel notifier (MockNotifier today). Unknown event types,
// flag-off events and duplicate deliveries are acked without side effects.
// A non-nil error means the event should be redelivered.
func (s *Service) HandleCloudEvent(ctx context.Context, event tenancy.CloudEvent) error {
	base := strings.TrimSuffix(event.Type, ".v1")
	if base == "lead.created" {
		return s.handleLeadCreated(ctx, event)
	}
	if base == "offer.issued" {
		return s.handleOfferIssued(ctx, event)
	}
	if base == "offer.accepted" {
		return s.handleOfferAccepted(ctx, event)
	}
	rule, ok := ruleForEvent(base)
	if !ok {
		return nil
	}
	tenantID := event.TenantID
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, rule.flagKey) && !s.gates.IsEnabled(ctx, tenantID, FeaturePushNotifications) {
		slog.Default().InfoContext(ctx, "notification side effect skipped; feature disabled",
			"event_type", base, "flag", rule.flagKey, "tenant_id", tenantID)
		return nil
	}

	if s.processedRepo != nil {
		claimed, err := s.processedRepo.Claim(ctx, tenantID, event.ID, base)
		if err != nil {
			return err
		}
		if !claimed {
			// Duplicate redelivery of an already-processed event.
			return nil
		}
	}

	if err := s.dispatchEvent(ctx, tenantID, base, rule, event); err != nil {
		// Release the claim so the redelivery can retry the side effect.
		if s.processedRepo != nil {
			if rErr := s.processedRepo.Release(ctx, tenantID, event.ID); rErr != nil {
				slog.Default().ErrorContext(ctx, "failed to release processed-event claim", "event_id", event.ID, "err", rErr)
			}
		}
		return err
	}
	return nil
}

func (s *Service) handleOfferAccepted(ctx context.Context, event tenancy.CloudEvent) error {
	var data struct {
		ApplicationID string `json:"application_id"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil || strings.TrimSpace(data.ApplicationID) == "" {
		return fmt.Errorf("notifications: invalid offer.accepted payload")
	}
	repo, ok := s.messageRepo.(ports.ScheduledMessageRepository)
	if !ok {
		return nil
	}
	return repo.CancelByApplication(ctx, event.TenantID, data.ApplicationID)
}

// DeliverScheduled dispatches a message already claimed by the scheduler.
func (s *Service) DeliverScheduled(ctx context.Context, message *domain.Message) error {
	deliver, err := s.prepareJourneyScheduled(ctx, message)
	if err != nil || !deliver {
		return err
	}
	_, deliveryErr := s.deliver(ctx, message.TenantID, message)
	finalizeErr := s.finalizeJourneyEnrollment(ctx, message)
	return errors.Join(deliveryErr, finalizeErr)
}

// handleOfferIssued creates an immediate offer notice and a durable scheduled
// reminder. Email is used only when CRM confirms current consent; otherwise the
// applicant receives an in-app notice without exposing contact details in the event.
func (s *Service) handleOfferIssued(ctx context.Context, event tenancy.CloudEvent) error {
	tenantID := event.TenantID
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureNotifications) {
		return nil
	}
	if s.processedRepo != nil {
		claimed, err := s.processedRepo.Claim(ctx, tenantID, event.ID, "offer.issued")
		if err != nil || !claimed {
			return err
		}
	}
	data, expiresAt, err := parseOfferIssued(event.Data)
	if err != nil {
		return s.releaseProcessed(ctx, tenantID, event.ID, err)
	}
	delivery, err := s.resolveOfferDelivery(ctx, event, data)
	if err != nil {
		return s.releaseProcessed(ctx, tenantID, event.ID, err)
	}
	body := "Your admission offer is ready. Review and accept it before " +
		expiresAt.UTC().Format("2 January 2006 at 15:04 UTC") + "."
	err = s.createAndDeliverOffer(ctx, tenantID, delivery, body)
	if err != nil {
		return s.releaseProcessed(ctx, tenantID, event.ID, err)
	}
	err = s.createOfferReminder(ctx, tenantID, delivery, body, expiresAt)
	if err != nil {
		return s.releaseProcessed(ctx, tenantID, event.ID, err)
	}
	return nil
}

func parseOfferIssued(raw json.RawMessage) (offerIssuedData, time.Time, error) {
	var data offerIssuedData
	if err := json.Unmarshal(raw, &data); err != nil ||
		strings.TrimSpace(data.ApplicationID) == "" ||
		strings.TrimSpace(data.ApplicantUserID) == "" {
		return data, time.Time{}, errors.New("notifications: invalid offer.issued payload")
	}
	expiresAt, err := time.Parse(time.RFC3339, data.OfferExpiresAt)
	if err != nil {
		return data, time.Time{}, errors.New("notifications: invalid offer expiry")
	}
	return data, expiresAt, nil
}

func (s *Service) resolveOfferDelivery(
	ctx context.Context,
	event tenancy.CloudEvent,
	data offerIssuedData,
) (offerDelivery, error) {
	delivery := offerDelivery{
		recipientID: data.ApplicantUserID,
		channel:     string(domain.ChannelInApp),
		metadata: map[string]any{
			"event_id":         event.ID,
			"event_type":       "offer.issued",
			"application_id":   data.ApplicationID,
			"template_key":     "admissions_offer_v1",
			"consent_verified": false,
		},
	}
	if data.LeadID == nil || strings.TrimSpace(*data.LeadID) == "" || s.leadResolver == nil {
		return delivery, nil
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, event.TenantID, FeatureEmailNotifications) {
		return delivery, nil
	}
	recipient, err := s.leadResolver.ResolveWelcomeRecipient(ctx, event.TenantID, *data.LeadID)
	if err != nil {
		return offerDelivery{}, err
	}
	if recipient.Eligible {
		delivery.recipientID = *data.LeadID
		delivery.channel = string(domain.ChannelEmail)
		delivery.metadata["delivery_address"] = recipient.Email
		delivery.metadata["consent_verified"] = true
	}
	return delivery, nil
}

func (s *Service) createAndDeliverOffer(
	ctx context.Context,
	tenantID string,
	delivery offerDelivery,
	body string,
) error {
	immediate, err := domain.NewMessage(
		tenantID,
		delivery.recipientID,
		delivery.channel,
		"Your admission offer is ready",
		body,
		nil,
		delivery.metadata,
		nil,
	)
	if err != nil {
		return err
	}
	if err := s.messageRepo.Create(ctx, tenantID, immediate); err != nil {
		return err
	}
	_, err = s.deliver(ctx, tenantID, immediate)
	return err
}

func (s *Service) createOfferReminder(
	ctx context.Context,
	tenantID string,
	delivery offerDelivery,
	body string,
	expiresAt time.Time,
) error {
	reminderAt := expiresAt.Add(-48 * time.Hour)
	if reminderAt.Before(time.Now().UTC()) {
		reminderAt = time.Now().UTC()
	}
	reminderMetadata := make(map[string]any, len(delivery.metadata)+1)
	for key, value := range delivery.metadata {
		reminderMetadata[key] = value
	}
	reminderMetadata["template_key"] = "admissions_offer_reminder_v1"
	reminder, err := domain.NewMessage(
		tenantID,
		delivery.recipientID,
		delivery.channel,
		"Reminder: your admission offer expires soon",
		body,
		nil,
		reminderMetadata,
		&reminderAt,
	)
	if err != nil {
		return err
	}
	return s.messageRepo.Create(ctx, tenantID, reminder)
}

func (s *Service) releaseProcessed(ctx context.Context, tenantID, eventID string, err error) error {
	if s.processedRepo != nil {
		if releaseErr := s.processedRepo.Release(ctx, tenantID, eventID); releaseErr != nil {
			return errors.Join(err, fmt.Errorf("notifications: release processed-event claim: %w", releaseErr))
		}
	}
	return err
}

// handleLeadCreated sends the approved welcome copy only after CRM confirms
// current channel consent over the private service boundary. Email remains the
// first choice, followed by WhatsApp and SMS when their tenant features and
// capture-time consent are both active.
func (s *Service) handleLeadCreated(ctx context.Context, event tenancy.CloudEvent) error {
	tenantID := event.TenantID
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, "growth_crm") {
		return nil
	}
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureEmailNotifications) &&
		!s.gates.IsEnabled(ctx, tenantID, FeatureWhatsAppNotifications) &&
		!s.gates.IsEnabled(ctx, tenantID, FeatureSMSNotifications) {
		return nil
	}
	if s.leadResolver == nil {
		return fmt.Errorf("notifications: lead resolver is not configured")
	}
	if s.processedRepo != nil {
		claimed, err := s.processedRepo.Claim(ctx, tenantID, event.ID, "lead.created")
		if err != nil || !claimed {
			return err
		}
	}
	var data struct {
		LeadID string `json:"lead_id"`
	}
	if err := json.Unmarshal(event.Data, &data); err != nil || strings.TrimSpace(data.LeadID) == "" {
		return s.releaseProcessed(ctx, tenantID, event.ID, errors.New("notifications: invalid lead.created payload"))
	}
	recipient, err := s.leadResolver.ResolveWelcomeRecipient(ctx, tenantID, data.LeadID)
	if err != nil {
		return s.releaseProcessed(ctx, tenantID, event.ID, err)
	}
	channel, address, templateKey, eligible := s.welcomeDelivery(ctx, tenantID, recipient)
	if !eligible {
		return nil
	}
	name := strings.TrimSpace(recipient.FirstName)
	if name == "" {
		name = "there"
	}
	body := fmt.Sprintf(
		"Hello %s, thank you for your interest. Our admissions team has received your enquiry and will be in touch.",
		name,
	)
	metadata := map[string]any{
		"event_id":         event.ID,
		"event_type":       "lead.created",
		"lead_id":          data.LeadID,
		"delivery_address": address,
		"template_key":     templateKey,
		"consent_verified": true,
	}
	m, err := domain.NewMessage(
		tenantID,
		data.LeadID,
		channel,
		"Welcome to AuraEDU",
		body,
		nil,
		metadata,
		nil,
	)
	if err == nil {
		err = s.messageRepo.Create(ctx, tenantID, m)
	}
	if err == nil {
		_, err = s.deliver(ctx, tenantID, m)
	}
	if err != nil {
		return s.releaseProcessed(ctx, tenantID, event.ID, err)
	}
	return nil
}

func (s *Service) welcomeDelivery(
	ctx context.Context,
	tenantID string,
	recipient ports.LeadWelcomeRecipient,
) (channel, address, templateKey string, eligible bool) {
	enabled := func(flag string) bool { return s.gates == nil || s.gates.IsEnabled(ctx, tenantID, flag) }
	switch {
	case (recipient.EmailEligible || recipient.Eligible) && strings.TrimSpace(recipient.Email) != "" && enabled(FeatureEmailNotifications):
		return string(domain.ChannelEmail), recipient.Email, "growth_welcome_email_v1", true
	case recipient.WhatsAppEligible && strings.TrimSpace(recipient.Phone) != "" && enabled(FeatureWhatsAppNotifications):
		return string(domain.ChannelWhatsApp), recipient.Phone, "growth_welcome_whatsapp_v1", true
	case recipient.SMSEligible && strings.TrimSpace(recipient.Phone) != "" && enabled(FeatureSMSNotifications):
		return string(domain.ChannelSMS), recipient.Phone, "growth_welcome_sms_v1", true
	default:
		return "", "", "", false
	}
}

// dispatchEvent builds the message for one event and sends it.
func (s *Service) dispatchEvent(ctx context.Context, tenantID, base string, rule eventRule, event tenancy.CloudEvent) error {
	var data map[string]any
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("notifications: decode %s payload: %w", base, err)
		}
	}

	recipient, subject, body, ok := rule.build(data)
	if !ok {
		return nil
	}

	channel := rule.channel
	metadata := map[string]any{
		"event_id":   event.ID,
		"event_type": base,
		"source":     "worker",
	}
	switch {
	case recipient == "":
		// No resolvable recipient in the payload: drop the notification into the
		// tenant's in-app inbox. Legacy UUID tenant IDs remain stable; canonical
		// tenant codes receive a deterministic UUID because messages.recipient_id
		// is deliberately UUID-typed.
		recipient = tenantID
		if _, err := uuid.Parse(recipient); err != nil {
			recipient = uuid.NewSHA1(uuid.NameSpaceOID, []byte("auraedu-tenant-inbox\x00"+tenantID)).String()
		}
		channel = string(domain.ChannelInApp)
		metadata["audience"] = "tenant"
		metadata["tenant_code"] = tenantID
	case s.gates != nil && s.gates.IsEnabled(ctx, tenantID, FeaturePushNotifications) &&
		s.hasActivePushDevice(ctx, tenantID, recipient):
		// OS permission plus an active installation token is the recipient's push
		// opt-in. Prefer it for time-sensitive school events.
		channel = string(domain.ChannelPush)
	case s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, rule.flagKey):
		channel = string(domain.ChannelInApp)
	case channel != string(domain.ChannelInApp) &&
		!s.hasEnabledSubscription(ctx, tenantID, recipient, channel):
		// Preferred channel unavailable for this recipient: fall back to in-app.
		channel = string(domain.ChannelInApp)
	}

	m, err := domain.NewMessage(tenantID, recipient, channel, subject, body, nil, metadata, nil)
	if err != nil {
		return err
	}
	if err := s.messageRepo.Create(ctx, tenantID, m); err != nil {
		return err
	}
	_, err = s.deliver(ctx, tenantID, m)
	return err
}

func (s *Service) hasActivePushDevice(ctx context.Context, tenantID, userID string) bool {
	if s.devices == nil {
		return false
	}
	devices, err := s.devices.ListActive(ctx, tenantID, userID)
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to resolve active push device", "user_id", userID, "err", err)
		return false
	}
	return len(devices) > 0
}

// hasEnabledSubscription reports whether the recipient has an enabled
// subscription for the channel. When no subscription repository is configured
// the preferred channel is kept.
func (s *Service) hasEnabledSubscription(ctx context.Context, tenantID, userID, channel string) bool {
	if s.subscriptionRepo == nil {
		return true
	}
	subs, _, err := s.subscriptionRepo.List(ctx, tenantID, ports.SubscriptionFilter{
		Limit:   1,
		Channel: channel,
		UserID:  userID,
	})
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to check channel subscription", "channel", channel, "err", err)
		return false
	}
	return len(subs) > 0 && subs[0].IsEnabled
}

// firstField returns the first non-empty string value among keys.
func firstField(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := stringField(data, k); v != "" {
			return v
		}
	}
	return ""
}

func stringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// amountField renders a numeric payload field compactly (72.5 → "72.5", 72 → "72").
func amountField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	switch v := data[key].(type) {
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	case string:
		return strings.TrimSpace(v)
	}
	return ""
}
