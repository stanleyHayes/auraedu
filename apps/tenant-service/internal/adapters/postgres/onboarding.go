package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	platformdb "github.com/auraedu/platform/db"
	"github.com/auraedu/platform/tenancy"
	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Repository) SubmitOnboarding(
	ctx context.Context,
	request *domain.OnboardingRequest,
	idempotencyHash string,
	payloadHash string,
	emailFingerprint string,
) (*domain.OnboardingRequest, bool, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("onboarding: begin submit: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO onboarding_requests (
			id, school_name, administrator_name, email, phone, country_code,
			plan, priorities, privacy_notice_version, status, idempotency_hash,
			payload_hash, email_fingerprint, submitted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`, request.ID, request.SchoolName, request.AdministratorName, request.Email, request.Phone,
		request.CountryCode, request.Plan, request.Priorities, request.PrivacyNoticeVersion,
		request.Status, idempotencyHash, payloadHash, emailFingerprint, request.SubmittedAt)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("onboarding: commit submit: %w", err)
		}
		return request, true, nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil, false, fmt.Errorf("onboarding: submit: %w", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		return nil, false, fmt.Errorf("onboarding: rollback duplicate: %w", err)
	}

	existing, existingPayload, err := r.findOnboardingReplay(ctx, idempotencyHash, emailFingerprint)
	if err != nil {
		return nil, false, err
	}
	if existingPayload != "" && existingPayload != payloadHash {
		return nil, false, domain.ErrConflict
	}
	return &existing, false, nil
}

func (r *Repository) findOnboardingReplay(ctx context.Context, idempotencyHash, emailFingerprint string) (domain.OnboardingRequest, string, error) {
	var request domain.OnboardingRequest
	var payloadHash string
	err := scanOnboarding(r.db.Pool().QueryRow(ctx, onboardingSelect+` WHERE idempotency_hash = $1`, idempotencyHash), &request, &payloadHash)
	if err == nil {
		return request, payloadHash, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.OnboardingRequest{}, "", err
	}
	err = scanOnboarding(r.db.Pool().QueryRow(ctx, onboardingSelect+`
		WHERE email_fingerprint = $1 AND status = 'pending_review' LIMIT 1
	`, emailFingerprint), &request, &payloadHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OnboardingRequest{}, "", domain.ErrConflict
	}
	// Email dedupe deliberately returns the existing generic receipt even when
	// other submitted fields differ; only idempotency-key reuse can conflict.
	return request, "", err
}

func (r *Repository) ListOnboarding(ctx context.Context, limit int, cursor, status string) ([]domain.OnboardingRequest, string, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	query := onboardingSelect + ` WHERE ($1 = '' OR status = $1)`
	args := []any{status}
	if cursor != "" {
		query += ` AND id < $2::uuid`
		args = append(args, cursor)
	}
	query += ` ORDER BY id DESC LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit)
	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("onboarding: list: %w", err)
	}
	defer rows.Close()
	out := make([]domain.OnboardingRequest, 0, limit)
	for rows.Next() {
		var request domain.OnboardingRequest
		var payloadHash string
		if err := scanOnboarding(rows, &request, &payloadHash); err != nil {
			return nil, "", err
		}
		out = append(out, request)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == limit {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

func (r *Repository) GetOnboarding(ctx context.Context, requestID string) (domain.OnboardingRequest, error) {
	var request domain.OnboardingRequest
	var payloadHash string
	err := scanOnboarding(r.db.Pool().QueryRow(ctx, onboardingSelect+` WHERE id = $1`, requestID), &request, &payloadHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OnboardingRequest{}, domain.ErrNotFound
	}
	return request, err
}

func (r *Repository) ApproveOnboarding(ctx context.Context, requestID string, tenant domain.Tenant, decidedBy string) (domain.OnboardingRequest, error) {
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return domain.OnboardingRequest{}, err
	}
	defer tx.Rollback(ctx)
	scoped := tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenant.Code})
	if err := platformdb.SetTenantID(scoped, tx); err != nil {
		return domain.OnboardingRequest{}, err
	}
	if err := platformdb.SetPlatformAdmin(ctx, tx); err != nil {
		return domain.OnboardingRequest{}, err
	}
	var request domain.OnboardingRequest
	var payloadHash string
	if err := scanOnboarding(tx.QueryRow(ctx, onboardingSelect+` WHERE id = $1 FOR UPDATE`, requestID), &request, &payloadHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OnboardingRequest{}, domain.ErrNotFound
		}
		return domain.OnboardingRequest{}, err
	}
	if request.Status != domain.OnboardingPending {
		return domain.OnboardingRequest{}, domain.ErrConflict
	}
	if err := createTenant(ctx, tx, tenant); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.OnboardingRequest{}, domain.ErrConflict
		}
		return domain.OnboardingRequest{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE tenants SET primary_contact_email = $2 WHERE code = $1`, tenant.Code, request.Email); err != nil {
		return domain.OnboardingRequest{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE onboarding_requests
		SET status = 'approved', tenant_code = $2, decided_by = $3, decided_at = $4
		WHERE id = $1
	`, requestID, tenant.Code, decidedBy, now); err != nil {
		return domain.OnboardingRequest{}, err
	}
	request.Status, request.TenantCode, request.DecidedAt = domain.OnboardingApproved, &tenant.Code, &now
	for _, event := range []struct {
		eventType string
		payload   map[string]any
	}{
		{
			eventType: "tenant.created.v1",
			payload: map[string]any{
				"tenant_code": tenant.Code,
				"name":        tenant.Name,
				"plan":        tenant.Plan,
			},
		},
		{
			eventType: "tenant.onboarding_approved.v1",
			payload: map[string]any{
				"request_id":  requestID,
				"tenant_code": tenant.Code,
				"plan":        tenant.Plan,
			},
		},
	} {
		encoded, err := json.Marshal(event.payload)
		if err != nil {
			return domain.OnboardingRequest{}, fmt.Errorf("onboarding: encode %s outbox event: %w", event.eventType, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO tenant_outbox (id, tenant_id, event_type, payload)
			VALUES ($1, $2, $3, $4)
		`, uuid.NewString(), tenant.Code, event.eventType, encoded); err != nil {
			return domain.OnboardingRequest{}, fmt.Errorf("onboarding: enqueue %s: %w", event.eventType, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.OnboardingRequest{}, err
	}
	return request, nil
}

// OnboardingEventsDurable marks PostgreSQL approval as transactional-outbox backed.
func (*Repository) OnboardingEventsDurable() {}

func (r *Repository) RejectOnboarding(ctx context.Context, requestID, reason, decidedBy string) (domain.OnboardingRequest, error) {
	var request domain.OnboardingRequest
	var payloadHash string
	tx, err := r.db.Pool().Begin(ctx)
	if err != nil {
		return request, err
	}
	defer tx.Rollback(ctx)
	if err := scanOnboarding(tx.QueryRow(ctx, onboardingSelect+` WHERE id = $1 FOR UPDATE`, requestID), &request, &payloadHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return request, domain.ErrNotFound
		}
		return request, err
	}
	if request.Status != domain.OnboardingPending {
		return request, domain.ErrConflict
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE onboarding_requests SET status = 'rejected', decision_reason = $2, decided_by = $3, decided_at = $4 WHERE id = $1
	`, requestID, reason, decidedBy, now); err != nil {
		return request, err
	}
	request.Status, request.DecisionReason, request.DecidedAt = domain.OnboardingRejected, &reason, &now
	if err := tx.Commit(ctx); err != nil {
		return request, err
	}
	return request, nil
}

const onboardingSelect = `
	SELECT id, school_name, administrator_name, email, phone, country_code,
	       plan, priorities, privacy_notice_version, status, tenant_code,
	       decision_reason, submitted_at, decided_at, payload_hash
	FROM onboarding_requests
`

type rowScanner interface{ Scan(...any) error }

func scanOnboarding(row rowScanner, request *domain.OnboardingRequest, payloadHash *string) error {
	return row.Scan(&request.ID, &request.SchoolName, &request.AdministratorName,
		&request.Email, &request.Phone, &request.CountryCode, &request.Plan,
		&request.Priorities, &request.PrivacyNoticeVersion, &request.Status,
		&request.TenantCode, &request.DecisionReason, &request.SubmittedAt,
		&request.DecidedAt, payloadHash)
}
