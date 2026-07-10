-- Identity Service schema (EP-04).
-- Users + credentials are tenant-scoped by tenant_code; platform admins have tenant_id NULL.
-- Password hashes use argon2id; refresh tokens, password resets and invites are tokenised.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   VARCHAR(50),
    email       CITEXT NOT NULL,
    name        TEXT   NOT NULL,
    role        VARCHAR(40) NOT NULL,
    permissions TEXT[] NOT NULL DEFAULT '{}',
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email)
);

CREATE TABLE credentials (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tenant_id   VARCHAR(50),
    algo        VARCHAR(20) NOT NULL DEFAULT 'argon2id',
    salt        BYTEA NOT NULL,
    hash        BYTEA NOT NULL,
    params      JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id   VARCHAR(50),
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE TABLE password_resets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id   VARCHAR(50),
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invites (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   VARCHAR(50) NOT NULL,
    email       CITEXT NOT NULL,
    role        VARCHAR(40) NOT NULL,
    permissions TEXT[] NOT NULL DEFAULT '{}',
    token_hash  TEXT NOT NULL UNIQUE,
    invited_by  UUID REFERENCES users(id),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_isolation ON users
    USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);

ALTER TABLE credentials ENABLE ROW LEVEL SECURITY;
CREATE POLICY credentials_isolation ON credentials
    USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);

ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY refresh_tokens_isolation ON refresh_tokens
    USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);

ALTER TABLE password_resets ENABLE ROW LEVEL SECURITY;
CREATE POLICY password_resets_isolation ON password_resets
    USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);

ALTER TABLE invites ENABLE ROW LEVEL SECURITY;
CREATE POLICY invites_isolation ON invites
    USING (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);
