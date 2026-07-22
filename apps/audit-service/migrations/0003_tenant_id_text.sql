-- +goose Up
-- +goose StatementBegin
-- Audit must accept every canonical CloudEvent tenant_id, including tenant codes
-- such as "upshs". Preserve the platform-admin read path while converting.
DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
ALTER TABLE audit_logs ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true) = 'true'
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true) = 'true'
    );
-- +goose StatementEnd

-- +goose Down
-- Irreversible by design: production tenant codes are not necessarily UUIDs.
