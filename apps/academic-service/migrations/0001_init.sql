-- +goose Up
-- +goose StatementBegin

-- Academic Service schema (EP-12): academic_years aggregate.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS academic_years (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    code        TEXT NOT NULL,
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    is_current  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_academic_years_tenant_id ON academic_years (tenant_id);
CREATE INDEX IF NOT EXISTS idx_academic_years_status ON academic_years (status);
CREATE INDEX IF NOT EXISTS idx_academic_years_created_at ON academic_years (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_academic_years_code_tenant ON academic_years (tenant_id, code);

ALTER TABLE academic_years ENABLE ROW LEVEL SECURITY;
ALTER TABLE academic_years FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS academic_years_tenant_isolation ON academic_years;
CREATE POLICY academic_years_tenant_isolation ON academic_years
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS academic_years_tenant_isolation ON academic_years;
DROP TABLE IF EXISTS academic_years;

-- +goose StatementEnd
