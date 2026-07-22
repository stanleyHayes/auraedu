-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS staff_assignments (
    id          UUID PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    staff_id    UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    class_id    UUID NOT NULL,
    subject_id  UUID,
    role        VARCHAR(100),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (tenant_id, staff_id, class_id, subject_id)
);

CREATE INDEX IF NOT EXISTS idx_staff_assignments_staff ON staff_assignments (tenant_id, staff_id, assigned_at, id);
CREATE INDEX IF NOT EXISTS idx_staff_assignments_class ON staff_assignments (tenant_id, class_id);

ALTER TABLE staff_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE staff_assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY staff_assignments_tenant_isolation ON staff_assignments
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS staff_assignments_tenant_isolation ON staff_assignments;
DROP TABLE IF EXISTS staff_assignments;

-- +goose StatementEnd
