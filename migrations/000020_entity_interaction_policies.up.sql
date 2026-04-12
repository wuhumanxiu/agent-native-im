ALTER TABLE entities
    ADD COLUMN IF NOT EXISTS friend_request_policy VARCHAR(32) NOT NULL DEFAULT 'platform_entities',
    ADD COLUMN IF NOT EXISTS direct_message_policy VARCHAR(32) NOT NULL DEFAULT 'friends_only';

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_friend_request_policy_chk;

ALTER TABLE entities
    ADD CONSTRAINT entities_friend_request_policy_chk
    CHECK (friend_request_policy IN ('nobody', 'platform_entities'));

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_direct_message_policy_chk;

ALTER TABLE entities
    ADD CONSTRAINT entities_direct_message_policy_chk
    CHECK (direct_message_policy IN ('friends_only', 'platform_entities'));

UPDATE entities
SET direct_message_policy = CASE
    WHEN allow_non_friend_chat THEN 'platform_entities'
    ELSE 'friends_only'
END
WHERE direct_message_policy IS NULL
   OR direct_message_policy = '';

UPDATE entities
SET friend_request_policy = 'platform_entities'
WHERE friend_request_policy IS NULL
   OR friend_request_policy = '';
