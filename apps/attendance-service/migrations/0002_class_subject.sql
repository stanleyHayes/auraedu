-- +goose Up
-- +goose StatementBegin

-- AURA-13.9 bulk class attendance: optional class/subject grouping attributes
-- (contracts/openapi/attendance.v1.yaml BulkAttendanceRequest). Nullable, additive;
-- the (tenant_id, student_id, academic_year_id, date) uniqueness rule is unchanged.

ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS class_id UUID;
ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS subject_id UUID;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE attendance_records DROP COLUMN IF EXISTS subject_id;
ALTER TABLE attendance_records DROP COLUMN IF EXISTS class_id;

-- +goose StatementEnd
