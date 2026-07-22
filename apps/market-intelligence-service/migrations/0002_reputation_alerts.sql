-- +goose Up
-- +goose StatementBegin
CREATE TABLE intelligence_alert_rules (
 tenant_id TEXT PRIMARY KEY,
 threshold INTEGER NOT NULL CHECK(threshold BETWEEN 2 AND 20),
 window_days INTEGER NOT NULL CHECK(window_days BETWEEN 1 AND 90),
 updated_by TEXT NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE intelligence_alerts (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, fingerprint TEXT NOT NULL,
 category TEXT NOT NULL CHECK(category IN('recurring_issue','misinformation')),
 programme_id UUID, campus_id UUID, observation_count INTEGER NOT NULL CHECK(observation_count>=2),
 threshold INTEGER NOT NULL, window_days INTEGER NOT NULL,
 first_observed_at TIMESTAMPTZ NOT NULL, last_observed_at TIMESTAMPTZ NOT NULL,
 reason TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN('open','acknowledged')),
 acknowledged_by TEXT, acknowledged_at TIMESTAMPTZ, acknowledgement_note TEXT,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(tenant_id,id)
);
CREATE UNIQUE INDEX intelligence_alerts_one_open_group ON intelligence_alerts(tenant_id,fingerprint) WHERE status='open';
CREATE INDEX intelligence_alerts_tenant_queue ON intelligence_alerts(tenant_id,status,last_observed_at DESC,id);
ALTER TABLE intelligence_alert_rules ENABLE ROW LEVEL SECURITY; ALTER TABLE intelligence_alert_rules FORCE ROW LEVEL SECURITY;
ALTER TABLE intelligence_alerts ENABLE ROW LEVEL SECURITY; ALTER TABLE intelligence_alerts FORCE ROW LEVEL SECURITY;
CREATE POLICY intelligence_alert_rules_tenant ON intelligence_alert_rules USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY intelligence_alerts_tenant ON intelligence_alerts USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS intelligence_alerts;
DROP TABLE IF EXISTS intelligence_alert_rules;
-- +goose StatementEnd
