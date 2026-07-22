// Package postgres provides tenant-isolated durable orchestrator persistence.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/ai-orchestrator-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)

func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func (r *Repository) FindReplay(ctx context.Context, tenantID, keyHash, _ string) (domain.Response, string, bool, error) {
	var response domain.Response
	var storedHash string
	found := false
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var citations []byte
		err := tx.QueryRow(ctx, `
			SELECT tenant_id, session_id, message_id, question, answer, confidence,
				citations, needs_human, escalation_message, locale, created_at, request_hash
			FROM assistant_exchanges
			WHERE tenant_id=$1 AND idempotency_key_hash=$2 AND expires_at>now()`, tenantID, keyHash).Scan(
			&response.TenantID, &response.SessionID, &response.MessageID, &response.Question, &response.Answer,
			&response.Confidence, &citations, &response.NeedsHuman, &response.EscalationMessage, &response.Locale,
			&response.CreatedAt, &storedHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := json.Unmarshal(citations, &response.Citations); err != nil {
			return fmt.Errorf("assistant: decode citations: %w", err)
		}
		found = true
		return nil
	})
	return response, storedHash, found, err
}

func (r *Repository) Save(ctx context.Context, response domain.Response, keyHash, requestHash string) error {
	citations, err := json.Marshal(response.Citations)
	if err != nil {
		return err
	}
	err = r.db.WithTx(withTenant(ctx, response.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO assistant_exchanges
			(message_id,tenant_id,session_id,question,answer,confidence,citations,needs_human,escalation_message,locale,idempotency_key_hash,request_hash,created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, response.MessageID, response.TenantID,
			response.SessionID, response.Question, response.Answer, response.Confidence, citations, response.NeedsHuman,
			response.EscalationMessage, response.Locale, keyHash, requestHash, response.CreatedAt)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

func (r *Repository) PurgeExpired(ctx context.Context) (int64, error) {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__platform_maintenance__"})
	var deleted int64
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM assistant_exchanges WHERE expires_at <= now()`)
		deleted = tag.RowsAffected()
		return err
	})
	return deleted, err
}
