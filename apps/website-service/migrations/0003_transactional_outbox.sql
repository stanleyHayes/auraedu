-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS website_outbox (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id TEXT NOT NULL, event_type TEXT NOT NULL,
 payload JSONB NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_error TEXT, published_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_website_outbox_pending ON website_outbox(next_attempt_at,created_at) WHERE published_at IS NULL;
ALTER TABLE website_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE website_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY website_outbox_tenant_isolation ON website_outbox FOR ALL
 USING (tenant_id=current_setting('app.tenant_id',true) OR current_setting('app.is_platform_admin',true)::boolean=true)
 WITH CHECK (tenant_id=current_setting('app.tenant_id',true) OR current_setting('app.is_platform_admin',true)::boolean=true);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS website_outbox_tenant_isolation ON website_outbox;
DROP TABLE IF EXISTS website_outbox;
-- +goose StatementEnd
