-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE TABLE campaigns (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL, objective TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN('draft','pending_approval','approved','scheduled','active','paused','completed','cancelled')),
 channel TEXT NOT NULL, audience_definition TEXT NOT NULL, programme_ids UUID[] NOT NULL DEFAULT '{}',
 budget NUMERIC(18,2) NOT NULL CHECK(budget>=0), currency CHAR(3) NOT NULL, start_at TIMESTAMPTZ NOT NULL,
 end_at TIMESTAMPTZ NOT NULL, approval_status TEXT NOT NULL, owner_user_id TEXT NOT NULL,
 submitted_by TEXT, submitted_at TIMESTAMPTZ, approved_by TEXT, approved_at TIMESTAMPTZ, review_note TEXT,
 tracking_url_parameters TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 CHECK(end_at>start_at), UNIQUE(tenant_id,id)
);
CREATE INDEX campaigns_tenant_status_time ON campaigns(tenant_id,status,start_at DESC,id);
ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaigns FORCE ROW LEVEL SECURITY;
CREATE POLICY campaigns_tenant_isolation ON campaigns USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS campaigns;
-- +goose StatementEnd
