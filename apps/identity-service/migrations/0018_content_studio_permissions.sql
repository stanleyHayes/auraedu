-- +goose Up
UPDATE users
SET permissions = ARRAY(
    SELECT DISTINCT permission
    FROM unnest(permissions || ARRAY[
        'content.generate','content.review'
    ]::text[]) AS permission
)
WHERE role = 'school_admin';

-- +goose Down
UPDATE users
SET permissions = ARRAY(
    SELECT permission
    FROM unnest(permissions) AS permission
    WHERE permission <> ALL(ARRAY[
        'content.generate','content.review'
    ]::text[])
)
WHERE role = 'school_admin';
