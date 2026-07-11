-- +goose Up
-- +goose StatementBegin

-- Billing Service schema (EP-22): SaaS plans, subscriptions, and invoices.
-- Row-Level Security is keyed on the app.tenant_id session variable (agent_plan §7).

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS billing_plans (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    code              VARCHAR(50) NOT NULL UNIQUE,
    description       TEXT,
    price_cents       INTEGER NOT NULL CHECK (price_cents >= 0),
    currency          VARCHAR(3) NOT NULL DEFAULT 'GHS',
    billing_interval  VARCHAR(20) NOT NULL CHECK (billing_interval IN ('monthly', 'yearly')),
    features          TEXT[] NOT NULL DEFAULT '{}',
    status            VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_plans_status ON billing_plans (status);
CREATE INDEX IF NOT EXISTS idx_billing_plans_created_at ON billing_plans (created_at, id);

CREATE TABLE IF NOT EXISTS billing_subscriptions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID NOT NULL,
    plan_id              UUID NOT NULL,
    status               VARCHAR(20) NOT NULL CHECK (status IN ('trialing', 'active', 'past_due', 'cancelled')),
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end   TIMESTAMPTZ NOT NULL,
    trial_ends_at        TIMESTAMPTZ,
    cancelled_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_subscriptions_plan
        FOREIGN KEY (plan_id)
        REFERENCES billing_plans (id)
);

CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_tenant_id ON billing_subscriptions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_plan_id ON billing_subscriptions (plan_id);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_status ON billing_subscriptions (status);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_created_at ON billing_subscriptions (created_at, id);

CREATE TABLE IF NOT EXISTS billing_invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    subscription_id UUID NOT NULL,
    amount_cents    INTEGER NOT NULL CHECK (amount_cents >= 0),
    status          VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'open', 'paid', 'uncollectible', 'void')),
    due_date        DATE,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_invoices_subscription
        FOREIGN KEY (subscription_id)
        REFERENCES billing_subscriptions (id)
);

CREATE INDEX IF NOT EXISTS idx_billing_invoices_tenant_id ON billing_invoices (tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_invoices_subscription_id ON billing_invoices (subscription_id);
CREATE INDEX IF NOT EXISTS idx_billing_invoices_status ON billing_invoices (status);
CREATE INDEX IF NOT EXISTS idx_billing_invoices_created_at ON billing_invoices (created_at, id);

ALTER TABLE billing_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_subscriptions FORCE ROW LEVEL SECURITY;
ALTER TABLE billing_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_invoices FORCE ROW LEVEL SECURITY;

-- Plans are global (not tenant-scoped), but still enable RLS so table owners cannot accidentally bypass it.
ALTER TABLE billing_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_plans FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS billing_plans_isolation ON billing_plans;
CREATE POLICY billing_plans_isolation ON billing_plans
    FOR ALL
    USING (true)
    WITH CHECK (true);

DROP POLICY IF EXISTS billing_subscriptions_isolation ON billing_subscriptions;
CREATE POLICY billing_subscriptions_isolation ON billing_subscriptions
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

DROP POLICY IF EXISTS billing_invoices_isolation ON billing_invoices;
CREATE POLICY billing_invoices_isolation ON billing_invoices
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP POLICY IF EXISTS billing_invoices_isolation ON billing_invoices;
DROP POLICY IF EXISTS billing_subscriptions_isolation ON billing_subscriptions;
DROP POLICY IF EXISTS billing_plans_isolation ON billing_plans;
DROP TABLE IF EXISTS billing_invoices;
DROP TABLE IF EXISTS billing_subscriptions;
DROP TABLE IF EXISTS billing_plans;

-- +goose StatementEnd
