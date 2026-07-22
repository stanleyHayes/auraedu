// Package application implements notification use cases and RBAC gates.
package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
)

// RBAC permission keys.
const (
	PermRead   = "notifications.read"
	PermSend   = "notifications.send"
	PermManage = "notifications.manage"
)

// FeatureNotifications is the feature flag key for the notification service.
const FeatureNotifications = "notifications"

// FeatureAnnouncements is the feature flag key gating the announcements API.
const FeatureAnnouncements = "announcements"

// Channel feature flag keys used by the worker when consuming domain events.
const (
	FeatureEmailNotifications    = "email_notifications"
	FeatureSMSNotifications      = "sms_notifications"
	FeatureWhatsAppNotifications = "whatsapp_notifications"
	FeaturePushNotifications     = "push_notifications"
)

// Service holds the notification use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	messageRepo      ports.MessageRepository
	templateRepo     ports.TemplateRepository
	subscriptionRepo ports.SubscriptionRepository
	announcementRepo ports.AnnouncementRepository
	processedRepo    ports.ProcessedEventRepository
	journeyRepo      ports.JourneyRepository
	leadResolver     ports.LeadResolver
	pub              ports.EventPublisher
	notifiers        map[string]ports.Notifier
	gates            flags.Gate
	devices          ports.DeviceTokenRepository
	metrics          deliveryMetrics
	publicAppURL     string
	unsubscribe      *UnsubscribeManager
}

// DeliverTransactionalEmail delivers a security-sensitive email without
// placing its recipient or token on the event bus. The persisted record is
// deliberately redacted after the notifier has consumed the full body.
func (s *Service) DeliverTransactionalEmail(ctx context.Context, tenantID, recipient, template string, data map[string]any) (*domain.Message, error) {
	if strings.TrimSpace(tenantID) == "" || !strings.Contains(recipient, "@") {
		return nil, fmt.Errorf("%w: tenant_id and valid recipient are required", domain.ErrValidation)
	}
	var subject, body string
	switch template {
	case "user_invite":
		token := stringField(data, "invite_token")
		role := stringField(data, "role")
		if token == "" {
			return nil, fmt.Errorf("%w: invite_token is required", domain.ErrValidation)
		}
		subject = "You are invited to AuraEDU"
		// Keep the one-time credential in the URL fragment. Browsers do not send
		// fragments to the web server, reverse proxy, or access logs.
		acceptURL := strings.TrimRight(s.publicAppURL, "/") +
			"/accept-invite?tenant=" + url.QueryEscape(strings.ToLower(strings.TrimSpace(tenantID))) +
			"#token=" + url.QueryEscape(token)
		body = fmt.Sprintf("You have been invited as %s. Accept your invitation securely: %s", role, acceptURL)
	case "password_reset":
		token := stringField(data, "reset_token")
		if token == "" {
			return nil, fmt.Errorf("%w: reset_token is required", domain.ErrValidation)
		}
		subject = "Reset your AuraEDU password"
		resetURL := strings.TrimRight(s.publicAppURL, "/") +
			"/reset-password?tenant=" + url.QueryEscape(strings.ToLower(strings.TrimSpace(tenantID))) +
			"#token=" + url.QueryEscape(token)
		body = "Reset your password securely within 15 minutes: " + resetURL
	default:
		return nil, fmt.Errorf("%w: unsupported transactional template", domain.ErrValidation)
	}
	deliveryAddress := strings.ToLower(strings.TrimSpace(recipient))
	recipientID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(tenantID+"\x00"+deliveryAddress)).String()
	m, err := domain.NewMessage(tenantID, recipientID, string(domain.ChannelEmail), subject, body, nil,
		map[string]any{"transactional_template": template, "sensitive_content_redacted": true, "delivery_address": deliveryAddress}, nil)
	if err != nil {
		return nil, err
	}
	if err := s.prepareProviderDelivery(ctx, tenantID, m, nil); err != nil {
		m.Body = "[transactional content redacted before delivery]"
		delete(m.Metadata, "delivery_address")
		m.MarkFailed(err.Error())
		if persistErr := s.persistDeliveryOutcome(ctx, tenantID, m, "", true, err); persistErr != nil {
			return nil, persistErr
		}
		return m, err
	}
	n, ok := s.notifiers[string(domain.ChannelEmail)]
	if !ok {
		reason := "email notifier is not configured"
		m.Body = "[transactional content redacted before delivery]"
		delete(m.Metadata, "delivery_address")
		m.MarkFailed(reason)
		if err := s.persistDeliveryOutcome(ctx, tenantID, m, "", true, errors.New(reason)); err != nil {
			return nil, err
		}
		return m, fmt.Errorf("%w: %s", domain.ErrValidation, reason)
	}
	deliveryStarted := time.Now()
	receipt, deliveryErr := sendWithReceipt(ctx, n, *m)
	if deliveryErr != nil {
		s.metrics.observe(ctx, string(domain.ChannelEmail), "failed", deliveryStarted)
	} else {
		s.metrics.observe(ctx, string(domain.ChannelEmail), "success", deliveryStarted)
	}
	m.Body = "[transactional content redacted after delivery attempt]"
	delete(m.Metadata, "delivery_address")
	if deliveryErr != nil {
		m.MarkFailed(deliveryErr.Error())
	} else {
		m.MarkSent()
		if receipt != nil {
			m.MarkProviderAccepted(receipt.Provider, time.Now().UTC())
		}
	}
	providerMessageID := ""
	if receipt != nil {
		providerMessageID = receipt.MessageID
	}
	if err := s.persistDeliveryOutcome(ctx, tenantID, m, providerMessageID, true, deliveryErr); err != nil {
		return nil, err
	}
	if deliveryErr != nil {
		return m, fmt.Errorf("transactional email delivery failed: %w", deliveryErr)
	}
	return m, nil
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithNotifiers sets the channel-specific notifier adapters.
func WithNotifiers(n map[string]ports.Notifier) Option { return func(s *Service) { s.notifiers = n } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

// WithAnnouncementRepository sets the announcement repository.
func WithAnnouncementRepository(r ports.AnnouncementRepository) Option {
	return func(s *Service) { s.announcementRepo = r }
}

// WithProcessedEventRepository sets the worker idempotency ledger.
func WithProcessedEventRepository(r ports.ProcessedEventRepository) Option {
	return func(s *Service) { s.processedRepo = r }
}

// WithJourneyRepository enables tenant-owned communication journey workflows.
func WithJourneyRepository(r ports.JourneyRepository) Option {
	return func(s *Service) { s.journeyRepo = r }
}

func WithLeadResolver(r ports.LeadResolver) Option { return func(s *Service) { s.leadResolver = r } }
func WithDeviceTokenRepository(r ports.DeviceTokenRepository) Option {
	return func(s *Service) { s.devices = r }
}

// WithPublicAppURL sets the trusted web origin used in security-sensitive transactional links.
func WithPublicAppURL(value string) Option {
	return func(s *Service) { s.publicAppURL = strings.TrimRight(value, "/") }
}

func WithUnsubscribeManager(manager *UnsubscribeManager) Option {
	return func(s *Service) { s.unsubscribe = manager }
}

// ValidatePublicAppURL rejects origins that could disclose one-time tokens to an
// attacker-controlled host. Production links must use a clean HTTPS origin.
func ValidatePublicAppURL(value string, production bool) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return errors.New("PUBLIC_APP_URL must be a credential-free origin")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("PUBLIC_APP_URL must use HTTP or HTTPS")
	}
	if production {
		host := strings.ToLower(parsed.Hostname())
		if parsed.Scheme != "https" || host == "localhost" || host == "127.0.0.1" || strings.HasSuffix(host, ".example") {
			return errors.New("production PUBLIC_APP_URL must be a non-placeholder HTTPS origin")
		}
	}
	return nil
}

type noopPublisher struct{}

func (noopPublisher) PublishMessageSent(context.Context, *domain.Message) error           { return nil }
func (noopPublisher) PublishMessageFailed(context.Context, *domain.Message, string) error { return nil }

// NewService constructs the application service.
func NewService(
	messageRepo ports.MessageRepository,
	templateRepo ports.TemplateRepository,
	subscriptionRepo ports.SubscriptionRepository,
	opts ...Option,
) *Service {
	s := &Service{
		messageRepo:      messageRepo,
		templateRepo:     templateRepo,
		subscriptionRepo: subscriptionRepo,
		pub:              noopPublisher{},
		notifiers:        map[string]ports.Notifier{},
		gates:            flags.NewStaticSnapshot(),
		metrics:          newDeliveryMetrics(),
		publicAppURL:     "http://localhost:3000",
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// --- Message use cases ---

// CreateMessageRequest is the input for creating a message.
type CreateMessageRequest struct {
	RecipientID string
	Channel     string
	TemplateID  *string
	Subject     string
	Body        string
	Metadata    map[string]any
	ScheduledAt *string
}

// UpdateMessageRequest is the input for patching a message.
type UpdateMessageRequest struct {
	RecipientID *string
	Channel     *string
	TemplateID  *string
	Subject     *string
	Body        *string
	Status      *string
	Metadata    map[string]any
	ScheduledAt *string
	SentAt      *string
	Error       *string
}

// CreateMessage validates and persists a new Message.
func (s *Service) CreateMessage(ctx context.Context, actor auth.Actor, req CreateMessageRequest) (*domain.Message, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}

	var scheduledAt *time.Time
	if req.ScheduledAt != nil {
		t, err := parseTime(*req.ScheduledAt)
		if err != nil {
			return nil, fmt.Errorf("%w: scheduled_at must be RFC3339", domain.ErrValidation)
		}
		scheduledAt = &t
	}

	m, err := domain.NewMessage(tenantID, req.RecipientID, req.Channel, req.Subject, req.Body, req.TemplateID, req.Metadata, scheduledAt)
	if err != nil {
		return nil, err
	}
	if err := s.messageRepo.Create(ctx, tenantID, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) RegisterDeviceToken(ctx context.Context, actor auth.Actor, deviceID, platform, token string) (*domain.DeviceToken, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	if s.devices == nil {
		return nil, domain.ErrUnavailable
	}
	device, err := domain.NewDeviceToken(tenantID, actor.UserID, deviceID, platform, token)
	if err != nil {
		return nil, err
	}
	return s.devices.Upsert(ctx, tenantID, device)
}

func (s *Service) UnregisterDeviceToken(ctx context.Context, actor auth.Actor, deviceID string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return err
	}
	if s.devices == nil {
		return domain.ErrUnavailable
	}
	if strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("%w: device_id is required", domain.ErrValidation)
	}
	return s.devices.DeleteByDevice(ctx, tenantID, actor.UserID, strings.TrimSpace(deviceID))
}

// ListMessages returns a tenant-scoped page of messages.
func (s *Service) ListMessages(ctx context.Context, actor auth.Actor, filter ports.MessageFilter) ([]*domain.Message, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.messageRepo.List(ctx, tenantID, filter)
}

// GetMessage returns a single message.
func (s *Service) GetMessage(ctx context.Context, actor auth.Actor, id string) (*domain.Message, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.messageRepo.GetByID(ctx, tenantID, id)
}

// UpdateMessage patches a message.
func (s *Service) UpdateMessage(ctx context.Context, actor auth.Actor, id string, req UpdateMessageRequest) (*domain.Message, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	m, err := s.messageRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	patch, err := toMessagePatch(req)
	if err != nil {
		return nil, err
	}

	changed, err := m.ApplyUpdate(patch)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return m, nil
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := s.messageRepo.Update(ctx, tenantID, m); err != nil {
		return nil, err
	}
	return m, nil
}

// DeleteMessage removes a message.
func (s *Service) DeleteMessage(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.messageRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.messageRepo.Delete(ctx, tenantID, id)
}

// SendMessage dispatches a pending message through its channel notifier.
func (s *Service) SendMessage(ctx context.Context, actor auth.Actor, id string) (*domain.Message, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermSend)
	if err != nil {
		return nil, err
	}
	m, err := s.messageRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if m.Status == string(domain.MessageStatusSent) {
		return m, nil
	}
	if m.Status == string(domain.MessageStatusCancelled) {
		return nil, fmt.Errorf("%w: cannot send a canceled message", domain.ErrValidation)
	}

	// Verify the recipient is subscribed to this channel.
	subs, _, err := s.subscriptionRepo.List(ctx, tenantID, ports.SubscriptionFilter{
		Limit:   1,
		Channel: m.Channel,
		UserID:  m.RecipientID,
	})
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 || !subs[0].IsEnabled {
		reason := "recipient is not subscribed to " + m.Channel
		m.MarkFailed(reason)
		if err := s.persistDeliveryOutcome(ctx, tenantID, m, "", false, errors.New(reason)); err != nil {
			return nil, err
		}
		return m, fmt.Errorf("%w: %s", domain.ErrValidation, reason)
	}

	return s.deliver(ctx, tenantID, m)
}

// deliver dispatches a pending message through its channel notifier and records
// the outcome. It is shared by the HTTP send path (after RBAC + subscription
// checks) and internal flows (worker side effects, announcements) which perform
// their own gating. Delivery failures mark the message failed and publish
// notification.failed.v1; success publishes notification.sent.v1.
func (s *Service) deliver(ctx context.Context, tenantID string, m *domain.Message) (*domain.Message, error) {
	notifier, ok := s.notifiers[m.Channel]
	if !ok {
		s.metrics.observe(ctx, m.Channel, "unconfigured", time.Now())
		reason := "no notifier configured for channel " + m.Channel
		m.MarkFailed(reason)
		if err := s.persistDeliveryOutcome(ctx, tenantID, m, "", false, errors.New(reason)); err != nil {
			return nil, err
		}
		return m, fmt.Errorf("%w: %s", domain.ErrValidation, reason)
	}
	minimizeAddress := shouldMinimizeDeliveryAddress(notifier, m.Channel)
	if err := s.prepareProviderDelivery(ctx, tenantID, m, notifier); err != nil {
		if minimizeAddress {
			delete(m.Metadata, "delivery_address")
		}
		m.MarkFailed(err.Error())
		if persistErr := s.persistDeliveryOutcome(ctx, tenantID, m, "", false, err); persistErr != nil {
			return nil, persistErr
		}
		return m, err
	}

	deliveryStarted := time.Now()
	deliveryMessage, err := s.emailWithUnsubscribeLink(*m)
	if err != nil {
		if minimizeAddress {
			delete(m.Metadata, "delivery_address")
		}
		m.MarkFailed(err.Error())
		if persistErr := s.persistDeliveryOutcome(ctx, tenantID, m, "", false, err); persistErr != nil {
			return nil, persistErr
		}
		return m, err
	}
	receipt, err := sendWithReceipt(ctx, notifier, deliveryMessage)
	if minimizeAddress {
		delete(m.Metadata, "delivery_address")
	}
	if err != nil {
		s.metrics.observe(ctx, m.Channel, "failed", deliveryStarted)
		reason := err.Error()
		m.MarkFailed(reason)
		if uErr := s.persistDeliveryOutcome(ctx, tenantID, m, "", false, err); uErr != nil {
			return nil, uErr
		}
		return m, fmt.Errorf("%w: send failed: %w", domain.ErrValidation, err)
	}
	s.metrics.observe(ctx, m.Channel, "success", deliveryStarted)

	m.MarkSent()
	providerMessageID := ""
	if receipt != nil {
		providerMessageID = receipt.MessageID
		m.MarkProviderAccepted(receipt.Provider, time.Now().UTC())
	}
	if err := s.persistDeliveryOutcome(ctx, tenantID, m, providerMessageID, false, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) persistDeliveryOutcome(
	ctx context.Context,
	tenantID string,
	m *domain.Message,
	providerMessageID string,
	create bool,
	deliveryErr error,
) error {
	eventType := "notification.sent.v1"
	payload := ports.MessageSentEventData(m)
	if deliveryErr != nil {
		eventType = "notification.failed.v1"
		payload = ports.MessageFailedEventData(m, deliveryErr.Error())
	}
	if durable, ok := s.messageRepo.(ports.DurableDeliveryRepository); ok {
		return durable.CommitDeliveryOutcome(ctx, tenantID, m, providerMessageID, create, eventType, payload)
	}
	var err error
	if create {
		err = s.messageRepo.Create(ctx, tenantID, m)
	} else {
		err = s.messageRepo.Update(ctx, tenantID, m)
	}
	if err != nil {
		return err
	}
	if deliveryErr != nil {
		err = s.pub.PublishMessageFailed(ctx, m, deliveryErr.Error())
	} else {
		err = s.pub.PublishMessageSent(ctx, m)
	}
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish delivery event", "message_id", m.ID, "event_type", eventType, "err", err)
	}
	return nil
}

func sendWithReceipt(ctx context.Context, notifier ports.Notifier, message domain.Message) (*ports.ProviderReceipt, error) {
	if withReceipt, ok := notifier.(ports.ReceiptNotifier); ok {
		receipt, err := withReceipt.SendWithReceipt(ctx, message)
		if err != nil {
			return nil, err
		}
		return &receipt, nil
	}
	return nil, notifier.Send(ctx, message)
}

func (s *Service) prepareProviderDelivery(ctx context.Context, tenantID string, message *domain.Message, notifier ports.Notifier) error {
	if message.Metadata == nil {
		message.Metadata = map[string]any{}
	}
	address, ok := message.Metadata["delivery_address"].(string)
	if !ok {
		address = ""
	}
	switch message.Channel {
	case string(domain.ChannelEmail):
		if address == "" && strings.Contains(message.RecipientID, "@") {
			address = message.RecipientID
		}
		address = strings.ToLower(strings.TrimSpace(address))
		if address == "" {
			return nil
		}
		if !strings.Contains(address, "@") {
			return fmt.Errorf("%w: valid email delivery_address is required", domain.ErrValidation)
		}
	case string(domain.ChannelSMS), string(domain.ChannelWhatsApp):
		if _, receiptCapable := notifier.(ports.ReceiptNotifier); !receiptCapable {
			return nil
		}
		if address == "" {
			address = message.RecipientID
		}
		address = normalizeDeliveryPhone(address)
		if !validE164DeliveryAddress(address) {
			return fmt.Errorf("%w: valid E.164 delivery_address is required", domain.ErrValidation)
		}
	default:
		return nil
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(address)))
	message.Metadata["delivery_address"] = address
	message.Metadata["delivery_address_hash"] = hash
	if message.Channel == string(domain.ChannelEmail) {
		feedback, ok := s.messageRepo.(ports.DeliveryFeedbackRepository)
		if !ok {
			return nil
		}
		suppressed, err := feedback.IsEmailSuppressed(ctx, tenantID, hash)
		if err != nil {
			return fmt.Errorf("%w: email suppression check unavailable", domain.ErrUnavailable)
		}
		if suppressed {
			return fmt.Errorf("%w: recipient email is suppressed", domain.ErrValidation)
		}
	}
	return nil
}

func shouldMinimizeDeliveryAddress(notifier ports.Notifier, channel string) bool {
	if _, receiptCapable := notifier.(ports.ReceiptNotifier); !receiptCapable {
		return false
	}
	return channel == string(domain.ChannelEmail) || channel == string(domain.ChannelSMS) ||
		channel == string(domain.ChannelWhatsApp)
}

func normalizeDeliveryPhone(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("whatsapp:") && strings.EqualFold(value[:len("whatsapp:")], "whatsapp:") {
		value = strings.TrimSpace(value[len("whatsapp:"):])
	}
	return value
}

func validE164DeliveryAddress(value string) bool {
	if len(value) < 9 || len(value) > 16 || value[0] != '+' || value[1] < '1' || value[1] > '9' {
		return false
	}
	for _, digit := range value[2:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// ApplyDeliveryFeedback stores a verified provider lifecycle event. Signature
// verification and payload minimization happen in the provider adapter.
func (s *Service) ApplyDeliveryFeedback(ctx context.Context, feedback ports.DeliveryFeedback) (bool, error) {
	repository, ok := s.messageRepo.(ports.DeliveryFeedbackRepository)
	if !ok {
		return false, domain.ErrUnavailable
	}
	return repository.ApplyDeliveryFeedback(ctx, feedback)
}

func (s *Service) emailWithUnsubscribeLink(message domain.Message) (domain.Message, error) {
	if message.Channel != string(domain.ChannelEmail) || s.unsubscribe == nil {
		return message, nil
	}
	consented, ok := message.Metadata["consent_verified"].(bool)
	if !ok {
		consented = false
	}
	if !consented && metadataString(message.Metadata, "journey_id") == "" {
		return message, nil
	}
	addressHash := metadataString(message.Metadata, "delivery_address_hash")
	link, err := s.unsubscribe.Link(message.TenantID, addressHash)
	if err != nil {
		return message, fmt.Errorf("%w: cannot create email preference link", domain.ErrUnavailable)
	}
	message.Body += "\n\nStop these admissions updates: " + link
	return message, nil
}

func (s *Service) UnsubscribeEmail(ctx context.Context, token string) error {
	if s.unsubscribe == nil {
		return domain.ErrUnavailable
	}
	repository, ok := s.messageRepo.(ports.DeliveryFeedbackRepository)
	if !ok {
		return domain.ErrUnavailable
	}
	tenantID, addressHash, err := s.unsubscribe.Verify(token)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return repository.SuppressEmail(ctx, tenantID, addressHash, "unsubscribed", fmt.Sprintf("unsubscribe:%x", digest[:]), time.Now().UTC())
}

// --- Template use cases ---

// CreateTemplateRequest is the input for creating a template.
type CreateTemplateRequest struct {
	Name            string
	Channel         string
	SubjectTemplate string
	BodyTemplate    string
}

// UpdateTemplateRequest is the input for patching a template.
type UpdateTemplateRequest struct {
	Name            *string
	Channel         *string
	SubjectTemplate *string
	BodyTemplate    *string
	Status          *string
}

// CreateTemplate validates and persists a new Template.
func (s *Service) CreateTemplate(ctx context.Context, actor auth.Actor, req CreateTemplateRequest) (*domain.Template, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	t, err := domain.NewTemplate(tenantID, req.Name, req.Channel, req.SubjectTemplate, req.BodyTemplate)
	if err != nil {
		return nil, err
	}
	if err := s.templateRepo.Create(ctx, tenantID, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ListTemplates returns a tenant-scoped page of templates.
func (s *Service) ListTemplates(ctx context.Context, actor auth.Actor, filter ports.TemplateFilter) ([]*domain.Template, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.templateRepo.List(ctx, tenantID, filter)
}

// GetTemplate returns a single template.
func (s *Service) GetTemplate(ctx context.Context, actor auth.Actor, id string) (*domain.Template, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.templateRepo.GetByID(ctx, tenantID, id)
}

// UpdateTemplate patches a template.
func (s *Service) UpdateTemplate(ctx context.Context, actor auth.Actor, id string, req UpdateTemplateRequest) (*domain.Template, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	t, err := s.templateRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	patch := domain.TemplatePatch{
		Name:            req.Name,
		Channel:         req.Channel,
		SubjectTemplate: req.SubjectTemplate,
		BodyTemplate:    req.BodyTemplate,
		Status:          req.Status,
	}
	changed, err := t.ApplyUpdate(patch)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return t, nil
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := s.templateRepo.Update(ctx, tenantID, t); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteTemplate removes a template.
func (s *Service) DeleteTemplate(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.templateRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.templateRepo.Delete(ctx, tenantID, id)
}

// --- Subscription use cases ---

// CreateSubscriptionRequest is the input for creating a subscription.
type CreateSubscriptionRequest struct {
	UserID    string
	Channel   string
	IsEnabled bool
}

// UpdateSubscriptionRequest is the input for patching a subscription.
type UpdateSubscriptionRequest struct {
	Channel   *string
	IsEnabled *bool
}

// CreateSubscription validates and persists a new Subscription.
func (s *Service) CreateSubscription(ctx context.Context, actor auth.Actor, req CreateSubscriptionRequest) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	sub, err := domain.NewSubscription(tenantID, req.UserID, req.Channel, req.IsEnabled)
	if err != nil {
		return nil, err
	}
	if err := s.subscriptionRepo.Create(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// ListSubscriptions returns a tenant-scoped page of subscriptions.
func (s *Service) ListSubscriptions(ctx context.Context, actor auth.Actor, filter ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, "", err
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.subscriptionRepo.List(ctx, tenantID, filter)
}

// GetSubscription returns a single subscription.
func (s *Service) GetSubscription(ctx context.Context, actor auth.Actor, id string) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermRead)
	if err != nil {
		return nil, err
	}
	return s.subscriptionRepo.GetByID(ctx, tenantID, id)
}

// UpdateSubscription patches a subscription.
func (s *Service) UpdateSubscription(ctx context.Context, actor auth.Actor, id string, req UpdateSubscriptionRequest) (*domain.Subscription, error) {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return nil, err
	}
	sub, err := s.subscriptionRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	patch := domain.SubscriptionPatch{
		Channel:   req.Channel,
		IsEnabled: req.IsEnabled,
	}
	changed, err := sub.ApplyUpdate(patch)
	if err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return sub, nil
	}
	if err := sub.Validate(); err != nil {
		return nil, err
	}
	if err := s.subscriptionRepo.Update(ctx, tenantID, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

// DeleteSubscription removes a subscription.
func (s *Service) DeleteSubscription(ctx context.Context, actor auth.Actor, id string) error {
	tenantID, err := s.requireAccess(ctx, actor, PermManage)
	if err != nil {
		return err
	}
	if _, err := s.subscriptionRepo.GetByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.subscriptionRepo.Delete(ctx, tenantID, id)
}

// --- helpers ---

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
	if s.gates != nil && !s.gates.IsEnabled(ctx, tenantID, FeatureNotifications) {
		return "", fmt.Errorf("%w: %s", flags.ErrFeatureDisabled, FeatureNotifications)
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

func parseTime(v string) (time.Time, error) {
	return time.Parse(time.RFC3339, v)
}

func toMessagePatch(req UpdateMessageRequest) (domain.MessagePatch, error) {
	patch := domain.MessagePatch{
		RecipientID: req.RecipientID,
		Channel:     req.Channel,
		TemplateID:  req.TemplateID,
		Subject:     req.Subject,
		Body:        req.Body,
		Status:      req.Status,
		Metadata:    req.Metadata,
		Error:       req.Error,
	}
	if req.ScheduledAt != nil {
		t, err := parseTime(*req.ScheduledAt)
		if err != nil {
			return domain.MessagePatch{}, fmt.Errorf("%w: scheduled_at must be RFC3339", domain.ErrValidation)
		}
		patch.ScheduledAt = &t
	}
	if req.SentAt != nil {
		t, err := parseTime(*req.SentAt)
		if err != nil {
			return domain.MessagePatch{}, fmt.Errorf("%w: sent_at must be RFC3339", domain.ErrValidation)
		}
		patch.SentAt = &t
	}
	return patch, nil
}

// IsNotFound reports whether an error is a not-found domain error.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

// IsValidation reports whether an error is a validation domain error.
func IsValidation(err error) bool { return errors.Is(err, domain.ErrValidation) }

// IsFeatureDisabled reports whether an error is a feature-disabled error.
func IsFeatureDisabled(err error) bool { return errors.Is(err, flags.ErrFeatureDisabled) }
