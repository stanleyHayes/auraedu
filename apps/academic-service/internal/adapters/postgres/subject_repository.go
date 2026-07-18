package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// SubjectRepository is the Postgres implementation of ports.SubjectRepository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type SubjectRepository struct {
	db *db.DB
}

var _ ports.SubjectRepository = (*SubjectRepository)(nil)

// NewSubjectRepository creates a Postgres-backed subject repository.
func NewSubjectRepository(database *db.DB) *SubjectRepository {
	return &SubjectRepository{db: database}
}

func (r *SubjectRepository) Create(ctx context.Context, tenantID string, s *domain.Subject) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO subjects (id, tenant_id, name, code, description, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, s.ID, tenantID, s.Name, s.Code, s.Description, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: create subject: %w", err)
		}
		return nil
	})
}

func (r *SubjectRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Subject, error) {
	var s *domain.Subject
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, code, description, created_at, updated_at
			FROM subjects
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanSubject(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("academic: get subject: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *SubjectRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Subject, string, error) {
	var out []*domain.Subject
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listSubjectsQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s, err := scanSubject(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("academic: list subject rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listSubjectsQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT id, tenant_id, name, code, description, created_at, updated_at
			FROM subjects
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM subjects WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, name, code, description, created_at, updated_at
		FROM subjects
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *SubjectRepository) Update(ctx context.Context, tenantID string, s *domain.Subject) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE subjects
			SET name = $3, code = $4, description = $5, updated_at = $6
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.Name, s.Code, s.Description, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: update subject: %w", err)
		}
		return nil
	})
}

func (r *SubjectRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM subjects WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("academic: delete subject: %w", err)
		}
		return nil
	})
}

func scanSubject(row scanner) (*domain.Subject, error) {
	var s domain.Subject
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.Name, &s.Code, &s.Description,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
