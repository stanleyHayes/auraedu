-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS academic_years_tenant_isolation ON academic_years;
DROP POLICY IF EXISTS terms_tenant_isolation ON terms;
DROP POLICY IF EXISTS classes_tenant_isolation ON classes;
DROP POLICY IF EXISTS subjects_tenant_isolation ON subjects;
ALTER TABLE academic_years ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE terms ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE classes ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE subjects ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
CREATE POLICY academic_years_tenant_isolation ON academic_years USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY terms_tenant_isolation ON terms USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY classes_tenant_isolation ON classes USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY subjects_tenant_isolation ON subjects USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- Irreversible: canonical tenant codes are not necessarily UUIDs.
