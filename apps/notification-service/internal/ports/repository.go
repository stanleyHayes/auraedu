package ports

import (
	"context"

	"github.com/auraedu/notification-service/internal/domain"
)

// MessageRepository persists Message aggregates. Implementations MUST scope
// every query by tenantID (defense-in-depth with Postgres RLS).
type MessageRepository interface {
	Create(ctx context.Context, tenantID string, m *domain.Message) error
	GetByID(ctx context.Context, tenantID, id string) (*domain.Message, error)
	List(ctx context.Context, tenantID string, filter MessageFilter) ([]*domain.Message, string, error)
	Update(ctx context.Context, tenantID string, m *domain.Message) error
	Delete(ctx context.Context, tenantID, id string) error
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
