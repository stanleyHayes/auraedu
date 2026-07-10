package ports

import (
	"context"
	"time"
)

// SessionStore persists refresh-token sessions. The Redis adapter prefixes every
// key with the tenant code so a single shared Redis instance stays tenant-scoped.
type SessionStore interface {
	Save(ctx context.Context, tenantID, userID, tokenHash string, ttl time.Duration) error
	Find(ctx context.Context, tenantID, tokenHash string) (userID string, ok bool, err error)
	Revoke(ctx context.Context, tenantID, tokenHash string) error
}
