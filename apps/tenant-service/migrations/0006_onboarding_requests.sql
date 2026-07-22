-- +goose Up
-- +goose StatementBegin
CREATE TABLE onboarding_requests (
    -- auraedu: rls-exempt platform-owned pre-tenant review workflow
    id                     UUID PRIMARY KEY,
    school_name            TEXT NOT NULL,
    administrator_name     TEXT NOT NULL,
    email                  TEXT NOT NULL,
    phone                  TEXT,
    country_code           CHAR(2) NOT NULL,
    plan                   TEXT NOT NULL,
    priorities             TEXT,
    privacy_notice_version TEXT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'pending_review',
    tenant_code            TEXT REFERENCES tenants(code),
    decision_reason        TEXT,
    decided_by             TEXT,
    decided_at             TIMESTAMPTZ,
    idempotency_hash       TEXT NOT NULL UNIQUE,
    payload_hash           TEXT NOT NULL,
    email_fingerprint      TEXT NOT NULL,
    submitted_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    retention_expires_at   TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '24 months'),
    CONSTRAINT onboarding_request_status_check CHECK (status IN ('pending_review', 'approved', 'rejected', 'provisioning_failed')),
    CONSTRAINT onboarding_request_plan_check CHECK (plan IN ('starter', 'growth', 'professional', 'ai_plus', 'enterprise'))
);
CREATE UNIQUE INDEX onboarding_requests_pending_email_unique
    ON onboarding_requests (email_fingerprint)
    WHERE status = 'pending_review';
CREATE INDEX onboarding_requests_queue_idx ON onboarding_requests (status, submitted_at DESC, id DESC);
CREATE INDEX onboarding_requests_retention_idx ON onboarding_requests (retention_expires_at);
-- Platform-owned pre-tenant data: access is exposed only through the service's
-- public submit and platform-admin application use cases, never tenant queries.
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS onboarding_requests;
-- +goose StatementEnd
