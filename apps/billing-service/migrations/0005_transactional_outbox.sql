-- +goose Up
-- +goose StatementBegin

CREATE TABLE billing_outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX billing_outbox_pending
    ON billing_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;

ALTER TABLE billing_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY billing_outbox_tenant_isolation ON billing_outbox
    FOR ALL
    USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    );

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS billing_outbox;
