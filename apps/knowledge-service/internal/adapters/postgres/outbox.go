package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/knowledge-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ ports.TransactionalApprovalRepository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)

// ApproveWithEvent closes the knowledge approval-to-event loss window. The
// source is not visible as approved unless its lifecycle event is durable too.
func (r *Repository) ApproveWithEvent(ctx context.Context, tenantID, id, reviewer, note string, now time.Time, eventType string) (domain.Source, error) {
	var source domain.Source
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		source, err = scanSource(tx.QueryRow(ctx, `UPDATE knowledge_sources
			SET status='approved', approved_by=$3, approved_at=$4, review_note=$5, updated_at=$4
			WHERE tenant_id=$1 AND id=$2 AND status='draft'
			RETURNING `+sourceColumns, tenantID, id, reviewer, now, note))
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_sources WHERE tenant_id=$1 AND id=$2)`, tenantID, id).Scan(&exists); checkErr != nil {
				return checkErr
			}
			if exists {
				return domain.ErrConflict
			}
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("knowledge: approve source: %w", err)
		}
		if eventType == "" {
			return errors.New("knowledge: approval event type is required")
		}
		payload, err := json.Marshal(ports.ApprovalEventData(source))
		if err != nil {
			return fmt.Errorf("knowledge: encode approval event: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO knowledge_outbox
			(id,tenant_id,event_type,payload,created_at,next_attempt_at)
			VALUES ($1,$2,$3,$4,$5,$5)`, uuid.NewString(), tenantID, eventType, payload, now); err != nil {
			return fmt.Errorf("knowledge: enqueue approval event: %w", err)
		}
		return nil
	})
	return source, err
}

func knowledgeOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__knowledge_outbox__"})
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(knowledgeOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE knowledge_outbox
			SET attempts=attempts+1,
			    next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM knowledge_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id::text,tenant_id,event_type,payload,created_at`, limit)
		if err != nil {
			return fmt.Errorf("knowledge: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CreatedAt); err != nil {
				return fmt.Errorf("knowledge: scan outbox: %w", err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.markOutbox(ctx, id, "", true)
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.markOutbox(ctx, id, message, false)
}

func (r *Repository) markOutbox(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(knowledgeOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		if published {
			_, err := tx.Exec(ctx, `UPDATE knowledge_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE knowledge_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}
