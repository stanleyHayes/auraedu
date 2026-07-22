package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/auraedu/tenant-service/internal/domain"
	"github.com/auraedu/tenant-service/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func enqueueTenantEvent(ctx context.Context, tx pgx.Tx, tenantCode, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s outbox event: %w", eventType, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_outbox (id, tenant_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`, uuid.NewString(), tenantCode, eventType, encoded); err != nil {
		return fmt.Errorf("enqueue %s: %w", eventType, err)
	}
	return nil
}

// ActivateOnboardingTenant atomically changes the onboarding tenant to active
// and records exactly one activation event. A repeated call is a successful
// no-op and does not create another event.
func (r *Repository) ActivateOnboardingTenant(ctx context.Context, code string) (bool, error) {
	changed := false
	err := r.db.WithTx(withTenant(ctx, code), func(ctx context.Context, tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM tenants WHERE code = $1 FOR UPDATE`, code).Scan(&status); err != nil {
			return tenantNotFound(err, code)
		}
		if status == "active" {
			return nil
		}
		if status != "onboarding" {
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE tenants SET status = 'active', updated_at = now() WHERE code = $1`, code); err != nil {
			return fmt.Errorf("activate tenant: %w", err)
		}
		if err := enqueueTenantEvent(ctx, tx, code, "tenant.activated.v1", map[string]any{
			"tenant_code": code,
			"status":      "active",
		}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

func (*Repository) TenantLifecycleEventsDurable() {}

var _ ports.DurableTenantLifecycleRepository = (*Repository)(nil)
