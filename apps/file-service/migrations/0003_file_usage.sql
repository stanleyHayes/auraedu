-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS file_usage (
    tenant_id       TEXT NOT NULL,
    date            DATE NOT NULL DEFAULT CURRENT_DATE,
    bytes_stored    BIGINT NOT NULL DEFAULT 0 CHECK (bytes_stored >= 0),
    bytes_delivered BIGINT NOT NULL DEFAULT 0 CHECK (bytes_delivered >= 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, date)
);

CREATE INDEX IF NOT EXISTS idx_file_usage_tenant_date ON file_usage (tenant_id, date DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS file_usage;

-- +goose StatementEnd
