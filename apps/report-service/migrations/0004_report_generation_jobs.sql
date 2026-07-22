-- +goose Up
-- +goose StatementBegin

CREATE TABLE report_generation_jobs (
    report_card_id   UUID PRIMARY KEY,
    tenant_id        TEXT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'queued'
                     CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    attempts         INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_expires_at TIMESTAMPTZ,
    last_error       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,
    CONSTRAINT fk_report_generation_card
        FOREIGN KEY (report_card_id) REFERENCES report_cards (id) ON DELETE CASCADE
);

CREATE INDEX report_generation_jobs_ready
    ON report_generation_jobs (next_attempt_at, created_at)
    WHERE status IN ('queued', 'running');

ALTER TABLE report_generation_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_generation_jobs FORCE ROW LEVEL SECURITY;

CREATE POLICY report_generation_jobs_tenant ON report_generation_jobs
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY report_generation_jobs_platform ON report_generation_jobs
    USING (current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (current_setting('app.is_platform_admin', true) = 'true');

CREATE TABLE report_outbox (
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

CREATE INDEX report_outbox_pending
    ON report_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;

ALTER TABLE report_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY report_outbox_tenant ON report_outbox
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

CREATE POLICY report_outbox_platform ON report_outbox
    USING (current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (current_setting('app.is_platform_admin', true) = 'true');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS report_outbox;
DROP TABLE IF EXISTS report_generation_jobs;
-- +goose StatementEnd
