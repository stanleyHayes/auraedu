-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE TABLE applications (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, applicant_user_id TEXT NOT NULL, lead_id UUID,
 programme_id UUID NOT NULL, intake_id UUID NOT NULL, legal_name TEXT NOT NULL DEFAULT '',
 email TEXT NOT NULL DEFAULT '', phone TEXT NOT NULL DEFAULT '', answers JSONB NOT NULL DEFAULT '{}',
 status TEXT NOT NULL CHECK(status IN('draft','submitted','admitted','rejected','withdrawn')),
 completion_percentage INTEGER NOT NULL DEFAULT 0 CHECK(completion_percentage BETWEEN 0 AND 100),
 missing_requirements TEXT[] NOT NULL DEFAULT '{}', submitted_at TIMESTAMPTZ,
 reviewed_by TEXT, reviewed_at TIMESTAMPTZ, review_note TEXT,
 offer_status TEXT NOT NULL CHECK(offer_status IN('none','issued','accepted','declined','expired')),
 offer_conditions TEXT, offer_expires_at TIMESTAMPTZ, offer_issued_by TEXT, offer_accepted_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(tenant_id,applicant_user_id,programme_id,intake_id), UNIQUE(tenant_id,id)
);
CREATE TABLE application_documents (
 id UUID PRIMARY KEY, tenant_id TEXT NOT NULL, application_id UUID NOT NULL,
 file_id UUID NOT NULL, document_type TEXT NOT NULL, file_name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL,
 UNIQUE(tenant_id,application_id,file_id),
 FOREIGN KEY(tenant_id,application_id) REFERENCES applications(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX applications_tenant_status ON applications(tenant_id,status,created_at DESC,id);
CREATE INDEX applications_applicant ON applications(tenant_id,applicant_user_id,created_at DESC);
ALTER TABLE applications ENABLE ROW LEVEL SECURITY; ALTER TABLE applications FORCE ROW LEVEL SECURITY;
ALTER TABLE application_documents ENABLE ROW LEVEL SECURITY; ALTER TABLE application_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY applications_tenant ON applications USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY application_documents_tenant ON application_documents USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS application_documents; DROP TABLE IF EXISTS applications;
-- +goose StatementEnd
