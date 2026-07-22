-- +goose Up
-- +goose StatementBegin
CREATE TABLE admissions_programmes (
 id UUID PRIMARY KEY,
 tenant_id TEXT NOT NULL,
 code TEXT NOT NULL,
 name TEXT NOT NULL,
 slug TEXT NOT NULL,
 summary TEXT NOT NULL,
 description TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN('draft','published','archived')),
 version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,code),
 UNIQUE(tenant_id,slug)
);

CREATE TABLE admissions_intakes (
 id UUID PRIMARY KEY,
 tenant_id TEXT NOT NULL,
 programme_id UUID NOT NULL,
 name TEXT NOT NULL,
 starts_at TIMESTAMPTZ NOT NULL,
 application_opens_at TIMESTAMPTZ NOT NULL,
 application_closes_at TIMESTAMPTZ NOT NULL,
 capacity INTEGER CHECK(capacity > 0),
 status TEXT NOT NULL CHECK(status IN('draft','open','closed','archived')),
 version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,programme_id,name,starts_at),
 FOREIGN KEY(tenant_id,programme_id) REFERENCES admissions_programmes(tenant_id,id) ON DELETE CASCADE,
 CHECK(application_opens_at < application_closes_at),
 CHECK(application_closes_at <= starts_at)
);

CREATE INDEX admissions_programmes_public ON admissions_programmes(tenant_id,status,name,id);
CREATE INDEX admissions_intakes_availability ON admissions_intakes(tenant_id,programme_id,status,application_opens_at,application_closes_at);

ALTER TABLE admissions_programmes ENABLE ROW LEVEL SECURITY;
ALTER TABLE admissions_programmes FORCE ROW LEVEL SECURITY;
ALTER TABLE admissions_intakes ENABLE ROW LEVEL SECURITY;
ALTER TABLE admissions_intakes FORCE ROW LEVEL SECURITY;
CREATE POLICY admissions_programmes_tenant ON admissions_programmes USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY admissions_intakes_tenant ON admissions_intakes USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS admissions_intakes;
DROP TABLE IF EXISTS admissions_programmes;
-- +goose StatementEnd
