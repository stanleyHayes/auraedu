-- +goose Up
UPDATE users SET permissions=ARRAY(SELECT DISTINCT p FROM unnest(permissions||ARRAY['admissions.application.read','admissions.application.review','admissions.offer.issue']::text[]) p) WHERE role='school_admin';
-- +goose Down
UPDATE users SET permissions=ARRAY(SELECT p FROM unnest(permissions) p WHERE p<>ALL(ARRAY['admissions.application.read','admissions.application.review','admissions.offer.issue']::text[])) WHERE role='school_admin';
