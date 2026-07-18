-- +goose Up
-- +goose StatementBegin

-- Announcements (AURA-18.5) plus the worker idempotency ledger for consumed
-- domain events. Row-Level Security is keyed on the app.tenant_id session
-- variable (agent_plan §7), matching 0001_init.sql.

CREATE TABLE IF NOT EXISTS announcements (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    audience   VARCHAR(20) NOT NULL DEFAULT 'all' CHECK (audience IN ('all', 'students', 'guardians', 'staff')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_announcements_tenant_id_id ON announcements (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_announcements_tenant_id ON announcements (tenant_id);
CREATE INDEX IF NOT EXISTS idx_announcements_audience ON announcements (audience);
CREATE INDEX IF NOT EXISTS idx_announcements_created_at ON announcements (created_at, id);

-- Worker idempotency: one row per consumed CloudEvent (tenant_id, event_id).
CREATE TABLE IF NOT EXISTS notification_processed_events (
    tenant_id    UUID NOT NULL,
    event_id     TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id)
);

ALTER TABLE announcements ENABLE ROW LEVEL SECURITY;
ALTER TABLE announcements FORCE ROW LEVEL SECURITY;
ALTER TABLE notification_processed_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_processed_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS announcements_tenant_isolation ON announcements;
CREATE POLICY announcements_tenant_isolation ON announcements
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS notification_processed_events_tenant_isolation ON notification_processed_events;
CREATE POLICY notification_processed_events_tenant_isolation ON notification_processed_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS notification_processed_events_tenant_isolation ON notification_processed_events;
DROP POLICY IF EXISTS announcements_tenant_isolation ON announcements;
DROP TABLE IF EXISTS notification_processed_events;
DROP TABLE IF EXISTS announcements;

-- +goose StatementEnd
