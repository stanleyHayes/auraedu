// Package postgres provides Postgres-backed repositories for the notification service.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
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
	_ ports.MessageRepository      = (*MessageRepository)(nil)
	_ ports.TemplateRepository     = (*TemplateRepository)(nil)
	_ ports.SubscriptionRepository = (*SubscriptionRepository)(nil)
)

// NewMessageRepository creates a Postgres-backed message repository.
func NewMessageRepository(database *db.DB) *MessageRepository {
	return &MessageRepository{db: database}
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
				metadata, scheduled_at, sent_at, error, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, '{}'::jsonb), $10, $11, $12, $13, $14)
		`, m.ID, tenantID, m.RecipientID, m.Channel, m.TemplateID, m.Subject, m.Body, m.Status, metadata, m.ScheduledAt, m.SentAt, m.Error, m.CreatedAt, m.UpdatedAt)
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
			SELECT id, tenant_id, recipient_id, channel, template_id, subject, body, status, metadata, scheduled_at, sent_at, error, created_at, updated_at
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
		SELECT id, tenant_id, recipient_id, channel, template_id, subject, body, status, metadata, scheduled_at, sent_at, error, created_at, updated_at
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
			    metadata = COALESCE($9, '{}'::jsonb), scheduled_at = $10, sent_at = $11, error = $12, updated_at = $13
			WHERE id = $1 AND tenant_id = $2
		`, m.ID, tenantID, m.RecipientID, m.Channel, m.TemplateID, m.Subject, m.Body, m.Status, metadata, m.ScheduledAt, m.SentAt, m.Error, m.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: update message: %w", err)
		}
		return nil
	})
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
		&m.Metadata, &m.ScheduledAt, &m.SentAt, &m.Error, &m.CreatedAt, &m.UpdatedAt,
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
