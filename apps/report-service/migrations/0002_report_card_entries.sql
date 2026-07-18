-- +goose Up
-- +goose StatementBegin

-- Report card materialization (AURA-15.9): score/attendance entries fed by
-- assessment.score_recorded.v1 and attendance.marked.v1 events, plus a term_id
-- on report cards so cards target a period (term) per report.published.v1.
--
-- academic_year_id / template_id become nullable: the event worker auto-creates
-- DRAFT report cards for students that have none, before a year/template is
-- assigned through the API.

ALTER TABLE report_cards ADD COLUMN IF NOT EXISTS term_id UUID;
ALTER TABLE report_cards ALTER COLUMN academic_year_id DROP NOT NULL;
ALTER TABLE report_cards ALTER COLUMN template_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_report_cards_term_id ON report_cards (term_id);

-- Fast draft lookup for the event worker (tenant + student + status).
CREATE INDEX IF NOT EXISTS idx_report_cards_draft_student
    ON report_cards (tenant_id, student_id)
    WHERE status = 'draft' AND deleted_at IS NULL;

-- At most one auto-created (template-less) draft per student+period so
-- concurrent event delivery cannot duplicate it; ON CONFLICT / re-read handles
-- the race. API-created cards (template set) are unaffected.
CREATE UNIQUE INDEX IF NOT EXISTS idx_report_cards_auto_draft_uniq
    ON report_cards (tenant_id, student_id, (COALESCE(term_id, '00000000-0000-0000-0000-000000000000'::uuid)))
    WHERE status = 'draft' AND deleted_at IS NULL AND template_id IS NULL;

-- Materialized subject scores. Natural idempotency key: (report_card_id,
-- source_key) where source_key is the assessment_id (falls back to score_id or
-- the event id when producers omit it). Replayed events rewrite the same row;
-- corrected scores (new event, same assessment) update it in place.
CREATE TABLE IF NOT EXISTS report_card_score_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    report_card_id  UUID NOT NULL REFERENCES report_cards (id) ON DELETE CASCADE,
    student_id      UUID NOT NULL,
    subject_id      UUID,
    source_key      TEXT NOT NULL,
    score           DOUBLE PRECISION NOT NULL,
    max_score       DOUBLE PRECISION,
    last_event_id   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_report_card_score_entries_card_source UNIQUE (report_card_id, source_key)
);

CREATE INDEX IF NOT EXISTS idx_report_card_score_entries_tenant_id ON report_card_score_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_card_score_entries_card ON report_card_score_entries (report_card_id);

-- Materialized attendance, one row per student day. Natural idempotency key:
-- (report_card_id, entry_date); re-marks update the status in place.
CREATE TABLE IF NOT EXISTS report_card_attendance_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    report_card_id  UUID NOT NULL REFERENCES report_cards (id) ON DELETE CASCADE,
    student_id      UUID NOT NULL,
    entry_date      DATE NOT NULL,
    status          VARCHAR(20) NOT NULL CHECK (status IN ('present', 'absent', 'late', 'excused')),
    last_event_id   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_report_card_attendance_entries_card_date UNIQUE (report_card_id, entry_date)
);

CREATE INDEX IF NOT EXISTS idx_report_card_attendance_entries_tenant_id ON report_card_attendance_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_card_attendance_entries_card ON report_card_attendance_entries (report_card_id);

ALTER TABLE report_card_score_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_card_score_entries FORCE ROW LEVEL SECURITY;
ALTER TABLE report_card_attendance_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_card_attendance_entries FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS report_card_score_entries_tenant_isolation ON report_card_score_entries;
CREATE POLICY report_card_score_entries_tenant_isolation ON report_card_score_entries
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS report_card_attendance_entries_tenant_isolation ON report_card_attendance_entries;
CREATE POLICY report_card_attendance_entries_tenant_isolation ON report_card_attendance_entries
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS report_card_attendance_entries_tenant_isolation ON report_card_attendance_entries;
DROP POLICY IF EXISTS report_card_score_entries_tenant_isolation ON report_card_score_entries;
DROP TABLE IF EXISTS report_card_attendance_entries;
DROP TABLE IF EXISTS report_card_score_entries;
DROP INDEX IF EXISTS idx_report_cards_auto_draft_uniq;
DROP INDEX IF EXISTS idx_report_cards_draft_student;
DROP INDEX IF EXISTS idx_report_cards_term_id;
ALTER TABLE report_cards DROP COLUMN IF EXISTS term_id;
ALTER TABLE report_cards ALTER COLUMN academic_year_id SET NOT NULL;
ALTER TABLE report_cards ALTER COLUMN template_id SET NOT NULL;

-- +goose StatementEnd
