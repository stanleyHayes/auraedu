-- AURA-4.16: revoke superseded and sibling password-reset credentials.
-- +goose Up
ALTER TABLE password_resets ADD COLUMN revoked_at TIMESTAMPTZ;

DROP INDEX IF EXISTS password_resets_cleanup_idx;
CREATE INDEX password_resets_cleanup_idx
    ON password_resets ((GREATEST(
        expires_at,
        COALESCE(used_at, expires_at),
        COALESCE(revoked_at, expires_at)
    )), id);

CREATE INDEX password_resets_active_user_idx
    ON password_resets (tenant_id, user_id, created_at DESC)
    WHERE used_at IS NULL AND revoked_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS password_resets_active_user_idx;
DROP INDEX IF EXISTS password_resets_cleanup_idx;

ALTER TABLE password_resets DROP COLUMN revoked_at;

CREATE INDEX password_resets_cleanup_idx
    ON password_resets (expires_at, id);
