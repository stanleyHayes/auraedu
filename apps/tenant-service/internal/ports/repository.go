// Package ports defines the tenant service repository boundary.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/tenant-service/internal/domain"
)

// Repository stores tenants and their feature flags. Adapters implement the
// context-aware interface so that platform/db can set app.tenant_id for RLS.
type Repository interface {
	SubmitOnboarding(
		ctx context.Context,
		request *domain.OnboardingRequest,
		idempotencyHash string,
		payloadHash string,
		emailFingerprint string,
	) (*domain.OnboardingRequest, bool, error)
	ListOnboarding(ctx context.Context, limit int, cursor, status string) ([]domain.OnboardingRequest, string, error)
	GetOnboarding(ctx context.Context, requestID string) (domain.OnboardingRequest, error)
	ApproveOnboarding(ctx context.Context, requestID string, tenant domain.Tenant, decidedBy string) (domain.OnboardingRequest, error)
	RejectOnboarding(ctx context.Context, requestID, reason, decidedBy string) (domain.OnboardingRequest, error)
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	GetTenant(ctx context.Context, code string) (domain.Tenant, error)
	CreateTenant(ctx context.Context, t domain.Tenant) error
	UpdateTenant(ctx context.Context, code string, upd domain.TenantUpdate) (domain.Tenant, error)
	DeleteTenant(ctx context.Context, code string) error
	ResolveTenant(ctx context.Context, domain, subdomain string) (domain.Tenant, error)
	Features(ctx context.Context, code string) ([]domain.FeatureFlag, error)
	SetFeature(ctx context.Context, code, key string, enabled bool, reason string) (domain.FeatureFlag, error)
	Settings(ctx context.Context, code string) (domain.Settings, error)
	UpdateSettings(ctx context.Context, code string, s domain.Settings) error
	RequestCustomDomain(ctx context.Context, registration domain.CustomDomain, challengeHash string) (domain.CustomDomain, error)
	GetCustomDomain(ctx context.Context, code string) (domain.CustomDomain, string, error)
	MarkCustomDomainVerified(ctx context.Context, code string, verifiedAt time.Time) (domain.CustomDomain, error)
	ActivateCustomDomain(ctx context.Context, code, providerReference string, activatedAt time.Time) (domain.CustomDomain, error)
	DeactivateCustomDomain(ctx context.Context, code, providerReference string, deactivatedAt time.Time) (domain.CustomDomain, error)
}

// DurableOnboardingRepository marks adapters that commit onboarding lifecycle
// events atomically with tenant provisioning. The application must not publish
// a duplicate direct event when this capability is present.
type DurableOnboardingRepository interface {
	OnboardingEventsDurable()
}

// DurableTenantLifecycleRepository marks adapters that commit every tenant
// lifecycle event in the same transaction as its state mutation. Activation is
// exposed separately so an onboarding-to-active transition cannot accidentally
// emit the more general tenant.updated event or duplicate on retry.
type DurableTenantLifecycleRepository interface {
	TenantLifecycleEventsDurable()
	ActivateOnboardingTenant(ctx context.Context, code string) (changed bool, err error)
}

type OutboxEvent struct {
	ID        string
	TenantID  string
	EventType string
	Payload   json.RawMessage
	CreatedAt time.Time
}

type OutboxRepository interface {
	ClaimPending(context.Context, int) ([]OutboxEvent, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, string) error
}
