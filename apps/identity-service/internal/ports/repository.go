package ports

import (
	"context"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
)

// Repository stores users, credentials, refresh tokens, password resets and invites.
type Repository interface {
	FindByEmail(ctx context.Context, email string) (domain.User, domain.Credential, bool, error)
	CreateUser(ctx context.Context, u domain.User, cred domain.Credential) (string, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	GetUser(ctx context.Context, id string) (domain.User, error)
	UpdateUser(ctx context.Context, id string, u domain.User) error
	DeleteUser(ctx context.Context, id string) error
	UpdateCredential(ctx context.Context, userID string, cred domain.Credential) error

	SaveRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tokenHash string) (string, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error

	SavePasswordResetToken(ctx context.Context, tenantID, userID, tokenHash string, expiresAt time.Time) error
	UsePasswordResetToken(ctx context.Context, tokenHash string) (string, error)

	SaveInvite(ctx context.Context, tenantID, email, role string, permissions []string, tokenHash string, invitedBy *string, expiresAt time.Time) error
	UseInvite(ctx context.Context, tokenHash string) (InviteDetails, error)
}

// InviteDetails is the payload recovered from a used invite token.
type InviteDetails struct {
	TenantID    string
	Email       string
	Role        string
	Permissions []string
}
