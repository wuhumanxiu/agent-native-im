# ANI Messaging Consistency Draft

Last updated: 2026-04-04

This draft defines the next-step consistency contract across ANI backend, web/PWA, and mobile for unread state, presence, inbox, and direct/group navigation.

## Goals

- Reduce cases where unread badges, conversation lists, and inbox cards disagree
- Stop showing `offline` when presence is actually unknown or stale
- Keep desktop and compact/mobile layouts different where needed, but keep the underlying product truth aligned

## Non-Goals

- Reworking the desktop-vs-compact navigation split itself
- Replacing WebSocket presence with a stronger server push protocol
- Introducing a brand-new notification model

## Product Rules

### 1. Inbox snapshot is the consistency anchor

`GET /api/v1/inbox/snapshot` is the preferred read model for:

- tracked entity identities for the active actor
- pending friend requests
- unread notifications
- sidebar/tab badge hydration

The snapshot response should include:

- `tracked_entity_ids`
- `acting_entities`
- `pending_friend_requests`
- `notifications`
- `generated_at`
- `summary`

Current summary fields:

- `tracked_entity_count`
- `pending_friend_request_count`
- `notification_unread_count`
- `notification_total_count`

### 2. Presence is tri-state, not boolean-only

Presence displayed in ANI clients must be interpreted as:

- `online`
- `offline`
- `unknown`

`unknown` means the client does not currently have fresh enough presence data to claim online or offline.

Rules:

- Do not silently map `unknown` to `offline`
- Direct-chat headers must show `Unknown` when presence has not been refreshed
- Avatar status dots may be hidden when presence is `unknown`
- Presence values from explicit fetches may overwrite stale local state

### 3. Desktop and compact layout may differ, but must not disagree

Allowed:

- desktop splits `Direct` and `Groups`
- compact/mobile unify both under `Chats`

Not allowed:

- unread badges implying activity that cannot be found from the next navigation step
- friend and inbox counts drifting from the snapshot model
- direct-chat header status implying the peer is offline when presence is only unknown

### 4. Friends and direct chat must stay consistent

When a user opens or creates a direct conversation from Friends:

- existing direct conversations should be reused
- new direct conversations should open immediately
- the resulting header should resolve the peer presence as `online`, `offline`, or `unknown`

### 5. Notification channels should not duplicate user intent

ANI should coordinate:

- WebSocket live state
- inbox cards and badges
- push notifications

Expected policy:

- foreground live session should rely primarily on in-app state
- background/offline session may rely on push
- duplicate badge inflation across surfaces should be avoided

## Surface Implications

### Backend

- `inbox/snapshot` remains the read-model endpoint for notification/friend badge hydration
- presence APIs should support batch refresh without forcing clients to interpret missing data as offline

### Web / PWA

- desktop direct headers should render `Unknown` when presence is stale
- friends and bot surfaces should use tri-state presence semantics
- sidebar badges should be explainable from inbox snapshot + conversation unread state

### Mobile

- compact direct headers should render `Unknown` when presence is stale
- friends and bot surfaces should use tri-state presence semantics
- compact tabs should use inbox snapshot for Friends/Inbox badges and conversation unread state for Chats

## Validation Focus

Priority validation after each consistency change:

1. inbox snapshot payload shape and summary values
2. direct-chat header presence in desktop and mobile
3. friends list and bot list status display when presence fetch succeeds
4. friends list and bot list behavior when presence fetch fails
5. unread badge consistency after notification and friend-request actions
