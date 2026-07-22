-- +goose Up
-- +goose StatementBegin
-- Event-level idempotency and tenant-local attribution for the Growth funnel.
-- Analytics remains event-driven: these tables never reach into CRM or Admissions databases.
CREATE TABLE IF NOT EXISTS analytics_processed_events (
    tenant_id   TEXT NOT NULL,
    event_id    TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, event_id)
);

CREATE TABLE IF NOT EXISTS growth_lead_attribution (
    tenant_id  TEXT NOT NULL,
    lead_id    UUID NOT NULL,
    source     TEXT NOT NULL,
    campaign_id UUID,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, lead_id)
);

CREATE TABLE IF NOT EXISTS growth_application_attribution (
    tenant_id      TEXT NOT NULL,
    application_id UUID NOT NULL,
    lead_id        UUID,
    programme_id   UUID NOT NULL,
    intake_id      UUID,
    started_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, application_id)
);

CREATE INDEX IF NOT EXISTS idx_growth_lead_attribution_campaign
    ON growth_lead_attribution (tenant_id, campaign_id) WHERE campaign_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_growth_application_attribution_programme
    ON growth_application_attribution (tenant_id, programme_id);

ALTER TABLE analytics_processed_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics_processed_events FORCE ROW LEVEL SECURITY;
ALTER TABLE growth_lead_attribution ENABLE ROW LEVEL SECURITY;
ALTER TABLE growth_lead_attribution FORCE ROW LEVEL SECURITY;
ALTER TABLE growth_application_attribution ENABLE ROW LEVEL SECURITY;
ALTER TABLE growth_application_attribution FORCE ROW LEVEL SECURITY;

CREATE POLICY analytics_processed_events_tenant_isolation ON analytics_processed_events
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY growth_lead_attribution_tenant_isolation ON growth_lead_attribution
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY growth_application_attribution_tenant_isolation ON growth_application_attribution
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS growth_application_attribution_tenant_isolation ON growth_application_attribution;
DROP POLICY IF EXISTS growth_lead_attribution_tenant_isolation ON growth_lead_attribution;
DROP POLICY IF EXISTS analytics_processed_events_tenant_isolation ON analytics_processed_events;
DROP TABLE IF EXISTS growth_application_attribution;
DROP TABLE IF EXISTS growth_lead_attribution;
DROP TABLE IF EXISTS analytics_processed_events;
-- +goose StatementEnd
