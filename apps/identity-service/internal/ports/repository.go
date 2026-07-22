package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
)

// RoleChangeEvent is the authorization-safe projection committed beside a
// role/permission mutation. It intentionally excludes email and display name.
type RoleChangeEvent struct {
	TenantID     string
	UserID       string
	PreviousRole string
	NewRole      string
	Permissions  []string
}

// RoleChangeEventData is the canonical authorization-safe payload shared by
// direct development publication and the transactional outbox.
func RoleChangeEventData(event RoleChangeEvent) map[string]any {
	payload := map[string]any{
		"user_id":       event.UserID,
		"previous_role": event.PreviousRole,
		"new_role":      event.NewRole,
	}
	if event.Permissions != nil {
		payload["permissions"] = event.Permissions
	}
	return payload
}

// DurableRoleChangeRepository commits the user mutation and role-change event
// atomically. Adapters without this capability retain direct publisher behavior.
type DurableRoleChangeRepository interface {
	UpdateUserWithRoleChange(ctx context.Context, id string, user domain.User, event RoleChangeEvent) error
}

// DurableUserSessionRepository commits a security-relevant user mutation and
// revokes every refresh session in the same transaction.
type DurableUserSessionRepository interface {
	UpdateUserAndRevokeSessions(ctx context.Context, id string, user domain.User) error
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

type AuthRetentionCutoffs struct {
	RefreshFamiliesBefore time.Time
	PasswordResetsBefore  time.Time
	InvitesBefore         time.Time
	PublishedOutboxBefore time.Time
	BatchSize             int
}

type AuthCleanupResult struct {
	RefreshTokens  int64
	PasswordResets int64
	Invites        int64
	OutboxEvents   int64
}

type AuthCleanupRepository interface {
	CleanupAuthArtifacts(context.Context, AuthRetentionCutoffs) (AuthCleanupResult, error)
}

// Repository stores users, credentials, refresh tokens, password resets and invites.
type Repository interface {
	FindByEmail(ctx context.Context, email string) (domain.User, domain.Credential, bool, error)
	CreateUser(ctx context.Context, u domain.User, cred domain.Credential) (string, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	GetUser(ctx context.Context, id string) (domain.User, error)
	UpdateUser(ctx context.Context, id string, u domain.User) error
	DeleteUser(ctx context.Context, id string) error
	GetMFA(ctx context.Context, userID string) (MFARecord, bool, error)
	SaveMFA(ctx context.Context, userID string, encryptedSecret []byte, acceptedCounter int64) error
	AdvanceMFACounter(ctx context.Context, userID string, acceptedCounter int64) error

	SaveRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tokenHash string) (string, error)
	RotateRefreshToken(ctx context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time) (string, error)
	RevokeRefreshFamily(ctx context.Context, tokenHash string) error
	RevokeUserSessions(ctx context.Context, userID string) error

	SavePasswordResetToken(ctx context.Context, tenantID, userID, tokenHash string, expiresAt time.Time) error
	ResetPasswordWithToken(ctx context.Context, tokenHash, tenantID string, credential domain.Credential) error

	SaveInvite(ctx context.Context, tenantID, email, role string, permissions []string, tokenHash string, invitedBy *string, expiresAt time.Time) error
	InspectInvite(ctx context.Context, tokenHash string) (InviteDetails, error)
	// AcceptInviteWithCredential keeps token use, user creation/exact matching
	// and first-credential installation atomic. password is used only to verify
	// a safe retry and must never be logged or persisted.
	AcceptInviteWithCredential(ctx context.Context, tokenHash, name, password string, credential domain.Credential) (domain.User, error)
}

type MFARecord struct {
	EncryptedSecret []byte
	LastCounter     int64
}

// InviteDetails is the payload recovered from a used invite token.
type InviteDetails struct {
	TenantID    string
	Email       string
	Role        string
	Permissions []string
}

// TransactionalNotifier delivers security-sensitive identity messages over a
// private service-to-service channel. Tokens must never be placed on the event bus.
type TransactionalNotifier interface {
	Deliver(ctx context.Context, tenantID, recipient, template string, data map[string]any) error
}

// TenantActivator completes school provisioning after the first school
// administrator has successfully created their identity.
type TenantActivator interface {
	Activate(ctx context.Context, tenantID string) error
}
