-- AURA-4.8: encrypted TOTP enrollment and atomic anti-replay counter.
-- +goose Up
CREATE TABLE user_mfa (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tenant_id     VARCHAR(50),
    secret_cipher BYTEA NOT NULL,
    last_counter  BIGINT NOT NULL CHECK (last_counter >= 0),
    enabled_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE user_mfa ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_mfa FORCE ROW LEVEL SECURITY;

CREATE POLICY user_mfa_isolation ON user_mfa
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')
        OR (tenant_id IS NULL AND current_setting('app.is_platform_admin', true)::boolean = true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')
        OR (tenant_id IS NULL AND current_setting('app.is_platform_admin', true)::boolean = true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    );

-- Existing privileged refresh sessions predate the MFA assurance boundary.
-- Force one password + TOTP login when this migration is introduced.
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, now())
WHERE user_id IN (
    SELECT id FROM users
    WHERE role IN ('school_admin', 'super_admin', 'platform_super_admin')
);
