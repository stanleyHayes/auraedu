// Package postgres is the website-service Postgres repository adapter.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/website-service/internal/domain"
	"github.com/auraedu/website-service/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository is the Postgres implementation of ports.Repository.
// It uses platform/db.WithTx so that app.tenant_id is set on the same connection
// that executes the query, which makes the Row-Level Security policy effective.
// Every query also filters by tenant_id explicitly as defense-in-depth.
type Repository struct {
	db *db.DB
}

var (
	_ ports.Repository          = (*Repository)(nil)
	_ ports.LifecycleRepository = (*Repository)(nil)
	_ ports.OutboxRepository    = (*Repository)(nil)
)

// NewRepository creates a Postgres-backed website repository.
func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

// Pages.

func (r *Repository) CreatePage(ctx context.Context, tenantID string, p *domain.Page) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO website_pages (id, tenant_id, slug, title, status, meta_description, layout, created_at, updated_at, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, p.ID, tenantID, p.Slug, p.Title, p.Status, p.MetaDescription, p.Layout, p.CreatedAt, p.UpdatedAt, p.PublishedAt)
		if err != nil {
			return fmt.Errorf("website: create page: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetPageByID(ctx context.Context, tenantID, id string) (*domain.Page, error) {
	var p *domain.Page
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, slug, title, status, meta_description, layout, created_at, updated_at, published_at
			FROM website_pages
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanPage(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("website: get page: %w", err)
		}
		p = got
		return nil
	})
	return p, err
}

func (r *Repository) GetPageBySlug(ctx context.Context, tenantID, slug string) (*domain.Page, error) {
	var p *domain.Page
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, slug, title, status, meta_description, layout, created_at, updated_at, published_at
			FROM website_pages
			WHERE slug = $1 AND tenant_id = $2
		`, domain.NormalizeSlug(slug), tenantID)
		got, err := scanPage(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("website: get page by slug: %w", err)
		}
		p = got
		return nil
	})
	return p, err
}

func (r *Repository) ListPages(ctx context.Context, tenantID string, limit int, cursor string, filter ports.PageFilter) ([]*domain.Page, string, error) {
	var out []*domain.Page
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listPagesQuery(ctx, tx, tenantID, limit, cursor, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			p, err := scanPage(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("website: list pages rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listPagesQuery(ctx context.Context, tx pgx.Tx, tenantID string, limit int, cursor string, filter ports.PageFilter) (pgx.Rows, error) {
	args := []any{tenantID}
	where := "tenant_id = $1"

	if filter.Status != nil && *filter.Status != "" {
		args = append(args, *filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Layout != nil && *filter.Layout != "" {
		args = append(args, *filter.Layout)
		where += fmt.Sprintf(" AND layout = $%d", len(args))
	}
	if cursor != "" {
		args = append(args, cursor)
		where += fmt.Sprintf(" AND (created_at, id) > (SELECT created_at, id FROM website_pages WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, slug, title, status, meta_description, layout, created_at, updated_at, published_at
		FROM website_pages
		WHERE %s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdatePage(ctx context.Context, tenantID string, p *domain.Page) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE website_pages
			SET slug = $3, title = $4, status = $5, meta_description = $6, layout = $7, updated_at = $8, published_at = $9
			WHERE id = $1 AND tenant_id = $2
		`, p.ID, tenantID, p.Slug, p.Title, p.Status, p.MetaDescription, p.Layout, p.UpdatedAt, p.PublishedAt)
		if err != nil {
			return fmt.Errorf("website: update page: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeletePage(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM website_pages WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("website: delete page: %w", err)
		}
		return nil
	})
}

// Sections.

func (r *Repository) CreateSection(ctx context.Context, tenantID string, s *domain.Section) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		content, err := json.Marshal(s.Content)
		if err != nil {
			return fmt.Errorf("website: marshal section content: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO website_sections (id, tenant_id, page_id, type, content, sort_order, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, COALESCE($5, '{}'::jsonb), $6, $7, $8, $9)
		`, s.ID, tenantID, s.PageID, s.Type, content, s.SortOrder, s.Status, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("website: create section: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetSectionByID(ctx context.Context, tenantID, id string) (*domain.Section, error) {
	var s *domain.Section
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT id, tenant_id, page_id, type, content, sort_order, status, created_at, updated_at
			FROM website_sections
			WHERE id = $1 AND tenant_id = $2
		`, id, tenantID)
		got, err := scanSection(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return fmt.Errorf("website: get section: %w", err)
		}
		s = got
		return nil
	})
	return s, err
}

func (r *Repository) ListSections(
	ctx context.Context,
	tenantID, pageID string,
	limit int,
	cursor string,
	filter ports.SectionFilter,
) ([]*domain.Section, string, error) {
	var out []*domain.Section
	var nextCursor string
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := listSectionsQuery(ctx, tx, tenantID, pageID, limit, cursor, filter)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			s, err := scanSection(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("website: list sections rows: %w", err)
		}
		if len(out) == limit && len(out) > 0 {
			nextCursor = out[len(out)-1].ID
		}
		return nil
	})
	return out, nextCursor, err
}

func listSectionsQuery(ctx context.Context, tx pgx.Tx, tenantID, pageID string, limit int, cursor string, filter ports.SectionFilter) (pgx.Rows, error) {
	args := []any{tenantID, pageID}
	where := "tenant_id = $1 AND page_id = $2"

	if filter.Status != nil && *filter.Status != "" {
		args = append(args, *filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Type != nil && *filter.Type != "" {
		args = append(args, *filter.Type)
		where += fmt.Sprintf(" AND type = $%d", len(args))
	}
	if cursor != "" {
		args = append(args, cursor)
		where += fmt.Sprintf(" AND (sort_order, id) > (SELECT sort_order, id FROM website_sections WHERE id = $%d AND tenant_id = $1)", len(args))
	}

	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT id, tenant_id, page_id, type, content, sort_order, status, created_at, updated_at
		FROM website_sections
		WHERE %s
		ORDER BY sort_order ASC, id ASC
		LIMIT $%d
	`, where, len(args))
	return tx.Query(ctx, sql, args...)
}

func (r *Repository) UpdateSection(ctx context.Context, tenantID string, s *domain.Section) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		content, err := json.Marshal(s.Content)
		if err != nil {
			return fmt.Errorf("website: marshal section content: %w", err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE website_sections
			SET type = $3, content = COALESCE($4, '{}'::jsonb), sort_order = $5, status = $6, updated_at = $7
			WHERE id = $1 AND tenant_id = $2
		`, s.ID, tenantID, s.Type, content, s.SortOrder, s.Status, s.UpdatedAt)
		if err != nil {
			return fmt.Errorf("website: update section: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteSection(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM website_sections WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		if err != nil {
			return fmt.Errorf("website: delete section: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteSectionsByPage(ctx context.Context, tenantID, pageID string) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM website_sections WHERE page_id = $1 AND tenant_id = $2`, pageID, tenantID)
		if err != nil {
			return fmt.Errorf("website: delete sections by page: %w", err)
		}
		return nil
	})
}

func insertPage(ctx context.Context, tx pgx.Tx, tenantID string, p *domain.Page) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO website_pages
			(id, tenant_id, slug, title, status, meta_description, layout, created_at, updated_at, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, p.ID, tenantID, p.Slug, p.Title, p.Status, p.MetaDescription, p.Layout,
		p.CreatedAt, p.UpdatedAt, p.PublishedAt)
	return err
}

func insertSection(ctx context.Context, tx pgx.Tx, tenantID string, s *domain.Section) error {
	content, err := json.Marshal(s.Content)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO website_sections
			(id, tenant_id, page_id, type, content, sort_order, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,COALESCE($5,'{}'::jsonb),$6,$7,$8,$9)
	`, s.ID, tenantID, s.PageID, s.Type, content, s.SortOrder, s.Status, s.CreatedAt, s.UpdatedAt)
	return err
}

func enqueueWebsiteEvents(ctx context.Context, tx pgx.Tx, tenantID string, events []ports.LifecycleEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO website_outbox (id, tenant_id, event_type, payload)
			VALUES ($1,$2,$3,$4)
		`, uuid.NewString(), tenantID, event.EventType, payload); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CommitWebsiteLifecycle(
	ctx context.Context,
	tenantID string,
	mutation string,
	page *domain.Page,
	section *domain.Section,
	events []ports.LifecycleEvent,
) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		switch mutation {
		case ports.WebsiteMutationPageCreate:
			err = insertPage(ctx, tx, tenantID, page)
		case ports.WebsiteMutationPageUpdate:
			_, err = tx.Exec(ctx, `
				UPDATE website_pages
				SET slug=$3, title=$4, status=$5, meta_description=$6, layout=$7,
				    updated_at=$8, published_at=$9
				WHERE id=$1 AND tenant_id=$2
			`, page.ID, tenantID, page.Slug, page.Title, page.Status,
				page.MetaDescription, page.Layout, page.UpdatedAt, page.PublishedAt)
		case ports.WebsiteMutationPageDelete:
			_, err = tx.Exec(ctx, `DELETE FROM website_pages WHERE id=$1 AND tenant_id=$2`, page.ID, tenantID)
		case ports.WebsiteMutationSectionCreate:
			err = insertSection(ctx, tx, tenantID, section)
		case ports.WebsiteMutationSectionUpdate:
			var content []byte
			content, err = json.Marshal(section.Content)
			if err == nil {
				_, err = tx.Exec(ctx, `
					UPDATE website_sections
					SET type=$3, content=COALESCE($4,'{}'::jsonb), sort_order=$5,
					    status=$6, updated_at=$7
					WHERE id=$1 AND tenant_id=$2
				`, section.ID, tenantID, section.Type, content,
					section.SortOrder, section.Status, section.UpdatedAt)
			}
		case ports.WebsiteMutationSectionDelete:
			_, err = tx.Exec(ctx, `DELETE FROM website_sections WHERE id=$1 AND tenant_id=$2`, section.ID, tenantID)
		default:
			return fmt.Errorf("website: unsupported lifecycle mutation %q", mutation)
		}
		if err != nil {
			return fmt.Errorf("website: lifecycle mutation: %w", err)
		}
		if err := enqueueWebsiteEvents(ctx, tx, tenantID, events); err != nil {
			return fmt.Errorf("website: enqueue lifecycle event: %w", err)
		}
		return nil
	})
}

func (r *Repository) ProvisionDefaultWebsite(
	ctx context.Context,
	tenantID string,
	page *domain.Page,
	section *domain.Section,
	events []ports.LifecycleEvent,
) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := insertPage(ctx, tx, tenantID, page); err != nil {
			return err
		}
		if err := insertSection(ctx, tx, tenantID, section); err != nil {
			return err
		}
		return enqueueWebsiteEvents(ctx, tx, tenantID, events)
	})
}

func websiteOutboxContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__website_outbox__"})
}

func (r *Repository) ClaimPendingWebsiteEvents(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := make([]ports.OutboxEvent, 0, limit)
	err := r.db.WithTx(websiteOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE website_outbox
			SET attempts=attempts+1,
			    next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second')
			WHERE id IN (
				SELECT id FROM website_outbox
				WHERE published_at IS NULL AND next_attempt_at<=now()
				ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
			)
			RETURNING id::text, tenant_id, event_type, payload
		`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}
func (r *Repository) MarkWebsiteEventPublished(ctx context.Context, id string) error {
	return r.db.WithTx(websiteOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE website_outbox SET published_at=now(),last_error=NULL WHERE id=$1`, id)
		return err
	})
}
func (r *Repository) MarkWebsiteEventFailed(ctx context.Context, id, message string) error {
	return r.db.WithTx(websiteOutboxContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE website_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}

// Scanners.

type scanner interface {
	Scan(dest ...any) error
}

func scanPage(row scanner) (*domain.Page, error) {
	var p domain.Page
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.Slug, &p.Title, &p.Status, &p.MetaDescription, &p.Layout, &p.CreatedAt, &p.UpdatedAt, &p.PublishedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func scanSection(row scanner) (*domain.Section, error) {
	var s domain.Section
	if err := row.Scan(
		&s.ID, &s.TenantID, &s.PageID, &s.Type, &s.Content, &s.SortOrder, &s.Status, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
