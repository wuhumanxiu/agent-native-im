## Bot Social And External Access Task Brief

Date: 2026-04-04
Status: Planned and in implementation

### Scope

Implement bot-first friendship and direct-chat policy controls while clarifying external public access settings.

### Workstreams

#### Backend

1. Add `friend_request_policy` and `direct_message_policy` to `entities`
2. Backfill policy values from existing rows
3. Enforce `friend_request_policy` in `POST /friends/requests`
4. Enforce `direct_message_policy` in direct conversation authorization
5. Add `source_entity_id` support to `POST /conversations`
6. Preserve backward compatibility for `allow_non_friend_chat`

#### Web

1. Update entity types and API request types
2. Regroup bot settings into:
   - Platform Visibility
   - Platform Interaction
   - External Access
3. Use `source_entity_id` when creating direct chats while acting as a bot

#### Mobile

1. Update entity types and API request types
2. Regroup bot settings using the same section model
3. Use `source_entity_id` when creating direct chats while acting as a bot

#### Docs

1. Update product baseline links
2. Update API reference
3. Update user stories and test cases for backend, web, and mobile

#### Tests

1. Backend:
   - friend request blocked when policy is `nobody`
   - non-friend direct chat allowed only when `direct_message_policy = platform_entities`
   - direct conversation can be created while acting as an owned bot
2. Web:
   - policy mapping coverage
   - direct conversation request includes `source_entity_id` when acting as a bot
3. Mobile:
   - policy mapping coverage
   - direct conversation request includes `source_entity_id` when acting as a bot
