-- +goose Up
-- +goose StatementBegin

-- Students are scoped by tenant *code* (e.g. upshs), not a UUID — align with the
-- platform convention (identity VARCHAR(50), file-service 0004_tenant_id_text).
-- Also adds the platform-admin bypass to guardian policies (was missing).
DROP POLICY IF EXISTS students_tenant_isolation ON students;
DROP POLICY IF EXISTS guardians_tenant_isolation ON guardians;
DROP POLICY IF EXISTS student_guardians_tenant_isolation ON student_guardians;

ALTER TABLE students ALTER COLUMN tenant_id TYPE TEXT;
ALTER TABLE guardians ALTER COLUMN tenant_id TYPE TEXT;
ALTER TABLE student_guardians ALTER COLUMN tenant_id TYPE TEXT;

CREATE POLICY students_tenant_isolation ON students
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

CREATE POLICY guardians_tenant_isolation ON guardians
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

CREATE POLICY student_guardians_tenant_isolation ON student_guardians
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)
           OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS students_tenant_isolation ON students;
DROP POLICY IF EXISTS guardians_tenant_isolation ON guardians;
DROP POLICY IF EXISTS student_guardians_tenant_isolation ON student_guardians;

ALTER TABLE students ALTER COLUMN tenant_id TYPE UUID USING tenant_id::uuid;
ALTER TABLE guardians ALTER COLUMN tenant_id TYPE UUID USING tenant_id::uuid;
ALTER TABLE student_guardians ALTER COLUMN tenant_id TYPE UUID USING tenant_id::uuid;

CREATE POLICY students_tenant_isolation ON students
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid
           OR current_setting('app.is_platform_admin', true)::boolean = true);

CREATE POLICY guardians_tenant_isolation ON guardians
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE POLICY student_guardians_tenant_isolation ON student_guardians
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd
