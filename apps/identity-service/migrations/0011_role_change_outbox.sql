-- +goose Up
CREATE TABLE identity_outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       VARCHAR(50) NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);

CREATE INDEX identity_outbox_pending
    ON identity_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;

ALTER TABLE identity_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY identity_outbox_isolation ON identity_outbox
    USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    );

-- +goose Down
DROP TABLE IF EXISTS identity_outbox;
