-- +goose Up
CREATE TABLE IF NOT EXISTS feature_store_metrics (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(64) NOT NULL,
    student_id varchar(36) NOT NULL,
    metric_key varchar(128) NOT NULL,
    value double precision NOT NULL,
    source varchar(32) NOT NULL,
    recorded_at timestamp NOT NULL,
    created_at timestamp NOT NULL
);

DROP POLICY IF EXISTS feature_store_metrics_tenant_isolation ON feature_store_metrics;
ALTER TABLE feature_store_metrics ALTER COLUMN tenant_id TYPE varchar(64);
ALTER TABLE feature_store_metrics ALTER COLUMN student_id TYPE varchar(36);
ALTER TABLE feature_store_metrics ALTER COLUMN metric_key TYPE varchar(128);
ALTER TABLE feature_store_metrics ALTER COLUMN source TYPE varchar(32);
CREATE INDEX IF NOT EXISTS ix_feature_store_metrics_tenant_id ON feature_store_metrics (tenant_id);
CREATE INDEX IF NOT EXISTS ix_feature_store_metrics_student_id ON feature_store_metrics (student_id);
CREATE INDEX IF NOT EXISTS ix_feature_store_metrics_tenant_student ON feature_store_metrics (tenant_id, student_id);
ALTER TABLE feature_store_metrics ENABLE ROW LEVEL SECURITY;
ALTER TABLE feature_store_metrics FORCE ROW LEVEL SECURITY;
CREATE POLICY feature_store_metrics_tenant_isolation ON feature_store_metrics
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

CREATE TABLE IF NOT EXISTS predictions (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(64) NOT NULL,
    student_id varchar(36) NOT NULL,
    prediction_type varchar(50) NOT NULL,
    title varchar(200) NOT NULL DEFAULT '',
    value double precision NOT NULL DEFAULT 0,
    confidence double precision NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending',
    explanation text NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL
);

DROP POLICY IF EXISTS predictions_tenant_isolation ON predictions;
ALTER TABLE predictions ALTER COLUMN tenant_id TYPE varchar(64);
CREATE INDEX IF NOT EXISTS ix_predictions_tenant_id ON predictions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_prediction_student ON predictions (tenant_id, student_id);
ALTER TABLE predictions ENABLE ROW LEVEL SECURITY;
ALTER TABLE predictions FORCE ROW LEVEL SECURITY;
CREATE POLICY predictions_tenant_isolation ON predictions
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
