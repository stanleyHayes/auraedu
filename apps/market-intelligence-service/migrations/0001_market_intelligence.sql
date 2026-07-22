-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE TABLE intelligence_sources (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, kind TEXT NOT NULL CHECK(kind IN('reputation','competitor')),
 name TEXT NOT NULL, canonical_url TEXT NOT NULL, collection_method TEXT NOT NULL CHECK(collection_method IN('manual','official_api')),
 terms_reference TEXT NOT NULL, compliance_status TEXT NOT NULL CHECK(compliance_status IN('pending_review','approved','rejected')),
 created_by TEXT NOT NULL, reviewed_by TEXT, reviewed_at TIMESTAMPTZ, review_note TEXT,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, UNIQUE(tenant_id,id), UNIQUE(tenant_id,canonical_url)
);
CREATE INDEX intelligence_sources_tenant_kind_status ON intelligence_sources(tenant_id,kind,compliance_status,created_at DESC,id);
CREATE TABLE intelligence_observations (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, source_id UUID NOT NULL, kind TEXT NOT NULL CHECK(kind IN('reputation','competitor')),
 category TEXT NOT NULL CHECK(category IN('mention','recurring_issue','misinformation','programme','fee','scholarship','deadline','campaign')),
 title TEXT NOT NULL, evidence_excerpt TEXT NOT NULL, evidence_sha256 CHAR(64) NOT NULL,
 sentiment TEXT NOT NULL CHECK(sentiment IN('positive','neutral','negative','unknown')),
 programme_id UUID, campus_id UUID, response_draft TEXT NOT NULL DEFAULT '', status TEXT NOT NULL CHECK(status IN('pending_review','approved','rejected','resolved')),
 created_by TEXT NOT NULL, observed_at TIMESTAMPTZ NOT NULL, reviewed_by TEXT, reviewed_at TIMESTAMPTZ, review_note TEXT,
 resolution_note TEXT, resolved_by TEXT, resolved_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 FOREIGN KEY(tenant_id,source_id) REFERENCES intelligence_sources(tenant_id,id), UNIQUE(tenant_id,id)
);
CREATE INDEX intelligence_observations_tenant_queue ON intelligence_observations(tenant_id,kind,status,observed_at DESC,id);
CREATE TABLE intelligence_outbox (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, event_type TEXT NOT NULL, payload JSONB NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(), last_error TEXT,
 created_at TIMESTAMPTZ NOT NULL, published_at TIMESTAMPTZ
);
CREATE INDEX intelligence_outbox_pending ON intelligence_outbox(next_attempt_at,created_at) WHERE published_at IS NULL;
ALTER TABLE intelligence_sources ENABLE ROW LEVEL SECURITY; ALTER TABLE intelligence_sources FORCE ROW LEVEL SECURITY;
ALTER TABLE intelligence_observations ENABLE ROW LEVEL SECURITY; ALTER TABLE intelligence_observations FORCE ROW LEVEL SECURITY;
ALTER TABLE intelligence_outbox ENABLE ROW LEVEL SECURITY; ALTER TABLE intelligence_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY intelligence_sources_tenant ON intelligence_sources USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY intelligence_observations_tenant ON intelligence_observations USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY intelligence_outbox_tenant ON intelligence_outbox USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY intelligence_outbox_platform ON intelligence_outbox USING(current_setting('app.is_platform_admin',true)='true') WITH CHECK(current_setting('app.is_platform_admin',true)='true');
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS intelligence_outbox;
DROP TABLE IF EXISTS intelligence_observations;
DROP TABLE IF EXISTS intelligence_sources;
-- +goose StatementEnd
