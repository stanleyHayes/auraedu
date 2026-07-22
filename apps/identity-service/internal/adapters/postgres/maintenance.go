package postgres

import (
	"context"
	"fmt"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/jackc/pgx/v5"
)

var _ ports.AuthCleanupRepository = (*Repository)(nil)

// CleanupAuthArtifacts removes only data whose security or delivery lifecycle
// has fully ended. A refresh family is retained until every descendant has
// expired so replay of an old member can still revoke a live successor.
func (r *Repository) CleanupAuthArtifacts(ctx context.Context, cutoffs ports.AuthRetentionCutoffs) (ports.AuthCleanupResult, error) {
	if cutoffs.BatchSize <= 0 || cutoffs.BatchSize > 10_000 {
		return ports.AuthCleanupResult{}, fmt.Errorf("identity cleanup batch size must be between 1 and 10000")
	}
	result := ports.AuthCleanupResult{}
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		var err error
		result.RefreshTokens, err = deleteCount(ctx, tx, `
			WITH expired_families AS (
				SELECT family_id
				FROM refresh_tokens
				GROUP BY family_id
				HAVING MAX(expires_at) < $1
				ORDER BY MAX(expires_at), family_id
				LIMIT $2
			), deleted AS (
				DELETE FROM refresh_tokens
				WHERE family_id IN (SELECT family_id FROM expired_families)
				RETURNING 1
			)
			SELECT COUNT(*) FROM deleted
		`, cutoffs.RefreshFamiliesBefore, cutoffs.BatchSize)
		if err != nil {
			return err
		}
		result.PasswordResets, err = deleteCount(ctx, tx, `
			WITH candidates AS (
				SELECT id FROM password_resets
				WHERE GREATEST(
					expires_at,
					COALESCE(used_at, expires_at),
					COALESCE(revoked_at, expires_at)
				) < $1
				ORDER BY GREATEST(
					expires_at,
					COALESCE(used_at, expires_at),
					COALESCE(revoked_at, expires_at)
				), id
				LIMIT $2 FOR UPDATE SKIP LOCKED
			), deleted AS (
				DELETE FROM password_resets
				WHERE id IN (SELECT id FROM candidates) RETURNING 1
			)
			SELECT COUNT(*) FROM deleted
		`, cutoffs.PasswordResetsBefore, cutoffs.BatchSize)
		if err != nil {
			return err
		}
		result.Invites, err = deleteCount(ctx, tx, `
			WITH candidates AS (
				SELECT id FROM invites
				WHERE GREATEST(
					expires_at,
					COALESCE(used_at, expires_at),
					COALESCE(revoked_at, expires_at)
				) < $1
				ORDER BY GREATEST(
					expires_at,
					COALESCE(used_at, expires_at),
					COALESCE(revoked_at, expires_at)
				), id
				LIMIT $2 FOR UPDATE SKIP LOCKED
			), deleted AS (
				DELETE FROM invites
				WHERE id IN (SELECT id FROM candidates)
				RETURNING 1
			)
			SELECT COUNT(*) FROM deleted
		`, cutoffs.InvitesBefore, cutoffs.BatchSize)
		if err != nil {
			return err
		}
		result.OutboxEvents, err = deleteCount(ctx, tx, `
			WITH candidates AS (
				SELECT id FROM identity_outbox
				WHERE published_at IS NOT NULL AND published_at < $1
				ORDER BY published_at, id
				LIMIT $2 FOR UPDATE SKIP LOCKED
			), deleted AS (
				DELETE FROM identity_outbox
				WHERE id IN (SELECT id FROM candidates)
				RETURNING 1
			)
			SELECT COUNT(*) FROM deleted
		`, cutoffs.PublishedOutboxBefore, cutoffs.BatchSize)
		return err
	})
	return result, err
}

func deleteCount(ctx context.Context, tx pgx.Tx, query string, cutoff any, batchSize int) (int64, error) {
	var count int64
	err := tx.QueryRow(ctx, query, cutoff, batchSize).Scan(&count)
	return count, err
}
