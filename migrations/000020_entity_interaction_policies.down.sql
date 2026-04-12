ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_friend_request_policy_chk;

ALTER TABLE entities
    DROP CONSTRAINT IF EXISTS entities_direct_message_policy_chk;

ALTER TABLE entities
    DROP COLUMN IF EXISTS friend_request_policy,
    DROP COLUMN IF EXISTS direct_message_policy;
