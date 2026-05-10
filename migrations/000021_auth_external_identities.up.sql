CREATE TABLE IF NOT EXISTS auth_external_identities (
    id BIGSERIAL PRIMARY KEY,
    entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    upstream_provider VARCHAR(32) NOT NULL DEFAULT '',
    upstream_subject VARCHAR(255) NOT NULL DEFAULT '',
    site_id VARCHAR(255) NOT NULL DEFAULT '',
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url VARCHAR(1024) NOT NULL DEFAULT '',
    raw_profile JSONB NOT NULL DEFAULT '{}'::jsonb,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_subject)
);

CREATE INDEX IF NOT EXISTS idx_auth_external_identities_entity
    ON auth_external_identities(entity_id);

CREATE INDEX IF NOT EXISTS idx_auth_external_identities_upstream
    ON auth_external_identities(provider, upstream_provider, upstream_subject);

INSERT INTO auth_external_identities (
    entity_id,
    provider,
    provider_subject,
    upstream_provider,
    upstream_subject,
    site_id,
    display_name,
    avatar_url,
    raw_profile,
    linked_at,
    last_used_at
)
SELECT
    id,
    '1pass',
    'site:' || COALESCE(metadata->'onepass'->>'site_id', '') || ':openid:' || COALESCE(metadata->'onepass'->>'openid', ''),
    'wechat',
    COALESCE(NULLIF(metadata->'onepass'->>'unionid', ''), metadata->'onepass'->>'openid', ''),
    COALESCE(metadata->'onepass'->>'site_id', ''),
    display_name,
    avatar_url,
    COALESCE(metadata->'onepass', '{}'::jsonb),
    created_at,
    NOW()
FROM entities
WHERE entity_type = 'user'
  AND metadata->>'auth_provider' = '1pass'
  AND metadata->'onepass'->>'openid' IS NOT NULL
ON CONFLICT (provider, provider_subject) DO NOTHING;
