-- +goose Up
-- +goose StatementBegin
-- Retain one PII-free fact per lifecycle event so attribution can be resolved
-- correctly even when JetStream subjects are delivered out of order.
CREATE TABLE IF NOT EXISTS growth_event_facts (
    tenant_id      TEXT NOT NULL,
    event_id       TEXT NOT NULL,
    event_type     TEXT NOT NULL,
    stage          TEXT NOT NULL CHECK (stage IN ('leads','applications_started','applications_submitted','admitted','offers_issued','offers_accepted')),
    bucket_date    DATE NOT NULL,
    lead_id        UUID,
    application_id UUID,
    programme_id   UUID,
    intake_id      UUID,
    source         TEXT,
    campaign_id    UUID,
    occurred_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, event_id)
);
CREATE INDEX IF NOT EXISTS idx_growth_event_facts_window
    ON growth_event_facts (tenant_id, bucket_date, stage);
ALTER TABLE growth_event_facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE growth_event_facts FORCE ROW LEVEL SECURITY;
CREATE POLICY growth_event_facts_tenant_isolation ON growth_event_facts
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS growth_event_facts_tenant_isolation ON growth_event_facts;
DROP TABLE IF EXISTS growth_event_facts;
-- +goose StatementEnd
