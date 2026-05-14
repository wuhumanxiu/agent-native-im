# Public ID Mention Assignment Upgrade Plan

Date: 2026-05-14

## Objective

Move ANI external messaging boundaries away from numeric `entity_id` usage while
adding first-class support for the product distinction between:

- people or bots mentioned in a message; and
- people or bots assigned to act on that message.

This is a compatibility upgrade, not a database primary-key migration.

## Background

ANI already has `entities.public_id` as the stable public UUID for every user,
bot, and service. Numeric `entities.id` remains the internal database key used by
joins, auth claims, participants, credentials, notifications, and WebSocket
routing.

The current external message protocol supports:

- `mentions`: legacy numeric internal entity IDs;
- `mention_public_ids`: public UUIDs that are resolved to internal IDs before
  persistence and delivery.

This solves the basic public ID migration, but it still treats every mention as
an execution signal. Product usage now requires a more precise distinction:

```text
@zhangsanfeng report your IP address to @alice. Then ask @alice to verify your
port 22 connectivity.
```

In that message both agents are mentioned, but the sender may want to explicitly
control which mentioned participants are assigned to act.

## Design Principles

1. `public_id` is the external identity. New public API, SDK, adapter, and UI
   paths must prefer public IDs.
2. Numeric IDs remain valid internal implementation details. Do not migrate
   database primary keys or joins in this phase.
3. `mention_refs` records text-level mentions and display context.
4. `assigned_public_ids` records which mentioned entities should be awakened or
   treated as task assignees.
5. `assigned_public_ids` must be a subset of the resolved `mention_refs` /
   `mention_public_ids` participants.
6. Legacy `mentions` stays temporarily supported for older clients, but it is
   deprecated for external clients.
7. The server must validate identity references against current conversation
   participants. No message may assign or mention a non-participant.
8. The server should not guess task assignment by scanning all `@...` text.
   Clients and adapters should send structured fields.

## Protocol Shape

Preferred external request shape:

```json
{
  "conversation_public_id": "conversation-public-uuid",
  "content_type": "text",
  "layers": {
    "summary": "@zhangsanfeng report your IP address to @alice."
  },
  "mention_refs": [
    {
      "public_id": "zhangsanfeng-public-uuid",
      "handle": "zhangsanfeng",
      "text": "@zhangsanfeng"
    },
    {
      "public_id": "alice-public-uuid",
      "handle": "bot_alice",
      "text": "@alice"
    }
  ],
  "assigned_public_ids": [
    "zhangsanfeng-public-uuid",
    "alice-public-uuid"
  ]
}
```

Compatibility fields:

- `conversation_id`: legacy internal conversation ID.
- `mention_public_ids`: shorthand for mentions when detailed refs are not
  needed.
- `mentions`: legacy numeric internal mention IDs.

Response shape should include:

- `conversation_public_id`
- `sender_public_id`
- `mention_refs`
- `mention_public_ids`
- `assigned_public_ids`
- legacy `mentions` only during the compatibility period.

## Mention Reference Semantics

`mention_refs` is an ordered list of structured references:

```json
{
  "public_id": "entity-public-uuid",
  "handle": "bot_alice",
  "display_name": "Alice",
  "entity_type": "bot",
  "text": "@alice"
}
```

Rules:

- `public_id` is preferred and uniquely identifies the entity.
- `handle` may be a user login name or bot handle, but is not a stable identity
  key.
- `display_name` is display-only and may be non-unique.
- `text` preserves the original message fragment selected by the UI or adapter.
- If only `handle` is provided, the server may resolve it only inside the target
  conversation participant set and only when unambiguous.

## Assignment Semantics

`assigned_public_ids` is the only assignment field in this phase.

Rules:

- If `assigned_public_ids` is omitted and the message has mentions, the server
  may default assignments to all resolved mentions for backward compatibility.
- If `assigned_public_ids` is present as an empty array, no mentioned entity is
  assigned to act.
- If `assigned_public_ids` is present and non-empty, each ID must also resolve
  to a mentioned participant.
- Bot/service delivery in group conversations should use assigned IDs first.
  Legacy messages without explicit assignments fall back to mention delivery.
- Human users still receive group messages based on normal conversation
  delivery and push behavior.

## Task List

### Backend

- Add `mention_refs` and `assigned_public_ids` to REST and WebSocket send
  payloads.
- Add `conversation_public_id` as a send-time alternative to `conversation_id`.
- Resolve `mention_refs`, `mention_public_ids`, and legacy `mentions` into
  internal IDs.
- Resolve `assigned_public_ids` into internal IDs and validate they are a subset
  of resolved mentions.
- Persist assigned internal IDs so delivery semantics survive reloads.
- Return public assignment and mention fields in message responses.
- Add tests for public ID, handle, assignment, and legacy compatibility.

### Protocol And Docs

- Update `docs/protocol/openapi.yaml`.
- Update `docs/protocol/ws-events.schema.json`.
- Update `docs/protocol/manifest.json` required public fields.
- Update `docs/api-reference.md`.
- Keep `api/openapi.yaml` removed.

### SDKs And Adapters

- Update SDK protocol snapshots after backend protocol changes.
- Add SDK send helpers for `mention_refs`, `assigned_public_ids`, and
  `conversation_public_id`.
- Adapter upgrades are follow-up unless tests show current adapter behavior is
  broken. Existing `mention_public_ids` remains valid.

## Acceptance Criteria

1. External clients can send a message using only public conversation/entity
   identifiers.
2. External clients can send `mention_refs` and receive them back in message
   responses.
3. External clients can send `assigned_public_ids`; assigned IDs are returned as
   public IDs.
4. Explicit empty `assigned_public_ids: []` means mention-only/no assignment.
5. `assigned_public_ids` cannot include an entity that is not mentioned.
6. A non-participant public ID cannot be mentioned or assigned.
7. Legacy `mention_public_ids` and numeric `mentions` still work.
8. Protocol checks cover the new public fields.
9. Backend tests cover REST and WebSocket send paths.
10. SDK tests cover the new public-id-first helper payloads.

## Non-Goals

- Replacing every database `entity_id` column.
- Removing numeric IDs from admin/debug/internal APIs.
- Building a full workflow engine or directive planner.
- Automatically parsing arbitrary text into assignments.

## Verification Commands

Backend:

```bash
make protocol-check
go test ./...
```

Python SDK:

```bash
python scripts/fetch_protocol.py
pytest
```

JavaScript SDK:

```bash
npm run protocol:fetch
npm test
```

