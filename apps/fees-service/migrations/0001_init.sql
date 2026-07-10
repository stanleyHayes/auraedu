-- +goose Up
-- +goose StatementBegin

-- Fees Service schema (EP-16): FeeStructure and Invoice aggregates.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS fee_structures (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    name              TEXT NOT NULL,
    academic_year_id  UUID NOT NULL,
    amount_cents      INTEGER NOT NULL CHECK (amount_cents >= 0),
    currency          VARCHAR(3) NOT NULL DEFAULT 'GHS',
    recurrence        VARCHAR(20) NOT NULL CHECK (recurrence IN ('one_time', 'termly', 'monthly', 'annually')),
    target            VARCHAR(20) NOT NULL CHECK (target IN ('all_students', 'specific_student')),
    due_day           INTEGER CHECK (due_day BETWEEN 1 AND 31),
    description       TEXT,
    status            VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_fee_structures_tenant_id_id ON fee_structures (tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_fee_structures_tenant_id ON fee_structures (tenant_id);
CREATE INDEX IF NOT EXISTS idx_fee_structures_academic_year_id ON fee_structures (academic_year_id);
CREATE INDEX IF NOT EXISTS idx_fee_structures_status ON fee_structures (status);
CREATE INDEX IF NOT EXISTS idx_fee_structures_created_at ON fee_structures (created_at, id);

CREATE TABLE IF NOT EXISTS invoices (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    student_id        UUID NOT NULL,
    fee_structure_id  UUID NOT NULL,
    amount_cents      INTEGER NOT NULL CHECK (amount_cents >= 0),
    balance_cents     INTEGER NOT NULL CHECK (balance_cents >= 0 AND balance_cents <= amount_cents),
    status            VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('draft', 'pending', 'paid', 'overdue', 'cancelled')),
    due_date          DATE,
    issued_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_invoices_fee_structure
        FOREIGN KEY (tenant_id, fee_structure_id)
        REFERENCES fee_structures (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_invoices_tenant_id ON invoices (tenant_id);
CREATE INDEX IF NOT EXISTS idx_invoices_student_id ON invoices (student_id);
CREATE INDEX IF NOT EXISTS idx_invoices_fee_structure_id ON invoices (fee_structure_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices (status);
CREATE INDEX IF NOT EXISTS idx_invoices_created_at ON invoices (created_at, id);

ALTER TABLE fee_structures ENABLE ROW LEVEL SECURITY;
ALTER TABLE fee_structures FORCE ROW LEVEL SECURITY;
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS fee_structures_tenant_isolation ON fee_structures;
CREATE POLICY fee_structures_tenant_isolation ON fee_structures
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS invoices_tenant_isolation ON invoices;
CREATE POLICY invoices_tenant_isolation ON invoices
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS invoices_tenant_isolation ON invoices;
DROP POLICY IF EXISTS fee_structures_tenant_isolation ON fee_structures;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS fee_structures;

-- +goose StatementEnd
