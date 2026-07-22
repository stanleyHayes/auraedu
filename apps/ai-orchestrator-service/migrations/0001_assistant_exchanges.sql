-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE assistant_exchanges (
    message_id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    session_id UUID NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    citations JSONB NOT NULL DEFAULT '[]'::jsonb,
    needs_human BOOLEAN NOT NULL,
    escalation_message TEXT,
    locale TEXT NOT NULL,
    idempotency_key_hash TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '90 days'),
    UNIQUE (tenant_id, idempotency_key_hash),
    UNIQUE (tenant_id, message_id)
);

CREATE INDEX assistant_exchanges_tenant_session_time ON assistant_exchanges (tenant_id, session_id, created_at, message_id);
CREATE INDEX assistant_exchanges_unanswered ON assistant_exchanges (tenant_id, created_at DESC) WHERE needs_human;
CREATE INDEX assistant_exchanges_retention ON assistant_exchanges (expires_at);

ALTER TABLE assistant_exchanges ENABLE ROW LEVEL SECURITY;
ALTER TABLE assistant_exchanges FORCE ROW LEVEL SECURITY;
CREATE POLICY assistant_exchanges_tenant_isolation ON assistant_exchanges
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS assistant_exchanges;
-- +goose StatementEnd
