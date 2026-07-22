-- +goose Up
-- +goose StatementBegin
CREATE TABLE assistant_outbox (
    id              UUID PRIMARY KEY,
    tenant_id       TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);
CREATE INDEX assistant_outbox_pending ON assistant_outbox (next_attempt_at, created_at) WHERE published_at IS NULL;
ALTER TABLE assistant_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE assistant_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY assistant_outbox_tenant ON assistant_outbox
    USING (tenant_id=current_setting('app.tenant_id',true))
    WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY assistant_outbox_platform ON assistant_outbox
    USING (current_setting('app.is_platform_admin',true)='true')
    WITH CHECK (current_setting('app.is_platform_admin',true)='true');
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS assistant_outbox;
-- +goose StatementEnd
