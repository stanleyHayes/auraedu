-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_students_tenant_id_id ON students (tenant_id, id);

CREATE TABLE enrollments (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    student_id UUID NOT NULL,
    class_id UUID NOT NULL,
    academic_year_id UUID NOT NULL,
    enrolled_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT enrollments_student_fk FOREIGN KEY (tenant_id, student_id)
        REFERENCES students (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT enrollments_student_year_unique UNIQUE (tenant_id, student_id, academic_year_id)
);

CREATE INDEX idx_enrollments_tenant_student ON enrollments (tenant_id, student_id, enrolled_at, id);
CREATE INDEX idx_enrollments_tenant_class ON enrollments (tenant_id, class_id, academic_year_id);

ALTER TABLE enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollments FORCE ROW LEVEL SECURITY;
CREATE POLICY enrollments_tenant_isolation ON enrollments
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS enrollments;
DROP INDEX IF EXISTS idx_students_tenant_id_id;
-- +goose StatementEnd
