CREATE TABLE IF NOT EXISTS releases (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    version VARCHAR(64) NOT NULL,
    component VARCHAR(64) NOT NULL,
    platform VARCHAR(64) NOT NULL DEFAULT 'all',
    channel VARCHAR(64) NOT NULL DEFAULT 'production',
    title VARCHAR(200) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    sections JSONB NOT NULL DEFAULT '[]'::jsonb,
    required_actions JSONB NOT NULL DEFAULT '[]'::jsonb,
    known_issues JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (component IN ('server', 'web', 'ios', 'android', 'openclaw_plugin', 'hermes_adapter', 'zebra_adapter', 'sdk', 'platform')),
    CHECK (platform IN ('all', 'web', 'ios', 'android', 'desktop', 'agent')),
    CHECK (channel IN ('production', 'preview', 'development'))
);

CREATE INDEX IF NOT EXISTS idx_releases_component_published
    ON releases(component, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_releases_channel_published
    ON releases(channel, published_at DESC);

CREATE TABLE IF NOT EXISTS release_reads (
    entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    release_id BIGINT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entity_id, release_id)
);

CREATE INDEX IF NOT EXISTS idx_release_reads_entity
    ON release_reads(entity_id, read_at DESC);

CREATE TABLE IF NOT EXISTS feedback_release_links (
    feedback_id BIGINT NOT NULL REFERENCES feedback_items(id) ON DELETE CASCADE,
    release_id BIGINT NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    link_type VARCHAR(32) NOT NULL DEFAULT 'related',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (feedback_id, release_id, link_type),
    CHECK (link_type IN ('related', 'fixed', 'known_issue'))
);

CREATE INDEX IF NOT EXISTS idx_feedback_release_links_release
    ON feedback_release_links(release_id, link_type);

INSERT INTO releases (
    public_id,
    version,
    component,
    platform,
    channel,
    title,
    summary,
    sections,
    required_actions,
    known_issues,
    published_at
) VALUES (
    'a4cd8c77-e63e-4b3e-9379-9f9e7ef18c78',
    '2026.5.14',
    'platform',
    'all',
    'production',
    'Structured mention assignment and agent adapter updates',
    'This release introduces explicit mention context and assignment semantics across ANI, Web, SDKs, and agent adapters.',
    '[
      {"kind":"new","title":"Structured mention assignment","items":["Messages can now keep visible mention context separate from the agents that should act.","Web mention chips can be set to assigned or notify-only."]},
      {"kind":"improved","title":"Agent adapters","items":["OpenClaw, Zebra, and Hermes adapters now forward public mention assignment metadata when available.","Python and JavaScript SDKs support public conversation sends and assignment fields."]},
      {"kind":"fixed","title":"Production rollout","items":["Production backend and Web were deployed with the new message fields.","iOS Expo push credentials were repaired after deployment."]}
    ]'::jsonb,
    '[
      {"component":"openclaw_plugin","title":"Upgrade OpenClaw ANI plugin","body":"Run the standard OpenClaw ANI installer update and restart the gateway."},
      {"component":"hermes_adapter","title":"Upgrade Hermes ANI adapter","body":"Update your hermes-ani-adapter checkout, reinstall it into your Hermes agent checkout, and restart the Hermes gateway."},
      {"component":"zebra_adapter","title":"Upgrade Zebra ANI adapter","body":"Upgrade Zebra to a build that includes ANI adapter 1.3.0 and restart zebra-gateway."}
    ]'::jsonb,
    '[]'::jsonb,
    '2026-05-14T02:32:00Z'
) ON CONFLICT (public_id) DO NOTHING;
