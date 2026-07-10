-- +goose Up
-- +goose StatementBegin

-- Attendance Service schema (EP-13): AttendanceRecord aggregate.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS attendance_records (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    student_id        UUID NOT NULL,
    academic_year_id  UUID NOT NULL,
    date              DATE NOT NULL,
    status            VARCHAR(20) NOT NULL CHECK (status IN ('present', 'absent', 'late', 'excused')),
    reason            TEXT,
    marked_by         UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_attendance_records_tenant_id ON attendance_records (tenant_id);
CREATE INDEX IF NOT EXISTS idx_attendance_records_student_id ON attendance_records (student_id);
CREATE INDEX IF NOT EXISTS idx_attendance_records_academic_year_id ON attendance_records (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_attendance_records_date ON attendance_records (date);
CREATE INDEX IF NOT EXISTS idx_attendance_records_status ON attendance_records (status);
CREATE INDEX IF NOT EXISTS idx_attendance_records_created_at ON attendance_records (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_records_unique_attendance
    ON attendance_records (tenant_id, student_id, academic_year_id, date)
    WHERE deleted_at IS NULL;

ALTER TABLE attendance_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance_records FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS attendance_records_tenant_isolation ON attendance_records;
CREATE POLICY attendance_records_tenant_isolation ON attendance_records
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS attendance_records_tenant_isolation ON attendance_records;
DROP TABLE IF EXISTS attendance_records;

-- +goose StatementEnd
