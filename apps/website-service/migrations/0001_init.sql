-- +goose Up
-- +goose StatementBegin

-- Website Service schema (EP-19): pages and sections aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS website_pages (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    slug             TEXT NOT NULL,
    title            TEXT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    meta_description TEXT,
    layout           VARCHAR(20) NOT NULL DEFAULT 'default' CHECK (layout IN ('default', 'landing', 'contact')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at     TIMESTAMPTZ,
    UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_website_pages_tenant_slug ON website_pages (tenant_id, slug);
CREATE INDEX IF NOT EXISTS idx_website_pages_tenant_id ON website_pages (tenant_id);
CREATE INDEX IF NOT EXISTS idx_website_pages_status ON website_pages (status);
CREATE INDEX IF NOT EXISTS idx_website_pages_layout ON website_pages (layout);
CREATE INDEX IF NOT EXISTS idx_website_pages_created_at ON website_pages (created_at, id);

ALTER TABLE website_pages ENABLE ROW LEVEL SECURITY;
ALTER TABLE website_pages FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS website_pages_tenant_isolation ON website_pages;
CREATE POLICY website_pages_tenant_isolation ON website_pages
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE TABLE IF NOT EXISTS website_sections (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    page_id     UUID NOT NULL,
    type        VARCHAR(20) NOT NULL CHECK (type IN ('hero', 'text', 'features', 'gallery', 'cta', 'contact')),
    content     JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    status      VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_website_sections_page
        FOREIGN KEY (tenant_id, page_id)
        REFERENCES website_pages (tenant_id, id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_website_sections_tenant_page ON website_sections (tenant_id, page_id);
CREATE INDEX IF NOT EXISTS idx_website_sections_type ON website_sections (type);
CREATE INDEX IF NOT EXISTS idx_website_sections_status ON website_sections (status);
CREATE INDEX IF NOT EXISTS idx_website_sections_sort_order ON website_sections (sort_order, id);

ALTER TABLE website_sections ENABLE ROW LEVEL SECURITY;
ALTER TABLE website_sections FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS website_sections_tenant_isolation ON website_sections;
CREATE POLICY website_sections_tenant_isolation ON website_sections
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS website_sections_tenant_isolation ON website_sections;
DROP TABLE IF EXISTS website_sections;
DROP POLICY IF EXISTS website_pages_tenant_isolation ON website_pages;
DROP TABLE IF EXISTS website_pages;

-- +goose StatementEnd
