-- +goose Up
-- +goose StatementBegin

-- Student Service schema (EP-10): students aggregate.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS students (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL,
    student_code  TEXT NOT NULL,
    date_of_birth DATE,
    gender        VARCHAR(20) CHECK (gender IN ('male', 'female', 'other')),
    status        VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'withdrawn', 'graduated')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_students_tenant_id ON students (tenant_id);
CREATE INDEX IF NOT EXISTS idx_students_status ON students (status);
CREATE INDEX IF NOT EXISTS idx_students_created_at ON students (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_students_code_tenant ON students (tenant_id, student_code);

ALTER TABLE students ENABLE ROW LEVEL SECURITY;
ALTER TABLE students FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS students_tenant_isolation ON students;
CREATE POLICY students_tenant_isolation ON students
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS students_tenant_isolation ON students;
DROP TABLE IF EXISTS students;

-- +goose StatementEnd
