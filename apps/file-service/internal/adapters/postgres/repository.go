package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/file-service/internal/domain"
	"github.com/auraedu/file-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type Repository struct {
	db *db.DB
}

var _ ports.Repository = (*Repository)(nil)

// NewRepository creates a Postgres-backed file repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) Create(ctx context.Context, tenantID string, f *domain.FileUpload) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO file_uploads (
				id, tenant_id, original_filename, storage_path, storage_backend,
				content_type, size_bytes, checksum, owner_id, purpose, status, metadata,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, f.ID, tenantID, f.OriginalFilename, f.StoragePath, f.StorageBackend,
			f.ContentType, f.SizeBytes, f.Checksum, f.OwnerID, f.Purpose, f.Status, f.Metadata,
			f.CreatedAt, f.UpdatedAt)
		if err != nil {
			return fmt.Errorf("file: create: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id string) (*domain.FileUpload, error) {
	var f *domain.FileUpload
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT
				id, tenant_id, original_filename, storage_path, storage_backend,
				content_type, size_bytes, checksum, owner_id, purpose, status, metadata,
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
				content_type, size_bytes, checksum, owner_id, purpose, status, metadata,
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
			content_type, size_bytes, checksum, owner_id, purpose, status, metadata,
			created_at, updated_at
		FROM file_uploads
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *Repository) Update(ctx context.Context, tenantID string, f *domain.FileUpload) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
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
			    metadata = $12,
			    updated_at = $13
			WHERE id = $1 AND tenant_id = $2
		`, f.ID, tenantID, f.OriginalFilename, f.StoragePath, f.StorageBackend,
			f.ContentType, f.SizeBytes, f.Checksum, f.OwnerID, f.Purpose, f.Status, f.Metadata,
			f.UpdatedAt)
		if err != nil {
			return fmt.Errorf("file: update: %w", err)
		}
		return nil
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
		&f.ContentType, &f.SizeBytes, &f.Checksum, &f.OwnerID, &f.Purpose, &f.Status, &f.Metadata,
		&f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &f, nil
}
