package postgres

import (
	"context"
	"fmt"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

type DeviceTokenRepository struct{ db *db.DB }

func NewDeviceTokenRepository(database *db.DB) *DeviceTokenRepository {
	return &DeviceTokenRepository{db: database}
}
func (r *DeviceTokenRepository) Upsert(
	ctx context.Context,
	tenant string,
	input *domain.DeviceToken,
) (*domain.DeviceToken, error) {
	var result domain.DeviceToken
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// An Expo token identifies one physical app installation. Transfer it away
		// from any previous account before binding the current authenticated user.
		if _, err := tx.Exec(ctx, `
			DELETE FROM device_push_tokens
			WHERE token=$1 AND NOT (tenant_id=$2 AND user_id=$3 AND device_id=$4)
		`, input.Token, tenant, input.UserID, input.DeviceID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			INSERT INTO device_push_tokens(
				id,tenant_id,user_id,device_id,platform,token,status,last_seen_at,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,'active',$7,$8,$9)
			ON CONFLICT(tenant_id,user_id,device_id) DO UPDATE SET
				platform=EXCLUDED.platform,token=EXCLUDED.token,status='active',
				last_seen_at=EXCLUDED.last_seen_at,updated_at=EXCLUDED.updated_at
			RETURNING id,tenant_id,user_id,device_id,platform,token,status,last_seen_at,created_at,updated_at
		`,
			input.ID,
			tenant,
			input.UserID,
			input.DeviceID,
			input.Platform,
			input.Token,
			input.LastSeenAt,
			input.CreatedAt,
			input.UpdatedAt,
		).Scan(
			&result.ID,
			&result.TenantID,
			&result.UserID,
			&result.DeviceID,
			&result.Platform,
			&result.Token,
			&result.Status,
			&result.LastSeenAt,
			&result.CreatedAt,
			&result.UpdatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("notifications: upsert device token: %w", err)
	}
	return &result, nil
}
func (r *DeviceTokenRepository) DeleteByDevice(ctx context.Context, tenant, user, device string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM device_push_tokens WHERE tenant_id=$1 AND user_id=$2 AND device_id=$3`, tenant, user, device)
		return err
	})
}
func (r *DeviceTokenRepository) ListActive(ctx context.Context, tenant, user string) ([]*domain.DeviceToken, error) {
	var out []*domain.DeviceToken
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id,tenant_id,user_id,device_id,platform,token,status,last_seen_at,created_at,updated_at
			FROM device_push_tokens
			WHERE tenant_id=$1 AND user_id=$2 AND status='active'
			ORDER BY created_at
		`, tenant, user)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d domain.DeviceToken
			if err := rows.Scan(
				&d.ID,
				&d.TenantID,
				&d.UserID,
				&d.DeviceID,
				&d.Platform,
				&d.Token,
				&d.Status,
				&d.LastSeenAt,
				&d.CreatedAt,
				&d.UpdatedAt,
			); err != nil {
				return err
			}
			out = append(out, &d)
		}
		return rows.Err()
	})
	if out == nil {
		out = []*domain.DeviceToken{}
	}
	return out, err
}
func (r *DeviceTokenRepository) MarkInvalid(ctx context.Context, tenant, token string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE device_push_tokens SET status='invalid',updated_at=now() WHERE tenant_id=$1 AND token=$2`, tenant, token)
		return err
	})
}
