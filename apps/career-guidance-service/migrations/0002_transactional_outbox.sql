-- +goose Up
CREATE TABLE IF NOT EXISTS guidance_outbox(id varchar(36) PRIMARY KEY,tenant_id varchar(64) NOT NULL,event_type varchar(128) NOT NULL,payload jsonb NOT NULL,attempts integer NOT NULL DEFAULT 0,next_attempt_at timestamptz NOT NULL DEFAULT now(),last_error text,published_at timestamptz,created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS guidance_outbox_pending ON guidance_outbox(next_attempt_at,created_at) WHERE published_at IS NULL;
ALTER TABLE guidance_outbox ENABLE ROW LEVEL SECURITY; ALTER TABLE guidance_outbox FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS guidance_outbox_tenant_isolation ON guidance_outbox;
CREATE POLICY guidance_outbox_tenant_isolation ON guidance_outbox FOR ALL USING(tenant_id=current_setting('app.tenant_id',true) OR current_setting('app.is_platform_admin',true)::boolean=true) WITH CHECK(tenant_id=current_setting('app.tenant_id',true) OR current_setting('app.is_platform_admin',true)::boolean=true);
