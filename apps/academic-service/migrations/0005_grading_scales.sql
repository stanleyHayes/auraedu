-- +goose Up
-- +goose StatementBegin
CREATE TABLE grading_scales (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ranges JSONB NOT NULL CHECK (jsonb_typeof(ranges) = 'array' AND jsonb_array_length(ranges) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT grading_scales_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE INDEX idx_grading_scales_tenant_created ON grading_scales (tenant_id, created_at, id);

ALTER TABLE grading_scales ENABLE ROW LEVEL SECURITY;
ALTER TABLE grading_scales FORCE ROW LEVEL SECURITY;
CREATE POLICY grading_scales_tenant_isolation ON grading_scales
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS grading_scales;
-- +goose StatementEnd
