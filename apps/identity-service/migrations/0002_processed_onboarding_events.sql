-- +goose Up
CREATE TABLE IF NOT EXISTS identity_processed_events (
    event_id     TEXT PRIMARY KEY,
    event_type   TEXT NOT NULL,
    tenant_id    VARCHAR(50) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS identity_processed_events;
