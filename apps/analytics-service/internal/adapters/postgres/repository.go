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
		return upsertMetricTx(ctx, tx, tenantID, m)
	})
}

// ApplyMetricEvent atomically deduplicates one CloudEvent and applies every
// metric derived from it. If any metric write fails, the event claim rolls
// back too so JetStream can safely redeliver the complete projection.
func (r *Repository) ApplyMetricEvent(ctx context.Context, tenantID, eventID, eventType string, metrics []*domain.Metric) error {
	if strings.TrimSpace(eventID) == "" || strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("analytics: event id and type are required")
	}
	for _, metric := range metrics {
		if metric == nil {
			return fmt.Errorf("analytics: metric event contains a nil metric")
		}
		if err := metric.Validate(); err != nil {
			return err
		}
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO analytics_processed_events (tenant_id,event_id,event_type)
			VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, tenantID, eventID, eventType)
		if err != nil {
			return fmt.Errorf("analytics: record processed metric event: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		for _, metric := range metrics {
			if err := upsertMetricTx(ctx, tx, tenantID, metric); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertMetricTx(ctx context.Context, tx pgx.Tx, tenantID string, m *domain.Metric) error {
	dims, err := json.Marshal(m.Dimensions)
	if err != nil {
		return fmt.Errorf("analytics: marshal dimensions: %w", err)
	}
	var sampleCount *int64
	if m.Unit == domain.UnitAverage && m.SampleCount != nil {
		v := *m.SampleCount
		sampleCount = &v
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
}

// ApplyGrowthEvent atomically deduplicates, attributes and counts one Growth event.
func (r *Repository) ApplyGrowthEvent(ctx context.Context, tenantID string, event domain.GrowthEvent) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		claimed, err := claimGrowthEvent(ctx, tx, tenantID, event)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		if err := storeGrowthAttribution(ctx, tx, tenantID, event); err != nil {
			return err
		}
		if err := storeGrowthFact(ctx, tx, tenantID, event); err != nil {
			return err
		}
		if err := hydrateGrowthAttribution(ctx, tx, tenantID, &event); err != nil {
			return err
		}
		metric, err := domain.NewMetric(
			tenantID, "growth.funnel."+event.Stage, event.BucketDate, 1, domain.UnitCount, growthDimensions(event),
		)
		if err != nil {
			return err
		}
		if err := upsertMetricTx(ctx, tx, tenantID, metric); err != nil {
			return err
		}
		// Preserve the EP-56 metric key used by existing smoke checks and clients.
		if event.Stage == domain.GrowthLeads {
			legacy, err := domain.NewMetric(tenantID, "growth.leads.count", event.BucketDate, 1, domain.UnitCount, nil)
			if err != nil {
				return err
			}
			return upsertMetricTx(ctx, tx, tenantID, legacy)
		}
		return nil
	})
}

func claimGrowthEvent(ctx context.Context, tx pgx.Tx, tenantID string, event domain.GrowthEvent) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO analytics_processed_events (tenant_id,event_id,event_type)
		VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, tenantID, event.EventID, event.EventType)
	if err != nil {
		return false, fmt.Errorf("analytics: record processed event: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func storeGrowthAttribution(ctx context.Context, tx pgx.Tx, tenantID string, event domain.GrowthEvent) error {
	if event.Stage == domain.GrowthLeads {
		_, err := tx.Exec(ctx, `
			INSERT INTO growth_lead_attribution (tenant_id,lead_id,source,campaign_id,created_at)
			VALUES ($1,$2,NULLIF($3,''),NULLIF($4,'')::uuid,$5)
			ON CONFLICT (tenant_id,lead_id) DO UPDATE
			SET source=EXCLUDED.source,campaign_id=EXCLUDED.campaign_id`,
			tenantID, event.LeadID, event.Source, event.CampaignID, event.OccurredAt)
		if err != nil {
			return fmt.Errorf("analytics: store lead attribution: %w", err)
		}
	}
	if event.ApplicationID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO growth_application_attribution (
			tenant_id,application_id,lead_id,programme_id,intake_id,started_at
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,'')::uuid,$6)
		ON CONFLICT (tenant_id,application_id) DO UPDATE SET
		lead_id=COALESCE(EXCLUDED.lead_id,growth_application_attribution.lead_id),
		programme_id=EXCLUDED.programme_id,
		intake_id=COALESCE(EXCLUDED.intake_id,growth_application_attribution.intake_id)`,
		tenantID, event.ApplicationID, event.LeadID, event.ProgrammeID, event.IntakeID, event.OccurredAt)
	if err != nil {
		return fmt.Errorf("analytics: store application attribution: %w", err)
	}
	return nil
}

func storeGrowthFact(ctx context.Context, tx pgx.Tx, tenantID string, event domain.GrowthEvent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO growth_event_facts (
			tenant_id,event_id,event_type,stage,bucket_date,lead_id,application_id,
			programme_id,intake_id,source,campaign_id,occurred_at
		) VALUES (
			$1,$2,$3,$4,$5,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,
			NULLIF($8,'')::uuid,NULLIF($9,'')::uuid,NULLIF($10,''),NULLIF($11,'')::uuid,$12
		)`, tenantID, event.EventID, event.EventType, event.Stage, event.BucketDate,
		event.LeadID, event.ApplicationID, event.ProgrammeID, event.IntakeID,
		event.Source, event.CampaignID, event.OccurredAt)
	if err != nil {
		return fmt.Errorf("analytics: store growth event fact: %w", err)
	}
	return nil
}

func hydrateGrowthAttribution(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	event *domain.GrowthEvent,
) error {
	if event.Source == "" && event.LeadID != "" {
		err := tx.QueryRow(ctx, `
			SELECT source,COALESCE(campaign_id::text,'')
			FROM growth_lead_attribution WHERE tenant_id=$1 AND lead_id=$2`,
			tenantID, event.LeadID).Scan(&event.Source, &event.CampaignID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("analytics: load lead attribution: %w", err)
		}
	}
	if event.Source == "" && event.ApplicationID != "" {
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(l.source,''),COALESCE(l.campaign_id::text,'')
			FROM growth_application_attribution a
			LEFT JOIN growth_lead_attribution l
				ON l.tenant_id=a.tenant_id AND l.lead_id=a.lead_id
			WHERE a.tenant_id=$1 AND a.application_id=$2`, tenantID, event.ApplicationID).
			Scan(&event.Source, &event.CampaignID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("analytics: load application attribution: %w", err)
		}
	}
	return nil
}

func growthDimensions(event domain.GrowthEvent) domain.Dimensions {
	dimensions := domain.Dimensions{}
	values := map[string]string{
		"source": event.Source, "campaign_id": event.CampaignID,
		"programme_id": event.ProgrammeID, "intake_id": event.IntakeID,
	}
	for key, value := range values {
		if value != "" {
			dimensions[key] = value
		}
	}
	return dimensions
}

func (r *Repository) GrowthRollups(ctx context.Context, tenantID, fromDate, toDate string) ([]domain.GrowthRollup, error) {
	var out []domain.GrowthRollup
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT f.stage,
			COALESCE(NULLIF(f.source,''),l.source,''),
			COALESCE(COALESCE(f.campaign_id,l.campaign_id)::text,''),
			COALESCE(COALESCE(f.programme_id,a.programme_id)::text,''),
			COALESCE(COALESCE(f.intake_id,a.intake_id)::text,''),
			COUNT(*)
			FROM growth_event_facts f
			LEFT JOIN growth_application_attribution a ON a.tenant_id=f.tenant_id AND a.application_id=f.application_id
			LEFT JOIN growth_lead_attribution l ON l.tenant_id=f.tenant_id AND l.lead_id=COALESCE(f.lead_id,a.lead_id)
			WHERE f.tenant_id=$1 AND f.bucket_date BETWEEN $2 AND $3
			GROUP BY 1,2,3,4,5 ORDER BY 1,2,4`, tenantID, fromDate, toDate)
		if err != nil {
			return fmt.Errorf("analytics: growth rollups: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.GrowthRollup
			if err := rows.Scan(&item.Stage, &item.Source, &item.CampaignID, &item.ProgrammeID, &item.IntakeID, &item.Value); err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
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
	if filter.StudentIDs != nil {
		args = append(args, filter.StudentIDs)
		where += fmt.Sprintf(" AND dimensions->>'student_id' = ANY($%d::text[])", len(args))
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
