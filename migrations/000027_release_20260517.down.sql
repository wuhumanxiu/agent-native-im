WITH target_release AS (
    SELECT id
    FROM releases
    WHERE public_id = '40c8b557-24cc-4e51-acb7-daf1aae92967'
)
DELETE FROM notifications
WHERE kind = 'release.published'
  AND data->>'release_id' IN (SELECT id::text FROM target_release);

DELETE FROM releases
WHERE public_id = '40c8b557-24cc-4e51-acb7-daf1aae92967';
