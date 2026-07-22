-- +goose Up
-- +goose StatementBegin
CREATE TABLE notification_outbox (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      TEXT NOT NULL,
    event_type     TEXT NOT NULL CHECK (event_type IN ('notification.sent.v1', 'notification.failed.v1')),
    payload        JSONB NOT NULL,
    attempts       INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ,
    last_error     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notification_outbox_pending_idx
    ON notification_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;

ALTER TABLE notification_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY notification_outbox_tenant_isolation ON notification_outbox
    USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true) = 'true'
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true) = 'true'
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_outbox;
-- +goose StatementEnd
