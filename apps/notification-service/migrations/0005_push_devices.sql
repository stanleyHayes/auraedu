-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_channel_check;
ALTER TABLE messages ADD CONSTRAINT messages_channel_check CHECK (channel IN ('email','sms','whatsapp','in_app','push'));
ALTER TABLE notification_templates DROP CONSTRAINT IF EXISTS notification_templates_channel_check;
ALTER TABLE notification_templates ADD CONSTRAINT notification_templates_channel_check CHECK (channel IN ('email','sms','whatsapp','in_app','push'));
ALTER TABLE notification_subscriptions DROP CONSTRAINT IF EXISTS notification_subscriptions_channel_check;
ALTER TABLE notification_subscriptions ADD CONSTRAINT notification_subscriptions_channel_check CHECK (channel IN ('email','sms','whatsapp','in_app','push'));

CREATE TABLE device_push_tokens (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  user_id UUID NOT NULL,
  device_id TEXT NOT NULL,
  platform TEXT NOT NULL CHECK (platform IN ('ios','android')),
  token TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','invalid')),
  last_seen_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id,user_id,device_id),
  UNIQUE (token)
);
CREATE INDEX idx_device_push_tokens_user ON device_push_tokens(tenant_id,user_id) WHERE status='active';
ALTER TABLE device_push_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE device_push_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY device_push_tokens_tenant_isolation ON device_push_tokens USING (tenant_id=current_setting('app.tenant_id',true)) WITH CHECK (tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS device_push_tokens;
-- +goose StatementEnd
