-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE content_brand_profiles (
 tenant_id TEXT PRIMARY KEY,
 tone_of_voice TEXT NOT NULL,
 approved_terms TEXT[] NOT NULL DEFAULT '{}',
 prohibited_claims TEXT[] NOT NULL DEFAULT '{}',
 required_disclaimers TEXT[] NOT NULL DEFAULT '{}',
 locale TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 updated_by TEXT NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE content_drafts (
 id UUID PRIMARY KEY,
 tenant_id TEXT NOT NULL,
 campaign_id UUID,
 content_type TEXT NOT NULL,
 title TEXT NOT NULL,
 brief TEXT NOT NULL,
 audience TEXT NOT NULL,
 locale TEXT NOT NULL,
 key_messages TEXT[] NOT NULL,
 facts JSONB NOT NULL,
 content TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN('draft','pending_review','approved','rejected','expired')),
 version INTEGER NOT NULL CHECK(version > 0),
 compliance_status TEXT NOT NULL CHECK(compliance_status IN('pass','needs_review','fail')),
 compliance_findings JSONB NOT NULL,
 generator TEXT NOT NULL,
 brand_profile_version INTEGER NOT NULL CHECK(brand_profile_version > 0),
 created_by TEXT NOT NULL,
 submitted_by TEXT,
 submitted_at TIMESTAMPTZ,
 reviewed_by TEXT,
 reviewed_at TIMESTAMPTZ,
 review_note TEXT,
 expires_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(tenant_id,id)
);
CREATE INDEX content_drafts_tenant_status_time ON content_drafts(tenant_id,status,updated_at DESC,id);
CREATE INDEX content_drafts_tenant_type_time ON content_drafts(tenant_id,content_type,updated_at DESC,id);
CREATE INDEX content_drafts_tenant_campaign ON content_drafts(tenant_id,campaign_id,updated_at DESC) WHERE campaign_id IS NOT NULL;

CREATE TABLE content_versions (
 tenant_id TEXT NOT NULL,
 content_id UUID NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 content TEXT NOT NULL,
 status TEXT NOT NULL,
 compliance_status TEXT NOT NULL,
 compliance_findings JSONB NOT NULL,
 generator TEXT NOT NULL,
 brand_profile_version INTEGER NOT NULL,
 created_by TEXT NOT NULL,
 change_note TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(tenant_id,content_id,version),
 FOREIGN KEY(tenant_id,content_id) REFERENCES content_drafts(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE content_idempotency (
 tenant_id TEXT NOT NULL,
 key_hash CHAR(64) NOT NULL,
 request_hash CHAR(64) NOT NULL,
 content_id UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(tenant_id,key_hash),
 FOREIGN KEY(tenant_id,content_id) REFERENCES content_drafts(tenant_id,id) ON DELETE RESTRICT
);

CREATE TABLE content_outbox (
 id UUID PRIMARY KEY,
 tenant_id TEXT NOT NULL,
 event_type TEXT NOT NULL,
 payload JSONB NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_error TEXT,
 created_at TIMESTAMPTZ NOT NULL,
 published_at TIMESTAMPTZ
);
CREATE INDEX content_outbox_pending ON content_outbox(next_attempt_at,created_at) WHERE published_at IS NULL;

ALTER TABLE content_brand_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_brand_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY content_brand_profiles_tenant ON content_brand_profiles
 USING(tenant_id=current_setting('app.tenant_id',true))
 WITH CHECK(tenant_id=current_setting('app.tenant_id',true));

ALTER TABLE content_drafts ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_drafts FORCE ROW LEVEL SECURITY;
CREATE POLICY content_drafts_tenant ON content_drafts
 USING(tenant_id=current_setting('app.tenant_id',true))
 WITH CHECK(tenant_id=current_setting('app.tenant_id',true));

ALTER TABLE content_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_versions_tenant ON content_versions
 USING(tenant_id=current_setting('app.tenant_id',true))
 WITH CHECK(tenant_id=current_setting('app.tenant_id',true));

ALTER TABLE content_idempotency ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_idempotency FORCE ROW LEVEL SECURITY;
CREATE POLICY content_idempotency_tenant ON content_idempotency
 USING(tenant_id=current_setting('app.tenant_id',true))
 WITH CHECK(tenant_id=current_setting('app.tenant_id',true));

ALTER TABLE content_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY content_outbox_tenant ON content_outbox
 USING(tenant_id=current_setting('app.tenant_id',true))
 WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY content_outbox_platform ON content_outbox
 USING(current_setting('app.is_platform_admin',true)='true')
 WITH CHECK(current_setting('app.is_platform_admin',true)='true');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS content_outbox;
DROP TABLE IF EXISTS content_idempotency;
DROP TABLE IF EXISTS content_versions;
DROP TABLE IF EXISTS content_drafts;
DROP TABLE IF EXISTS content_brand_profiles;
-- +goose StatementEnd
