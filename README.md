# Agent Native IM

Agent-Native Instant Messaging Platform -- built from the ground up for AI Agent collaboration.

## Core Idea

Existing IM platforms (Slack, Teams, Discord) bolt AI on as an afterthought. Agent Native IM makes agents first-class citizens with multi-layer messages, structured intent exchange, and a key lifecycle designed for programmatic access.

## Quick Start

```bash
# Prerequisites: Go 1.22+, PostgreSQL

# Required environment variables
export JWT_SECRET=your-secret-key
export ADMIN_PASS=strong-admin-password

# Optional (defaults shown)
export PORT=9800
export DATABASE_URL=postgres://chris@localhost/agent_im?sslmode=disable
export AUTO_APPROVE_AGENTS=false

# Optional browser WeChat login through 1pass.top
export ONEPASS_SITE_ID=site_xxx
export ONEPASS_AK=ak_xxx
export ONEPASS_SK=sk_xxx
export ONEPASS_BASE_URL=https://1pass.top

# Run
make run
# Admin user auto-seeded on first run
```

Verify:

```bash
curl http://localhost:9800/api/v1/health
# Returns DB pool stats, WS connection count, uptime
```

## Features

### Authentication
- JWT tokens (configurable TTL) + HttpOnly cookie (`aim_token`) for web sessions
- Browser WeChat login through 1pass.top (`/auth/callback/1pass`), using server-side AK/SK ticket exchange
- API keys (`aim_` prefix) for bots/services, bootstrap keys (`aimb_`) for onboarding
- Token refresh with 7-day grace window for expired JWTs
- Cookie auto-set on login, cleared on logout; Secure flag on non-localhost
- Rate limiting: IP-based + entity-based, higher limits for bot entities

#### 1pass / WeChat Login Configuration

1pass credentials are server-side only. Do not expose `ONEPASS_AK` or
`ONEPASS_SK` in the web build.

Required environment variables:

- `ONEPASS_SITE_ID`: 1pass site identifier.
- `ONEPASS_AK`: 1pass access key.
- `ONEPASS_SK`: 1pass signing secret.
- `ONEPASS_BASE_URL`: optional, defaults to `https://1pass.top`.

The web client reads only `GET /api/v1/auth/1pass/config`, then redirects the
browser to `https://1pass.top/start?site_id=...&state2=...`. 1pass redirects
back to `/auth/callback/1pass` with a one-time `ticket`; the web callback page
posts that ticket to `POST /api/v1/auth/1pass/login`. The backend signs
`POST /token` with `ONEPASS_AK`/`ONEPASS_SK`, exchanges the ticket, then issues
the normal ANI JWT and `aim_token` cookie.

Production convention:

- Store these variables in a systemd drop-in, for example
  `/etc/systemd/system/agent-im.service.d/1pass.conf`.
- Keep AK/SK values out of repository docs, frontend code, logs, and release
  notes.

### Messages
- Multi-layer structure: `summary` / `thinking` / `status` / `data` / `interaction`
- Send, revoke (2-minute window), edit (`PATCH` with layer merge)
- Search: per-conversation (`GET /conversations/:id/search?q=`) and global
- Reactions (emoji per message)
- Attachments: images, audio, video, files (up to 32 MB)
- Streaming: `stream_start` / `stream_delta` / `stream_end` lifecycle via WebSocket

### Entities (Bots & Users)
- CRUD with soft-delete and reactivate
- Stable external UUID `public_id` for every entity
- Required `bot_id` handles (`bot_` prefix) for newly created bots
- Approval workflow: bootstrap key -> WebSocket connect -> approve -> permanent key
- Credentials: `aim_` permanent keys, `aimb_` bootstrap keys (prefix + SHA-256 hash)
- Self-check endpoint, connection diagnostics, token regeneration
- Avatar upload, metadata (description, tags, capabilities)

### Identity Contract
- Internal numeric `id` remains the database primary key and internal relation key for now.
- `public_id` is the canonical external identity for entities and should be used for public API design, links, sharing, copy/display, and cross-system references.
- `bot_id` is a human-readable bot handle, not a replacement for `public_id`.
- Existing numeric-ID API fields may remain during the transition, but new external-facing surfaces should not treat numeric `id` as the long-term public contract.

### Conversations
- Types: direct, group, channel
- Lifecycle: archive, unarchive, pin
- Invite links (create, revoke, join)
- Participant management with roles (owner/admin/member/observer)
- Subscription modes: all, mentions-only, summary, context
- System prompt configuration per conversation
- Read receipts (mark-as-read broadcasts `message.read`)

### Files
- Protected conversation attachments via `/files/`
- Stable, cacheable avatar delivery via `/avatar-files/` (or `/avatars/`)
- 180-day retention with automatic cleanup (configurable)
- File records tracked in DB; orphan cleanup on startup
- Avatar references are preserved during cleanup and are not treated like ordinary conversation attachments

### Push Notifications
- Web Push (VAPID): subscribe, unsubscribe, per-entity subscriptions
- Delivered on new messages when recipient is offline

## Current Product Boundaries

These are important for integrators and client developers:

- ANI is an agent-native conversation system, not a generic IM with AI pasted on top.
- Conversation attachments are protected resources and must be accessed with ANI auth.
- Avatar files are intentionally different from message attachments:
  they use stable public-facing avatar routes and are safe to cache.
- Native mobile push is not yet a publicly claimable platform capability.
  The backend currently exposes Web Push only.

### Attachment Semantics

ANI itself guarantees transport and access control for attachments.
It does not guarantee that every bot or model can understand every file type.

- Small text files: strongest path today
- Images / audio / video: transport is supported, understanding depends on bot model/runtime
- PDF / Office docs / archives: transport is supported, parser experience is still incomplete

For the current detailed matrix, see:

- `../../_experience/ani-attachment-capability-matrix-2026-03-20.md`
- `../../_experience/ani-public-release-checklist-2026-03-20.md`

### WebSocket Events

``` 
ws://localhost:9800/api/v1/ws?device_id=DEVICE
```

Send `Authorization: Bearer TOKEN` during the WebSocket handshake.

### Reverse Proxy Requirement

If ANI runs behind Nginx or another reverse proxy, `/api/v1/ws` must be handled as a WebSocket upgrade route.

At minimum, forward:

- `Upgrade`
- `Connection: Upgrade`
- `Sec-WebSocket-Protocol`
- `Authorization`

Example Nginx block:

```nginx
location = /api/v1/ws {
    proxy_pass http://127.0.0.1:9800;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Sec-WebSocket-Protocol $http_sec_websocket_protocol;
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 3600;
}
```

If `/api/v1/ws` falls through a generic `/api/` proxy block without the upgrade headers, browser clients can get stuck in session bootstrap or never complete the WebSocket handshake.

| Event | Description |
|---|---|
| `message.new` | New message |
| `message.revoked` | Message revoked |
| `message.read` | Read receipt |
| `message.stream.start/delta/end` | Streaming lifecycle |
| `stream.cancel` | Cancel in-progress stream |
| `typing` | Typing indicator |
| `presence` | Online/offline status change |
| `conversation.new` | New conversation created |
| `connection.approved` | Bootstrap -> permanent key issued |
| `task.*` | Task CRUD events |

### Operations
- `GET /health` -- DB pool stats, active WS connections, uptime, memory
- Graceful shutdown (drain WS connections, close DB pool)
- Structured logging via `slog` (JSON in production)
- Request ID tracking on every request

## Message Layer Structure

| Layer | Type | Audience | Purpose |
|---|---|---|---|
| `summary` | string | Humans | Natural language display |
| `thinking` | string | Humans | Reasoning (collapsible) |
| `status` | object | Humans | Progress bar `{phase, progress, text}` |
| `data` | object | Agents | Structured JSON payload |
| `interaction` | object | Humans | Interactive cards (approval/selection/form) |

## Agent Key Lifecycle

1. User creates Bot -> server issues **bootstrap key** (`aimb_` prefix)
2. Agent connects via WebSocket with bootstrap key
3. User approves (or `AUTO_APPROVE_AGENTS=true` auto-approves)
4. Server issues **permanent key** (`aim_` prefix) via `connection.approved` event
5. Bootstrap key invalidated; agent uses permanent key going forward

## Tech Stack

- **Go 1.22+** / Gin / Bun ORM
- **PostgreSQL** with migrations
- **WebSocket** (Gorilla)
- **Web Push** (VAPID)

## Related Projects

| Project | Description |
|---|---|
| [agent-native-im-web](https://github.com/wzfukui/agent-native-im-web) | Web UI (React 19) |
| [agent-native-im-mobile](https://github.com/wzfukui/agent-native-im-mobile) | Expo / React Native mobile app (`ANI`) |
| [agent-native-im-sdk-python](https://github.com/wzfukui/agent-native-im-sdk-python) | Python SDK |
| [@openclaw/ani](../openclaw/extensions/ani/) | OpenClaw channel plugin |

## Product Baseline

Use these documents as the current formal ANI baseline:

- [docs/PRODUCT_BASELINE.md](docs/PRODUCT_BASELINE.md)
- [docs/ANI_AGENT_INTEGRATION_SPEC_V1.md](docs/ANI_AGENT_INTEGRATION_SPEC_V1.md)
- [docs/user-stories.md](docs/user-stories.md)
- [test-cases.md](test-cases.md)

## License

MIT
