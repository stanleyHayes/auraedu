-- +goose Up
ALTER TABLE users FORCE ROW LEVEL SECURITY;
ALTER TABLE credentials FORCE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens FORCE ROW LEVEL SECURITY;
ALTER TABLE password_resets FORCE ROW LEVEL SECURITY;
ALTER TABLE invites FORCE ROW LEVEL SECURITY;

ALTER TABLE identity_processed_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_processed_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS identity_processed_events_isolation ON identity_processed_events;
CREATE POLICY identity_processed_events_isolation ON identity_processed_events
    USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true)::boolean = true
    );

-- +goose Down
DROP POLICY IF EXISTS identity_processed_events_isolation ON identity_processed_events;
ALTER TABLE identity_processed_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE identity_processed_events DISABLE ROW LEVEL SECURITY;
ALTER TABLE invites NO FORCE ROW LEVEL SECURITY;
ALTER TABLE password_resets NO FORCE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE credentials NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
