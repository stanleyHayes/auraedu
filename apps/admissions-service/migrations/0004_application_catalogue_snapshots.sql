-- +goose Up
ALTER TABLE applications ADD COLUMN programme_name TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN intake_name TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE applications DROP COLUMN IF EXISTS intake_name;
ALTER TABLE applications DROP COLUMN IF EXISTS programme_name;
