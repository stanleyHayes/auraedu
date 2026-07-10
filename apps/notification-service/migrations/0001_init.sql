-- +goose Up
-- +goose StatementBegin

-- Notification Service schema (EP-18): Message, Template and Subscription aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS messages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    recipient_id UUID NOT NULL,
    channel      VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'sms', 'whatsapp', 'in_app')),
    template_id  UUID,
    subject      TEXT NOT NULL,
    body         TEXT NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'cancelled')),
    metadata     JSONB NOT NULL DEFAULT '{}'::jsonb,
    scheduled_at TIMESTAMPTZ,
    sent_at      TIMESTAMPTZ,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_tenant_id_id ON messages (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_messages_tenant_id ON messages (tenant_id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient_id ON messages (recipient_id);
CREATE INDEX IF NOT EXISTS idx_messages_channel ON messages (channel);
CREATE INDEX IF NOT EXISTS idx_messages_status ON messages (status);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages (created_at, id);

CREATE TABLE IF NOT EXISTS notification_templates (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    name             TEXT NOT NULL,
    channel          VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'sms', 'whatsapp', 'in_app')),
    subject_template TEXT NOT NULL,
    body_template    TEXT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_templates_tenant_id_id ON notification_templates (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_notification_templates_tenant_id ON notification_templates (tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_templates_channel ON notification_templates (channel);
CREATE INDEX IF NOT EXISTS idx_notification_templates_status ON notification_templates (status);
CREATE INDEX IF NOT EXISTS idx_notification_templates_created_at ON notification_templates (created_at, id);

CREATE TABLE IF NOT EXISTS notification_subscriptions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL,
    user_id    UUID NOT NULL,
    channel    VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'sms', 'whatsapp', 'in_app')),
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_notification_subscriptions_tenant_user_channel
        UNIQUE (tenant_id, user_id, channel)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_subscriptions_tenant_id_id ON notification_subscriptions (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_tenant_id ON notification_subscriptions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_user_id ON notification_subscriptions (user_id);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_channel ON notification_subscriptions (channel);
CREATE INDEX IF NOT EXISTS idx_notification_subscriptions_created_at ON notification_subscriptions (created_at, id);

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages FORCE ROW LEVEL SECURITY;
ALTER TABLE notification_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_templates FORCE ROW LEVEL SECURITY;
ALTER TABLE notification_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_subscriptions FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
CREATE POLICY messages_tenant_isolation ON messages
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS notification_templates_tenant_isolation ON notification_templates;
CREATE POLICY notification_templates_tenant_isolation ON notification_templates
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS notification_subscriptions_tenant_isolation ON notification_subscriptions;
CREATE POLICY notification_subscriptions_tenant_isolation ON notification_subscriptions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS notification_subscriptions_tenant_isolation ON notification_subscriptions;
DROP POLICY IF EXISTS notification_templates_tenant_isolation ON notification_templates;
DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
DROP TABLE IF EXISTS notification_subscriptions;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS messages;

-- +goose StatementEnd
