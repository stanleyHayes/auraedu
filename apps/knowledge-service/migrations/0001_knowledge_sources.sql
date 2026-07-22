-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE knowledge_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('programme','admissions','fees','scholarship','calendar','policy','campus','accommodation','faq','announcement','marketing','support')),
    title TEXT NOT NULL,
    owner TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','retired')),
    confidentiality TEXT NOT NULL CHECK (confidentiality IN ('public','internal')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    effective_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    programme TEXT,
    campus TEXT,
    intake TEXT,
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    review_note TEXT,
    search_document TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(programme, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(content, '')), 'B')
    ) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(title) BETWEEN 3 AND 200),
    CHECK (length(owner) BETWEEN 2 AND 120),
    CHECK (length(content) BETWEEN 20 AND 100000),
    CHECK (expires_at IS NULL OR expires_at > effective_at),
    UNIQUE (tenant_id, id)
);

CREATE INDEX knowledge_sources_search_idx ON knowledge_sources USING GIN (search_document);
CREATE INDEX knowledge_sources_tenant_status_effective_idx ON knowledge_sources (tenant_id, status, effective_at DESC, id);

ALTER TABLE knowledge_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE knowledge_sources FORCE ROW LEVEL SECURITY;
CREATE POLICY knowledge_sources_tenant_isolation ON knowledge_sources
    USING (tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS knowledge_sources;
-- +goose StatementEnd
