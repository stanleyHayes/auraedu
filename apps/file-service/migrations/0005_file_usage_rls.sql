-- +goose Up
ALTER TABLE file_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_usage FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS file_usage_tenant_isolation ON file_usage;
CREATE POLICY file_usage_tenant_isolation ON file_usage
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- +goose Down
DROP POLICY IF EXISTS file_usage_tenant_isolation ON file_usage;
ALTER TABLE file_usage NO FORCE ROW LEVEL SECURITY;
ALTER TABLE file_usage DISABLE ROW LEVEL SECURITY;
