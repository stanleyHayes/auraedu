-- +goose Up
-- +goose StatementBegin
CREATE TABLE file_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('file.uploaded.v1','file.updated.v1','file.deleted.v1')),
    payload JSONB NOT NULL,
    cleanup_path TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX file_outbox_pending ON file_outbox (next_attempt_at,created_at) WHERE published_at IS NULL;
ALTER TABLE file_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY file_outbox_tenant ON file_outbox USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY file_outbox_platform ON file_outbox USING (current_setting('app.is_platform_admin',true)='true') WITH CHECK (current_setting('app.is_platform_admin',true)='true');
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS file_outbox;
-- +goose StatementEnd
