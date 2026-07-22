package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
)

type ScheduledMessageRepository interface {
	ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]*domain.Message, error)
	CancelByApplication(ctx context.Context, tenantID, applicationID string) error
	NextJourneyDeliveryAllowedAt(ctx context.Context, tenantID, journeyID, recipientID string, window time.Duration, limit int) (*time.Time, error)
}

type JourneyRepository interface {
	CreateJourney(context.Context, string, *domain.Journey) error
	GetJourney(context.Context, string, string) (*domain.Journey, error)
	ListJourneys(context.Context, string, JourneyFilter) ([]*domain.Journey, error)
	UpdateJourneyStatus(context.Context, string, *domain.Journey, string) error
	ListActiveJourneysByTrigger(context.Context, string, string) ([]*domain.Journey, error)
	EnrollJourney(context.Context, JourneyEnrollment) (bool, error)
	CancelJourneysForEvent(context.Context, string, string, string, string) (int64, error)
	FinalizeJourneyEnrollment(context.Context, string, string) error
	JourneyStats(context.Context, string, string) (JourneyStats, error)
}

type JourneyEnrollment struct {
	ID           string
	TenantID     string
	JourneyID    string
	EventID      string
	TriggerEvent string
	LeadID       string
	Messages     []*domain.Message
	SkippedSteps int
}

type JourneyStats struct {
	Enrolled   int64 `json:"enrolled"`
	Scheduled  int64 `json:"scheduled"`
	Sent       int64 `json:"sent"`
	Failed     int64 `json:"failed"`
	Cancelled  int64 `json:"cancelled"`
	Skipped    int64 `json:"skipped"`
	Accepted   int64 `json:"accepted"`
	Delivered  int64 `json:"delivered"`
	Delayed    int64 `json:"delayed"`
	Bounced    int64 `json:"bounced"`
	Complained int64 `json:"complained"`
	Suppressed int64 `json:"suppressed"`
}

type JourneyFilter struct {
	Status       string
	TriggerEvent string
	Limit        int
}

type DeviceTokenRepository interface {
	Upsert(context.Context, string, *domain.DeviceToken) (*domain.DeviceToken, error)
	DeleteByDevice(context.Context, string, string, string) error
	ListActive(context.Context, string, string) ([]*domain.DeviceToken, error)
	MarkInvalid(context.Context, string, string) error
}

// MessageRepository persists Message aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type MessageRepository interface {
	Create(ctx context.Context, tenantID string, m *domain.Message) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Message, error)
	List(ctx context.Context, tenantID string, filter MessageFilter) ([]*domain.Message, string, error)
	Update(ctx context.Context, tenantID string, m *domain.Message) error
	Delete(ctx context.Context, tenantID, id string) error
}

// DurableDeliveryRepository atomically persists a provider delivery outcome
// and the integration event that advertises it. create is true for
// transactional messages that have not previously been stored.
type DurableDeliveryRepository interface {
	CommitDeliveryOutcome(
		ctx context.Context,
		tenantID string,
		message *domain.Message,
		providerMessageID string,
		create bool,
		eventType string,
		payload map[string]any,
	) error
}

// DeliveryFeedback is the privacy-minimized, verified projection of a provider
// webhook. AddressHash is SHA-256 of a normalized recipient address.
type DeliveryFeedback struct {
	ID                string
	Provider          string
	ProviderMessageID string
	MessageID         string
	EventType         string
	Status            string
	AddressHash       string
	OccurredAt        time.Time
}

// DeliveryFeedbackRepository enforces suppressions before provider IO and
// applies verified, replay-safe provider delivery events.
type DeliveryFeedbackRepository interface {
	IsEmailSuppressed(ctx context.Context, tenantID, addressHash string) (bool, error)
	ApplyDeliveryFeedback(ctx context.Context, feedback DeliveryFeedback) (bool, error)
	SuppressEmail(ctx context.Context, tenantID, addressHash, reason, eventID string, occurredAt time.Time) error
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
}

type OutboxRepository interface {
	ClaimPendingNotificationEvents(context.Context, int) ([]OutboxEvent, error)
	MarkNotificationEventPublished(context.Context, string) error
	MarkNotificationEventFailed(context.Context, string, string) error
}

// TemplateRepository persists Template aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type TemplateRepository interface {
	Create(ctx context.Context, tenantID string, t *domain.Template) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Template, error)
	List(ctx context.Context, tenantID string, filter TemplateFilter) ([]*domain.Template, string, error)
	Update(ctx context.Context, tenantID string, t *domain.Template) error
	Delete(ctx context.Context, tenantID, id string) error
}

// SubscriptionRepository persists Subscription aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type SubscriptionRepository interface {
	Create(ctx context.Context, tenantID string, s *domain.Subscription) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error)
	List(ctx context.Context, tenantID string, filter SubscriptionFilter) ([]*domain.Subscription, string, error)
	Update(ctx context.Context, tenantID string, s *domain.Subscription) error
	Delete(ctx context.Context, tenantID, id string) error
}

// AnnouncementRepository persists Announcement aggregates. Implementations MUST
// scope every query by tenantID (defense-in-depth with Postgres RLS).
type AnnouncementRepository interface {
	Create(ctx context.Context, tenantID string, a *domain.Announcement) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Announcement, error)
	List(ctx context.Context, tenantID string, filter AnnouncementFilter) ([]*domain.Announcement, string, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// ProcessedEventRepository is the worker idempotency ledger for consumed
// CloudEvents, deduplicated by (tenantID, eventID).
type ProcessedEventRepository interface {
	// Claim records eventID as processed for the tenant. It reports false when
	// the event was already claimed (idempotent redelivery).
	Claim(ctx context.Context, tenantID, eventID, eventType string) (bool, error)
	// Release removes a claim so a failed event can be retried on redelivery.
	Release(ctx context.Context, tenantID, eventID string) error
}

// AnnouncementFilter carries cursor pagination and optional equality filters.
type AnnouncementFilter struct {
	Limit     int
	Cursor    string
	Audience  string
	Audiences []string
}

// MessageFilter carries cursor pagination and optional equality filters.
type MessageFilter struct {
	Limit       int
	Cursor      string
	Channel     string
	Status      string
	RecipientID string
}

// TemplateFilter carries cursor pagination and optional equality filters.
type TemplateFilter struct {
	Limit   int
	Cursor  string
	Channel string
	Status  string
}

// SubscriptionFilter carries cursor pagination and optional equality filters.
type SubscriptionFilter struct {
	Limit   int
	Cursor  string
	Channel string
	UserID  string
}
