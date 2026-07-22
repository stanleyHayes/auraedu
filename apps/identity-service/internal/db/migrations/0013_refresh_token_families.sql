-- AURA-4.10: transactional refresh rotation and family-wide replay revocation.
-- +goose Up
ALTER TABLE refresh_tokens ADD COLUMN family_id UUID;

-- A pre-family token starts as the sole member of its own session family.
UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL;

ALTER TABLE refresh_tokens ALTER COLUMN family_id SET NOT NULL;
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

-- +goose Down
DROP INDEX IF EXISTS refresh_tokens_family_idx;
ALTER TABLE refresh_tokens DROP COLUMN family_id;
