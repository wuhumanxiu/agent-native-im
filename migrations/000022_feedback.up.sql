CREATE TABLE IF NOT EXISTS feedback_items (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) NOT NULL UNIQUE,
    submitter_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    feedback_type VARCHAR(32) NOT NULL DEFAULT 'other',
    severity VARCHAR(16) NOT NULL DEFAULT 'normal',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    status VARCHAR(24) NOT NULL DEFAULT 'open',
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    contact VARCHAR(255) NOT NULL DEFAULT '',
    attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_comment_at TIMESTAMPTZ,
    CHECK (feedback_type IN ('bug', 'feature', 'question', 'account', 'other')),
    CHECK (severity IN ('low', 'normal', 'high', 'urgent')),
    CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    CHECK (status IN ('open', 'triaged', 'planned', 'in_progress', 'resolved', 'closed'))
);

CREATE INDEX IF NOT EXISTS idx_feedback_items_submitter
    ON feedback_items(submitter_entity_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_feedback_items_status
    ON feedback_items(status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_feedback_items_type
    ON feedback_items(feedback_type, updated_at DESC);

CREATE TABLE IF NOT EXISTS feedback_comments (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES feedback_items(id) ON DELETE CASCADE,
    author_entity_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    visibility VARCHAR(16) NOT NULL DEFAULT 'public',
    attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (visibility IN ('public', 'internal'))
);

CREATE INDEX IF NOT EXISTS idx_feedback_comments_feedback
    ON feedback_comments(feedback_id, created_at ASC);
