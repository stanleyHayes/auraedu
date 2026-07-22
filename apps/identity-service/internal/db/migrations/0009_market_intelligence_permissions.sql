-- +goose Up
UPDATE users SET permissions = ARRAY(
 SELECT DISTINCT permission FROM unnest(permissions || ARRAY[
  'intelligence.read','intelligence.manage','intelligence.review'
 ]::text[]) AS permission
) WHERE role = 'school_admin';

-- +goose Down
UPDATE users SET permissions = ARRAY(
 SELECT permission FROM unnest(permissions) AS permission
 WHERE permission <> ALL(ARRAY['intelligence.read','intelligence.manage','intelligence.review']::text[])
) WHERE role = 'school_admin';
