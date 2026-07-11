-- +goose Up
-- +goose StatementBegin

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS locale TEXT,
    ADD COLUMN IF NOT EXISTS timezone TEXT,
    ADD COLUMN IF NOT EXISTS date_format TEXT,
    ADD COLUMN IF NOT EXISTS academic_year_start_month INT,
    ADD COLUMN IF NOT EXISTS primary_contact_email TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE tenants
    DROP COLUMN IF EXISTS locale,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS date_format,
    DROP COLUMN IF EXISTS academic_year_start_month,
    DROP COLUMN IF EXISTS primary_contact_email;

-- +goose StatementEnd
