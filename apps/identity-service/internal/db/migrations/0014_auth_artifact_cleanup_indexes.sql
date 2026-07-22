-- AURA-4.11: indexed, bounded authentication-artifact retention.
-- +goose Up
CREATE INDEX password_resets_cleanup_idx
    ON password_resets (expires_at, id);

CREATE INDEX invites_cleanup_idx
    ON invites ((GREATEST(expires_at, COALESCE(used_at, expires_at))), id);

CREATE INDEX identity_outbox_published_cleanup_idx
    ON identity_outbox (published_at, id)
    WHERE published_at IS NOT NULL;

CREATE INDEX refresh_tokens_family_expiry_idx
    ON refresh_tokens (family_id, expires_at DESC);

-- +goose Down
DROP INDEX IF EXISTS refresh_tokens_family_expiry_idx;
DROP INDEX IF EXISTS identity_outbox_published_cleanup_idx;
DROP INDEX IF EXISTS invites_cleanup_idx;
DROP INDEX IF EXISTS password_resets_cleanup_idx;
