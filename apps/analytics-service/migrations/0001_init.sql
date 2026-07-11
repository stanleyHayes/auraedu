-- +goose Up
-- +goose StatementBegin

-- Analytics Service schema (EP-21): time-bucketed Metric projections.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS metrics (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    metric_name   VARCHAR(100) NOT NULL,
    bucket_date   DATE NOT NULL,
    value         NUMERIC(20, 4) NOT NULL,
    unit          VARCHAR(20) NOT NULL CHECK (unit IN ('count', 'sum', 'average', 'percentage')),
    dimensions    JSONB NOT NULL DEFAULT '{}',
    sample_count  BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_metrics_tenant_id ON metrics (tenant_id);
CREATE INDEX IF NOT EXISTS idx_metrics_metric_name ON metrics (metric_name);
CREATE INDEX IF NOT EXISTS idx_metrics_bucket_date ON metrics (bucket_date);
CREATE INDEX IF NOT EXISTS idx_metrics_created_at ON metrics (created_at, id);
CREATE INDEX IF NOT EXISTS idx_metrics_dimensions ON metrics USING GIN (dimensions);

CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_unique_projection
    ON metrics (tenant_id, metric_name, bucket_date, dimensions);

ALTER TABLE metrics ENABLE ROW LEVEL SECURITY;
ALTER TABLE metrics FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS metrics_tenant_isolation ON metrics;
CREATE POLICY metrics_tenant_isolation ON metrics
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS metrics_tenant_isolation ON metrics;
DROP TABLE IF EXISTS metrics;

-- +goose StatementEnd
