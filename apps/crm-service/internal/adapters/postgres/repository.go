// Package postgres implements tenant-isolated CRM persistence.
//
//nolint:lll // SQL statements stay adjacent to ordered arguments for transactional review.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/auraedu/crm-service/internal/domain"
	"github.com/auraedu/crm-service/internal/ports"
	"github.com/auraedu/platform/db"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db *db.DB }

var _ ports.Repository = (*Repository)(nil)
var _ ports.FeedbackRepository = (*Repository)(nil)
var _ ports.CallbackRepository = (*Repository)(nil)

func NewRepository(database *db.DB) *Repository { return &Repository{db: database} }

const leadColumns = `id,tenant_id,institution_id,first_name,last_name,email,phone,preferred_programme_ids,preferred_intake_id,source,campaign_id,stage,owner_user_id,score,score_version,score_confidence,score_positive_factors,score_negative_factors,scored_at,consent,created_at,updated_at`

func (r *Repository) FindCallbackReplay(ctx context.Context, tenantID, keyHash, requestHash string) (ports.CallbackResult, bool, error) {
	var result ports.CallbackResult
	found := false
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var resourceID, storedHash string
		err := tx.QueryRow(ctx, `SELECT resource_id,request_hash FROM crm_idempotency_keys WHERE tenant_id=$1 AND scope='callback.schedule' AND key_hash=$2 AND expires_at>now()`, tenantID, keyHash).Scan(&resourceID, &storedHash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("crm: read callback idempotency: %w", err)
		}
		if storedHash != requestHash {
			return domain.ErrConflict
		}
		callback, err := getCallbackTx(ctx, tx, tenantID, resourceID)
		if err != nil {
			return err
		}
		result, found = ports.CallbackResult{Callback: callback, Replay: true}, true
		return nil
	})
	return result, found, err
}

func (r *Repository) ScheduleCallback(ctx context.Context, callback *domain.CallbackRequest, keyHash, requestHash string) (ports.CallbackResult, error) {
	var result ports.CallbackResult
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var resourceID, storedHash string
		err := tx.QueryRow(ctx, `SELECT resource_id,request_hash FROM crm_idempotency_keys WHERE tenant_id=$1 AND scope='callback.schedule' AND key_hash=$2 AND expires_at>now()`, callback.TenantID, keyHash).Scan(&resourceID, &storedHash)
		if err == nil {
			if storedHash != requestHash {
				return domain.ErrConflict
			}
			stored, err := getCallbackTx(ctx, tx, callback.TenantID, resourceID)
			if err != nil {
				return err
			}
			result = ports.CallbackResult{Callback: stored, Replay: true}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("crm: read callback idempotency: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crm_callback_requests(id,tenant_id,lead_id,preferred_at,timezone,locale,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, callback.ID, callback.TenantID, callback.LeadID, callback.PreferredAt, callback.Timezone, callback.Locale, callback.Status, callback.CreatedAt, callback.UpdatedAt); err != nil {
			return fmt.Errorf("crm: insert callback request: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crm_idempotency_keys(tenant_id,scope,key_hash,request_hash,resource_id,response_code,expires_at) VALUES($1,'callback.schedule',$2,$3,$4,201,now()+interval '24 hours')`, callback.TenantID, keyHash, requestHash, callback.ID); err != nil {
			return fmt.Errorf("crm: save callback idempotency: %w", err)
		}
		result.Callback = callback
		return nil
	})
	return result, err
}

func (r *Repository) ListCallbacks(ctx context.Context, tenantID string, status domain.CallbackStatus, limit int) ([]*domain.CallbackRequest, error) {
	var callbacks []*domain.CallbackRequest
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,tenant_id,lead_id,preferred_at,timezone,locale,status,created_at,updated_at FROM crm_callback_requests WHERE tenant_id=$1 AND ($2='' OR status=$2) ORDER BY preferred_at,id LIMIT $3`, tenantID, status, limit)
		if err != nil {
			return fmt.Errorf("crm: list callback requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			callback, err := scanCallback(rows)
			if err != nil {
				return err
			}
			callbacks = append(callbacks, callback)
		}
		return rows.Err()
	})
	return callbacks, err
}

func getCallbackTx(ctx context.Context, tx pgx.Tx, tenantID, callbackID string) (*domain.CallbackRequest, error) {
	callback, err := scanCallback(tx.QueryRow(ctx, `SELECT id,tenant_id,lead_id,preferred_at,timezone,locale,status,created_at,updated_at FROM crm_callback_requests WHERE tenant_id=$1 AND id=$2`, tenantID, callbackID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return callback, err
}

func scanCallback(row scanner) (*domain.CallbackRequest, error) {
	var callback domain.CallbackRequest
	if err := row.Scan(&callback.ID, &callback.TenantID, &callback.LeadID, &callback.PreferredAt, &callback.Timezone, &callback.Locale, &callback.Status, &callback.CreatedAt, &callback.UpdatedAt); err != nil {
		return nil, err
	}
	return &callback, nil
}

func (r *Repository) SubmitFeedback(ctx context.Context, feedback *domain.Feedback, keyHash, requestHash string) (ports.FeedbackResult, error) {
	var result ports.FeedbackResult
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var resourceID, storedHash string
		err := tx.QueryRow(ctx, `SELECT resource_id,request_hash FROM crm_idempotency_keys WHERE tenant_id=$1 AND scope='feedback.submit' AND key_hash=$2 AND expires_at>now()`, feedback.TenantID, keyHash).Scan(&resourceID, &storedHash)
		if err == nil {
			if storedHash != requestHash {
				return domain.ErrConflict
			}
			feedback.ID = resourceID
			result = ports.FeedbackResult{Feedback: feedback, Replay: true}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("crm: read feedback idempotency: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crm_feedback (id,tenant_id,interaction_id,ai_run_id,feedback_type,rating,comment,review_status,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, feedback.ID, feedback.TenantID, feedback.InteractionID, feedback.AIRunID, feedback.FeedbackType, feedback.Rating, feedback.Comment, feedback.ReviewStatus, feedback.CreatedAt); err != nil {
			return fmt.Errorf("crm: insert feedback: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crm_idempotency_keys (tenant_id,scope,key_hash,request_hash,resource_id,response_code,expires_at) VALUES ($1,'feedback.submit',$2,$3,$4,202,now()+interval '24 hours')`, feedback.TenantID, keyHash, requestHash, feedback.ID); err != nil {
			return fmt.Errorf("crm: save feedback idempotency: %w", err)
		}
		result.Feedback = feedback
		return nil
	})
	return result, err
}

//nolint:gocognit // Capture intentionally keeps deduplication and merge operations in one transaction.
func (r *Repository) Capture(ctx context.Context, lead *domain.Lead, keyHash, requestHash string, initial *domain.Interaction) (ports.CaptureResult, error) {
	var result ports.CaptureResult
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var resourceID string
		var storedHash string
		var responseCode int
		err := tx.QueryRow(ctx, `SELECT resource_id, request_hash, response_code FROM crm_idempotency_keys WHERE tenant_id=$1 AND scope='lead.capture' AND key_hash=$2 AND expires_at > now()`, lead.TenantID, keyHash).Scan(&resourceID, &storedHash, &responseCode)
		if err == nil {
			if storedHash != requestHash {
				return domain.ErrConflict
			}
			got, getErr := getLeadTx(ctx, tx, lead.TenantID, resourceID)
			if getErr != nil {
				return getErr
			}
			result = ports.CaptureResult{Lead: got, Created: responseCode == 201, Replay: true}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("crm: read idempotency: %w", err)
		}

		existing, err := findContactTx(ctx, tx, lead)
		created := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !created {
			return err
		}
		if created {
			if err := insertLeadTx(ctx, tx, lead); err != nil {
				return err
			}
			existing = lead
		} else {
			existing.FirstName, existing.LastName = lead.FirstName, lead.LastName
			existing.Consent.Email = existing.Consent.Email || lead.Consent.Email
			existing.Consent.SMS = existing.Consent.SMS || lead.Consent.SMS
			existing.Consent.WhatsApp = existing.Consent.WhatsApp || lead.Consent.WhatsApp
			existing.Consent.Voice = existing.Consent.Voice || lead.Consent.Voice
			existing.Consent.PrivacyNoticeVersion = lead.Consent.PrivacyNoticeVersion
			existing.UpdatedAt = time.Now().UTC()
			consent, marshalErr := json.Marshal(existing.Consent)
			if marshalErr != nil {
				return fmt.Errorf("crm: encode merged consent: %w", marshalErr)
			}
			if _, err := tx.Exec(ctx, `UPDATE crm_leads SET first_name=$3,last_name=$4,consent=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, lead.TenantID, existing.ID, existing.FirstName, existing.LastName, consent, existing.UpdatedAt); err != nil {
				return fmt.Errorf("crm: merge lead: %w", err)
			}
		}
		if initial != nil {
			initial.LeadID = existing.ID
			if err := insertInteractionTx(ctx, tx, initial); err != nil {
				return err
			}
		}
		code := 200
		if created {
			code = 201
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crm_idempotency_keys (tenant_id,scope,key_hash,request_hash,resource_id,response_code,expires_at) VALUES ($1,'lead.capture',$2,$3,$4,$5,now()+interval '24 hours')`, lead.TenantID, keyHash, requestHash, existing.ID, code); err != nil {
			return fmt.Errorf("crm: save idempotency: %w", err)
		}
		result = ports.CaptureResult{Lead: existing, Created: created}
		return nil
	})
	return result, err
}

func findContactTx(ctx context.Context, tx pgx.Tx, lead *domain.Lead) (*domain.Lead, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM crm_leads WHERE tenant_id=$1 AND (($2::text IS NOT NULL AND normalized_email=$2) OR ($3::text IS NOT NULL AND normalized_phone=$3)) ORDER BY created_at LIMIT 2`, lead.TenantID, lead.Email, lead.Phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, pgx.ErrNoRows
	}
	if len(ids) > 1 {
		return nil, fmt.Errorf("%w: contact identifiers belong to different leads", domain.ErrConflict)
	}
	return getLeadTx(ctx, tx, lead.TenantID, ids[0])
}

func insertLeadTx(ctx context.Context, tx pgx.Tx, lead *domain.Lead) error {
	consent, err := json.Marshal(lead.Consent)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO crm_leads (id,tenant_id,institution_id,first_name,last_name,email,normalized_email,phone,normalized_phone,preferred_programme_ids,preferred_intake_id,source,campaign_id,stage,owner_user_id,consent,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, lead.ID, lead.TenantID, lead.InstitutionID, lead.FirstName, lead.LastName, lead.Email, lead.Phone, lead.PreferredProgrammeIDs, lead.PreferredIntakeID, lead.Source, lead.CampaignID, lead.Stage, lead.OwnerUserID, consent, lead.CreatedAt, lead.UpdatedAt)
	if err != nil {
		return fmt.Errorf("crm: insert lead: %w", err)
	}
	return nil
}

func (r *Repository) GetLead(ctx context.Context, tenantID, leadID string) (*domain.Lead, error) {
	var lead *domain.Lead
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		lead, err = getLeadTx(ctx, tx, tenantID, leadID)
		return err
	})
	return lead, err
}

func getLeadTx(ctx context.Context, tx pgx.Tx, tenantID, leadID string) (*domain.Lead, error) {
	lead, err := scanLead(tx.QueryRow(ctx, `SELECT `+leadColumns+` FROM crm_leads WHERE tenant_id=$1 AND id=$2`, tenantID, leadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return lead, err
}

func (r *Repository) ListLeads(ctx context.Context, tenantID string, limit int, cursor string, filter ports.LeadFilter) ([]*domain.Lead, string, error) {
	var leads []*domain.Lead
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+leadColumns+` FROM crm_leads WHERE tenant_id=$1 AND ($2::text IS NULL OR stage=$2) AND ($3::uuid IS NULL OR owner_user_id=$3) AND ($4='' OR first_name ILIKE '%'||$4||'%' OR last_name ILIKE '%'||$4||'%' OR email ILIKE '%'||$4||'%') AND ($5='' OR (created_at,id) < (SELECT created_at,id FROM crm_leads WHERE tenant_id=$1 AND id=$5::uuid)) ORDER BY created_at DESC,id DESC LIMIT $6`, tenantID, filter.Stage, filter.OwnerUserID, filter.Search, cursor, limit)
		if err != nil {
			return fmt.Errorf("crm: list leads: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			lead, err := scanLead(rows)
			if err != nil {
				return err
			}
			leads = append(leads, lead)
		}
		return rows.Err()
	})
	next := ""
	if len(leads) == limit && len(leads) > 0 {
		next = leads[len(leads)-1].ID
	}
	return leads, next, err
}

func (r *Repository) UpdateLead(ctx context.Context, tenantID string, lead *domain.Lead) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE crm_leads SET stage=$3,owner_user_id=$4,preferred_programme_ids=$5,updated_at=$6 WHERE tenant_id=$1 AND id=$2`, tenantID, lead.ID, lead.Stage, lead.OwnerUserID, lead.PreferredProgrammeIDs, lead.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// ProjectAdmissionsStage advances (never regresses) the CRM funnel and records
// a privacy-safe system interaction in the same idempotent transaction.
func (r *Repository) ProjectAdmissionsStage(ctx context.Context, tenantID, leadID, eventID, eventType string, stage domain.LeadStage, occurredAt time.Time) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO crm_processed_events(event_id,event_type,tenant_id) VALUES($1,$2,$3) ON CONFLICT(event_id) DO NOTHING`, eventID, eventType, tenantID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		tag, err = tx.Exec(ctx, `UPDATE crm_leads SET stage=$3,updated_at=$4 WHERE tenant_id=$1 AND id=$2 AND stage NOT IN('lost','deferred','withdrawn') AND CASE stage WHEN 'new' THEN 0 WHEN 'contacted' THEN 1 WHEN 'engaged' THEN 2 WHEN 'qualified' THEN 3 WHEN 'application_started' THEN 4 WHEN 'application_completed' THEN 5 WHEN 'under_review' THEN 6 WHEN 'admitted' THEN 7 WHEN 'offer_accepted' THEN 8 WHEN 'deposit_paid' THEN 9 WHEN 'enrolled' THEN 10 ELSE 99 END < CASE $3 WHEN 'application_started' THEN 4 WHEN 'application_completed' THEN 5 WHEN 'admitted' THEN 7 WHEN 'offer_accepted' THEN 8 ELSE 99 END`, tenantID, leadID, stage, occurredAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crm_leads WHERE tenant_id=$1 AND id=$2)`, tenantID, leadID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrNotFound
			}
			return nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO crm_interactions(id,tenant_id,lead_id,channel,direction,actor_type,summary,occurred_at) VALUES($1,$2,$3,'system','inbound','system',$4,$5) ON CONFLICT(id) DO NOTHING`, eventID, tenantID, leadID, "Admissions lifecycle advanced to "+string(stage), occurredAt)
		return err
	})
}

func (r *Repository) CreateInteraction(ctx context.Context, tenantID string, interaction *domain.Interaction) error {
	return r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if interaction.TenantID != tenantID {
			return domain.ErrForbidden
		}
		return insertInteractionTx(ctx, tx, interaction)
	})
}

func insertInteractionTx(ctx context.Context, tx pgx.Tx, interaction *domain.Interaction) error {
	_, err := tx.Exec(ctx, `INSERT INTO crm_interactions (id,tenant_id,lead_id,channel,direction,actor_type,actor_id,summary,occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, interaction.ID, interaction.TenantID, interaction.LeadID, interaction.Channel, interaction.Direction, interaction.ActorType, interaction.ActorID, interaction.Summary, interaction.OccurredAt)
	if err != nil {
		return fmt.Errorf("crm: insert interaction: %w", err)
	}
	return nil
}

func (r *Repository) ListInteractions(ctx context.Context, tenantID, leadID string, limit int, cursor string) ([]*domain.Interaction, string, error) {
	var items []*domain.Interaction
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id,tenant_id,lead_id,channel,direction,actor_type,actor_id,summary,occurred_at FROM crm_interactions WHERE tenant_id=$1 AND lead_id=$2 AND ($3='' OR (occurred_at,id)<(SELECT occurred_at,id FROM crm_interactions WHERE tenant_id=$1 AND id=$3::uuid)) ORDER BY occurred_at DESC,id DESC LIMIT $4`, tenantID, leadID, cursor, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.Interaction
			if err := rows.Scan(&item.ID, &item.TenantID, &item.LeadID, &item.Channel, &item.Direction, &item.ActorType, &item.ActorID, &item.Summary, &item.OccurredAt); err != nil {
				return err
			}
			items = append(items, &item)
		}
		return rows.Err()
	})
	next := ""
	if len(items) == limit && len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return items, next, err
}

func (r *Repository) GetScoringEvidence(ctx context.Context, tenantID, leadID string) (domain.ScoringEvidence, error) {
	var evidence domain.ScoringEvidence
	err := r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE direction='inbound' AND actor_type='prospect'),max(occurred_at) FILTER (WHERE direction='inbound' AND actor_type='prospect') FROM crm_interactions WHERE tenant_id=$1 AND lead_id=$2`, tenantID, leadID).Scan(&evidence.InboundProspectInteractions, &evidence.LastInboundAt)
	})
	return evidence, err
}

func (r *Repository) SaveLeadScore(ctx context.Context, tenantID, leadID, triggeredBy string, score domain.LeadScore) (bool, error) {
	changed := false
	positive, err := json.Marshal(score.PositiveFactors)
	if err != nil {
		return false, err
	}
	negative, err := json.Marshal(score.NegativeFactors)
	if err != nil {
		return false, err
	}
	err = r.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var same bool
		err := tx.QueryRow(ctx, `SELECT COALESCE(score=$3 AND score_version=$4 AND score_confidence=$5 AND score_positive_factors=$6::jsonb AND score_negative_factors=$7::jsonb,false) FROM crm_leads WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, leadID, score.Score, score.RuleVersion, score.Confidence, positive, negative).Scan(&same)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if same {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE crm_leads SET score=$3,score_version=$4,score_confidence=$5,score_positive_factors=$6,score_negative_factors=$7,scored_at=$8,updated_at=$8 WHERE tenant_id=$1 AND id=$2`, tenantID, leadID, score.Score, score.RuleVersion, score.Confidence, positive, negative, score.EvaluatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO crm_lead_score_evaluations(tenant_id,lead_id,score,confidence,positive_factors,negative_factors,rule_version,triggered_by,evaluated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, tenantID, leadID, score.Score, score.Confidence, positive, negative, score.RuleVersion, triggeredBy, score.EvaluatedAt); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

type scanner interface{ Scan(...any) error }

func scanLead(row scanner) (*domain.Lead, error) {
	var lead domain.Lead
	var consent, positive, negative []byte
	if err := row.Scan(&lead.ID, &lead.TenantID, &lead.InstitutionID, &lead.FirstName, &lead.LastName, &lead.Email, &lead.Phone, &lead.PreferredProgrammeIDs, &lead.PreferredIntakeID, &lead.Source, &lead.CampaignID, &lead.Stage, &lead.OwnerUserID, &lead.Score, &lead.ScoreVersion, &lead.ScoreConfidence, &positive, &negative, &lead.ScoredAt, &consent, &lead.CreatedAt, &lead.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(consent, &lead.Consent); err != nil {
		return nil, fmt.Errorf("crm: decode consent: %w", err)
	}
	if err := json.Unmarshal(positive, &lead.ScorePositiveFactors); err != nil {
		return nil, fmt.Errorf("crm: decode positive score factors: %w", err)
	}
	if err := json.Unmarshal(negative, &lead.ScoreNegativeFactors); err != nil {
		return nil, fmt.Errorf("crm: decode negative score factors: %w", err)
	}
	return &lead, nil
}
