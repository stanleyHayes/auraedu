-- +goose Up
-- +goose StatementBegin

-- Tenant Service schema (AURA-5.x): schools and per-tenant feature flags.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS tenants (
    code            TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    short           TEXT,
    status          TEXT NOT NULL, -- active | onboarding | suspended
    domain          TEXT,
    plan            TEXT NOT NULL,
    brand_primary   TEXT,
    brand_secondary TEXT,
    logo_url        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tenants_created_at ON tenants (created_at);

CREATE TABLE IF NOT EXISTS tenant_features (
    tenant_code TEXT NOT NULL REFERENCES tenants(code) ON DELETE CASCADE,
    feature_key TEXT NOT NULL,
    is_enabled  BOOLEAN NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_code, feature_key)
);

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_features ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_features FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenants_isolation ON tenants;
CREATE POLICY tenants_isolation ON tenants
    FOR ALL
    USING (code = current_setting('app.tenant_id', true))
    WITH CHECK (code = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS tenant_features_isolation ON tenant_features;
CREATE POLICY tenant_features_isolation ON tenant_features
    FOR ALL
    USING (tenant_code = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_code = current_setting('app.tenant_id', true));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS tenant_features_isolation ON tenant_features;
DROP POLICY IF EXISTS tenants_isolation ON tenants;
DROP TABLE IF EXISTS tenant_features;
DROP TABLE IF EXISTS tenants;

-- +goose StatementEnd
