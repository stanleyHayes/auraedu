-- +goose Up
-- +goose StatementBegin

-- Audit Service schema (EP-23): immutable audit log aggregate.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS audit_logs (
    id             UUID PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    event_id       TEXT NOT NULL,
    event_type     TEXT NOT NULL,
    source_service TEXT NOT NULL,
    timestamp      TIMESTAMPTZ NOT NULL,
    received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload        JSONB,
    actor_id       TEXT,
    action         TEXT NOT NULL,
    resource_type  TEXT,
    resource_id    TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_id ON audit_logs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON audit_logs (event_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_received_at ON audit_logs (received_at, id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs (resource_type, resource_id);

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
DROP TABLE IF EXISTS audit_logs;

-- +goose StatementEnd
