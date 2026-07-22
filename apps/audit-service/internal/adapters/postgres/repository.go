// Package postgres provides the Postgres implementation of the audit repository port.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auraedu/audit-service/internal/domain"
	"github.com/auraedu/audit-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository. Every query
// uses platform/db.WithTx so that app.tenant_id is set on the connection and
// the Row-Level Security policy is enforced.
type Repository struct {
	db *db.DB
}

var _ ports.Repository = (*Repository)(nil)

// NewRepository creates a Postgres-backed audit log repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{db: database}
}

// Insert persists an immutable audit log record.
func (r *Repository) Insert(ctx context.Context, log *domain.AuditLog) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var payload any
		if len(log.Payload) > 0 {
			payload = string(log.Payload)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO audit_logs (
				id, tenant_id, event_id, event_type, source_service,
				timestamp, received_at, payload, actor_id, action,
				resource_type, resource_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, log.ID, log.TenantID, log.EventID, log.EventType, log.SourceService,
			log.Timestamp, log.ReceivedAt, payload, log.ActorID, log.Action,
			log.ResourceType, log.ResourceID)
		if err != nil {
			return fmt.Errorf("audit: insert: %w", err)
		}
		return nil
	})
}

// List returns a tenant-scoped page ordered newest-first by id (UUID v7).
func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	var out []*domain.AuditLog
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			log, err := scanAuditLog(rows)
			if err != nil {
				return err
			}
			out = append(out, log)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("audit: list rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID.String()
		}
		return nil
	})
	return out, nextCursor, err
}

// ListAll returns a cross-tenant page ordered newest-first by id (UUID v7).
// It is reserved for platform super admins: it cannot use db.WithTx (which
// requires a tenant context), so it sets only app.is_platform_admin on the
// transaction and relies on the RLS policy's platform-admin bypass.
func (r *Repository) ListAll(ctx context.Context, limit int, cursor string) ([]*domain.AuditLog, string, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("audit: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := db.SetPlatformAdmin(ctx, tx); err != nil {
		return nil, "", err
	}

	var out []*domain.AuditLog
	var nextCursor string
	rows, err := listAllQuery(ctx, tx, limit, cursor)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	for rows.Next() {
		log, err := scanAuditLog(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, log)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("audit: list rows: %w", err)
	}
	if len(out) == limit && len(out) > 0 {
		nextCursor = out[len(out)-1].ID.String()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("audit: commit: %w", err)
	}
	return out, nextCursor, nil
}

func listAllQuery(ctx context.Context, tx pgx.Tx, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil {
			return nil, fmt.Errorf("audit: invalid cursor: %w", err)
		}
		return tx.Query(ctx, `
			SELECT id, tenant_id, event_id, event_type, source_service,
			       timestamp, received_at, payload, actor_id, action,
			       resource_type, resource_id
			FROM audit_logs
			WHERE id < $1::uuid
			ORDER BY id DESC
			LIMIT $2
		`, id, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, event_id, event_type, source_service,
		       timestamp, received_at, payload, actor_id, action,
		       resource_type, resource_id
		FROM audit_logs
		ORDER BY id DESC
		LIMIT $1
	`, limit)
}

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil {
			return nil, fmt.Errorf("audit: invalid cursor: %w", err)
		}
		return tx.Query(ctx, `
			SELECT id, tenant_id, event_id, event_type, source_service,
			       timestamp, received_at, payload, actor_id, action,
			       resource_type, resource_id
			FROM audit_logs
			WHERE tenant_id = $1 AND id < $2::uuid
			ORDER BY id DESC
			LIMIT $3
		`, tenantID, id, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, event_id, event_type, source_service,
		       timestamp, received_at, payload, actor_id, action,
		       resource_type, resource_id
		FROM audit_logs
		WHERE tenant_id = $1
		ORDER BY id DESC
		LIMIT $2
	`, tenantID, limit)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAuditLog(row scanner) (*domain.AuditLog, error) {
	var log domain.AuditLog
	var payload []byte
	if err := row.Scan(
		&log.ID, &log.TenantID, &log.EventID, &log.EventType, &log.SourceService,
		&log.Timestamp, &log.ReceivedAt, &payload, &log.ActorID, &log.Action,
		&log.ResourceType, &log.ResourceID,
	); err != nil {
		return nil, err
	}
	if payload != nil {
		log.Payload = json.RawMessage(payload)
	}
	return &log, nil
}
