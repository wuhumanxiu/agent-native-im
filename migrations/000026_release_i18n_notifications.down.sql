DROP INDEX IF EXISTS notifications_release_published_once_idx;

DELETE FROM notifications
WHERE kind = 'release.published';

ALTER TABLE releases
    DROP COLUMN IF EXISTS known_issues_i18n,
    DROP COLUMN IF EXISTS required_actions_i18n,
    DROP COLUMN IF EXISTS sections_i18n,
    DROP COLUMN IF EXISTS summary_i18n,
    DROP COLUMN IF EXISTS title_i18n;
