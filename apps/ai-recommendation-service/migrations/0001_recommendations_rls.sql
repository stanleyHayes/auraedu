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

CREATE TABLE IF NOT EXISTS recommendations (
    id varchar(36) PRIMARY KEY,
    tenant_id varchar(64) NOT NULL,
    student_id varchar(36) NOT NULL,
    recommendation_type varchar(64) NOT NULL,
    title varchar(255) NOT NULL,
    description text,
    status varchar(16) NOT NULL DEFAULT 'pending',
    confidence double precision NOT NULL DEFAULT 0,
    explanation text,
    approved_by varchar(36),
    approved_at timestamp,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL
);

DROP POLICY IF EXISTS recommendations_tenant_isolation ON recommendations;
ALTER TABLE recommendations ALTER COLUMN tenant_id TYPE varchar(64);
CREATE INDEX IF NOT EXISTS ix_recommendations_tenant_id ON recommendations (tenant_id);
CREATE INDEX IF NOT EXISTS ix_recommendations_student_id ON recommendations (student_id);
CREATE INDEX IF NOT EXISTS ix_recommendations_tenant_student_status ON recommendations (tenant_id, student_id, status);
ALTER TABLE recommendations ENABLE ROW LEVEL SECURITY;
ALTER TABLE recommendations FORCE ROW LEVEL SECURITY;
CREATE POLICY recommendations_tenant_isolation ON recommendations
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
