ALTER TABLE push_subscriptions
    ADD COLUMN provider VARCHAR(32) NOT NULL DEFAULT 'webpush',
    ADD COLUMN platform VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN last_success_at TIMESTAMPTZ NULL,
    ADD COLUMN last_error_at TIMESTAMPTZ NULL,
    ADD COLUMN last_error TEXT NOT NULL DEFAULT '';

ALTER TABLE push_subscriptions
    DROP CONSTRAINT IF EXISTS push_subscriptions_entity_id_endpoint_key;

ALTER TABLE push_subscriptions
    ADD CONSTRAINT push_subscriptions_entity_provider_endpoint_key
    UNIQUE (entity_id, provider, endpoint);

CREATE INDEX IF NOT EXISTS idx_push_subs_entity_provider
    ON push_subscriptions(entity_id, provider);
