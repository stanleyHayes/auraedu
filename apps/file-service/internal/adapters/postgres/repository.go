// Package postgres provides Postgres-backed repositories for the file service.
//
//nolint:lll // SQL statements remain adjacent to their ordered arguments for transactional review.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type Repository struct {
	db *db.DB
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

// NewRepository creates a Postgres-backed file repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, f *domain.FileUpload) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return insertFile(ctx, tx, tenantID, f)
	})
}

func insertFile(ctx context.Context, tx pgx.Tx, tenantID string, f *domain.FileUpload) error {
	_, err := tx.Exec(ctx, `
			INSERT INTO file_uploads (
				id, tenant_id, original_filename, storage_path, storage_backend,
				content_type, size_bytes, checksum, owner_id, purpose, status, secure_url, metadata,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`, f.ID, tenantID, f.OriginalFilename, f.StoragePath, f.StorageBackend,
		f.ContentType, f.SizeBytes, f.Checksum, f.OwnerID, f.Purpose, f.Status, f.SecureURL, f.Metadata,
		f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return fmt.Errorf("file: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.FileUpload, error) {
	var f *domain.FileUpload
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, tenant_id, original_filename, storage_path, storage_backend,
				content_type, size_bytes, checksum, owner_id, purpose, status, secure_url, metadata,
				created_at, updated_at
			FROM file_uploads
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanFile(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("file: get: %w", err)
		}
		f = got
		return nil
	})
	return f, err
}

func (r *Repository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.FileUpload, string, error) {
	var out []*domain.FileUpload
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			f, err := scanFile(rows)
			if err != nil {
				return err
			}
			out = append(out, f)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("file: list rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT
				id, tenant_id, original_filename, storage_path, storage_backend,
				content_type, size_bytes, checksum, owner_id, purpose, status, secure_url, metadata,
				created_at, updated_at
			FROM file_uploads
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM file_uploads WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT
			id, tenant_id, original_filename, storage_path, storage_backend,
			content_type, size_bytes, checksum, owner_id, purpose, status, secure_url, metadata,
			created_at, updated_at
		FROM file_uploads
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *Repository) Update(ctx context.Context, tenantID string, f *domain.FileUpload) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return updateFile(ctx, tx, tenantID, f)
	})
}

func updateFile(ctx context.Context, tx pgx.Tx, tenantID string, f *domain.FileUpload) error {
	tag, err := tx.Exec(ctx, `
			UPDATE file_uploads
			SET original_filename = $3,
			    storage_path = $4,
			    storage_backend = $5,
			    content_type = $6,
			    size_bytes = $7,
			    checksum = $8,
			    owner_id = $9,
			    purpose = $10,
			    status = $11,
			    secure_url = $12,
			    metadata = $13,
			    updated_at = $14
			WHERE id = $1 AND tenant_id = $2
		`, f.ID, tenantID, f.OriginalFilename, f.StoragePath, f.StorageBackend,
		f.ContentType, f.SizeBytes, f.Checksum, f.OwnerID, f.Purpose, f.Status, f.SecureURL, f.Metadata,
		f.UpdatedAt)
	if err != nil {
		return fmt.Errorf("file: update: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) CommitFileLifecycle(ctx context.Context, tenantID string, file *domain.FileUpload, mutation, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("file: encode lifecycle event: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		cleanupPath := ""
		switch mutation {
		case ports.FileMutationCreate:
			if err := insertFile(ctx, tx, tenantID, file); err != nil {
				return err
			}
		case ports.FileMutationUpdate:
			if err := updateFile(ctx, tx, tenantID, file); err != nil {
				return err
			}
		case ports.FileMutationDelete:
			tag, err := tx.Exec(ctx, `DELETE FROM file_uploads WHERE id=$1 AND tenant_id=$2`, file.ID, tenantID)
			if err != nil {
				return fmt.Errorf("file: delete: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
			cleanupPath = file.StoragePath
		default:
			return fmt.Errorf("file: unsupported lifecycle mutation %q", mutation)
		}
		_, err := tx.Exec(ctx, `INSERT INTO file_outbox (id,tenant_id,event_type,payload,cleanup_path) VALUES ($1,$2,$3,$4,$5)`, uuid.NewString(), tenantID, eventType, encoded, cleanupPath)
		if err != nil {
			return fmt.Errorf("file: enqueue lifecycle event: %w", err)
		}
		return nil
	})
}

func fileOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__file_outbox__"})
}

func (r *Repository) ClaimPendingFileEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(fileOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE file_outbox SET attempts=attempts+1,next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second') WHERE id IN (SELECT id FROM file_outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id::text,tenant_id,event_type,payload,cleanup_path`, limit)
		if err != nil {
			return fmt.Errorf("file: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CleanupPath); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) MarkFileEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(fileOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE file_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}
func (r *Repository) MarkFileEventFailed(ctx context.Context, id, message string) error {
	return r.db.WithTx(fileOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE file_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM file_uploads WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("file: delete: %w", err)
		}
		return nil
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanFile(row scanner) (*domain.FileUpload, error) {
	var f domain.FileUpload
	if err := row.Scan(
		&f.ID, &f.TenantID, &f.OriginalFilename, &f.StoragePath, &f.StorageBackend,
		&f.ContentType, &f.SizeBytes, &f.Checksum, &f.OwnerID, &f.Purpose, &f.Status, &f.SecureURL, &f.Metadata,
		&f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *Repository) RecordStorage(ctx context.Context, tenantID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO file_usage (tenant_id, date, bytes_stored)
			VALUES ($1, CURRENT_DATE, $2)
			ON CONFLICT (tenant_id, date)
			DO UPDATE SET bytes_stored = file_usage.bytes_stored + EXCLUDED.bytes_stored,
			              updated_at = now()
		`, tenantID, bytes)
		if err != nil {
			return fmt.Errorf("file: record storage: %w", err)
		}
		return nil
	})
}

func (r *Repository) RecordDelivery(ctx context.Context, tenantID string, bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO file_usage (tenant_id, date, bytes_delivered)
			VALUES ($1, CURRENT_DATE, $2)
			ON CONFLICT (tenant_id, date)
			DO UPDATE SET bytes_delivered = file_usage.bytes_delivered + EXCLUDED.bytes_delivered,
			              updated_at = now()
		`, tenantID, bytes)
		if err != nil {
			return fmt.Errorf("file: record delivery: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetUsage(ctx context.Context, tenantID string, limit int) ([]*ports.UsageRecord, error) {
	if limit <= 0 {
		limit = 30
	}
	var out []*ports.UsageRecord
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT tenant_id, date, bytes_stored, bytes_delivered
			FROM file_usage
			WHERE tenant_id = $1
			ORDER BY date DESC
			LIMIT $2
		`, tenantID, limit)
		if err != nil {
			return fmt.Errorf("file: get usage: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var rec ports.UsageRecord
			var date time.Time
			if err := rows.Scan(&rec.TenantID, &date, &rec.BytesStored, &rec.BytesDelivered); err != nil {
				return fmt.Errorf("file: scan usage: %w", err)
			}
			rec.Date = date.Format(time.DateOnly)
			out = append(out, &rec)
		}
		return rows.Err()
	})
	return out, err
}
