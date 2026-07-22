-- +goose Up
-- +goose StatementBegin
-- Current-state facts let score updates and deletes replace projections rather
-- than permanently inflating append-only aggregates.
CREATE TABLE assessment_score_facts (
    tenant_id        TEXT NOT NULL,
    score_id         UUID NOT NULL,
    assessment_id    UUID NOT NULL,
    student_id       UUID NOT NULL,
    subject_id       UUID NOT NULL,
    academic_year_id UUID NOT NULL,
    bucket_date      DATE NOT NULL,
    score            NUMERIC(20, 4) NOT NULL CHECK (score >= 0),
    max_score        NUMERIC(20, 4) NOT NULL CHECK (max_score > 0 AND score <= max_score),
    recorded_at      TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, score_id)
);

CREATE INDEX idx_assessment_score_facts_rollup
    ON assessment_score_facts (tenant_id, bucket_date, student_id, subject_id, academic_year_id);

ALTER TABLE assessment_score_facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment_score_facts FORCE ROW LEVEL SECURITY;

CREATE POLICY assessment_score_facts_tenant_isolation ON assessment_score_facts
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS assessment_score_facts_tenant_isolation ON assessment_score_facts;
DROP TABLE IF EXISTS assessment_score_facts;
-- +goose StatementEnd
