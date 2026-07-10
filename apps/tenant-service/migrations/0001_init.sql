-- Tenant Service schema (reference for the Postgres+RLS adapter, next story).
-- The in-memory adapter serves the same shape today; this is the target DDL (spec §3.2, §5.2).

CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_code VARCHAR(50)  NOT NULL UNIQUE,
    name        TEXT         NOT NULL,
    short_name  TEXT         NOT NULL,
    status      VARCHAR(20)  NOT NULL DEFAULT 'onboarding', -- active | onboarding | suspended
    domain      TEXT,
    plan        VARCHAR(30)  NOT NULL DEFAULT 'starter',
    logo_url    TEXT,
    brand_primary   VARCHAR(9),  -- hex; replaces --color-brand in the UI (BRAND.md §5)
    brand_secondary VARCHAR(9),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE tenant_features (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    feature_key   VARCHAR(100) NOT NULL,
    is_enabled    BOOLEAN NOT NULL DEFAULT false,
    plan_required VARCHAR(50),
    config        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, feature_key)
);

-- Defense-in-depth: Row-Level Security keyed on the app.tenant_id session var (agent_plan §7).
ALTER TABLE tenant_features ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_features_isolation ON tenant_features
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
