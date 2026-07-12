-- +goose Up
-- Allow platform super admins to bypass tenant RLS for cross-tenant reads.

DROP POLICY IF EXISTS tenants_isolation ON tenants;
CREATE POLICY tenants_isolation ON tenants
    FOR ALL
    USING (code = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (code = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);

DROP POLICY IF EXISTS tenant_features_isolation ON tenant_features;
CREATE POLICY tenant_features_isolation ON tenant_features
    FOR ALL
    USING (tenant_code = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true)
    WITH CHECK (tenant_code = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true)::boolean = true);

-- +goose Down
DROP POLICY IF EXISTS tenant_features_isolation ON tenant_features;
CREATE POLICY tenant_features_isolation ON tenant_features
    FOR ALL
    USING (tenant_code = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_code = current_setting('app.tenant_id', true));

DROP POLICY IF EXISTS tenants_isolation ON tenants;
CREATE POLICY tenants_isolation ON tenants
    FOR ALL
    USING (code = current_setting('app.tenant_id', true))
    WITH CHECK (code = current_setting('app.tenant_id', true));
