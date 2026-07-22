-- +goose Up
-- +goose StatementBegin

-- Actor→record linkage (AURA-10.12): soft link from a student/guardian record to the
-- identity-service user that owns it, so portal "me" endpoints can resolve the caller's
-- record from the JWT user_id. Nullable, additive; no cross-service FK (agent_plan §6).
-- No RLS change needed — the columns are covered by the existing *_tenant_isolation
-- table policies. The unique partial indexes guarantee at most one linked record per
-- (tenant, user) while leaving unlinked rows (user_id IS NULL) unrestricted.

ALTER TABLE students ADD COLUMN IF NOT EXISTS user_id UUID;
ALTER TABLE guardians ADD COLUMN IF NOT EXISTS user_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_students_tenant_user
    ON students (tenant_id, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_guardians_tenant_user
    ON guardians (tenant_id, user_id) WHERE user_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_guardians_tenant_user;
DROP INDEX IF EXISTS idx_students_tenant_user;
ALTER TABLE guardians DROP COLUMN IF EXISTS user_id;
ALTER TABLE students DROP COLUMN IF EXISTS user_id;

-- +goose StatementEnd
