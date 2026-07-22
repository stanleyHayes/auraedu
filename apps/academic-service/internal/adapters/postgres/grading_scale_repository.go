package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/academic-service/internal/domain"
	"github.com/auraedu/academic-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// GradingScaleRepository persists tenant-owned grading policies.
type GradingScaleRepository struct{ db *db.DB }

var _ ports.GradingScaleRepository = (*GradingScaleRepository)(nil)

func NewGradingScaleRepository(database *db.DB) *GradingScaleRepository {
	return &GradingScaleRepository{db: database}
}

func (r *GradingScaleRepository) Create(ctx context.Context, tenantID string, scale *domain.GradingScale) error {
	ranges, err := json.Marshal(scale.Ranges)
	if err != nil {
		return fmt.Errorf("academic: encode grading ranges: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO grading_scales (id, tenant_id, name, ranges, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, scale.ID, tenantID, scale.Name, ranges, scale.CreatedAt, scale.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: create grading scale: %w", err)
		}
		return nil
	})
}

func (r *GradingScaleRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.GradingScale, error) {
	var scale *domain.GradingScale
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		got, err := scanGradingScale(tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, ranges, created_at, updated_at
			FROM grading_scales WHERE id = $1 AND tenant_id = $2
		`, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("academic: get grading scale: %w", err)
		}
		scale = got
		return nil
	})
	return scale, err
}

func (r *GradingScaleRepository) List(ctx context.Context, tenantID string, limit int, cursor string) ([]*domain.GradingScale, string, error) {
	var out []*domain.GradingScale
	var next string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var rows pgx.Rows
		var err error
		if cursor == "" {
			rows, err = tx.Query(ctx, `
				SELECT id, tenant_id, name, ranges, created_at, updated_at
				FROM grading_scales WHERE tenant_id = $1
				ORDER BY created_at, id LIMIT $2
			`, tenantID, limit)
		} else {
			rows, err = tx.Query(ctx, `
				SELECT id, tenant_id, name, ranges, created_at, updated_at
				FROM grading_scales
				WHERE tenant_id = $1 AND (created_at, id) > (
					SELECT created_at, id FROM grading_scales WHERE tenant_id = $1 AND id = $2
				)
				ORDER BY created_at, id LIMIT $3
			`, tenantID, cursor, limit)
		}
		if err != nil {
			return fmt.Errorf("academic: list grading scales: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			scale, err := scanGradingScale(rows)
			if err != nil {
				return fmt.Errorf("academic: scan grading scale: %w", err)
			}
			out = append(out, scale)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("academic: list grading scale rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			next = out[len(out)-1].ID
		}
		return nil
	})
	return out, next, err
}

func (r *GradingScaleRepository) Update(ctx context.Context, tenantID string, scale *domain.GradingScale) error {
	ranges, err := json.Marshal(scale.Ranges)
	if err != nil {
		return fmt.Errorf("academic: encode grading ranges: %w", err)
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE grading_scales SET name = $3, ranges = $4, updated_at = $5
			WHERE id = $1 AND tenant_id = $2
		`, scale.ID, tenantID, scale.Name, ranges, scale.UpdatedAt)
		if err != nil {
			return fmt.Errorf("academic: update grading scale: %w", err)
		}
		if result.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *GradingScaleRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM grading_scales WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("academic: delete grading scale: %w", err)
		}
		if result.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func scanGradingScale(row scanner) (*domain.GradingScale, error) {
	var scale domain.GradingScale
	var ranges []byte
	if err := row.Scan(&scale.ID, &scale.TenantID, &scale.Name, &ranges, &scale.CreatedAt, &scale.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(ranges, &scale.Ranges); err != nil {
		return nil, err
	}
	return &scale, nil
}
