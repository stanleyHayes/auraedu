-- +goose Up
-- +goose StatementBegin

-- Report Service schema (EP-15): ReportTemplate and ReportCard aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS report_templates (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    name              TEXT NOT NULL,
    academic_year_id  UUID NOT NULL,
    body_template     TEXT NOT NULL,
    status            VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'active', 'archived')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_report_templates_tenant_id_id ON report_templates (tenant_id, id);

CREATE TABLE IF NOT EXISTS report_cards (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    student_id        UUID NOT NULL,
    academic_year_id  UUID NOT NULL,
    template_id       UUID NOT NULL,
    status            VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'generating', 'published', 'archived')),
    pdf_path          TEXT,
    generated_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT fk_report_cards_template
        FOREIGN KEY (tenant_id, template_id)
        REFERENCES report_templates (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_report_templates_tenant_id ON report_templates (tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_templates_academic_year_id ON report_templates (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_report_templates_status ON report_templates (status);
CREATE INDEX IF NOT EXISTS idx_report_templates_created_at ON report_templates (created_at, id);

CREATE INDEX IF NOT EXISTS idx_report_cards_tenant_id ON report_cards (tenant_id);
CREATE INDEX IF NOT EXISTS idx_report_cards_academic_year_id ON report_cards (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_report_cards_template_id ON report_cards (template_id);
CREATE INDEX IF NOT EXISTS idx_report_cards_student_id ON report_cards (student_id);
CREATE INDEX IF NOT EXISTS idx_report_cards_status ON report_cards (status);
CREATE INDEX IF NOT EXISTS idx_report_cards_created_at ON report_cards (created_at, id);

ALTER TABLE report_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_templates FORCE ROW LEVEL SECURITY;
ALTER TABLE report_cards ENABLE ROW LEVEL SECURITY;
ALTER TABLE report_cards FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS report_templates_tenant_isolation ON report_templates;
CREATE POLICY report_templates_tenant_isolation ON report_templates
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS report_cards_tenant_isolation ON report_cards;
CREATE POLICY report_cards_tenant_isolation ON report_cards
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS report_cards_tenant_isolation ON report_cards;
DROP POLICY IF EXISTS report_templates_tenant_isolation ON report_templates;
DROP TABLE IF EXISTS report_cards;
DROP TABLE IF EXISTS report_templates;

-- +goose StatementEnd
