package postgres

import (
	"context"
	"fmt"

	"github.com/auraedu/identity-service/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	events := make([]ports.OutboxEvent, 0, limit)
	err := r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE identity_outbox
			SET attempts = attempts + 1,
			    next_attempt_at = now() + (LEAST(300, power(2, attempts)) * interval '1 second')
			WHERE id IN (
				SELECT id FROM identity_outbox
				WHERE published_at IS NULL AND next_attempt_at <= now()
				ORDER BY created_at, id
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			RETURNING id, tenant_id, event_type, payload, created_at
		`, limit)
		if err != nil {
			return fmt.Errorf("identity outbox: claim: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var event ports.OutboxEvent
			if err := rows.Scan(&event.ID, &event.TenantID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
				return fmt.Errorf("identity outbox: scan: %w", err)
			}
			events = append(events, event)
		}
		return rows.Err()
	})
	return events, err
}

func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE identity_outbox SET published_at = now(), last_error = NULL WHERE id = $1`, id)
		return err
	})
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.withPrivilegedTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE identity_outbox SET last_error = left($2, 1000) WHERE id = $1`, id, message)
		return err
	})
}

var _ ports.DurableRoleChangeRepository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)
