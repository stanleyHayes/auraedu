-- +goose Up
-- +goose StatementBegin

-- Add secure_url for Cloudinary delivery URLs.
ALTER TABLE file_uploads ADD COLUMN IF NOT EXISTS secure_url TEXT;

-- Allow Cloudinary-backed uploads.
ALTER TABLE file_uploads DROP CONSTRAINT IF EXISTS file_uploads_storage_backend_check;
ALTER TABLE file_uploads ADD CONSTRAINT file_uploads_storage_backend_check CHECK (storage_backend IN ('local', 'cloudinary'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE file_uploads DROP COLUMN IF EXISTS secure_url;
ALTER TABLE file_uploads DROP CONSTRAINT IF EXISTS file_uploads_storage_backend_check;
ALTER TABLE file_uploads ADD CONSTRAINT file_uploads_storage_backend_check CHECK (storage_backend IN ('local'));

-- +goose StatementEnd
