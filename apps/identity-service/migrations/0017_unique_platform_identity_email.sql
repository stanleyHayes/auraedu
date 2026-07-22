-- Platform identities have tenant_id NULL, so PostgreSQL's ordinary
-- UNIQUE (tenant_id, email) constraint does not prevent duplicates.
-- Refuse to make an ambiguous privileged identity authoritative and then
-- enforce one canonical platform identity per email address.
-- +goose Up

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM users
        WHERE tenant_id IS NULL
        GROUP BY LOWER(BTRIM(email::text))
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate platform identity emails must be reconciled before migration 0017';
    END IF;
END $$;

CREATE UNIQUE INDEX users_platform_email_unique_idx
    ON users (LOWER(BTRIM(email::text)))
    WHERE tenant_id IS NULL;
