-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE crm_leads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    institution_id UUID,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT,
    normalized_email TEXT,
    phone TEXT,
    normalized_phone TEXT,
    preferred_programme_ids UUID[] NOT NULL DEFAULT '{}',
    preferred_intake_id UUID,
    source TEXT NOT NULL,
    campaign_id UUID,
    stage TEXT NOT NULL DEFAULT 'new' CHECK (stage IN ('new','contacted','engaged','qualified','application_started','application_completed','under_review','admitted','offer_accepted','deposit_paid','enrolled','lost','deferred','withdrawn')),
    score SMALLINT CHECK (score BETWEEN 0 AND 100),
    score_version TEXT,
    owner_user_id UUID,
    consent JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (normalized_email IS NOT NULL OR normalized_phone IS NOT NULL),
    UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX crm_leads_tenant_email_unique ON crm_leads (tenant_id, normalized_email) WHERE normalized_email IS NOT NULL;
CREATE UNIQUE INDEX crm_leads_tenant_phone_unique ON crm_leads (tenant_id, normalized_phone) WHERE normalized_phone IS NOT NULL;
CREATE INDEX crm_leads_tenant_stage_created ON crm_leads (tenant_id, stage, created_at DESC, id);
CREATE INDEX crm_leads_tenant_owner ON crm_leads (tenant_id, owner_user_id) WHERE owner_user_id IS NOT NULL;

CREATE TABLE crm_interactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    lead_id UUID NOT NULL,
    channel TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('prospect', 'staff', 'ai', 'system')),
    actor_id UUID,
    summary TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT crm_interactions_lead_fk FOREIGN KEY (tenant_id, lead_id)
        REFERENCES crm_leads (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX crm_interactions_tenant_lead_time ON crm_interactions (tenant_id, lead_id, occurred_at DESC, id);

CREATE TABLE crm_idempotency_keys (
    tenant_id TEXT NOT NULL,
    scope TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    resource_id UUID,
    response_code INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, scope, key_hash)
);
CREATE INDEX crm_idempotency_expiry ON crm_idempotency_keys (expires_at);

ALTER TABLE crm_leads ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_leads FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_leads_tenant_isolation ON crm_leads
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

ALTER TABLE crm_interactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_interactions FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_interactions_tenant_isolation ON crm_interactions
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

ALTER TABLE crm_idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE crm_idempotency_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_idempotency_tenant_isolation ON crm_idempotency_keys
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS crm_idempotency_keys;
DROP TABLE IF EXISTS crm_interactions;
DROP TABLE IF EXISTS crm_leads;
-- +goose StatementEnd
