-- +goose Up
-- +goose StatementBegin
ALTER TABLE knowledge_sources
    ADD COLUMN locale TEXT NOT NULL DEFAULT 'en'
    CHECK (locale IN ('en', 'en-GH', 'fr', 'fr-GH'));

CREATE INDEX knowledge_sources_tenant_locale_status_effective_idx
    ON knowledge_sources (tenant_id, locale, status, effective_at DESC, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS knowledge_sources_tenant_locale_status_effective_idx;
ALTER TABLE knowledge_sources DROP COLUMN IF EXISTS locale;
-- +goose StatementEnd
