-- +goose Up
-- +goose StatementBegin

-- CBT Service schema (EP-24): QuestionBank, ExamSession and Submission aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS cbt_questions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    academic_year_id  UUID NOT NULL,
    subject_id        UUID NOT NULL,
    question_text     TEXT NOT NULL,
    question_type     VARCHAR(20) NOT NULL CHECK (question_type IN ('multiple_choice', 'true_false', 'short_answer')),
    options           JSONB,
    correct_answer    TEXT NOT NULL,
    marks             INTEGER NOT NULL CHECK (marks > 0),
    status            VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS cbt_exam_sessions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    title             TEXT NOT NULL,
    academic_year_id  UUID NOT NULL,
    subject_id        UUID NOT NULL,
    question_ids      UUID[] NOT NULL,
    duration_minutes  INTEGER NOT NULL CHECK (duration_minutes > 0),
    start_at          TIMESTAMPTZ,
    end_at            TIMESTAMPTZ,
    status            VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'published', 'active', 'closed', 'archived')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,
    UNIQUE (tenant_id, id)
);

CREATE TABLE IF NOT EXISTS cbt_submissions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    exam_session_id   UUID NOT NULL,
    student_id        UUID NOT NULL,
    answers           JSONB NOT NULL DEFAULT '{}'::jsonb,
    status            VARCHAR(20) NOT NULL CHECK (status IN ('in_progress', 'submitted', 'graded')),
    score             INTEGER,
    max_score         INTEGER NOT NULL DEFAULT 0 CHECK (max_score >= 0),
    submitted_at      TIMESTAMPTZ,
    graded_at         TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT fk_submissions_exam_session
        FOREIGN KEY (tenant_id, exam_session_id)
        REFERENCES cbt_exam_sessions (tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cbt_questions_tenant_id_id ON cbt_questions (tenant_id, id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cbt_submissions_tenant_id_id ON cbt_submissions (tenant_id, id);

CREATE INDEX IF NOT EXISTS idx_cbt_questions_tenant_id ON cbt_questions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cbt_questions_academic_year_id ON cbt_questions (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_cbt_questions_subject_id ON cbt_questions (subject_id);
CREATE INDEX IF NOT EXISTS idx_cbt_questions_status ON cbt_questions (status);
CREATE INDEX IF NOT EXISTS idx_cbt_questions_created_at ON cbt_questions (created_at, id);

CREATE INDEX IF NOT EXISTS idx_cbt_exam_sessions_tenant_id ON cbt_exam_sessions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cbt_exam_sessions_academic_year_id ON cbt_exam_sessions (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_cbt_exam_sessions_subject_id ON cbt_exam_sessions (subject_id);
CREATE INDEX IF NOT EXISTS idx_cbt_exam_sessions_status ON cbt_exam_sessions (status);
CREATE INDEX IF NOT EXISTS idx_cbt_exam_sessions_created_at ON cbt_exam_sessions (created_at, id);

CREATE INDEX IF NOT EXISTS idx_cbt_submissions_tenant_id ON cbt_submissions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cbt_submissions_exam_session_id ON cbt_submissions (exam_session_id);
CREATE INDEX IF NOT EXISTS idx_cbt_submissions_student_id ON cbt_submissions (student_id);
CREATE INDEX IF NOT EXISTS idx_cbt_submissions_status ON cbt_submissions (status);
CREATE INDEX IF NOT EXISTS idx_cbt_submissions_created_at ON cbt_submissions (created_at, id);

ALTER TABLE cbt_questions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cbt_questions FORCE ROW LEVEL SECURITY;
ALTER TABLE cbt_exam_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cbt_exam_sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE cbt_submissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cbt_submissions FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cbt_questions_tenant_isolation ON cbt_questions;
CREATE POLICY cbt_questions_tenant_isolation ON cbt_questions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS cbt_exam_sessions_tenant_isolation ON cbt_exam_sessions;
CREATE POLICY cbt_exam_sessions_tenant_isolation ON cbt_exam_sessions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS cbt_submissions_tenant_isolation ON cbt_submissions;
CREATE POLICY cbt_submissions_tenant_isolation ON cbt_submissions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS cbt_submissions_tenant_isolation ON cbt_submissions;
DROP POLICY IF EXISTS cbt_exam_sessions_tenant_isolation ON cbt_exam_sessions;
DROP POLICY IF EXISTS cbt_questions_tenant_isolation ON cbt_questions;

DROP TABLE IF EXISTS cbt_submissions;
DROP TABLE IF EXISTS cbt_exam_sessions;
DROP TABLE IF EXISTS cbt_questions;

-- +goose StatementEnd
