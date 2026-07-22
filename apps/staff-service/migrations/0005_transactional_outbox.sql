-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS staff_outbox (
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

CREATE INDEX IF NOT EXISTS idx_staff_outbox_pending
    ON staff_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;

ALTER TABLE staff_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE staff_outbox FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS staff_outbox_tenant_isolation ON staff_outbox;
CREATE POLICY staff_outbox_tenant_isolation ON staff_outbox
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS staff_outbox_tenant_isolation ON staff_outbox;
DROP TABLE IF EXISTS staff_outbox;

-- +goose StatementEnd
