-- AURA-4.15: distinguish superseded invites from successfully used invites.
-- +goose Up
ALTER TABLE invites ADD COLUMN revoked_at TIMESTAMPTZ;

DROP INDEX IF EXISTS invites_cleanup_idx;
CREATE INDEX invites_cleanup_idx
    ON invites ((GREATEST(
        expires_at,
        COALESCE(used_at, expires_at),
        COALESCE(revoked_at, expires_at)
    )), id);

CREATE INDEX invites_active_email_idx
    ON invites (tenant_id, email, created_at DESC)
    WHERE used_at IS NULL AND revoked_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS invites_active_email_idx;
DROP INDEX IF EXISTS invites_cleanup_idx;

ALTER TABLE invites DROP COLUMN revoked_at;

CREATE INDEX invites_cleanup_idx
    ON invites ((GREATEST(expires_at, COALESCE(used_at, expires_at))), id);
