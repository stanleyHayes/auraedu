package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/auraedu/ai-orchestrator-service/internal/domain"
	"github.com/auraedu/platform/tenancy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const actionColumns = `
	id, tenant_id, action, level, policy_version, target_type, target_id,
	payload, payload_hash, reason, status, proposed_by, proposer_role,
	reviewed_by, reviewer_role, review_note, reviewed_at, execution_attempts,
	result, failure_code, failure_detail, executed_at, created_at, updated_at,
	idempotency_key_hash, request_hash`

type scanner interface{ Scan(...any) error }

func scanAction(row scanner) (domain.ActionProposal, error) {
	var action domain.ActionProposal
	var payload, result []byte
	err := row.Scan(
		&action.ID, &action.TenantID, &action.Action, &action.Level,
		&action.PolicyVersion, &action.TargetType, &action.TargetID, &payload,
		&action.PayloadHash, &action.Reason, &action.Status, &action.ProposedBy,
		&action.ProposerRole, &action.ReviewedBy, &action.ReviewerRole,
		&action.ReviewNote, &action.ReviewedAt, &action.ExecutionAttempts,
		&result, &action.FailureCode, &action.FailureDetail, &action.ExecutedAt,
		&action.CreatedAt, &action.UpdatedAt, &action.IdempotencyKeyHash,
		&action.RequestHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ActionProposal{}, domain.ErrNotFound
	}
	action.Payload, action.Result = payload, result
	return action, err
}

func actionContext(ctx context.Context, tenantID string) context.Context {
	return tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID})
}

func insertActionAudit(ctx context.Context, tx pgx.Tx, tenantID string, entry domain.ActionAuditEntry) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO ai_action_audit(
			id, tenant_id, action_id, event, actor_id, actor_role, evidence, occurred_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		entry.ID, tenantID, entry.ActionID, entry.Event, entry.ActorID, entry.ActorRole, entry.Evidence, entry.OccurredAt)
	return err
}

func (r *Repository) FindActionReplay(ctx context.Context, tenantID, keyHash string) (domain.ActionProposal, bool, error) {
	action, err := r.GetActionBy(ctx, tenantID, `idempotency_key_hash=$2`, keyHash)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ActionProposal{}, false, nil
	}
	return action, err == nil, err
}

func (r *Repository) GetActionBy(ctx context.Context, tenantID, predicate string, value any) (domain.ActionProposal, error) {
	var action domain.ActionProposal
	err := r.db.WithTx(actionContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		action, err = scanAction(tx.QueryRow(
			ctx,
			`SELECT `+actionColumns+` FROM ai_action_proposals WHERE tenant_id=$1 AND `+predicate,
			tenantID,
			value,
		))
		return err
	})
	return action, err
}

func (r *Repository) CreateAction(ctx context.Context, action domain.ActionProposal, audit domain.ActionAuditEntry) error {
	return r.db.WithTx(actionContext(ctx, action.TenantID), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO ai_action_proposals(
				id, tenant_id, action, level, policy_version, target_type, target_id,
				payload, payload_hash, reason, status, proposed_by, proposer_role,
				idempotency_key_hash, request_hash, created_at, updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			action.ID, action.TenantID, action.Action, action.Level, action.PolicyVersion, action.TargetType, action.TargetID, action.Payload,
			action.PayloadHash, action.Reason, action.Status, action.ProposedBy, action.ProposerRole,
			action.IdempotencyKeyHash, action.RequestHash, action.CreatedAt, action.UpdatedAt)
		if err != nil {
			return err
		}
		return insertActionAudit(ctx, tx, action.TenantID, audit)
	})
}

func (r *Repository) ListActions(ctx context.Context, tenantID, status string, limit int) ([]domain.ActionProposal, error) {
	items := []domain.ActionProposal{}
	err := r.db.WithTx(actionContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+actionColumns+`
			FROM ai_action_proposals
			WHERE tenant_id=$1 AND ($2='' OR status=$2)
			ORDER BY created_at DESC,id DESC LIMIT $3`, tenantID, status, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			action, err := scanAction(rows)
			if err != nil {
				return err
			}
			items = append(items, action)
		}
		return rows.Err()
	})
	return items, err
}

func (r *Repository) GetAction(ctx context.Context, tenantID, id string) (domain.ActionProposal, error) {
	return r.GetActionBy(ctx, tenantID, `id=$2`, id)
}

func (r *Repository) ReviewAction(
	ctx context.Context,
	tenantID, id, reviewerID, reviewerRole, note string,
	approve bool,
	now time.Time,
) (domain.ActionProposal, error) {
	var action domain.ActionProposal
	status, event := domain.ActionRejected, "rejected"
	if approve {
		status, event = domain.ActionApproved, "approved"
	}
	err := r.db.WithTx(actionContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		action, err = scanAction(tx.QueryRow(ctx, `
			UPDATE ai_action_proposals
			SET status=$3, reviewed_by=$4, reviewer_role=$5,
				review_note=NULLIF($6,''), reviewed_at=$7, updated_at=$7
			WHERE tenant_id=$1 AND id=$2
				AND status='pending_approval' AND proposed_by<>$4
			RETURNING `+actionColumns, tenantID, id, status, reviewerID, reviewerRole, note, now))
		if err != nil {
			return err
		}
		evidence, marshalErr := json.Marshal(map[string]any{
			"note":           note,
			"payload_hash":   action.PayloadHash,
			"policy_version": action.PolicyVersion,
		})
		if marshalErr != nil {
			return marshalErr
		}
		return insertActionAudit(ctx, tx, tenantID, domain.ActionAuditEntry{
			ID: uuid.NewString(), ActionID: id, Event: event,
			ActorID: reviewerID, ActorRole: reviewerRole,
			Evidence: evidence, OccurredAt: now,
		})
	})
	return action, err
}

func (r *Repository) StartActionExecution(
	ctx context.Context,
	tenantID, id, actorID, actorRole string,
	now time.Time,
) (domain.ActionProposal, error) {
	var action domain.ActionProposal
	err := r.db.WithTx(actionContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		action, err = scanAction(tx.QueryRow(ctx, `
			UPDATE ai_action_proposals
			SET status='executing', execution_attempts=execution_attempts+1,
				failure_code=NULL, failure_detail=NULL, updated_at=$3
			WHERE tenant_id=$1 AND id=$2 AND status IN ('approved','failed')
			RETURNING `+actionColumns, tenantID, id, now))
		if err != nil {
			return err
		}
		evidence, marshalErr := json.Marshal(map[string]any{
			"attempt": action.ExecutionAttempts, "payload_hash": action.PayloadHash,
		})
		if marshalErr != nil {
			return marshalErr
		}
		return insertActionAudit(ctx, tx, tenantID, domain.ActionAuditEntry{
			ID: uuid.NewString(), ActionID: id, Event: "execution_started",
			ActorID: actorID, ActorRole: actorRole, Evidence: evidence, OccurredAt: now,
		})
	})
	return action, err
}

func (r *Repository) FinishActionExecution(
	ctx context.Context,
	tenantID, id string,
	succeeded bool,
	result json.RawMessage,
	code, detail string,
	now time.Time,
) (domain.ActionProposal, error) {
	var action domain.ActionProposal
	status, event := domain.ActionFailed, "execution_failed"
	if succeeded {
		status, event = domain.ActionSucceeded, "execution_succeeded"
	}
	err := r.db.WithTx(actionContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		var err error
		action, err = scanAction(tx.QueryRow(ctx, `
			UPDATE ai_action_proposals
			SET status=$3, result=$4, failure_code=NULLIF($5,''),
				failure_detail=NULLIF($6,''), executed_at=$7, updated_at=$7
			WHERE tenant_id=$1 AND id=$2 AND status='executing'
			RETURNING `+actionColumns, tenantID, id, status, nullableJSON(result), code, detail, now))
		if err != nil {
			return err
		}
		evidence, marshalErr := json.Marshal(map[string]any{
			"attempt": action.ExecutionAttempts, "result": result,
			"failure_code": code, "failure_detail": detail,
		})
		if marshalErr != nil {
			return marshalErr
		}
		return insertActionAudit(ctx, tx, tenantID, domain.ActionAuditEntry{
			ID: uuid.NewString(), ActionID: id, Event: event,
			ActorID: "ai-orchestrator-service", ActorRole: "service",
			Evidence: evidence, OccurredAt: now,
		})
	})
	return action, err
}

func (r *Repository) ListActionAudit(ctx context.Context, tenantID, id string) ([]domain.ActionAuditEntry, error) {
	items := []domain.ActionAuditEntry{}
	err := r.db.WithTx(actionContext(ctx, tenantID), func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, action_id, event, actor_id, actor_role, evidence, occurred_at
			FROM ai_action_audit
			WHERE tenant_id=$1 AND action_id=$2
			ORDER BY occurred_at,id`, tenantID, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item domain.ActionAuditEntry
			if err := rows.Scan(
				&item.ID, &item.ActionID, &item.Event, &item.ActorID,
				&item.ActorRole, &item.Evidence, &item.OccurredAt,
			); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
