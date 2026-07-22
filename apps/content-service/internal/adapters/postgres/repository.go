// Package postgres persists governed content and its transactional outbox.
//
//nolint:lll // SQL statements remain single literals so placeholders stay reviewable against their argument lists.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)
var _ ports.OutboxRepository = (*Repository)(nil)

func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

func tenantContext(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func platformContext(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__content_outbox__"})
}

func (r *Repository) GetBrandProfile(ctx context.Context, tenantID string) (domain.BrandProfile, error) {
	var profile domain.BrandProfile
	err := r.db.WithTx(tenantContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT tenant_id,tone_of_voice,approved_terms,prohibited_claims,required_disclaimers,locale,version,updated_by,updated_at FROM content_brand_profiles WHERE tenant_id=$1`, tenantID).Scan(&profile.TenantID, &profile.ToneOfVoice, &profile.ApprovedTerms, &profile.ProhibitedClaims, &profile.RequiredDisclaimers, &profile.Locale, &profile.Version, &profile.UpdatedBy, &profile.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	})
	return profile, err
}

func (r *Repository) UpsertBrandProfile(ctx context.Context, profile domain.BrandProfile, expected int) error {
	return r.db.WithTx(tenantContext(ctx, profile.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		var current int
		err := tx.QueryRow(ctx, `SELECT version FROM content_brand_profiles WHERE tenant_id=$1 FOR UPDATE`, profile.TenantID).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			if expected != 0 {
				return domain.ErrConflict
			}
			_, err = tx.Exec(ctx, `INSERT INTO content_brand_profiles(tenant_id,tone_of_voice,approved_terms,prohibited_claims,required_disclaimers,locale,version,updated_by,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, profile.TenantID, profile.ToneOfVoice, profile.ApprovedTerms, profile.ProhibitedClaims, profile.RequiredDisclaimers, profile.Locale, profile.Version, profile.UpdatedBy, profile.UpdatedAt)
			return err
		}
		if err != nil {
			return err
		}
		if current != expected {
			return domain.ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE content_brand_profiles SET tone_of_voice=$2,approved_terms=$3,prohibited_claims=$4,required_disclaimers=$5,locale=$6,version=$7,updated_by=$8,updated_at=$9 WHERE tenant_id=$1 AND version=$10`, profile.TenantID, profile.ToneOfVoice, profile.ApprovedTerms, profile.ProhibitedClaims, profile.RequiredDisclaimers, profile.Locale, profile.Version, profile.UpdatedBy, profile.UpdatedAt, expected)
		return err
	})
}

const draftColumns = `id,tenant_id,campaign_id,content_type,title,brief,audience,locale,key_messages,facts,content,status,version,compliance_status,compliance_findings,generator,brand_profile_version,created_by,submitted_by,submitted_at,reviewed_by,reviewed_at,review_note,expires_at,created_at,updated_at`

type scanner interface{ Scan(...any) error }

func scanDraft(row scanner) (domain.Draft, error) {
	var draft domain.Draft
	var facts, findings []byte
	err := row.Scan(&draft.ID, &draft.TenantID, &draft.CampaignID, &draft.ContentType, &draft.Title, &draft.Brief, &draft.Audience, &draft.Locale, &draft.KeyMessages, &facts, &draft.Content, &draft.Status, &draft.Version, &draft.ComplianceStatus, &findings, &draft.Generator, &draft.BrandProfileVersion, &draft.CreatedBy, &draft.SubmittedBy, &draft.SubmittedAt, &draft.ReviewedBy, &draft.ReviewedAt, &draft.ReviewNote, &draft.ExpiresAt, &draft.CreatedAt, &draft.UpdatedAt)
	if err != nil {
		return domain.Draft{}, err
	}
	if err := json.Unmarshal(facts, &draft.Facts); err != nil {
		return domain.Draft{}, err
	}
	if err := json.Unmarshal(findings, &draft.ComplianceFindings); err != nil {
		return domain.Draft{}, err
	}
	return draft, nil
}

func encodeJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("content JSON: %w", err)
	}
	return encoded, nil
}

func insertVersion(ctx context.Context, tx pgx.Tx, tenantID string, version domain.Version) error {
	findings, err := encodeJSON(version.ComplianceFindings)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO content_versions(tenant_id,content_id,version,content,status,compliance_status,compliance_findings,generator,brand_profile_version,created_by,change_note,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, tenantID, version.ContentID, version.Version, version.Content, version.Status, version.ComplianceStatus, findings, version.Generator, version.BrandProfileVersion, version.CreatedBy, version.ChangeNote, version.CreatedAt)
	return err
}

func (r *Repository) FindReplay(ctx context.Context, tenantID, keyHash, _ string) (domain.Draft, string, bool, error) {
	var draft domain.Draft
	var requestHash, contentID string
	err := r.db.WithTx(tenantContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT request_hash,content_id FROM content_idempotency WHERE tenant_id=$1 AND key_hash=$2`, tenantID, keyHash).Scan(&requestHash, &contentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		draft, err = scanDraft(tx.QueryRow(ctx, `SELECT `+draftColumns+` FROM content_drafts WHERE tenant_id=$1 AND id=$2`, tenantID, contentID))
		return err
	})
	if err != nil {
		return domain.Draft{}, "", false, err
	}
	return draft, requestHash, draft.ID != "", nil
}

func (r *Repository) CreateDraftWithEvent(ctx context.Context, draft domain.Draft, version domain.Version, keyHash, requestHash string, payload map[string]any) error {
	facts, err := encodeJSON(draft.Facts)
	if err != nil {
		return err
	}
	findings, err := encodeJSON(draft.ComplianceFindings)
	if err != nil {
		return err
	}
	eventPayload, err := encodeJSON(payload)
	if err != nil {
		return err
	}
	return r.db.WithTx(tenantContext(ctx, draft.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO content_drafts(`+draftColumns+`) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)`, draft.ID, draft.TenantID, draft.CampaignID, draft.ContentType, draft.Title, draft.Brief, draft.Audience, draft.Locale, draft.KeyMessages, facts, draft.Content, draft.Status, draft.Version, draft.ComplianceStatus, findings, draft.Generator, draft.BrandProfileVersion, draft.CreatedBy, draft.SubmittedBy, draft.SubmittedAt, draft.ReviewedBy, draft.ReviewedAt, draft.ReviewNote, draft.ExpiresAt, draft.CreatedAt, draft.UpdatedAt)
		if err != nil {
			return err
		}
		if err := insertVersion(ctx, tx, draft.TenantID, version); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO content_idempotency(tenant_id,key_hash,request_hash,content_id,created_at) VALUES($1,$2,$3,$4,$5)`, draft.TenantID, keyHash, requestHash, draft.ID, draft.CreatedAt); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO content_outbox(id,tenant_id,event_type,payload,created_at,next_attempt_at) VALUES($1,$2,'content.draft_generated.v1',$3,$4,$4)`, uuid.NewString(), draft.TenantID, eventPayload, draft.CreatedAt)
		return err
	})
}

func (r *Repository) GetDraft(ctx context.Context, tenantID, id string) (domain.Draft, []domain.Version, error) {
	var draft domain.Draft
	versions := []domain.Version{}
	err := r.db.WithTx(tenantContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		draft, err = scanDraft(tx.QueryRow(ctx, `SELECT `+draftColumns+` FROM content_drafts WHERE tenant_id=$1 AND id=$2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT content_id,version,content,status,compliance_status,compliance_findings,generator,brand_profile_version,created_by,change_note,created_at FROM content_versions WHERE tenant_id=$1 AND content_id=$2 ORDER BY version`, tenantID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var version domain.Version
			var findings []byte
			if err := rows.Scan(&version.ContentID, &version.Version, &version.Content, &version.Status, &version.ComplianceStatus, &findings, &version.Generator, &version.BrandProfileVersion, &version.CreatedBy, &version.ChangeNote, &version.CreatedAt); err != nil {
				return err
			}
			if err := json.Unmarshal(findings, &version.ComplianceFindings); err != nil {
				return err
			}
			versions = append(versions, version)
		}
		return rows.Err()
	})
	return draft, versions, err
}

func (r *Repository) ListDrafts(ctx context.Context, tenantID string, filter ports.ListFilter) ([]domain.Draft, error) {
	items := []domain.Draft{}
	err := r.db.WithTx(tenantContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+draftColumns+` FROM content_drafts WHERE tenant_id=$1 AND ($2='' OR status=$2) AND ($3='' OR content_type=$3) AND ($4='' OR campaign_id::text=$4) ORDER BY updated_at DESC,id DESC LIMIT $5`, tenantID, filter.Status, filter.ContentType, filter.CampaignID, filter.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			draft, err := scanDraft(rows)
			if err != nil {
				return err
			}
			items = append(items, draft)
		}
		return rows.Err()
	})
	return items, err
}

func updateDraft(ctx context.Context, tx pgx.Tx, draft domain.Draft, expectedVersion int, expectedUpdatedAt time.Time) error {
	findings, err := encodeJSON(draft.ComplianceFindings)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE content_drafts SET content=$3,status=$4,version=$5,compliance_status=$6,compliance_findings=$7,generator=$8,brand_profile_version=$9,created_by=$10,submitted_by=$11,submitted_at=$12,reviewed_by=$13,reviewed_at=$14,review_note=$15,expires_at=$16,updated_at=$17 WHERE tenant_id=$1 AND id=$2 AND version=$18 AND updated_at=$19`, draft.TenantID, draft.ID, draft.Content, draft.Status, draft.Version, draft.ComplianceStatus, findings, draft.Generator, draft.BrandProfileVersion, draft.CreatedBy, draft.SubmittedBy, draft.SubmittedAt, draft.ReviewedBy, draft.ReviewedAt, draft.ReviewNote, draft.ExpiresAt, draft.UpdatedAt, expectedVersion, expectedUpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *Repository) UpdateDraftWithVersionAndEvent(ctx context.Context, draft domain.Draft, expectedVersion int, expectedUpdatedAt time.Time, version *domain.Version, eventType string, payload map[string]any) error {
	return r.db.WithTx(tenantContext(ctx, draft.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		if err := updateDraft(ctx, tx, draft, expectedVersion, expectedUpdatedAt); err != nil {
			return err
		}
		if version != nil {
			if err := insertVersion(ctx, tx, draft.TenantID, *version); err != nil {
				return err
			}
		}
		if eventType == "" {
			return nil
		}
		encoded, err := encodeJSON(payload)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO content_outbox(id,tenant_id,event_type,payload,created_at,next_attempt_at) VALUES($1,$2,$3,$4,$5,$5)`, uuid.NewString(), draft.TenantID, eventType, encoded, draft.UpdatedAt)
		return err
	})
}

func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	items := []ports.OutboxEvent{}
	err := r.db.WithTx(platformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE content_outbox SET attempts=attempts+1,next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second') WHERE id IN(SELECT id FROM content_outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id,tenant_id,event_type,payload,created_at`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if err := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CreatedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.mark(ctx, id, "", true)
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.mark(ctx, id, message, false)
}

func (r *Repository) mark(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(platformContext(ctx), func(ctx context.Context, tx pgx.Tx) error {
		if published {
			_, err := tx.Exec(ctx, `UPDATE content_outbox SET published_at=$2,last_error=NULL WHERE id=$1`, id, time.Now().UTC())
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE content_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return err
	})
}
