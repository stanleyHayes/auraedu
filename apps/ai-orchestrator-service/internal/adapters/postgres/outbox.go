package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ ports.TransactionalExchangeRepository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)

func (r *Repository) SaveWithEvent(ctx context.Context, response domain.Response, keyHash, requestHash, eventType string, payload map[string]any) error {
	citations, err := json.Marshal(response.Citations)
	if err != nil {
		return fmt.Errorf("assistant: encode citations: %w", err)
	}
	eventPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("assistant: encode escalation event: %w", err)
	}
	err = r.db.WithTx(withTenant(ctx, response.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if eventType == "" {
			return errors.New("assistant: escalation event type is required")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO assistant_exchanges
			(message_id,tenant_id,session_id,question,answer,confidence,citations,needs_human,escalation_message,locale,idempotency_key_hash,request_hash,created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, response.MessageID, response.TenantID,
			response.SessionID, response.Question, response.Answer, response.Confidence, citations, response.NeedsHuman,
			response.EscalationMessage, response.Locale, keyHash, requestHash, response.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO assistant_outbox
			(id,tenant_id,event_type,payload,created_at,next_attempt_at)
			VALUES ($1,$2,$3,$4,$5,now())`,
			uuid.NewString(), response.TenantID, eventType, eventPayload, response.CreatedAt,
		); err != nil {
			return fmt.Errorf("assistant: enqueue escalation event: %w", err)
		}
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

func assistantOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__assistant_outbox__"})
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(assistantOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE assistant_outbox
			SET attempts=attempts+1,next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (SELECT id FROM assistant_outbox WHERE published_at IS NULL AND next_attempt_at<=now()
			ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1)
			RETURNING id::text,tenant_id,event_type,payload,created_at`, limit)
		if err != nil {
			return fmt.Errorf("assistant: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CreatedAt); err != nil {
				return fmt.Errorf("assistant: scan outbox: %w", err)
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
	return r.db.WithTx(assistantOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		if published {
			_, err := tx.Exec(ctx, `UPDATE assistant_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE assistant_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}
