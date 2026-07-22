-- +goose Up
-- +goose StatementBegin
CREATE TABLE communication_journeys (
    id UUID NOT NULL,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 3 AND 160),
    trigger_event TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','paused','archived')),
    timezone TEXT NOT NULL,
    quiet_hours_start_minute INTEGER CHECK (quiet_hours_start_minute BETWEEN 0 AND 1439),
    quiet_hours_end_minute INTEGER CHECK (quiet_hours_end_minute BETWEEN 0 AND 1439),
    frequency_window_hours INTEGER NOT NULL CHECK (frequency_window_hours BETWEEN 1 AND 720),
    frequency_limit INTEGER NOT NULL CHECK (frequency_limit BETWEEN 1 AND 100),
    cancel_on_events TEXT[] NOT NULL DEFAULT '{}',
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by UUID NOT NULL,
    activated_by UUID,
    activated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id),
    UNIQUE (tenant_id, id),
    CHECK ((quiet_hours_start_minute IS NULL) = (quiet_hours_end_minute IS NULL)),
    CHECK (quiet_hours_start_minute IS NULL OR quiet_hours_start_minute <> quiet_hours_end_minute)
);

CREATE INDEX communication_journeys_trigger_idx
    ON communication_journeys (tenant_id, trigger_event, status);

CREATE TABLE communication_journey_steps (
    id UUID NOT NULL,
    tenant_id TEXT NOT NULL,
    journey_id UUID NOT NULL,
    position INTEGER NOT NULL CHECK (position BETWEEN 1 AND 10),
    channel TEXT NOT NULL CHECK (channel IN ('email','sms','whatsapp','in_app')),
    template_id UUID NOT NULL,
    delay_minutes INTEGER NOT NULL CHECK (delay_minutes BETWEEN 0 AND 129600),
    condition_operator TEXT NOT NULL CHECK (condition_operator IN ('always','equals','not_equals')),
    condition_field TEXT NOT NULL DEFAULT '',
    condition_value TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, journey_id, position),
    FOREIGN KEY (tenant_id, journey_id) REFERENCES communication_journeys (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, template_id) REFERENCES notification_templates (tenant_id, id),
    CHECK (
        (condition_operator = 'always' AND condition_field = '' AND condition_value = '')
        OR (condition_operator <> 'always' AND condition_field <> '' AND condition_value <> '')
    )
);

CREATE TABLE communication_journey_enrollments (
    id UUID NOT NULL,
    tenant_id TEXT NOT NULL,
    journey_id UUID NOT NULL,
    journey_version INTEGER NOT NULL CHECK (journey_version > 0),
    event_id TEXT NOT NULL CHECK (char_length(event_id) BETWEEN 1 AND 255),
    trigger_event TEXT NOT NULL,
    lead_id UUID NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','completed','cancelled')),
    skipped_steps INTEGER NOT NULL DEFAULT 0 CHECK (skipped_steps BETWEEN 0 AND 10),
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at TIMESTAMPTZ,
    cancellation_event TEXT,
    PRIMARY KEY (id),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, journey_id, event_id),
    FOREIGN KEY (tenant_id, journey_id) REFERENCES communication_journeys (tenant_id, id)
);

CREATE INDEX communication_journey_enrollments_lead_idx
    ON communication_journey_enrollments (tenant_id, lead_id, status);

CREATE INDEX messages_journey_frequency_idx
    ON messages (tenant_id, recipient_id, sent_at)
    WHERE status = 'sent' AND metadata ? 'journey_id';

ALTER TABLE notification_outbox DROP CONSTRAINT IF EXISTS notification_outbox_event_type_check;
ALTER TABLE notification_outbox ADD CONSTRAINT notification_outbox_event_type_check
    CHECK (event_type IN ('notification.sent.v1', 'notification.failed.v1', 'communication.journey_changed.v1'));

ALTER TABLE communication_journeys ENABLE ROW LEVEL SECURITY;
ALTER TABLE communication_journeys FORCE ROW LEVEL SECURITY;
ALTER TABLE communication_journey_steps ENABLE ROW LEVEL SECURITY;
ALTER TABLE communication_journey_steps FORCE ROW LEVEL SECURITY;
ALTER TABLE communication_journey_enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE communication_journey_enrollments FORCE ROW LEVEL SECURITY;

CREATE POLICY communication_journeys_tenant_isolation ON communication_journeys
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY communication_journey_steps_tenant_isolation ON communication_journey_steps
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
CREATE POLICY communication_journey_enrollments_tenant_isolation ON communication_journey_enrollments
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS messages_journey_frequency_idx;
ALTER TABLE notification_outbox DROP CONSTRAINT IF EXISTS notification_outbox_event_type_check;
ALTER TABLE notification_outbox ADD CONSTRAINT notification_outbox_event_type_check
    CHECK (event_type IN ('notification.sent.v1', 'notification.failed.v1'));
DROP TABLE IF EXISTS communication_journey_enrollments;
DROP TABLE IF EXISTS communication_journey_steps;
DROP TABLE IF EXISTS communication_journeys;
-- +goose StatementEnd
