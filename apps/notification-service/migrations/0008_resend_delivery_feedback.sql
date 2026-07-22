-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages
    ADD COLUMN provider TEXT,
    ADD COLUMN provider_message_id TEXT,
    ADD COLUMN delivery_status TEXT CHECK (delivery_status IN (
        'accepted','delivered','delayed','bounced','complained','failed','suppressed'
    )),
    ADD COLUMN delivery_status_at TIMESTAMPTZ;

CREATE UNIQUE INDEX messages_provider_message_id_idx
    ON messages (provider, provider_message_id)
    WHERE provider IS NOT NULL AND provider_message_id IS NOT NULL;

CREATE TABLE notification_delivery_events (
    id TEXT PRIMARY KEY CHECK (char_length(id) BETWEEN 1 AND 255),
    tenant_id TEXT NOT NULL,
    message_id UUID NOT NULL,
    provider TEXT NOT NULL,
    provider_message_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN (
        'accepted','delivered','delayed','bounced','complained','failed','suppressed'
    )),
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, message_id) REFERENCES messages (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX notification_delivery_events_message_idx
    ON notification_delivery_events (tenant_id, message_id, occurred_at DESC);

CREATE TABLE notification_email_suppressions (
    tenant_id TEXT NOT NULL,
    address_hash CHAR(64) NOT NULL CHECK (address_hash ~ '^[0-9a-f]{64}$'),
    reason TEXT NOT NULL CHECK (reason IN ('bounced','complained','suppressed','unsubscribed')),
    provider TEXT NOT NULL,
    first_event_id TEXT NOT NULL,
    last_event_id TEXT NOT NULL,
    suppressed_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, address_hash)
);

ALTER TABLE notification_delivery_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_delivery_events FORCE ROW LEVEL SECURITY;
ALTER TABLE notification_email_suppressions ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_email_suppressions FORCE ROW LEVEL SECURITY;

CREATE POLICY notification_delivery_events_tenant_isolation ON notification_delivery_events
    USING (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true) = 'true'
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)
        OR current_setting('app.is_platform_admin', true) = 'true'
    );

CREATE POLICY notification_email_suppressions_tenant_isolation ON notification_email_suppressions
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
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_email_suppressions;
DROP TABLE IF EXISTS notification_delivery_events;
DROP INDEX IF EXISTS messages_provider_message_id_idx;
ALTER TABLE messages
    DROP COLUMN IF EXISTS delivery_status_at,
    DROP COLUMN IF EXISTS delivery_status,
    DROP COLUMN IF EXISTS provider_message_id,
    DROP COLUMN IF EXISTS provider;
-- +goose StatementEnd
