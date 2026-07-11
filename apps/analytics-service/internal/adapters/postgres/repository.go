// Package postgres provides the Postgres implementation of the analytics-service repository.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/auraedu/analytics-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
type Repository struct {
	db *db.DB
}

var _ ports.Repository = (*Repository)(nil)

// NewRepository creates a Postgres-backed metrics repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func (r *Repository) UpsertMetric(ctx context.Context, tenantID string, m *domain.Metric) error {
	if err := m.Validate(); err != nil {
		return err
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		dims, err := json.Marshal(m.Dimensions)
		if err != nil {
			return fmt.Errorf("analytics: marshal dimensions: %w", err)
		}

		var sampleCount *int64
		if m.Unit == domain.UnitAverage {
			if m.SampleCount != nil {
				v := *m.SampleCount
				sampleCount = &v
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO metrics (
				id, tenant_id, metric_name, bucket_date, value, unit, dimensions, sample_count, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (tenant_id, metric_name, bucket_date, dimensions)
			DO UPDATE SET
				value = CASE
					WHEN EXCLUDED.unit = 'count' THEN metrics.value + EXCLUDED.value
					WHEN EXCLUDED.unit = 'sum' THEN metrics.value + EXCLUDED.value
					WHEN EXCLUDED.unit = 'average' THEN
						CASE
							WHEN COALESCE(metrics.sample_count, 0) <= 0 THEN EXCLUDED.value
							ELSE (
								metrics.value * metrics.sample_count +
								EXCLUDED.value * COALESCE(EXCLUDED.sample_count, 1)
							) / (
								metrics.sample_count + COALESCE(EXCLUDED.sample_count, 1)
							)
						END
					ELSE EXCLUDED.value
				END,
				sample_count = CASE
					WHEN EXCLUDED.unit = 'average' THEN COALESCE(metrics.sample_count, 0) + COALESCE(EXCLUDED.sample_count, 1)
					ELSE metrics.sample_count
				END,
				updated_at = EXCLUDED.updated_at
		`, m.ID, tenantID, m.MetricName, m.BucketDate.String(), m.Value, string(m.Unit), dims, sampleCount, m.CreatedAt, m.UpdatedAt)
		if err != nil {
			return fmt.Errorf("analytics: upsert metric: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListMetrics(ctx context.Context, tenantID string, filter ports.ListFilter) ([]*domain.Metric, string, error) {
	var out []*domain.Metric
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			m, err := scanMetric(rows)
			if err != nil {
				return err
			}
			out = append(out, m)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("analytics: list rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.ListFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.MetricName != "" {
		args = append(args, filter.MetricName)
		where += fmt.Sprintf(" AND metric_name = $%d", len(args))
	}
	if filter.BucketDateFrom != "" {
		args = append(args, filter.BucketDateFrom)
		where += fmt.Sprintf(" AND bucket_date >= $%d", len(args))
	}
	if filter.BucketDateTo != "" {
		args = append(args, filter.BucketDateTo)
		where += fmt.Sprintf(" AND bucket_date <= $%d", len(args))
	}
	if filter.DimensionKey != "" && filter.DimensionValue != "" {
		args = append(args, filter.DimensionKey, filter.DimensionValue)
		where += fmt.Sprintf(" AND dimensions->>$%d = $%d", len(args)-1, len(args))
	}

	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM metrics WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	args = append(args, limit)

	sql := fmt.Sprintf(`
		SELECT id, tenant_id, metric_name, bucket_date, value, unit, dimensions, sample_count, created_at, updated_at
		FROM metrics
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMetric(row scanner) (*domain.Metric, error) {
	var m domain.Metric
	var bucket time.Time
	var unitStr string
	var dimsJSON []byte
	var sampleCount *int64
	if err := row.Scan(
		&m.ID, &m.TenantID, &m.MetricName, &bucket, &m.Value, &unitStr, &dimsJSON, &sampleCount, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	m.BucketDate = domain.Date{Time: bucket}
	m.Unit = domain.Unit(unitStr)
	m.SampleCount = sampleCount
	if len(dimsJSON) > 0 && !strings.EqualFold(string(dimsJSON), "null") {
		var dims domain.Dimensions
		if err := json.Unmarshal(dimsJSON, &dims); err != nil {
			return nil, fmt.Errorf("analytics: unmarshal dimensions: %w", err)
		}
		m.Dimensions = dims
	}
	return &m, nil
}

// ParseNumeric delegates to strconv.ParseFloat for test helpers.
func ParseNumeric(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ErrNoRows returns pgx.ErrNoRows for error comparisons.
func ErrNoRows() error { return errors.New("no rows") }
