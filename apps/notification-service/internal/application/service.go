// Package application implements notification use cases and RBAC gates.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

// RBAC permission keys.
const (
	PermRead   = "notifications.read"
	PermSend   = "notifications.send"
	PermManage = "notifications.manage"
)

// FeatureNotifications is the feature flag key for the notification service.
const FeatureNotifications = "notifications"

// Service holds the notification use cases. Tenant scope + RBAC + feature-flag checks
// belong here (agent_plan §5), never in HTTP handlers.
type Service struct {
	messageRepo      ports.MessageRepository
	templateRepo     ports.TemplateRepository
	subscriptionRepo ports.SubscriptionRepository
	pub              ports.EventPublisher
	notifiers        map[string]ports.Notifier
	gates            flags.Gate
}

// Option configures the service.
type Option func(*Service)

// WithPublisher sets the event publisher.
func WithPublisher(pub ports.EventPublisher) Option { return func(s *Service) { s.pub = pub } }

// WithNotifiers sets the channel-specific notifier adapters.
func WithNotifiers(n map[string]ports.Notifier) Option { return func(s *Service) { s.notifiers = n } }

// WithFeatureGate sets the feature-flag gate.
func WithFeatureGate(g flags.Gate) Option { return func(s *Service) { s.gates = g } }

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
		if err := s.messageRepo.Update(ctx, tenantID, m); err != nil {
			return nil, err
		}
		if err := s.pub.PublishMessageFailed(ctx, m, reason); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish message failed event", "message_id", m.ID, "err", err)
		}
		return m, fmt.Errorf("%w: %s", domain.ErrValidation, reason)
	}

	notifier, ok := s.notifiers[m.Channel]
	if !ok {
		reason := "no notifier configured for channel " + m.Channel
		m.MarkFailed(reason)
		if err := s.messageRepo.Update(ctx, tenantID, m); err != nil {
			return nil, err
		}
		if err := s.pub.PublishMessageFailed(ctx, m, reason); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish message failed event", "message_id", m.ID, "err", err)
		}
		return m, fmt.Errorf("%w: %s", domain.ErrValidation, reason)
	}

	if err := notifier.Send(ctx, *m); err != nil {
		reason := err.Error()
		m.MarkFailed(reason)
		if uErr := s.messageRepo.Update(ctx, tenantID, m); uErr != nil {
			return nil, uErr
		}
		if err := s.pub.PublishMessageFailed(ctx, m, reason); err != nil {
			slog.Default().ErrorContext(ctx, "failed to publish message failed event", "message_id", m.ID, "err", err)
		}
		return m, fmt.Errorf("%w: send failed: %w", domain.ErrValidation, err)
	}

	m.MarkSent()
	if err := s.messageRepo.Update(ctx, tenantID, m); err != nil {
		return nil, err
	}
	if err := s.pub.PublishMessageSent(ctx, m); err != nil {
		slog.Default().ErrorContext(ctx, "failed to publish message sent event", "message_id", m.ID, "err", err)
	}
	return m, nil
}

// DispatchEvent creates pending messages for interested subscriptions.
// This is a minimal optional use case; it is not required for the story.
func (s *Service) DispatchEvent(ctx context.Context, actor auth.Actor, eventType, tenantID string, payload json.RawMessage) error {
	if _, err := s.requireAccess(ctx, actor, PermManage); err != nil {
		return err
	}
	_ = eventType
	_ = payload
	_ = tenantID
	return nil
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
