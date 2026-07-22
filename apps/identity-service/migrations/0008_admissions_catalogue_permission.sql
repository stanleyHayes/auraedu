-- +goose Up
UPDATE users SET permissions=ARRAY(SELECT DISTINCT p FROM unnest(permissions||ARRAY['admissions.catalogue.manage']::text[]) p) WHERE role='school_admin';
-- +goose Down
UPDATE users SET permissions=ARRAY(SELECT p FROM unnest(permissions) p WHERE p<>'admissions.catalogue.manage') WHERE role='school_admin';
