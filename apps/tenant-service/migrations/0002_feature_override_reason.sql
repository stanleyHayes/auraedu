-- +goose Up
-- +goose StatementBegin

ALTER TABLE tenant_features ADD COLUMN IF NOT EXISTS reason TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tenant_features DROP COLUMN IF EXISTS reason;

-- +goose StatementEnd
