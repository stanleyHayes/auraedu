-- +goose Up
-- +goose StatementBegin

-- File Service schema (EP-20): file_uploads aggregate.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS file_uploads (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL,
    original_filename  TEXT NOT NULL,
    storage_path       TEXT NOT NULL,
    storage_backend    VARCHAR(20) NOT NULL DEFAULT 'local' CHECK (storage_backend IN ('local')),
    content_type       TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes         BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    checksum           TEXT NOT NULL DEFAULT '',
    owner_id           TEXT NOT NULL DEFAULT '',
    purpose            TEXT NOT NULL DEFAULT '',
    status             VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'archived', 'deleted')),
    metadata           JSONB NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_file_uploads_tenant_id ON file_uploads (tenant_id);
CREATE INDEX IF NOT EXISTS idx_file_uploads_status ON file_uploads (status);
CREATE INDEX IF NOT EXISTS idx_file_uploads_purpose ON file_uploads (purpose);
CREATE INDEX IF NOT EXISTS idx_file_uploads_created_at ON file_uploads (created_at, id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_file_uploads_path_tenant ON file_uploads (tenant_id, storage_path);

ALTER TABLE file_uploads ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_uploads FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS file_uploads_tenant_isolation ON file_uploads;
CREATE POLICY file_uploads_tenant_isolation ON file_uploads
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS file_uploads_tenant_isolation ON file_uploads;
DROP TABLE IF EXISTS file_uploads;

-- +goose StatementEnd
