-- +goose Up
-- +goose StatementBegin

-- Academic Service schema (AURA-12.2/12.3/12.4): terms, classes, subjects.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7),
-- mirroring 0001_init.sql (academic_years).

CREATE TABLE IF NOT EXISTS terms (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    academic_year_id UUID NOT NULL REFERENCES academic_years (id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    start_date       DATE NOT NULL,
    end_date         DATE NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_terms_tenant_id ON terms (tenant_id);
CREATE INDEX IF NOT EXISTS idx_terms_academic_year_id ON terms (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_terms_created_at ON terms (created_at, id);

ALTER TABLE terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE terms FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS terms_tenant_isolation ON terms;
CREATE POLICY terms_tenant_isolation ON terms
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS classes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    name             TEXT NOT NULL,
    academic_year_id UUID NOT NULL REFERENCES academic_years (id) ON DELETE CASCADE,
    class_teacher_id UUID,
    capacity         INTEGER CHECK (capacity IS NULL OR capacity > 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_classes_tenant_id ON classes (tenant_id);
CREATE INDEX IF NOT EXISTS idx_classes_academic_year_id ON classes (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_classes_created_at ON classes (created_at, id);

ALTER TABLE classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE classes FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS classes_tenant_isolation ON classes;
CREATE POLICY classes_tenant_isolation ON classes
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS subjects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    code        TEXT,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_subjects_tenant_id ON subjects (tenant_id);
CREATE INDEX IF NOT EXISTS idx_subjects_created_at ON subjects (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subjects_code_tenant ON subjects (tenant_id, code) WHERE code IS NOT NULL;

ALTER TABLE subjects ENABLE ROW LEVEL SECURITY;
ALTER TABLE subjects FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS subjects_tenant_isolation ON subjects;
CREATE POLICY subjects_tenant_isolation ON subjects
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS subjects_tenant_isolation ON subjects;
DROP TABLE IF EXISTS subjects;

DROP POLICY IF EXISTS classes_tenant_isolation ON classes;
DROP TABLE IF EXISTS classes;

DROP POLICY IF EXISTS terms_tenant_isolation ON terms;
DROP TABLE IF EXISTS terms;

-- +goose StatementEnd
