package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/auraedu/platform/auth"
	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/jackc/pgx/v5"
)

func outboxPlatformContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__tenant_outbox__"})
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	events := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(outboxPlatformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		// SetPlatformAdmin is explicit here because tenant-service platform reads
		// already use the same privileged transaction boundary.
		if err := platformdb.SetPlatformAdmin(ctx, tx); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			UPDATE tenant_outbox
			SET attempts = attempts + 1,
			    next_attempt_at = now() + (LEAST(300, power(2, attempts)) * interval '1 second')
			WHERE id IN (
				SELECT id FROM tenant_outbox
				WHERE published_at IS NULL AND next_attempt_at <= now()
				ORDER BY created_at, id
				FOR UPDATE SKIP LOCKED
				LIMIT $1
			)
			RETURNING id, tenant_id, event_type, payload, created_at
		`, limit)
		if err != nil {
			return fmt.Errorf("tenant outbox: claim: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var event ports.OutboxEvent
			if err := rows.Scan(&event.ID, &event.TenantID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
				return fmt.Errorf("tenant outbox: scan: %w", err)
			}
			events = append(events, event)
		}
		return rows.Err()
	})
	return events, err
}

func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.markOutbox(ctx, id, "", true)
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.markOutbox(ctx, id, message, false)
}

func (r *Repository) markOutbox(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(outboxPlatformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		if err := platformdb.SetPlatformAdmin(ctx, tx); err != nil {
			return err
		}
		if published {
			_, err := tx.Exec(ctx, `UPDATE tenant_outbox SET published_at = $2, last_error = NULL WHERE id = $1`, id, time.Now().UTC())
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE tenant_outbox SET last_error = left($2, 1000) WHERE id = $1`, id, message)
		return err
	})
}

var _ ports.OutboxRepository = (*Repository)(nil)
