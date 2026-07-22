-- +goose Up
-- +goose StatementBegin
-- Tenant identity is the canonical tenant code across AuraEDU services. Earlier
-- notification migrations used UUID and could not consume events for tenants
-- such as "upshs". Drop and recreate RLS policies around the type conversion.
DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
DROP POLICY IF EXISTS notification_templates_tenant_isolation ON notification_templates;
DROP POLICY IF EXISTS notification_subscriptions_tenant_isolation ON notification_subscriptions;
DROP POLICY IF EXISTS announcements_tenant_isolation ON announcements;
DROP POLICY IF EXISTS notification_processed_events_tenant_isolation ON notification_processed_events;

ALTER TABLE messages ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE notification_templates ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE notification_subscriptions ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE announcements ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;
ALTER TABLE notification_processed_events ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::text;

CREATE POLICY messages_tenant_isolation ON messages
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY notification_templates_tenant_isolation ON notification_templates
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY notification_subscriptions_tenant_isolation ON notification_subscriptions
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY announcements_tenant_isolation ON announcements
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY notification_processed_events_tenant_isolation ON notification_processed_events
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- Irreversible by design: production tenant codes are not necessarily UUIDs.
