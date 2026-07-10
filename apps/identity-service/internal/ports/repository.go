package ports

import "github.com/auraedu/identity-service/internal/domain"

// Repository stores users + their credentials. The memory adapter seeds it today;
// the Postgres adapter (argon2id, per-tenant users) is a later story.
type Repository interface {
	// FindByEmail returns the user + credential for an email (case-insensitive),
	// or ok=false if none. It must not distinguish "unknown" from other errors to callers.
	FindByEmail(email string) (domain.User, domain.Credential, bool)
}
