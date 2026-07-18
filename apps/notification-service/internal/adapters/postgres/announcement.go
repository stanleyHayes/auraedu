package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/notification-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

// AnnouncementRepository is the Postgres implementation of ports.AnnouncementRepository.
type AnnouncementRepository struct {
	db *db.DB
}

// ProcessedEventRepository is the Postgres implementation of ports.ProcessedEventRepository.
type ProcessedEventRepository struct {
	db *db.DB
}

var (
	_ ports.AnnouncementRepository   = (*AnnouncementRepository)(nil)
	_ ports.ProcessedEventRepository = (*ProcessedEventRepository)(nil)
)

// NewAnnouncementRepository creates a Postgres-backed announcement repository.
func NewAnnouncementRepository(database *db.DB) *AnnouncementRepository {
	return &AnnouncementRepository{db: database}
}

// NewProcessedEventRepository creates a Postgres-backed processed-event ledger.
func NewProcessedEventRepository(database *db.DB) *ProcessedEventRepository {
	return &ProcessedEventRepository{db: database}
}

// --- Announcement persistence ---

func (r *AnnouncementRepository) Create(ctx context.Context, tenantID string, a *domain.Announcement) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO announcements (id, tenant_id, title, body, audience, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, a.ID, tenantID, a.Title, a.Body, a.Audience, a.CreatedAt, a.UpdatedAt)
		if err != nil {
			return fmt.Errorf("notifications: create announcement: %w", err)
		}
		return nil
	})
}

func (r *AnnouncementRepository) GetByID(ctx context.Context, tenantID, id string) (*domain.Announcement, error) {
	var a *domain.Announcement
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, title, body, audience, created_at, updated_at
			FROM announcements
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanAnnouncement(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("notifications: get announcement: %w", err)
		}
		a = got
		return nil
	})
	return a, err
}

func (r *AnnouncementRepository) List(ctx context.Context, tenantID string, filter ports.AnnouncementFilter) ([]*domain.Announcement, string, error) {
	var out []*domain.Announcement
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		args := []any{tenantID}
		where := "tenant_id = $1"
		if filter.Audience != "" {
			args = append(args, filter.Audience)
			where += fmt.Sprintf(" AND audience = $%d", len(args))
		}
		if filter.Cursor != "" {
			args = append(args, filter.Cursor)
			where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM announcements WHERE id = $%d AND tenant_id = $1)", len(args))
		}
		args = append(args, filter.Limit)
		sql := fmt.Sprintf(`
			SELECT id, tenant_id, title, body, audience, created_at, updated_at
			FROM announcements
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
			a, err := scanAnnouncement(rows)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("notifications: list announcements rows: %w", err)
		}
		if len(out) == filter.Limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func (r *AnnouncementRepository) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM announcements
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		if err != nil {
			return fmt.Errorf("notifications: delete announcement: %w", err)
		}
		return nil
	})
}

// --- Processed-event ledger ---

func (r *ProcessedEventRepository) Claim(ctx context.Context, tenantID, eventID, eventType string) (bool, error) {
	claimed := false
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO notification_processed_events (tenant_id, event_id, event_type)
			VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, event_id) DO NOTHING
		`, tenantID, eventID, eventType)
		if err != nil {
			return fmt.Errorf("notifications: claim processed event: %w", err)
		}
		claimed = tag.RowsAffected() == 1
		return nil
	})
	return claimed, err
}

func (r *ProcessedEventRepository) Release(ctx context.Context, tenantID, eventID string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			DELETE FROM notification_processed_events
			WHERE tenant_id = $1 AND event_id = $2
		`, tenantID, eventID)
		if err != nil {
			return fmt.Errorf("notifications: release processed event: %w", err)
		}
		return nil
	})
}

// --- scanners ---

func scanAnnouncement(row scanner) (*domain.Announcement, error) {
	var a domain.Announcement
	if err := row.Scan(
		&a.ID, &a.TenantID, &a.Title, &a.Body, &a.Audience, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &a, nil
}
