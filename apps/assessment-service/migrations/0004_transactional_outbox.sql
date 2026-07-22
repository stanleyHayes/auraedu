-- +goose Up
-- +goose StatementBegin
CREATE TABLE assessment_outbox(id UUID PRIMARY KEY DEFAULT gen_random_uuid(),tenant_id TEXT NOT NULL,event_type TEXT NOT NULL,payload JSONB NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),last_error TEXT,published_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now());CREATE INDEX idx_assessment_outbox_pending ON assessment_outbox(next_attempt_at,created_at)WHERE published_at IS NULL;ALTER TABLE assessment_outbox ENABLE ROW LEVEL SECURITY;ALTER TABLE assessment_outbox FORCE ROW LEVEL SECURITY;CREATE POLICY assessment_outbox_tenant_isolation ON assessment_outbox FOR ALL USING(tenant_id=current_setting('app.tenant_id',true)OR current_setting('app.is_platform_admin',true)::boolean=true)WITH CHECK(tenant_id=current_setting('app.tenant_id',true)OR current_setting('app.is_platform_admin',true)::boolean=true);
-- +goose StatementEnd
-- +goose Down
DROP TABLE IF EXISTS assessment_outbox;
