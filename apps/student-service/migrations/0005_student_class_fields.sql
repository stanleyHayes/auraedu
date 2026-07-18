-- +goose Up
-- +goose StatementBegin

-- Class roster support (AURA-10.11): persist the class/academic-year assignment that
-- CreateStudent has accepted since AURA-10.4 (previously only carried on the
-- student.enrolled.v1 event). Nullable, additive; no RLS change needed — the columns
-- are covered by the existing students_tenant_isolation table policy.
-- class_id/academic_year_id are soft references to academic-service aggregates
-- (no cross-service FK, agent_plan §6); UUID type matches attendance-service 0002.

ALTER TABLE students ADD COLUMN IF NOT EXISTS class_id UUID;
ALTER TABLE students ADD COLUMN IF NOT EXISTS academic_year_id UUID;

CREATE INDEX IF NOT EXISTS idx_students_tenant_class ON students (tenant_id, class_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_students_tenant_class;
ALTER TABLE students DROP COLUMN IF EXISTS academic_year_id;
ALTER TABLE students DROP COLUMN IF EXISTS class_id;

-- +goose StatementEnd
