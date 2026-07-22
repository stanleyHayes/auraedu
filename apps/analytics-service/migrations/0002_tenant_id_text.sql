-- +goose Up
-- +goose StatementBegin
-- Canonical AuraEDU tenant identities are tenant codes, not UUIDs. This lets
-- analytics consume the same tenant_id carried by CloudEvents and HTTP context.
DROP POLICY IF EXISTS metrics_tenant_isolation ON metrics;
ALTER TABLE metrics ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
CREATE POLICY metrics_tenant_isolation ON metrics
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- Irreversible by design: production tenant codes are not necessarily UUIDs.
