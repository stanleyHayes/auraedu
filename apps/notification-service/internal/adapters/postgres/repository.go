// Package postgres provides Postgres-backed repositories for the notification service.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MessageRepository is the Postgres implementation of ports.MessageRepository.
type MessageRepository struct {
	db *db.DB
}

// TemplateRepository is the Postgres implementation of ports.TemplateRepository.
type TemplateRepository struct {
	db *db.DB
}

// SubscriptionRepository is the Postgres implementation of ports.SubscriptionRepository.
type SubscriptionRepository struct {
	db *db.DB
}

var (
	_ ports.MessageRepository          = (*MessageRepository)(nil)
	_ ports.ScheduledMessageRepository = (*MessageRepository)(nil)
	_ ports.DurableDeliveryRepository  = (*MessageRepository)(nil)
	_ ports.DeliveryFeedbackRepository = (*MessageRepository)(nil)
	_ ports.OutboxRepository           = (*MessageRepository)(nil)
	_ ports.TemplateRepository         = (*TemplateRepository)(nil)
	_ ports.SubscriptionRepository     = (*SubscriptionRepository)(nil)
)

// NewMessageRepository creates a Postgres-backed message repository.
func NewMessageRepository(database *db.DB) *MessageRepository {
	return &MessageRepository{db: database}
}

func (r *MessageRepository) ClaimDue(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]*domain.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	adminCtx := auth.WithActor(ctx, auth.Actor{UserID: "notification-scheduler", PlatformAdmin: true})
	rows, err := r.db.Query(adminCtx, `
		WITH due AS (
			SELECT id FROM messages
			WHERE status='pending' AND scheduled_at IS NOT NULL AND scheduled_at <= now()
			ORDER BY scheduled_at ASC FOR UPDATE SKIP LOCKED LIMIT $1
		)
		UPDATE messages m SET scheduled_at=now()+$2::interval, updated_at=now()
		FROM due WHERE m.id=due.id
		RETURNING m.id,m.tenant_id,m.recipient_id,m.channel,m.template_id,m.subject,m.body,
			m.status,m.metadata,m.scheduled_at,m.sent_at,m.error,m.provider,m.delivery_status,
			m.delivery_status_at,m.created_at,m.updated_at
	`, limit, lease.String())
	if err != nil {
		return nil, fmt.Errorf("notifications: claim due messages: %w", err)
	}
	defer rows.Close()
	var messages []*domain.Message
	for rows.Next() {
		message, scanErr := scanMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *MessageRepository) CancelByApplication(
	ctx context.Context,
	tenantID string,
	applicationID string,
) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE messages SET status='cancelled',updated_at=now()
			WHERE tenant_id=$1 AND status='pending' AND metadata->>'application_id'=$2
		`, tenantID, applicationID)
		return err
	})
}

// NewTemplateRepository creates a Postgres-backed template repository.
func NewTemplateRepository(database *db.DB) *TemplateRepository {
	return &TemplateRepository{db: database}
}

// NewSubscriptionRepository creates a Postgres-backed subscription repository.
func NewSubscriptionRepository(database *db.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: database}
}

// --- Message persistence ---

func (r *MessageRepository) Create(ctx context.Context, tenantID string, m *domain.Message) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		metadata, err := json.Marshal(m.Metadata)
		if err != nil {
			return fmt.Errorf("notifications: marshal metadata: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO messages (
				id, tenant_id, recipient_id, channel, template_id, subject, body, status,
				metadata, scheduled_at, sent_at, error, provider, delivery_status, delivery_status_at,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, '{}'::jsonb), $10, $11, $12, $13, $14, $15, $16, $17)
		`, m.ID, tenantID, m.RecipientID, m.Channel, m.TemplateID, m.Subject, m.Body, m.Status,
			metadata, m.ScheduledAt, m.SentAt, m.Error, m.Provider, m.DeliveryStatus,
			m.DeliveryStatusAt, m.CreatedAt, m.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: create message: %w", err)
		}
		return nil
	})
}

func (r *MessageRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Message, error) {
	var m *domain.Message
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, recipient_id, channel, template_id, subject, body, status,
			       metadata, scheduled_at, sent_at, error, provider, delivery_status,
			       delivery_status_at, created_at, updated_at
			FROM messages
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanMessage(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("notifications: get message: %w", err)
		}
		m = got
		return nil
	})
	return m, err
}

func (r *MessageRepository) List(ctx context.Context, tenantID string, filter ports.MessageFilter) ([]*domain.Message, string, error) {
	var out []*domain.Message
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listMessagesQuery(ctx, tx, tenantID, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			m, err := scanMessage(rows)
			if err != nil {
				return err
			}
			out = append(out, m)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("notifications: list messages rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listMessagesQuery(ctx context.Context, tx pgx.Tx, tenantID string, filter ports.MessageFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.Channel != "" {
		args = append(args, filter.Channel)
		where += fmt.Sprintf(" AND channel = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.RecipientID != "" {
		args = append(args, filter.RecipientID)
		where += fmt.Sprintf(" AND recipient_id = $%d", len(args))
	}
	if filter.Cursor != "" {
		args = append(args, filter.Cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM messages WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, filter.Limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, recipient_id, channel, template_id, subject, body, status,
		       metadata, scheduled_at, sent_at, error, provider, delivery_status,
		       delivery_status_at, created_at, updated_at
		FROM messages
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *MessageRepository) Update(ctx context.Context, tenantID string, m *domain.Message) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		metadata, err := json.Marshal(m.Metadata)
		if err != nil {
			return fmt.Errorf("notifications: marshal metadata: %w", err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE messages
			SET recipient_id = $3, channel = $4, template_id = $5, subject = $6, body = $7, status = $8,
			    metadata = COALESCE($9, '{}'::jsonb), scheduled_at = $10, sent_at = $11, error = $12,
			    provider = $13, delivery_status = $14, delivery_status_at = $15, updated_at = $16
			WHERE id = $1 AND tenant_id = $2
		`, m.ID, tenantID, m.RecipientID, m.Channel, m.TemplateID, m.Subject, m.Body, m.Status,
			metadata, m.ScheduledAt, m.SentAt, m.Error, m.Provider, m.DeliveryStatus,
			m.DeliveryStatusAt, m.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: update message: %w", err)
		}
		return nil
	})
}

// CommitDeliveryOutcome makes the stored delivery state and its integration
// event one database fact. The external provider call happens before this
// transaction; stable message IDs are used as provider idempotency identifiers.
func (r *MessageRepository) CommitDeliveryOutcome(
	ctx context.Context,
	tenantID string,
	m *domain.Message,
	providerMessageID string,
	create bool,
	eventType string,
	payload map[string]any,
) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		metadata, err := json.Marshal(m.Metadata)
		if err != nil {
			return fmt.Errorf("notifications: marshal delivery metadata: %w", err)
		}
		if create {
			_, err = tx.Exec(ctx, `
				INSERT INTO messages (
					id, tenant_id, recipient_id, channel, template_id, subject, body, status,
					metadata, scheduled_at, sent_at, error, provider, provider_message_id,
					delivery_status, delivery_status_at, created_at, updated_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,COALESCE($9,'{}'::jsonb),$10,$11,$12,$13,NULLIF($14,''),$15,$16,$17,$18)
			`, m.ID, tenantID, m.RecipientID, m.Channel, m.TemplateID, m.Subject, m.Body, m.Status,
				metadata, m.ScheduledAt, m.SentAt, m.Error, m.Provider, providerMessageID,
				m.DeliveryStatus, m.DeliveryStatusAt, m.CreatedAt, m.UpdatedAt)
		} else {
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, `
				UPDATE messages SET status=$3, metadata=COALESCE($4,'{}'::jsonb), scheduled_at=$5,
					sent_at=$6, error=$7, provider=$8, provider_message_id=NULLIF($9,''),
					delivery_status=$10, delivery_status_at=$11, updated_at=$12
				WHERE id=$1 AND tenant_id=$2
			`, m.ID, tenantID, m.Status, metadata, m.ScheduledAt, m.SentAt, m.Error,
				m.Provider, providerMessageID, m.DeliveryStatus, m.DeliveryStatusAt, m.UpdatedAt)
			if err == nil && tag.RowsAffected() != 1 {
				return domain.ErrNotFound
			}
		}
		if err != nil {
			return fmt.Errorf("notifications: persist delivery outcome: %w", err)
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("notifications: marshal delivery event: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO notification_outbox (tenant_id,event_type,payload)
			VALUES ($1,$2,$3)
		`, tenantID, eventType, body)
		if err != nil {
			return fmt.Errorf("notifications: enqueue delivery event: %w", err)
		}
		return nil
	})
}

func (r *MessageRepository) ClaimPendingNotificationEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var events []ports.OutboxEvent
	err := r.db.WithTx(notificationOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE notification_outbox
			SET attempts=attempts+1,
				next_attempt_at=now() + (LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM notification_outbox
				WHERE published_at IS NULL AND next_attempt_at <= now()
				ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id::text,tenant_id,event_type,payload
		`, limit)
		if err != nil {
			return fmt.Errorf("notifications: claim outbox: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var event ports.OutboxEvent
			if err := rows.Scan(&event.ID, &event.TenantID, &event.EventType, &event.Payload); err != nil {
				return fmt.Errorf("notifications: scan outbox: %w", err)
			}
			events = append(events, event)
		}
		return rows.Err()
	})
	return events, err
}

func (r *MessageRepository) MarkNotificationEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(notificationOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE notification_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}

func (r *MessageRepository) MarkNotificationEventFailed(ctx context.Context, id, reason string) error {
	return r.db.WithTx(notificationOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE notification_outbox
			SET last_error=left($2,2000)
			WHERE id=$1
		`, id, reason)
		return err
	})
}

func notificationOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{UserID: "notification-outbox-worker", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__notification_outbox__"})
}

func deliveryFeedbackContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{UserID: "notification-delivery-webhook", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__notification_delivery_feedback__"})
}

func (r *MessageRepository) IsEmailSuppressed(ctx context.Context, tenantID, addressHash string) (bool, error) {
	var suppressed bool
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM notification_email_suppressions
				WHERE tenant_id=$1 AND address_hash=$2
			)
		`, tenantID, addressHash).Scan(&suppressed)
	})
	if err != nil {
		return false, fmt.Errorf("notifications: inspect email suppression: %w", err)
	}
	return suppressed, nil
}

func (r *MessageRepository) SuppressEmail(ctx context.Context, tenantID, addressHash, reason, eventID string, occurredAt time.Time) error {
	ctx = tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID, ActorRole: "public_email_preference"})
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO notification_email_suppressions (
				tenant_id,address_hash,reason,provider,first_event_id,last_event_id,suppressed_at,updated_at
			) VALUES ($1,$2,$3,'auraedu',$4,$4,$5,now())
			ON CONFLICT (tenant_id,address_hash) DO UPDATE SET
				reason=EXCLUDED.reason,provider=EXCLUDED.provider,last_event_id=EXCLUDED.last_event_id,
				suppressed_at=LEAST(notification_email_suppressions.suppressed_at,EXCLUDED.suppressed_at),updated_at=now()
		`, tenantID, addressHash, reason, eventID, occurredAt)
		return err
	})
	if err != nil {
		return fmt.Errorf("notifications: persist email opt-out: %w", err)
	}
	return nil
}

// ApplyDeliveryFeedback stores every verified provider event exactly once,
// advances the message projection monotonically, and creates a
// tenant-scoped suppression without retaining the recipient address.
func (r *MessageRepository) ApplyDeliveryFeedback(ctx context.Context, feedback ports.DeliveryFeedback) (bool, error) {
	applied := false
	err := r.db.WithTx(deliveryFeedbackContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		var tenantID string
		err := tx.QueryRow(ctx, `
			SELECT tenant_id FROM messages
			WHERE id=$1 AND provider=$2 AND provider_message_id=$3
			  AND metadata->>'delivery_address_hash'=$4
		`, feedback.MessageID, feedback.Provider, feedback.ProviderMessageID, feedback.AddressHash).Scan(&tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("notifications: correlate delivery feedback: %w", err)
		}

		tag, err := tx.Exec(ctx, `
			INSERT INTO notification_delivery_events (
				id,tenant_id,message_id,provider,provider_message_id,event_type,status,occurred_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO NOTHING
		`, feedback.ID, tenantID, feedback.MessageID, feedback.Provider, feedback.ProviderMessageID, feedback.EventType, feedback.Status, feedback.OccurredAt)
		if err != nil {
			return fmt.Errorf("notifications: persist delivery feedback: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		applied = true

		_, err = tx.Exec(ctx, `
			UPDATE messages SET delivery_status=$4,delivery_status_at=$5,updated_at=now()
			WHERE id=$1 AND provider=$2 AND provider_message_id=$3
			  AND (
				delivery_status IS NULL OR
				CASE delivery_status
					WHEN 'accepted' THEN 1 WHEN 'delayed' THEN 2 WHEN 'delivered' THEN 3
					WHEN 'failed' THEN 4 WHEN 'bounced' THEN 5 WHEN 'suppressed' THEN 6
					WHEN 'complained' THEN 7 ELSE 0 END
				< CASE $4::text
					WHEN 'accepted' THEN 1 WHEN 'delayed' THEN 2 WHEN 'delivered' THEN 3
					WHEN 'failed' THEN 4 WHEN 'bounced' THEN 5 WHEN 'suppressed' THEN 6
					WHEN 'complained' THEN 7 ELSE 0 END
				OR (delivery_status_at < $5 AND
					CASE delivery_status
						WHEN 'accepted' THEN 1 WHEN 'delayed' THEN 2 WHEN 'delivered' THEN 3
						WHEN 'failed' THEN 4 WHEN 'bounced' THEN 5 WHEN 'suppressed' THEN 6
						WHEN 'complained' THEN 7 ELSE 0 END
					= CASE $4::text
						WHEN 'accepted' THEN 1 WHEN 'delayed' THEN 2 WHEN 'delivered' THEN 3
						WHEN 'failed' THEN 4 WHEN 'bounced' THEN 5 WHEN 'suppressed' THEN 6
						WHEN 'complained' THEN 7 ELSE 0 END)
			  )
		`, feedback.MessageID, feedback.Provider, feedback.ProviderMessageID, feedback.Status, feedback.OccurredAt)
		if err != nil {
			return fmt.Errorf("notifications: project delivery feedback: %w", err)
		}

		if feedback.Status == string(domain.DeliveryStatusBounced) ||
			feedback.Status == string(domain.DeliveryStatusComplained) ||
			feedback.Status == string(domain.DeliveryStatusSuppressed) {
			_, err = tx.Exec(ctx, `
				INSERT INTO notification_email_suppressions (
					tenant_id,address_hash,reason,provider,first_event_id,last_event_id,suppressed_at,updated_at
				) VALUES ($1,$2,$3,$4,$5,$5,$6,now())
				ON CONFLICT (tenant_id,address_hash) DO UPDATE SET
					reason=EXCLUDED.reason,provider=EXCLUDED.provider,last_event_id=EXCLUDED.last_event_id,
					suppressed_at=LEAST(notification_email_suppressions.suppressed_at,EXCLUDED.suppressed_at),updated_at=now()
			`, tenantID, feedback.AddressHash, feedback.Status, feedback.Provider, feedback.ID, feedback.OccurredAt)
			if err != nil {
				return fmt.Errorf("notifications: persist email suppression: %w", err)
			}
		}
		return nil
	})
	return applied, err
}

func (r *MessageRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM messages
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("notifications: delete message: %w", err)
		}
		return nil
	})
}

// --- Template persistence ---

func (r *TemplateRepository) Create(ctx context.Context, tenantID string, t *domain.Template) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO notification_templates (id, tenant_id, name, channel, subject_template, body_template, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, t.ID, tenantID, t.Name, t.Channel, t.SubjectTemplate, t.BodyTemplate, t.Status, t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: create template: %w", err)
		}
		return nil
	})
}

func (r *TemplateRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Template, error) {
	var t *domain.Template
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, channel, subject_template, body_template, status, created_at, updated_at
			FROM notification_templates
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanTemplate(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("notifications: get template: %w", err)
		}
		t = got
		return nil
	})
	return t, err
}

func (r *TemplateRepository) List(ctx context.Context, tenantID string, filter ports.TemplateFilter) ([]*domain.Template, string, error) {
	var out []*domain.Template
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID}
		where := "tenant_id = $1"
		if filter.Channel != "" {
			args = append(args, filter.Channel)
			where += fmt.Sprintf(" AND channel = $%d", len(args))
		}
		if filter.Status != "" {
			args = append(args, filter.Status)
			where += fmt.Sprintf(" AND status = $%d", len(args))
		}
		if filter.Cursor != "" {
			args = append(args, filter.Cursor)
			where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM notification_templates WHERE id = $%d AND tenant_id = $1)", len(args))
		}
		args = append(args, filter.Limit)
		sql := fmt.Sprintf(`
			SELECT id, tenant_id, name, channel, subject_template, body_template, status, created_at, updated_at
			FROM notification_templates
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		`, where, len(args))

		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			t, err := scanTemplate(rows)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("notifications: list templates rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func (r *TemplateRepository) Update(ctx context.Context, tenantID string, t *domain.Template) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE notification_templates
			SET name = $3, channel = $4, subject_template = $5, body_template = $6, status = $7, updated_at = $8
			WHERE id = $1 AND tenant_id = $2
		`, t.ID, tenantID, t.Name, t.Channel, t.SubjectTemplate, t.BodyTemplate, t.Status, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: update template: %w", err)
		}
		return nil
	})
}

func (r *TemplateRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM notification_templates
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("notifications: delete template: %w", err)
		}
		return nil
	})
}

// --- Subscription persistence ---

func (r *SubscriptionRepository) Create(ctx context.Context, tenantID string, s *domain.Subscription) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO notification_subscriptions (id, tenant_id, user_id, channel, is_enabled, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, s.ID, tenantID, s.UserID, s.Channel, s.IsEnabled, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: create subscription: %w", err)
		}
		return nil
	})
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	var s *domain.Subscription
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, user_id, channel, is_enabled, created_at, updated_at
			FROM notification_subscriptions
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanSubscription(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("notifications: get subscription: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *SubscriptionRepository) List(ctx context.Context, tenantID string, filter ports.SubscriptionFilter) ([]*domain.Subscription, string, error) {
	var out []*domain.Subscription
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID}
		where := "tenant_id = $1"
		if filter.Channel != "" {
			args = append(args, filter.Channel)
			where += fmt.Sprintf(" AND channel = $%d", len(args))
		}
		if filter.UserID != "" {
			args = append(args, filter.UserID)
			where += fmt.Sprintf(" AND user_id = $%d", len(args))
		}
		if filter.Cursor != "" {
			args = append(args, filter.Cursor)
			where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM notification_subscriptions WHERE id = $%d AND tenant_id = $1)", len(args))
		}
		args = append(args, filter.Limit)
		sql := fmt.Sprintf(`
			SELECT id, tenant_id, user_id, channel, is_enabled, created_at, updated_at
			FROM notification_subscriptions
			WHERE %s
			ORDER BY created_at ASC, id ASC
			LIMIT $%d
		`, where, len(args))

		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s, err := scanSubscription(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("notifications: list subscriptions rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func (r *SubscriptionRepository) Update(ctx context.Context, tenantID string, s *domain.Subscription) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE notification_subscriptions
			SET user_id = $3, channel = $4, is_enabled = $5, updated_at = $6
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.UserID, s.Channel, s.IsEnabled, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: update subscription: %w", err)
		}
		return nil
	})
}

func (r *SubscriptionRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM notification_subscriptions
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("notifications: delete subscription: %w", err)
		}
		return nil
	})
}

// --- scanners ---

type scanner interface {
	Scan(dest ...any) error
}

func scanMessage(row scanner) (*domain.Message, error) {
	var m domain.Message
	if err := row.Scan(
		&m.ID, &m.TenantID, &m.RecipientID, &m.Channel, &m.TemplateID, &m.Subject, &m.Body, &m.Status,
		&m.Metadata, &m.ScheduledAt, &m.SentAt, &m.Error, &m.Provider, &m.DeliveryStatus,
		&m.DeliveryStatusAt, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &m, nil
}

func scanTemplate(row scanner) (*domain.Template, error) {
	var t domain.Template
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.Name, &t.Channel, &t.SubjectTemplate, &t.BodyTemplate, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

func scanSubscription(row scanner) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.UserID, &s.Channel, &s.IsEnabled, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
