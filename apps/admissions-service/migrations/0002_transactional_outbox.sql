-- +goose Up
-- +goose StatementBegin
CREATE TABLE admissions_outbox(id UUID PRIMARY KEY,tenant_id TEXT NOT NULL,event_type TEXT NOT NULL,payload JSONB NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),last_error TEXT,created_at TIMESTAMPTZ NOT NULL,published_at TIMESTAMPTZ);
CREATE INDEX admissions_outbox_pending ON admissions_outbox(next_attempt_at,created_at) WHERE published_at IS NULL;
ALTER TABLE admissions_outbox ENABLE ROW LEVEL SECURITY; ALTER TABLE admissions_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY admissions_outbox_tenant ON admissions_outbox USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY admissions_outbox_platform ON admissions_outbox USING(current_setting('app.is_platform_admin',true)='true') WITH CHECK(current_setting('app.is_platform_admin',true)='true');
-- +goose StatementEnd
-- +goose Down
DROP TABLE IF EXISTS admissions_outbox;
