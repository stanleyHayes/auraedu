-- Identity Service schema (reference for the Postgres adapter, next story).
-- Users + credentials are tenant-scoped (spec §8); platform admins have tenant_id NULL.
-- Target hash is argon2id; the in-memory adapter uses stdlib PBKDF2-HMAC-SHA256 today.

CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID,                        -- NULL for platform super admins
    email       CITEXT NOT NULL,             -- case-insensitive
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
    algo        VARCHAR(20) NOT NULL DEFAULT 'argon2id',
    salt        BYTEA NOT NULL,
    hash        BYTEA NOT NULL,
    params      JSONB NOT NULL,              -- KDF cost params
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tenant-scoped rows enforce isolation via RLS (agent_plan §7).
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_isolation ON users
    USING (tenant_id IS NULL OR tenant_id = current_setting('app.tenant_id', true)::uuid);
