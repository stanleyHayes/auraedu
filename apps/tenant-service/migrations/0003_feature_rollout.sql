-- +goose Up
-- +goose StatementBegin

ALTER TABLE tenant_features
    ADD COLUMN IF NOT EXISTS rollout_percentage INT,
    ADD COLUMN IF NOT EXISTS rollout_updated_by TEXT,
    ADD COLUMN IF NOT EXISTS rollout_reason TEXT,
    ADD COLUMN IF NOT EXISTS config JSONB;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tenant_features
    DROP COLUMN IF EXISTS rollout_percentage,
    DROP COLUMN IF EXISTS rollout_updated_by,
    DROP COLUMN IF EXISTS rollout_reason,
    DROP COLUMN IF EXISTS config;

-- +goose StatementEnd
