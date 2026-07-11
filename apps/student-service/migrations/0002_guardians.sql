-- +goose Up
-- +goose StatementBegin

-- Guardian + Student↔Guardian link schema (AURA-10.10).

CREATE TABLE IF NOT EXISTS guardians (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL,
    relationship  TEXT NOT NULL,
    phone         TEXT,
    email         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS student_guardians (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    student_id    UUID NOT NULL,
    guardian_id   UUID NOT NULL,
    relationship  TEXT,
    is_primary    BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, student_id, guardian_id)
);

CREATE INDEX IF NOT EXISTS idx_guardians_tenant_id ON guardians (tenant_id);
CREATE INDEX IF NOT EXISTS idx_guardians_name ON guardians (tenant_id, last_name, first_name);
CREATE INDEX IF NOT EXISTS idx_student_guardians_student ON student_guardians (tenant_id, student_id);
CREATE INDEX IF NOT EXISTS idx_student_guardians_guardian ON student_guardians (tenant_id, guardian_id);

ALTER TABLE guardians ENABLE ROW LEVEL SECURITY;
ALTER TABLE guardians FORCE ROW LEVEL SECURITY;
ALTER TABLE student_guardians ENABLE ROW LEVEL SECURITY;
ALTER TABLE student_guardians FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS guardians_tenant_isolation ON guardians;
CREATE POLICY guardians_tenant_isolation ON guardians
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS student_guardians_tenant_isolation ON student_guardians;
CREATE POLICY student_guardians_tenant_isolation ON student_guardians
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS student_guardians_tenant_isolation ON student_guardians;
DROP POLICY IF EXISTS guardians_tenant_isolation ON guardians;
DROP TABLE IF EXISTS student_guardians;
DROP TABLE IF EXISTS guardians;

-- +goose StatementEnd
