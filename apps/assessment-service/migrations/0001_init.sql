-- +goose Up
-- +goose StatementBegin

-- Assessment Service schema (EP-14): Assessment and Score aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS assessments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    academic_year_id  UUID NOT NULL,
    subject_id        UUID NOT NULL,
    type              VARCHAR(20) NOT NULL CHECK (type IN ('assignment', 'test', 'exam')),
    title             TEXT NOT NULL,
    description       TEXT,
    max_score         INTEGER NOT NULL CHECK (max_score > 0),
    due_date          TIMESTAMPTZ,
    status            VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_assessments_tenant_id_id ON assessments (tenant_id, id);

CREATE TABLE IF NOT EXISTS scores (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    assessment_id     UUID NOT NULL,
    student_id        UUID NOT NULL,
    score             INTEGER NOT NULL CHECK (score >= 0),
    recorded_by       UUID NOT NULL,
    notes             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT fk_scores_assessment
        FOREIGN KEY (tenant_id, assessment_id)
        REFERENCES assessments (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_assessments_tenant_id ON assessments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_assessments_academic_year_id ON assessments (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_assessments_subject_id ON assessments (subject_id);
CREATE INDEX IF NOT EXISTS idx_assessments_type ON assessments (type);
CREATE INDEX IF NOT EXISTS idx_assessments_status ON assessments (status);
CREATE INDEX IF NOT EXISTS idx_assessments_created_at ON assessments (created_at, id);

CREATE INDEX IF NOT EXISTS idx_scores_tenant_id ON scores (tenant_id);
CREATE INDEX IF NOT EXISTS idx_scores_assessment_id ON scores (assessment_id);
CREATE INDEX IF NOT EXISTS idx_scores_student_id ON scores (student_id);
CREATE INDEX IF NOT EXISTS idx_scores_created_at ON scores (created_at, id);

ALTER TABLE assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessments FORCE ROW LEVEL SECURITY;
ALTER TABLE scores ENABLE ROW LEVEL SECURITY;
ALTER TABLE scores FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS assessments_tenant_isolation ON assessments;
CREATE POLICY assessments_tenant_isolation ON assessments
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS scores_tenant_isolation ON scores;
CREATE POLICY scores_tenant_isolation ON scores
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS scores_tenant_isolation ON scores;
DROP POLICY IF EXISTS assessments_tenant_isolation ON assessments;
DROP TABLE IF EXISTS scores;
DROP TABLE IF EXISTS assessments;

-- +goose StatementEnd
