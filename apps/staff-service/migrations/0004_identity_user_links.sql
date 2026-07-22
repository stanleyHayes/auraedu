-- +goose Up
-- +goose StatementBegin

ALTER TABLE staff ADD COLUMN IF NOT EXISTS user_id UUID;
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_user_tenant
    ON staff (tenant_id, user_id) WHERE user_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_staff_user_tenant;
ALTER TABLE staff DROP COLUMN IF EXISTS user_id;

-- +goose StatementEnd
