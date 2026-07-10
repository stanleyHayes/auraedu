-- +goose Up
-- +goose StatementBegin

-- Staff Service schema (EP-11): staff aggregate.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS staff (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL,
    staff_type    VARCHAR(20) NOT NULL CHECK (staff_type IN ('teacher', 'non_teaching')),
    email         TEXT,
    staff_code    TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_staff_tenant_id ON staff (tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_status ON staff (status);
CREATE INDEX IF NOT EXISTS idx_staff_created_at ON staff (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_code_tenant ON staff (tenant_id, staff_code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_email_tenant ON staff (tenant_id, email) WHERE email IS NOT NULL;

ALTER TABLE staff ENABLE ROW LEVEL SECURITY;
ALTER TABLE staff FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS staff_tenant_isolation ON staff;
CREATE POLICY staff_tenant_isolation ON staff
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS staff_tenant_isolation ON staff;
DROP TABLE IF EXISTS staff;

-- +goose StatementEnd
