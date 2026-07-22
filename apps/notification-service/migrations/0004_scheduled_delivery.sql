-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_messages_due ON messages (scheduled_at, id) WHERE status = 'pending' AND scheduled_at IS NOT NULL;
DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
CREATE POLICY messages_tenant_isolation ON messages
    USING (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true) = 'true')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true) OR current_setting('app.is_platform_admin', true) = 'true');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_messages_due;
DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
CREATE POLICY messages_tenant_isolation ON messages
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd
