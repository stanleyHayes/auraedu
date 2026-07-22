// Package postgres persists tenant-scoped knowledge and its transactional outbox.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/knowledge-service/internal/domain"
	"github.com/auraedu/knowledge-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)

func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func (r *Repository) Create(ctx context.Context, source domain.Source) error {
	return r.db.WithTx(withTenant(ctx, source.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO knowledge_sources
			(id,tenant_id,source_type,title,owner,content,status,confidentiality,locale,version,effective_at,expires_at,programme,campus,intake,created_at,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			source.ID, source.TenantID, source.SourceType, source.Title, source.Owner, source.Content, source.Status,
			source.Confidentiality, source.Locale, source.Version, source.EffectiveAt, source.ExpiresAt, source.Programme,
			source.Campus, source.Intake, source.CreatedAt, source.UpdatedAt)
		if err != nil {
			return fmt.Errorf("knowledge: create source: %w", err)
		}
		return nil
	})
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Source, error) {
	var source domain.Source
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		source, err = scanSource(tx.QueryRow(ctx, sourceSelect+` WHERE tenant_id=$1 AND id=$2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	})
	return source, err
}

func (r *Repository) List(ctx context.Context, tenantID string, status domain.Status, limit int) ([]domain.Source, error) {
	items := make([]domain.Source, 0)
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sourceSelect+` WHERE tenant_id=$1 AND ($2='' OR status=$2) ORDER BY created_at DESC,id DESC LIMIT $3`, tenantID, status, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			source, err := scanSource(rows)
			if err != nil {
				return err
			}
			items = append(items, source)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) Approve(ctx context.Context, tenantID, id, reviewer, note string, now time.Time) (domain.Source, error) {
	var source domain.Source
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `UPDATE knowledge_sources SET status='approved',approved_by=$3,approved_at=$4,review_note=$5,updated_at=$4
			WHERE tenant_id=$1 AND id=$2 AND status='draft' RETURNING `+sourceColumns, tenantID, id, reviewer, now, note)
		var err error
		source, err = scanSource(row)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_sources WHERE tenant_id=$1 AND id=$2)`, tenantID, id).Scan(&exists); checkErr != nil {
				return checkErr
			}
			if exists {
				return domain.ErrConflict
			}
			return domain.ErrNotFound
		}
		return err
	})
	return source, err
}

func (r *Repository) Retire(ctx context.Context, tenantID, id string, now time.Time) (domain.Source, error) {
	var source domain.Source
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		source, err = scanSource(tx.QueryRow(ctx, `UPDATE knowledge_sources SET status='retired',updated_at=$3
			WHERE tenant_id=$1 AND id=$2 AND status='approved' RETURNING `+sourceColumns, tenantID, id, now))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrConflict
		}
		return err
	})
	return source, err
}

func (r *Repository) Search(ctx context.Context, tenantID, query, locale string, limit int, asOf time.Time) ([]domain.SearchResult, error) {
	results := make([]domain.SearchResult, 0)
	err := r.db.WithTx(withTenant(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `WITH q AS (
			SELECT to_tsquery('simple',array_to_string(tsvector_to_array(to_tsvector('simple',$2)),' | ')) AS query
		)
			SELECT id,title,left(content,800),source_type,locale,version,
			least(1.0,ts_rank_cd(search_document,q.query)::float8),effective_at,expires_at
			FROM knowledge_sources CROSS JOIN q WHERE tenant_id=$1 AND status='approved' AND confidentiality='public'
			AND split_part(locale,'-',1)=split_part($3,'-',1)
			AND effective_at <= $4 AND (expires_at IS NULL OR expires_at > $4)
			AND search_document @@ q.query
			ORDER BY (locale=$3) DESC,ts_rank_cd(search_document,q.query) DESC,effective_at DESC,id LIMIT $5`, tenantID, query, locale, asOf, limit)
		if err != nil {
			return fmt.Errorf("knowledge: search: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var result domain.SearchResult
			if err := rows.Scan(
				&result.SourceID, &result.Title, &result.Passage, &result.SourceType, &result.Locale,
				&result.Version, &result.Score, &result.EffectiveAt, &result.ExpiresAt,
			); err != nil {
				return err
			}
			results = append(results, result)
		}
		return rows.Err()
	})
	return results, err
}

const sourceColumns = `id,tenant_id,source_type,title,owner,content,status,confidentiality,locale,version,` +
	`effective_at,expires_at,programme,campus,intake,approved_by,approved_at,review_note,created_at,updated_at`
const sourceSelect = `SELECT ` + sourceColumns + ` FROM knowledge_sources`

type scanner interface{ Scan(...any) error }

func scanSource(row scanner) (domain.Source, error) {
	var source domain.Source
	err := row.Scan(&source.ID, &source.TenantID, &source.SourceType, &source.Title, &source.Owner, &source.Content,
		&source.Status, &source.Confidentiality, &source.Locale, &source.Version, &source.EffectiveAt, &source.ExpiresAt,
		&source.Programme, &source.Campus, &source.Intake, &source.ApprovedBy, &source.ApprovedAt,
		&source.ReviewNote, &source.CreatedAt, &source.UpdatedAt)
	return source, err
}
