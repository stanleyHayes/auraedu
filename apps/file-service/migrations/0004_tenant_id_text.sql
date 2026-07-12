-- +goose Up
-- +goose StatementBegin

-- The file service is scoped by tenant *code* (e.g. upshs), not a UUID.
-- Align the RLS policy and column type with the string tenant IDs used by
-- the gateway and web client.
DROP POLICY IF EXISTS file_uploads_tenant_isolation ON file_uploads;

ALTER TABLE file_uploads ALTER COLUMN tenant_id TYPE TEXT;

CREATE POLICY file_uploads_tenant_isolation ON file_uploads
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS file_uploads_tenant_isolation ON file_uploads;

ALTER TABLE file_uploads ALTER COLUMN tenant_id TYPE UUID;

CREATE POLICY file_uploads_tenant_isolation ON file_uploads
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd
