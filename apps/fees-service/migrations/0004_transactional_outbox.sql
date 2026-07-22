-- +goose Up
-- +goose StatementBegin

CREATE TABLE fees_outbox (
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

CREATE INDEX fees_outbox_pending
    ON fees_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;

ALTER TABLE fees_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE fees_outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY fees_outbox_tenant ON fees_outbox
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY fees_outbox_platform ON fees_outbox
    USING (current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (current_setting('app.is_platform_admin', true) = 'true');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fees_outbox;
-- +goose StatementEnd
