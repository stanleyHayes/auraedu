package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/analytics-service/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ApplyAssessmentScoreEvent atomically deduplicates one lifecycle event,
// mutates the current score fact, and recomputes every affected aggregate.
func (r *Repository) ApplyAssessmentScoreEvent(ctx context.Context, tenantID string, event domain.AssessmentScoreEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO analytics_processed_events (tenant_id,event_id,event_type)
			VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, tenantID, event.EventID, event.EventType)
		if err != nil {
			return fmt.Errorf("analytics: record processed score event: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}

		previous, err := loadScoreFact(ctx, tx, tenantID, event.ScoreID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if event.Operation == domain.ScoreDeleted {
			if _, err := tx.Exec(ctx, `DELETE FROM assessment_score_facts WHERE tenant_id=$1 AND score_id=$2`, tenantID, event.ScoreID); err != nil {
				return fmt.Errorf("analytics: delete score fact: %w", err)
			}
		} else {
			occurredAt := event.OccurredAt
			if occurredAt.IsZero() {
				occurredAt = time.Now().UTC()
			}
			if _, err := tx.Exec(ctx, `INSERT INTO assessment_score_facts
				(tenant_id,score_id,assessment_id,student_id,subject_id,academic_year_id,bucket_date,score,max_score,recorded_at,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
				ON CONFLICT (tenant_id,score_id) DO UPDATE SET
				assessment_id=EXCLUDED.assessment_id,student_id=EXCLUDED.student_id,subject_id=EXCLUDED.subject_id,
				academic_year_id=EXCLUDED.academic_year_id,bucket_date=EXCLUDED.bucket_date,score=EXCLUDED.score,
				max_score=EXCLUDED.max_score,recorded_at=EXCLUDED.recorded_at,updated_at=EXCLUDED.updated_at`,
				tenantID, event.ScoreID, event.AssessmentID, event.StudentID, event.SubjectID, event.AcademicYearID,
				event.BucketDate(), event.Score, event.MaxScore, event.RecordedAt, occurredAt); err != nil {
				return fmt.Errorf("analytics: upsert score fact: %w", err)
			}
		}

		keys := []scoreRollupKey{}
		if previous != nil {
			keys = append(keys, previous.key())
		}
		if event.Operation != domain.ScoreDeleted {
			keys = appendUniqueScoreKey(keys, scoreRollupKey{
				BucketDate: event.BucketDate(), StudentID: event.StudentID,
				SubjectID: event.SubjectID, AcademicYearID: event.AcademicYearID,
			})
		}
		for _, key := range keys {
			if err := recomputeScoreRollup(ctx, tx, tenantID, key); err != nil {
				return err
			}
		}
		return nil
	})
}

type scoreFact struct {
	BucketDate     string
	StudentID      string
	SubjectID      string
	AcademicYearID string
}

func (fact scoreFact) key() scoreRollupKey {
	return scoreRollupKey(fact)
}

type scoreRollupKey struct {
	BucketDate     string
	StudentID      string
	SubjectID      string
	AcademicYearID string
}

func loadScoreFact(ctx context.Context, tx pgx.Tx, tenantID, scoreID string) (*scoreFact, error) {
	var fact scoreFact
	err := tx.QueryRow(ctx, `SELECT bucket_date::text,student_id::text,subject_id::text,academic_year_id::text
		FROM assessment_score_facts WHERE tenant_id=$1 AND score_id=$2`, tenantID, scoreID).
		Scan(&fact.BucketDate, &fact.StudentID, &fact.SubjectID, &fact.AcademicYearID)
	if err != nil {
		return nil, err
	}
	return &fact, nil
}

func appendUniqueScoreKey(keys []scoreRollupKey, candidate scoreRollupKey) []scoreRollupKey {
	for _, key := range keys {
		if key == candidate {
			return keys
		}
	}
	return append(keys, candidate)
}

func recomputeScoreRollup(ctx context.Context, tx pgx.Tx, tenantID string, key scoreRollupKey) error {
	var count int64
	var sum, average, percentage float64
	err := tx.QueryRow(ctx, `SELECT COUNT(*),COALESCE(SUM(score),0),COALESCE(AVG(score),0),COALESCE(AVG(score/max_score*100),0)
		FROM assessment_score_facts
		WHERE tenant_id=$1 AND bucket_date=$2 AND student_id=$3 AND subject_id=$4 AND academic_year_id=$5`,
		tenantID, key.BucketDate, key.StudentID, key.SubjectID, key.AcademicYearID).
		Scan(&count, &sum, &average, &percentage)
	if err != nil {
		return fmt.Errorf("analytics: aggregate score facts: %w", err)
	}
	dimensions := domain.Dimensions{
		"student_id": key.StudentID, "subject_id": key.SubjectID, "academic_year_id": key.AcademicYearID,
	}
	if count == 0 {
		dimsJSON, marshalErr := json.Marshal(dimensions)
		if marshalErr != nil {
			return fmt.Errorf("analytics: marshal score dimensions: %w", marshalErr)
		}
		_, err = tx.Exec(ctx, `DELETE FROM metrics WHERE tenant_id=$1 AND bucket_date=$2 AND dimensions=$3::jsonb
			AND metric_name = ANY($4::text[])`, tenantID, key.BucketDate, dimsJSON,
			[]string{"assessments.count", "assessments.sum_score", "assessments.avg_score", "assessments.avg_percentage"})
		return err
	}
	for _, spec := range []struct {
		name        string
		value       float64
		unit        domain.Unit
		sampleCount *int64
	}{
		{name: "assessments.count", value: float64(count), unit: domain.UnitCount},
		{name: "assessments.sum_score", value: sum, unit: domain.UnitSum},
		{name: "assessments.avg_score", value: average, unit: domain.UnitAverage, sampleCount: &count},
		{name: "assessments.avg_percentage", value: percentage, unit: domain.UnitAverage, sampleCount: &count},
	} {
		metric, err := domain.NewMetric(tenantID, spec.name, key.BucketDate, spec.value, spec.unit, dimensions)
		if err != nil {
			return err
		}
		metric.SampleCount = spec.sampleCount
		if err := replaceMetricTx(ctx, tx, tenantID, metric); err != nil {
			return err
		}
	}
	return nil
}

func replaceMetricTx(ctx context.Context, tx pgx.Tx, tenantID string, metric *domain.Metric) error {
	dimensions, err := json.Marshal(metric.Dimensions)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO metrics
		(id,tenant_id,metric_name,bucket_date,value,unit,dimensions,sample_count,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (tenant_id,metric_name,bucket_date,dimensions) DO UPDATE SET
		value=EXCLUDED.value,unit=EXCLUDED.unit,sample_count=EXCLUDED.sample_count,updated_at=EXCLUDED.updated_at`,
		metric.ID, tenantID, metric.MetricName, metric.BucketDate.String(), metric.Value, string(metric.Unit), dimensions,
		metric.SampleCount, metric.CreatedAt, metric.UpdatedAt)
	if err != nil {
		return fmt.Errorf("analytics: replace metric: %w", err)
	}
	return nil
}
