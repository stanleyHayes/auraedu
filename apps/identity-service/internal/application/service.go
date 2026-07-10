// Package application holds the Identity Service use cases: authenticate a user
// and issue a JWT (agent_plan §8). The gateway later verifies that JWT and injects
// the resulting actor into private services.
package application

import (
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
	"github.com/auraedu/platform/auth"
)

type Service struct {
	repo       ports.Repository
	signingKey []byte
	ttl        time.Duration
	now        func() time.Time
}

func NewService(repo ports.Repository, signingKey []byte, ttl time.Duration) *Service {
	return &Service{repo: repo, signingKey: signingKey, ttl: ttl, now: time.Now}
}

// dummyCredential equalises work when the email is unknown, so response timing does
// not reveal whether an account exists.
var dummyCredential, _ = domain.NewCredential("timing-equaliser")

// Login verifies credentials and returns a signed JWT + the user. It returns the same
// error for an unknown email and a wrong password (no account enumeration).
func (s *Service) Login(email, password string) (token string, user domain.User, expires time.Time, err error) {
	found, cred, ok := s.repo.FindByEmail(email)
	if !ok {
		dummyCredential.Verify(password)
		return "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}
	if !cred.Verify(password) {
		return "", domain.User{}, time.Time{}, domain.ErrInvalidCredentials
	}

	now := s.now()
	expires = now.Add(s.ttl)
	claims := auth.Claims{
		Subject:     found.ID,
		TenantID:    found.TenantID,
		Role:        found.Role,
		Permissions: found.Permissions,
		IssuedAt:    now.Unix(),
		ExpiresAt:   expires.Unix(),
	}
	token, err = auth.Sign(claims, s.signingKey)
	if err != nil {
		return "", domain.User{}, time.Time{}, err
	}
	return token, found, expires, nil
}

// Verify validates a bearer token and returns the actor — the same operation the
// gateway performs before injecting X-Actor-* headers.
func (s *Service) Verify(token string) (auth.Actor, error) {
	claims, err := auth.Verify(token, s.signingKey, s.now())
	if err != nil {
		return auth.Actor{}, err
	}
	return claims.Actor(), nil
}
