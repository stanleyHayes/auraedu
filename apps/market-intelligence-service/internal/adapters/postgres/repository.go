// Package postgres persists market intelligence and its transactional outbox.
//
//nolint:lll // SQL literals remain adjacent to ordered arguments for transactional review.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/auraedu/market-intelligence-service/internal/domain"
	"github.com/auraedu/market-intelligence-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)

func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }
func tenantCtx(ctx context.Context, id string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: id})
}

type scanner interface{ Scan(...any) error }

const sourceColumns = `id,tenant_id,kind,name,canonical_url,collection_method,terms_reference,compliance_status,created_by,reviewed_by,reviewed_at,review_note,created_at,updated_at`

func scanSource(row scanner) (domain.Source, error) {
	var s domain.Source
	err := row.Scan(&s.ID, &s.TenantID, &s.Kind, &s.Name, &s.CanonicalURL, &s.CollectionMethod, &s.TermsReference, &s.ComplianceStatus, &s.CreatedBy, &s.ReviewedBy, &s.ReviewedAt, &s.ReviewNote, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

const observationColumns = `id,tenant_id,source_id,kind,category,title,evidence_excerpt,evidence_sha256,sentiment,programme_id,campus_id,response_draft,status,created_by,observed_at,reviewed_by,reviewed_at,review_note,resolution_note,resolved_by,resolved_at,created_at,updated_at`

func scanObservation(row scanner) (domain.Observation, error) {
	var o domain.Observation
	err := row.Scan(&o.ID, &o.TenantID, &o.SourceID, &o.Kind, &o.Category, &o.Title, &o.EvidenceExcerpt, &o.EvidenceSHA256, &o.Sentiment, &o.ProgrammeID, &o.CampusID, &o.ResponseDraft, &o.Status, &o.CreatedBy, &o.ObservedAt, &o.ReviewedBy, &o.ReviewedAt, &o.ReviewNote, &o.ResolutionNote, &o.ResolvedBy, &o.ResolvedAt, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}
func enqueue(ctx context.Context, tx pgx.Tx, tenant, eventType string, payload map[string]any, at any) error {
	b, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO intelligence_outbox(id,tenant_id,event_type,payload,created_at,next_attempt_at) VALUES($1,$2,$3,$4,$5,$5)`, uuid.NewString(), tenant, eventType, b, at)
	return e
}
func conflict(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) && p.Code == "23505" {
		return domain.ErrConflict
	}
	return e
}
func (r *Repository) CreateSource(ctx context.Context, s domain.Source, event string, payload map[string]any) error {
	e := r.db.WithTx(tenantCtx(ctx, s.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO intelligence_sources(`+sourceColumns+`) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, s.ID, s.TenantID, s.Kind, s.Name, s.CanonicalURL, s.CollectionMethod, s.TermsReference, s.ComplianceStatus, s.CreatedBy, s.ReviewedBy, s.ReviewedAt, s.ReviewNote, s.CreatedAt, s.UpdatedAt)
		if e != nil {
			return e
		}
		return enqueue(ctx, tx, s.TenantID, event, payload, s.CreatedAt)
	})
	return conflict(e)
}
func (r *Repository) GetSource(ctx context.Context, t, id string) (domain.Source, error) {
	var s domain.Source
	e := r.db.WithTx(tenantCtx(ctx, t), func(ctx context.Context, tx pgx.Tx) error {
		var e error
		s, e = scanSource(tx.QueryRow(ctx, `SELECT `+sourceColumns+` FROM intelligence_sources WHERE tenant_id=$1 AND id=$2`, t, id))
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return e
	})
	return s, e
}
func (r *Repository) ListSources(ctx context.Context, t string, k domain.Kind, limit int) ([]domain.Source, error) {
	out := []domain.Source{}
	e := r.db.WithTx(tenantCtx(ctx, t), func(ctx context.Context, tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT `+sourceColumns+` FROM intelligence_sources WHERE tenant_id=$1 AND ($2='' OR kind=$2) ORDER BY created_at DESC,id DESC LIMIT $3`, t, k, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			s, e := scanSource(rows)
			if e != nil {
				return e
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, e
}
func (r *Repository) UpdateSource(ctx context.Context, s domain.Source, expected domain.Status, event string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, s.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE intelligence_sources SET compliance_status=$3,reviewed_by=$4,reviewed_at=$5,review_note=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2 AND compliance_status=$8`, s.TenantID, s.ID, s.ComplianceStatus, s.ReviewedBy, s.ReviewedAt, s.ReviewNote, s.UpdatedAt, expected)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return enqueue(ctx, tx, s.TenantID, event, payload, s.UpdatedAt)
	})
}
func (r *Repository) CreateObservation(ctx context.Context, o domain.Observation, event string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, o.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO intelligence_observations(`+observationColumns+`) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`, o.ID, o.TenantID, o.SourceID, o.Kind, o.Category, o.Title, o.EvidenceExcerpt, o.EvidenceSHA256, o.Sentiment, o.ProgrammeID, o.CampusID, o.ResponseDraft, o.Status, o.CreatedBy, o.ObservedAt, o.ReviewedBy, o.ReviewedAt, o.ReviewNote, o.ResolutionNote, o.ResolvedBy, o.ResolvedAt, o.CreatedAt, o.UpdatedAt)
		if e != nil {
			return e
		}
		return enqueue(ctx, tx, o.TenantID, event, payload, o.CreatedAt)
	})
}
func (r *Repository) GetObservation(ctx context.Context, t, id string) (domain.Observation, error) {
	var o domain.Observation
	e := r.db.WithTx(tenantCtx(ctx, t), func(ctx context.Context, tx pgx.Tx) error {
		var e error
		o, e = scanObservation(tx.QueryRow(ctx, `SELECT `+observationColumns+` FROM intelligence_observations WHERE tenant_id=$1 AND id=$2`, t, id))
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return e
	})
	return o, e
}
func (r *Repository) ListObservations(ctx context.Context, t string, k domain.Kind, status domain.Status, limit int) ([]domain.Observation, error) {
	out := []domain.Observation{}
	e := r.db.WithTx(tenantCtx(ctx, t), func(ctx context.Context, tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT `+observationColumns+` FROM intelligence_observations WHERE tenant_id=$1 AND ($2='' OR kind=$2) AND ($3='' OR status=$3) ORDER BY observed_at DESC,id DESC LIMIT $4`, t, k, status, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			o, e := scanObservation(rows)
			if e != nil {
				return e
			}
			out = append(out, o)
		}
		return rows.Err()
	})
	return out, e
}
func (r *Repository) UpdateObservation(ctx context.Context, o domain.Observation, expected domain.Status, event string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, o.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE intelligence_observations SET status=$3,reviewed_by=$4,reviewed_at=$5,review_note=$6,resolution_note=$7,resolved_by=$8,resolved_at=$9,updated_at=$10 WHERE tenant_id=$1 AND id=$2 AND status=$11`, o.TenantID, o.ID, o.Status, o.ReviewedBy, o.ReviewedAt, o.ReviewNote, o.ResolutionNote, o.ResolvedBy, o.ResolvedAt, o.UpdatedAt, expected)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		if err := enqueue(ctx, tx, o.TenantID, event, payload, o.UpdatedAt); err != nil {
			return err
		}
		if o.Kind == domain.KindReputation && o.Status == domain.StatusApproved && (o.Category == "recurring_issue" || o.Category == "misinformation") {
			return evaluateAlert(ctx, tx, o)
		}
		return nil
	})
}

func evaluateAlert(ctx context.Context, tx pgx.Tx, o domain.Observation) error {
	threshold, window := domain.DefaultAlertThreshold, domain.DefaultAlertWindowDays
	if e := tx.QueryRow(ctx, `SELECT threshold,window_days FROM intelligence_alert_rules WHERE tenant_id=$1`, o.TenantID).Scan(&threshold, &window); e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	var count int
	var first, last time.Time
	e := tx.QueryRow(ctx, `SELECT count(*),min(observed_at),max(observed_at) FROM intelligence_observations WHERE tenant_id=$1 AND kind='reputation' AND category=$2 AND status IN('approved','resolved') AND programme_id IS NOT DISTINCT FROM $3 AND campus_id IS NOT DISTINCT FROM $4 AND observed_at >= $5`, o.TenantID, o.Category, o.ProgrammeID, o.CampusID, o.ObservedAt.AddDate(0, 0, -window)).Scan(&count, &first, &last)
	if e != nil {
		return e
	}
	if count < threshold {
		return nil
	}
	fingerprint := domain.AlertFingerprint(o.Category, o.ProgrammeID, o.CampusID)
	reason := domain.AlertReason(o.Category, count, threshold, window)
	actor := o.CreatedBy
	if o.ReviewedBy != nil {
		actor = *o.ReviewedBy
	}
	id := uuid.NewString()
	var alertID string
	e = tx.QueryRow(ctx, `INSERT INTO intelligence_alerts(id,tenant_id,fingerprint,category,programme_id,campus_id,observation_count,threshold,window_days,first_observed_at,last_observed_at,reason,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'open',$13,$13) ON CONFLICT(tenant_id,fingerprint) WHERE status='open' DO UPDATE SET observation_count=EXCLUDED.observation_count,threshold=EXCLUDED.threshold,window_days=EXCLUDED.window_days,first_observed_at=EXCLUDED.first_observed_at,last_observed_at=EXCLUDED.last_observed_at,reason=EXCLUDED.reason,updated_at=EXCLUDED.updated_at RETURNING id`, id, o.TenantID, fingerprint, o.Category, o.ProgrammeID, o.CampusID, count, threshold, window, first, last, reason, o.UpdatedAt).Scan(&alertID)
	if e != nil {
		return e
	}
	return enqueue(ctx, tx, o.TenantID, "intelligence.alert.changed.v1", map[string]any{"id": alertID, "kind": domain.KindReputation, "category": o.Category, "observation_count": count, "threshold": threshold, "window_days": window, "reason": reason, "actor_user_id": actor, "occurred_at": o.UpdatedAt.Format(time.RFC3339Nano)}, o.UpdatedAt)
}

func (r *Repository) GetAlertRule(ctx context.Context, tenant string) (domain.AlertRule, error) {
	rule := domain.AlertRule{TenantID: tenant, Threshold: domain.DefaultAlertThreshold, WindowDays: domain.DefaultAlertWindowDays, UpdatedBy: "system-default"}
	e := r.db.WithTx(tenantCtx(ctx, tenant), func(ctx context.Context, tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `SELECT tenant_id,threshold,window_days,updated_by,updated_at FROM intelligence_alert_rules WHERE tenant_id=$1`, tenant).Scan(&rule.TenantID, &rule.Threshold, &rule.WindowDays, &rule.UpdatedBy, &rule.UpdatedAt)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		return e
	})
	return rule, e
}
func (r *Repository) UpsertAlertRule(ctx context.Context, rule domain.AlertRule, event string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, rule.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO intelligence_alert_rules(tenant_id,threshold,window_days,updated_by,updated_at)VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id)DO UPDATE SET threshold=EXCLUDED.threshold,window_days=EXCLUDED.window_days,updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`, rule.TenantID, rule.Threshold, rule.WindowDays, rule.UpdatedBy, rule.UpdatedAt)
		if e != nil {
			return e
		}
		return enqueue(ctx, tx, rule.TenantID, event, payload, rule.UpdatedAt)
	})
}

const alertColumns = `id,tenant_id,fingerprint,category,programme_id,campus_id,observation_count,threshold,window_days,first_observed_at,last_observed_at,reason,status,acknowledged_by,acknowledged_at,acknowledgement_note,created_at,updated_at`

func scanAlert(row scanner) (domain.Alert, error) {
	var a domain.Alert
	e := row.Scan(&a.ID, &a.TenantID, &a.Fingerprint, &a.Category, &a.ProgrammeID, &a.CampusID, &a.ObservationCount, &a.Threshold, &a.WindowDays, &a.FirstObservedAt, &a.LastObservedAt, &a.Reason, &a.Status, &a.AcknowledgedBy, &a.AcknowledgedAt, &a.AcknowledgementNote, &a.CreatedAt, &a.UpdatedAt)
	return a, e
}
func (r *Repository) ListAlerts(ctx context.Context, tenant, status string, limit int) ([]domain.Alert, error) {
	out := []domain.Alert{}
	e := r.db.WithTx(tenantCtx(ctx, tenant), func(ctx context.Context, tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT `+alertColumns+` FROM intelligence_alerts WHERE tenant_id=$1 AND ($2='' OR status=$2) ORDER BY last_observed_at DESC,id DESC LIMIT $3`, tenant, status, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			a, e := scanAlert(rows)
			if e != nil {
				return e
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, e
}
func (r *Repository) GetAlert(ctx context.Context, tenant, id string) (domain.Alert, error) {
	var a domain.Alert
	e := r.db.WithTx(tenantCtx(ctx, tenant), func(ctx context.Context, tx pgx.Tx) error {
		var e error
		a, e = scanAlert(tx.QueryRow(ctx, `SELECT `+alertColumns+` FROM intelligence_alerts WHERE tenant_id=$1 AND id=$2`, tenant, id))
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return e
	})
	return a, e
}
func (r *Repository) AcknowledgeAlert(ctx context.Context, a domain.Alert, event string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, a.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE intelligence_alerts SET status=$3,acknowledged_by=$4,acknowledged_at=$5,acknowledgement_note=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2 AND status='open'`, a.TenantID, a.ID, a.Status, a.AcknowledgedBy, a.AcknowledgedAt, a.AcknowledgementNote, a.UpdatedAt)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return enqueue(ctx, tx, a.TenantID, event, payload, a.UpdatedAt)
	})
}

func platformCtx(ctx context.Context) context.Context {
	ctx = auth.WithActor(ctx, auth.Actor{Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true})
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: "__intelligence_outbox__"})
}
func (r *Repository) ClaimPending(ctx context.Context, limit int) ([]ports.OutboxEvent, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	items := []ports.OutboxEvent{}
	e := r.db.WithTx(platformCtx(ctx), func(ctx context.Context, tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `UPDATE intelligence_outbox SET attempts=attempts+1,next_attempt_at=now()+(LEAST(300,power(2,attempts))*interval '1 second') WHERE id IN (SELECT id FROM intelligence_outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id,tenant_id,event_type,payload,created_at`, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var item ports.OutboxEvent
			if e := rows.Scan(&item.ID, &item.TenantID, &item.EventType, &item.Payload, &item.CreatedAt); e != nil {
				return e
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, e
}
func (r *Repository) MarkPublished(ctx context.Context, id string) error {
	return r.mark(ctx, id, "", true)
}
func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	return r.mark(ctx, id, message, false)
}
func (r *Repository) mark(ctx context.Context, id, message string, published bool) error {
	return r.db.WithTx(platformCtx(ctx), func(ctx context.Context, tx pgx.Tx) error {
		if published {
			_, e := tx.Exec(ctx, `UPDATE intelligence_outbox SET published_at=$2,last_error=NULL WHERE id=$1`, id, time.Now().UTC())
			return e
		}
		_, e := tx.Exec(ctx, `UPDATE intelligence_outbox SET last_error=left($2,1000) WHERE id=$1`, id, message)
		return e
	})
}

func (r *Repository) BuildSummaryItems(ctx context.Context, tenant string, from, to time.Time) ([]domain.SummaryItem, error) {
	out := []domain.SummaryItem{}
	e := r.db.WithTx(tenantCtx(ctx, tenant), func(ctx context.Context, tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `WITH versioned AS (SELECT source_id,category,programme_id,campus_id,title,evidence_excerpt,evidence_sha256,observed_at,lag(evidence_excerpt) OVER groups AS previous_excerpt,lag(evidence_sha256) OVER groups AS previous_hash,lag(observed_at) OVER groups AS previous_at FROM intelligence_observations WHERE tenant_id=$1 AND kind='competitor' AND status='approved' AND observed_at<=$3 WINDOW groups AS (PARTITION BY source_id,category,programme_id,campus_id ORDER BY observed_at,id)),ranked AS (SELECT *,row_number() OVER(PARTITION BY source_id,category,programme_id,campus_id ORDER BY observed_at DESC) AS position FROM versioned) SELECT source_id,category,programme_id,campus_id,title,evidence_excerpt,evidence_sha256,observed_at,previous_excerpt,previous_hash,previous_at FROM ranked WHERE position=1 AND observed_at>=$2 ORDER BY observed_at DESC,source_id`, tenant, from, to)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.SummaryItem
			if e := rows.Scan(&item.SourceID, &item.Category, &item.ProgrammeID, &item.CampusID, &item.LatestTitle, &item.LatestExcerpt, &item.LatestEvidenceSHA256, &item.LatestObservedAt, &item.PreviousExcerpt, &item.PreviousEvidenceSHA256, &item.PreviousObservedAt); e != nil {
				return e
			}
			item.ChangeType = "first_seen"
			if item.PreviousEvidenceSHA256 != nil && *item.PreviousEvidenceSHA256 != item.LatestEvidenceSHA256 {
				item.ChangeType = "changed"
			}
			if item.PreviousEvidenceSHA256 != nil && *item.PreviousEvidenceSHA256 == item.LatestEvidenceSHA256 {
				continue
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, e
}

const summaryColumns = `id,tenant_id,period_from,period_to,status,items,item_count,source_count,generated_by,reviewed_by,reviewed_at,review_note,created_at,updated_at`

func scanSummary(row scanner) (domain.CompetitorSummary, error) {
	var s domain.CompetitorSummary
	var raw []byte
	e := row.Scan(&s.ID, &s.TenantID, &s.PeriodFrom, &s.PeriodTo, &s.Status, &raw, &s.ItemCount, &s.SourceCount, &s.GeneratedBy, &s.ReviewedBy, &s.ReviewedAt, &s.ReviewNote, &s.CreatedAt, &s.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(raw, &s.Items)
	}
	return s, e
}
func (r *Repository) CreateSummary(ctx context.Context, s domain.CompetitorSummary, event string, payload map[string]any) error {
	raw, e := json.Marshal(s.Items)
	if e != nil {
		return e
	}
	return r.db.WithTx(tenantCtx(ctx, s.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO competitor_summaries(`+summaryColumns+`)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, s.ID, s.TenantID, s.PeriodFrom, s.PeriodTo, s.Status, raw, s.ItemCount, s.SourceCount, s.GeneratedBy, s.ReviewedBy, s.ReviewedAt, s.ReviewNote, s.CreatedAt, s.UpdatedAt)
		if e != nil {
			return e
		}
		return enqueue(ctx, tx, s.TenantID, event, payload, s.CreatedAt)
	})
}
func (r *Repository) GetSummary(ctx context.Context, tenant, id string) (domain.CompetitorSummary, error) {
	var s domain.CompetitorSummary
	e := r.db.WithTx(tenantCtx(ctx, tenant), func(ctx context.Context, tx pgx.Tx) error {
		var e error
		s, e = scanSummary(tx.QueryRow(ctx, `SELECT `+summaryColumns+` FROM competitor_summaries WHERE tenant_id=$1 AND id=$2`, tenant, id))
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return e
	})
	return s, e
}
func (r *Repository) ListSummaries(ctx context.Context, tenant string, status domain.Status, limit int) ([]domain.CompetitorSummary, error) {
	out := []domain.CompetitorSummary{}
	e := r.db.WithTx(tenantCtx(ctx, tenant), func(ctx context.Context, tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT `+summaryColumns+` FROM competitor_summaries WHERE tenant_id=$1 AND ($2='' OR status=$2) ORDER BY period_to DESC,id DESC LIMIT $3`, tenant, status, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			s, e := scanSummary(rows)
			if e != nil {
				return e
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, e
}
func (r *Repository) UpdateSummary(ctx context.Context, s domain.CompetitorSummary, expected domain.Status, event string, payload map[string]any) error {
	return r.db.WithTx(tenantCtx(ctx, s.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE competitor_summaries SET status=$3,reviewed_by=$4,reviewed_at=$5,review_note=$6,updated_at=$7 WHERE tenant_id=$1 AND id=$2 AND status=$8`, s.TenantID, s.ID, s.Status, s.ReviewedBy, s.ReviewedAt, s.ReviewNote, s.UpdatedAt, expected)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		return enqueue(ctx, tx, s.TenantID, event, payload, s.UpdatedAt)
	})
}
