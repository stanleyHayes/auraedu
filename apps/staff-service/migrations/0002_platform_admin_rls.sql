-- +goose Up
-- Platform super admins may read/modify staff records across tenants.

DROP POLICY IF EXISTS staff_tenant_isolation ON staff;
CREATE POLICY staff_tenant_isolation ON staff
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose Down
DROP POLICY IF EXISTS staff_tenant_isolation ON staff;
CREATE POLICY staff_tenant_isolation ON staff
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
