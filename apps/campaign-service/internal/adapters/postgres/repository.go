// Package postgres provides tenant-isolated Campaign persistence.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/campaign-service/internal/domain"
	"github.com/auraedu/campaign-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)

func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }
func tenantCtx(ctx context.Context, id string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: id})
}

const columns = `
	id, tenant_id, name, objective, status, channel, audience_definition,
	programme_ids, budget, currency, start_at, end_at, approval_status,
	owner_user_id, submitted_by, submitted_at, approved_by, approved_at,
	review_note, tracking_url_parameters, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (domain.Campaign, error) {
	var c domain.Campaign
	err := row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.Objective, &c.Status, &c.Channel,
		&c.AudienceDefinition, &c.ProgrammeIDs, &c.Budget, &c.Currency,
		&c.StartAt, &c.EndAt, &c.ApprovalStatus, &c.OwnerUserID,
		&c.SubmittedBy, &c.SubmittedAt, &c.ApprovedBy, &c.ApprovedAt,
		&c.ReviewNote, &c.TrackingURLParameters, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}
func (r *Repository) Create(ctx context.Context, c domain.Campaign) error {
	return r.db.WithTx(tenantCtx(ctx, c.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO campaigns (`+columns+`)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
			campaignArgs(c)...)
		return err
	})
}
func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Campaign, error) {
	var c domain.Campaign
	err := r.db.WithTx(tenantCtx(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		c, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM campaigns WHERE tenant_id=$1 AND id=$2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	})
	return c, err
}
func (r *Repository) List(ctx context.Context, tenantID string, status domain.Status, limit int) ([]domain.Campaign, error) {
	items := []domain.Campaign{}
	err := r.db.WithTx(tenantCtx(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+columns+` FROM campaigns
			WHERE tenant_id=$1 AND ($2='' OR status=$2)
			ORDER BY created_at DESC,id DESC LIMIT $3`, tenantID, status, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scan(rows)
			if err != nil {
				return err
			}
			items = append(items, c)
		}
		return rows.Err()
	})
	return items, err
}
func (r *Repository) Update(ctx context.Context, c domain.Campaign, expected domain.Status) error {
	return r.db.WithTx(tenantCtx(ctx, c.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		return updateCampaign(ctx, tx, c, expected)
	})
}

func updateCampaign(ctx context.Context, tx pgx.Tx, c domain.Campaign, expected domain.Status) error {
	tag, err := tx.Exec(ctx, `
		UPDATE campaigns SET name=$3,objective=$4,status=$5,audience_definition=$6,
			programme_ids=$7,budget=$8,currency=$9,start_at=$10,end_at=$11,
			approval_status=$12,submitted_by=$13,submitted_at=$14,
			approved_by=$15,approved_at=$16,review_note=$17,updated_at=$18
		WHERE tenant_id=$1 AND id=$2 AND status=$19`, campaignUpdateArgs(c, expected)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *Repository) UpdateWithEvent(ctx context.Context, c domain.Campaign, expected domain.Status, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("campaign outbox payload: %w", err)
	}
	return r.db.WithTx(tenantCtx(ctx, c.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if err := updateCampaign(ctx, tx, c, expected); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO campaign_outbox(id,tenant_id,event_type,payload,created_at,next_attempt_at)
			VALUES($1,$2,$3,$4,$5,now())`, uuid.NewString(), c.TenantID, eventType, encoded, c.UpdatedAt)
		return err
	})
}

func platformCtx(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__campaign_outbox__"})
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := []ports.OutboxEvent{}
	err := r.db.WithTx(platformCtx(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE campaign_outbox SET attempts=attempts+1,
				next_attempt_at=now() + (LEAST(300, power(2, attempts)) * interval '1 second')
			WHERE id IN (
				SELECT id FROM campaign_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			) RETURNING id,tenant_id,event_type,payload,created_at`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var event ports.OutboxEvent
			if err := rows.Scan(&event.ID, &event.TenantID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
				return err
			}
			items = append(items, event)
		}
		return rows.Err()
	})
	return items, err
}

func campaignArgs(c domain.Campaign) []any {
	return []any{
		c.ID, c.TenantID, c.Name, c.Objective, c.Status, c.Channel,
		c.AudienceDefinition, c.ProgrammeIDs, c.Budget, c.Currency,
		c.StartAt, c.EndAt, c.ApprovalStatus, c.OwnerUserID, c.SubmittedBy,
		c.SubmittedAt, c.ApprovedBy, c.ApprovedAt, c.ReviewNote,
		c.TrackingURLParameters, c.CreatedAt, c.UpdatedAt,
	}
}

func campaignUpdateArgs(c domain.Campaign, expected domain.Status) []any {
	return []any{
		c.TenantID, c.ID, c.Name, c.Objective, c.Status, c.AudienceDefinition,
		c.ProgrammeIDs, c.Budget, c.Currency, c.StartAt, c.EndAt,
		c.ApprovalStatus, c.SubmittedBy, c.SubmittedAt, c.ApprovedBy,
		c.ApprovedAt, c.ReviewNote, c.UpdatedAt, expected,
	}
}

func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.mark(ctx, id, "", true)
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.mark(ctx, id, message, false)
}

func (r *Repository) mark(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(platformCtx(ctx), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if published {
			_, err = tx.Exec(ctx, `UPDATE campaign_outbox SET published_at=$2,last_error=NULL WHERE id=$1`, id, time.Now().UTC())
		} else {
			_, err = tx.Exec(ctx, `UPDATE campaign_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		}
		return err
	})
}
