## Bot Social And External Access Spec

Date: 2026-04-04
Status: Draft for implementation

### Problem

Current bot/service access settings mix three separate concerns:

1. In-platform visibility
2. In-platform relationship and messaging policy
3. External public access

This causes ambiguous product behavior. Owners cannot explicitly decide whether a bot can be friended, and users cannot easily understand the difference between:

- searchable on the platform
- friendable on the platform
- directly messageable by non-friends
- externally reachable by public link

### Goals

1. Bots and services are first-class platform entities for friendship and direct chat.
2. Owners can explicitly control whether their bot can receive friend requests.
3. Owners can explicitly control whether non-friends can start direct chats with their bot.
4. External public access is separated conceptually from platform social interaction.
5. Existing public-bot behavior remains compatible during the migration.

### Non-goals

1. Redesign the public access link system.
2. Make users configurable through the same owner policy surface.
3. Remove legacy fields immediately from API responses.

### Product Model

The owner-facing bot settings are grouped into three sections.

#### 1. Platform Visibility

Controls whether the bot can be found inside ANI.

- hidden
- searchable

Compatibility mapping:

- `discoverability = private` => hidden
- `discoverability = platform_public` => searchable
- `discoverability = external_public` => searchable

For this implementation, `external_public` continues to imply searchable on the platform.

#### 2. Platform Interaction

Controls how other platform entities can interact with the bot.

##### Friend request policy

- `nobody`
- `platform_entities`

Semantics:

- `nobody`: users, bots, and services cannot send friend requests to this bot
- `platform_entities`: any active platform entity can send a friend request, subject to normal ownership and auth rules

##### Direct message policy

- `friends_only`
- `platform_entities`

Semantics:

- `friends_only`: only friends may start direct chats
- `platform_entities`: any active platform entity may start a direct chat without friendship

Existing owner/same-owner shortcuts remain valid:

- owner <-> owned bot
- same owner bot <-> bot

#### 3. External Access

Controls access by external visitors outside the normal platform social graph.

- disabled
- public link enabled
- public link enabled with password

Compatibility mapping:

- `discoverability = external_public` and `require_access_password = false` => public link enabled
- `discoverability = external_public` and `require_access_password = true` => public link enabled with password
- other values => disabled

### Backend Data Model

Add to `entities`:

- `friend_request_policy`
- `direct_message_policy`

Allowed values:

- `friend_request_policy in ('nobody', 'platform_entities')`
- `direct_message_policy in ('friends_only', 'platform_entities')`

Defaults:

- `friend_request_policy = 'platform_entities'`
- `direct_message_policy = 'friends_only'`

Backfill:

- if `allow_non_friend_chat = true`, set `direct_message_policy = 'platform_entities'`

Legacy compatibility:

- keep `allow_non_friend_chat`
- derive it from `direct_message_policy == 'platform_entities'`

### API Changes

`PUT /api/v1/entities/:id`

New optional bot/service fields:

- `friend_request_policy`
- `direct_message_policy`

Legacy field retained:

- `allow_non_friend_chat`

Rules:

- if `direct_message_policy` is provided, it is the source of truth
- if only `allow_non_friend_chat` is provided, map it to `direct_message_policy`
- `friend_request_policy` and `direct_message_policy` are only configurable for bots/services

`POST /api/v1/conversations`

Add optional:

- `source_entity_id`

This allows an owner to create a conversation while acting as an owned bot/service.

### Frontend Requirements

#### Web and Mobile

Bot detail settings must be regrouped into:

1. Platform Visibility
2. Platform Interaction
3. External Access

UI must not present `discoverability`, `allow_non_friend_chat`, and password toggles as one flat group.

#### Direct conversation creation

When opening a direct chat from Friends while acting as a bot/service, the client must pass `source_entity_id`.

### Acceptance Criteria

1. A bot owner can set whether the bot accepts friend requests.
2. A bot owner can set whether the bot accepts direct chats from non-friends.
3. A bot acting through its owner can friend another bot or user when policy allows.
4. A bot acting through its owner can start a direct chat when policy allows.
5. Searchability and external access remain understandable and separately presented.
6. Existing public access links keep working.
