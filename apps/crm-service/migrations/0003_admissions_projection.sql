-- +goose Up
-- +goose StatementBegin
CREATE TABLE crm_processed_events(event_id UUID PRIMARY KEY,event_type TEXT NOT NULL,tenant_id TEXT NOT NULL,processed_at TIMESTAMPTZ NOT NULL DEFAULT now());
ALTER TABLE crm_processed_events ENABLE ROW LEVEL SECURITY; ALTER TABLE crm_processed_events FORCE ROW LEVEL SECURITY;
CREATE POLICY crm_processed_events_tenant ON crm_processed_events USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
DROP TABLE IF EXISTS crm_processed_events;
