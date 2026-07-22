package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

type JourneyRepository struct {
	db *db.DB
}

var _ ports.JourneyRepository = (*JourneyRepository)(nil)

func NewJourneyRepository(database *db.DB) *JourneyRepository {
	return &JourneyRepository{db: database}
}

func (r *JourneyRepository) CreateJourney(ctx context.Context, tenantID string, journey *domain.Journey) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO communication_journeys (
				id,tenant_id,name,trigger_event,status,timezone,quiet_hours_start_minute,
				quiet_hours_end_minute,frequency_window_hours,frequency_limit,cancel_on_events,
				version,created_by,activated_by,activated_at,created_at,updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		`, journey.ID, tenantID, journey.Name, journey.TriggerEvent, journey.Status, journey.Timezone,
			journey.QuietHoursStartMinute, journey.QuietHoursEndMinute, journey.FrequencyWindowHours,
			journey.FrequencyLimit, journey.CancelOnEvents, journey.Version, journey.CreatedBy,
			journey.ActivatedBy, journey.ActivatedAt, journey.CreatedAt, journey.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: create communication journey: %w", err)
		}
		for _, step := range journey.Steps {
			_, err = tx.Exec(ctx, `
				INSERT INTO communication_journey_steps (
					id,tenant_id,journey_id,position,channel,template_id,delay_minutes,
					condition_operator,condition_field,condition_value
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			`, step.ID, tenantID, journey.ID, step.Position, step.Channel, step.TemplateID,
				step.DelayMinutes, step.ConditionOperator, step.ConditionField, step.ConditionValue)
			if err != nil {
				return fmt.Errorf("notifications: create communication journey step: %w", err)
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (tenant_id,event_type,payload)
			VALUES ($1,'communication.journey_changed.v1',jsonb_build_object(
				'journey_id',$2::text,'status',$3::text,'trigger_event',$4::text,'version',$5::integer,
				'step_count',$6::integer,'changed_by',$7::text,'changed_at',$8::timestamptz
			))
		`, tenantID, journey.ID, journey.Status, journey.TriggerEvent, journey.Version,
			len(journey.Steps), journey.CreatedBy, journey.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: enqueue communication journey audit event: %w", err)
		}
		return nil
	})
}

func (r *JourneyRepository) GetJourney(ctx context.Context, tenantID, id string) (*domain.Journey, error) {
	var journey *domain.Journey
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		journey, err = getJourney(ctx, tx, tenantID, id)
		return err
	})
	return journey, err
}

func (r *JourneyRepository) ListJourneys(ctx context.Context, tenantID string, filter ports.JourneyFilter) ([]*domain.Journey, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 25
	}
	var journeys []*domain.Journey
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text FROM communication_journeys
			WHERE tenant_id=$1 AND ($2='' OR status=$2) AND ($3='' OR trigger_event=$3)
			ORDER BY created_at DESC,id DESC LIMIT $4
		`, tenantID, filter.Status, filter.TriggerEvent, filter.Limit)
		if err != nil {
			return fmt.Errorf("notifications: list communication journeys: %w", err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return fmt.Errorf("notifications: scan communication journey id: %w", err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, id := range ids {
			journey, err := getJourney(ctx, tx, tenantID, id)
			if err != nil {
				return err
			}
			journeys = append(journeys, journey)
		}
		return nil
	})
	return journeys, err
}

func (r *JourneyRepository) UpdateJourneyStatus(ctx context.Context, tenantID string, journey *domain.Journey, changedBy string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE communication_journeys
			SET status=$3,activated_by=$4,activated_at=$5,updated_at=$6
			WHERE tenant_id=$1 AND id=$2
		`, tenantID, journey.ID, journey.Status, journey.ActivatedBy, journey.ActivatedAt, journey.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: update communication journey status: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrNotFound
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (tenant_id,event_type,payload)
			VALUES ($1,'communication.journey_changed.v1',jsonb_build_object(
				'journey_id',$2::text,'status',$3::text,'trigger_event',$4::text,'version',$5::integer,
				'step_count',$6::integer,'changed_by',$7::text,'changed_at',$8::timestamptz
			))
		`, tenantID, journey.ID, journey.Status, journey.TriggerEvent, journey.Version,
			len(journey.Steps), changedBy, journey.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: enqueue communication journey audit event: %w", err)
		}
		return nil
	})
}

func (r *JourneyRepository) ListActiveJourneysByTrigger(ctx context.Context, tenantID, eventType string) ([]*domain.Journey, error) {
	return r.ListJourneys(ctx, tenantID, ports.JourneyFilter{Status: string(domain.JourneyStatusActive), TriggerEvent: eventType, Limit: 100})
}

func (r *JourneyRepository) EnrollJourney(ctx context.Context, enrollment ports.JourneyEnrollment) (bool, error) {
	created := false
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var version int
		err := tx.QueryRow(ctx, `
			SELECT version FROM communication_journeys
			WHERE tenant_id=$1 AND id=$2 AND status='active'
			FOR SHARE
		`, enrollment.TenantID, enrollment.JourneyID).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("notifications: lock communication journey: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO communication_journey_enrollments (
				id,tenant_id,journey_id,journey_version,event_id,trigger_event,lead_id,
				status,skipped_steps
			) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8)
			ON CONFLICT (tenant_id,journey_id,event_id) DO NOTHING
		`, enrollment.ID, enrollment.TenantID, enrollment.JourneyID, version, enrollment.EventID,
			enrollment.TriggerEvent, enrollment.LeadID, enrollment.SkippedSteps)
		if err != nil {
			return fmt.Errorf("notifications: enroll communication journey: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		for _, message := range enrollment.Messages {
			metadata, marshalErr := json.Marshal(message.Metadata)
			if marshalErr != nil {
				return fmt.Errorf("notifications: marshal journey message metadata: %w", marshalErr)
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO messages (
					id,tenant_id,recipient_id,channel,template_id,subject,body,status,metadata,
					scheduled_at,sent_at,error,created_at,updated_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			`, message.ID, enrollment.TenantID, message.RecipientID, message.Channel, message.TemplateID,
				message.Subject, message.Body, message.Status, metadata, message.ScheduledAt,
				message.SentAt, message.Error, message.CreatedAt, message.UpdatedAt)
			if err != nil {
				return fmt.Errorf("notifications: schedule journey message: %w", err)
			}
		}
		if len(enrollment.Messages) == 0 {
			_, err = tx.Exec(ctx, `
				UPDATE communication_journey_enrollments SET status='completed'
				WHERE tenant_id=$1 AND id=$2
			`, enrollment.TenantID, enrollment.ID)
			if err != nil {
				return fmt.Errorf("notifications: complete empty journey enrollment: %w", err)
			}
		}
		created = true
		return nil
	})
	return created, err
}

func (r *JourneyRepository) CancelJourneysForEvent(ctx context.Context, tenantID, leadID, eventID, eventType string) (int64, error) {
	var cancelled int64
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE communication_journey_enrollments e
			SET status='cancelled',cancelled_at=now(),cancellation_event=$3
			FROM communication_journeys j
			WHERE e.tenant_id=$1 AND e.lead_id=$2 AND e.status='active'
			  AND j.tenant_id=e.tenant_id AND j.id=e.journey_id
			  AND $3=ANY(j.cancel_on_events)
			RETURNING e.id::text
		`, tenantID, leadID, eventType)
		if err != nil {
			return fmt.Errorf("notifications: cancel journey enrollments: %w", err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, id := range ids {
			tag, err := tx.Exec(ctx, `
				UPDATE messages SET status='cancelled',updated_at=now(),
					metadata=metadata || jsonb_build_object(
						'journey_cancellation_event',$3::text,
						'journey_cancellation_event_id',$4::text
					)
				WHERE tenant_id=$1 AND status='pending' AND metadata->>'journey_enrollment_id'=$2
			`, tenantID, id, eventType, eventID)
			if err != nil {
				return fmt.Errorf("notifications: cancel journey messages: %w", err)
			}
			cancelled += tag.RowsAffected()
		}
		return nil
	})
	return cancelled, err
}

func (r *JourneyRepository) FinalizeJourneyEnrollment(ctx context.Context, tenantID, enrollmentID string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE communication_journey_enrollments e SET status='completed'
			WHERE e.tenant_id=$1 AND e.id=$2 AND e.status='active'
			  AND NOT EXISTS (
				SELECT 1 FROM messages m
				WHERE m.tenant_id=e.tenant_id
				  AND m.metadata->>'journey_enrollment_id'=e.id::text
				  AND m.status='pending'
			  )
		`, tenantID, enrollmentID)
		if err != nil {
			return fmt.Errorf("notifications: finalize journey enrollment: %w", err)
		}
		return nil
	})
}

func (r *JourneyRepository) JourneyStats(ctx context.Context, tenantID, journeyID string) (ports.JourneyStats, error) {
	var stats ports.JourneyStats
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT count(*),COALESCE(sum(skipped_steps),0)
			FROM communication_journey_enrollments
			WHERE tenant_id=$1 AND journey_id=$2
		`, tenantID, journeyID).Scan(&stats.Enrolled, &stats.Skipped)
		if err != nil {
			return fmt.Errorf("notifications: communication journey enrollment stats: %w", err)
		}
		return tx.QueryRow(ctx, `
			SELECT
				count(*) FILTER (WHERE status='pending'),
				count(*) FILTER (WHERE status='sent'),
				count(*) FILTER (WHERE status='failed'),
				count(*) FILTER (WHERE status='cancelled'),
				count(*) FILTER (WHERE provider IS NOT NULL AND status='sent'),
				count(*) FILTER (WHERE delivery_status='delivered'),
				count(*) FILTER (WHERE delivery_status='delayed'),
				count(*) FILTER (WHERE delivery_status='bounced'),
				count(*) FILTER (WHERE delivery_status='complained'),
				count(*) FILTER (WHERE delivery_status='suppressed')
			FROM messages
			WHERE tenant_id=$1 AND metadata->>'journey_id'=$2
		`, tenantID, journeyID).Scan(
			&stats.Scheduled, &stats.Sent, &stats.Failed, &stats.Cancelled,
			&stats.Accepted, &stats.Delivered, &stats.Delayed, &stats.Bounced,
			&stats.Complained, &stats.Suppressed,
		)
	})
	return stats, err
}

func (r *MessageRepository) NextJourneyDeliveryAllowedAt(
	ctx context.Context,
	tenantID, journeyID, recipientID string,
	window time.Duration,
	limit int,
) (*time.Time, error) {
	if window <= 0 || limit <= 0 {
		return nil, nil
	}
	var next *time.Time
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var count int
		var oldest *time.Time
		if err := tx.QueryRow(ctx, `
			SELECT count(*),min(sent_at)
			FROM messages
			WHERE tenant_id=$1 AND recipient_id=$2 AND status='sent'
			  AND metadata->>'journey_id'=$3 AND sent_at > now()-$4::interval
		`, tenantID, recipientID, journeyID, window.String()).Scan(&count, &oldest); err != nil {
			return fmt.Errorf("notifications: inspect journey frequency: %w", err)
		}
		if count >= limit && oldest != nil {
			allowed := oldest.Add(window).UTC()
			next = &allowed
		}
		return nil
	})
	return next, err
}

func getJourney(ctx context.Context, tx pgx.Tx, tenantID, id string) (*domain.Journey, error) {
	journey := &domain.Journey{}
	err := tx.QueryRow(ctx, `
		SELECT id::text,tenant_id,name,trigger_event,status,timezone,quiet_hours_start_minute,
			quiet_hours_end_minute,frequency_window_hours,frequency_limit,cancel_on_events,
			version,created_by::text,activated_by::text,activated_at,created_at,updated_at
		FROM communication_journeys WHERE tenant_id=$1 AND id=$2
	`, tenantID, id).Scan(
		&journey.ID, &journey.TenantID, &journey.Name, &journey.TriggerEvent, &journey.Status,
		&journey.Timezone, &journey.QuietHoursStartMinute, &journey.QuietHoursEndMinute,
		&journey.FrequencyWindowHours, &journey.FrequencyLimit, &journey.CancelOnEvents,
		&journey.Version, &journey.CreatedBy, &journey.ActivatedBy, &journey.ActivatedAt,
		&journey.CreatedAt, &journey.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("notifications: get communication journey: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text,position,channel,template_id::text,delay_minutes,
			condition_operator,condition_field,condition_value
		FROM communication_journey_steps
		WHERE tenant_id=$1 AND journey_id=$2 ORDER BY position
	`, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("notifications: list communication journey steps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var step domain.JourneyStep
		if err := rows.Scan(&step.ID, &step.Position, &step.Channel, &step.TemplateID,
			&step.DelayMinutes, &step.ConditionOperator, &step.ConditionField, &step.ConditionValue); err != nil {
			return nil, fmt.Errorf("notifications: scan communication journey step: %w", err)
		}
		journey.Steps = append(journey.Steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return journey, nil
}
