DROP INDEX IF EXISTS idx_push_subs_entity_provider;

ALTER TABLE push_subscriptions
    DROP CONSTRAINT IF EXISTS push_subscriptions_entity_provider_endpoint_key;

ALTER TABLE push_subscriptions
    ADD CONSTRAINT push_subscriptions_entity_id_endpoint_key
    UNIQUE (entity_id, endpoint);

ALTER TABLE push_subscriptions
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS platform,
    DROP COLUMN IF EXISTS last_success_at,
    DROP COLUMN IF EXISTS last_error_at,
    DROP COLUMN IF EXISTS last_error;
