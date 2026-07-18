package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// TermRepository is the Postgres implementation of ports.TermRepository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type TermRepository struct {
	db *db.DB
}

var _ ports.TermRepository = (*TermRepository)(nil)

// NewTermRepository creates a Postgres-backed term repository.
func NewTermRepository(database *db.DB) *TermRepository { return &TermRepository{db: database} }

func (r *TermRepository) Create(ctx context.Context, tenantID string, t *domain.Term) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO terms (id, tenant_id, academic_year_id, name, start_date, end_date, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, t.ID, tenantID, t.AcademicYearID, t.Name, t.StartDate, t.EndDate, t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: create term: %w", err)
		}
		return nil
	})
}

func (r *TermRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Term, error) {
	var t *domain.Term
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, academic_year_id, name, start_date, end_date, created_at, updated_at
			FROM terms
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanTerm(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("academic: get term: %w", err)
		}
		t = got
		return nil
	})
	return t, err
}

func (r *TermRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.Term, string, error) {
	var out []*domain.Term
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listTermsQuery(ctx, tx, tenantID, limit, cursor)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			t, err := scanTerm(rows)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("academic: list term rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listTermsQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string) (pgx.Rows, error) {
	if cursor != "" {
		return tx.Query(ctx, `
			SELECT id, tenant_id, academic_year_id, name, start_date, end_date, created_at, updated_at
			FROM terms
			WHERE tenant_id = $1 AND (created_at, id) > (
				SELECT created_at, id FROM terms WHERE id = $2 AND tenant_id = $1
			)
			ORDER BY created_at ASC, id ASC
			LIMIT $3
		`, tenantID, cursor, limit)
	}
	return tx.Query(ctx, `
		SELECT id, tenant_id, academic_year_id, name, start_date, end_date, created_at, updated_at
		FROM terms
		WHERE tenant_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, tenantID, limit)
}

func (r *TermRepository) Update(ctx context.Context, tenantID string, t *domain.Term) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE terms
			SET name = $3, start_date = $4, end_date = $5, updated_at = $6
			WHERE id = $1 AND tenant_id = $2
		`, t.ID, tenantID, t.Name, t.StartDate, t.EndDate, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: update term: %w", err)
		}
		return nil
	})
}

func (r *TermRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM terms WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("academic: delete term: %w", err)
		}
		return nil
	})
}

func scanTerm(row scanner) (*domain.Term, error) {
	var t domain.Term
	var start, end time.Time
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.AcademicYearID, &t.Name, &start, &end,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.StartDate = domain.Date{Time: start}
	t.EndDate = domain.Date{Time: end}
	return &t, nil
}
