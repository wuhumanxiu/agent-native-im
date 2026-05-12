# ANI Protocol Contract

This directory is the canonical contract source for SDKs and external agent adapters.

## Files

- `manifest.json`: protocol version, source service, and required SDK contract checks.
- `openapi.yaml`: REST API contract snapshot for ANI `/api/v1`.
- `ws-events.schema.json`: WebSocket event envelope and core event payload schema.

## Rules

- SDKs should vendor or fetch this directory and run contract tests against it.
- API additions should update `openapi.yaml`.
- WebSocket event additions or payload changes should update `ws-events.schema.json`.
- Breaking changes must update `manifest.json` and be reflected in SDK handoff docs.

## Current SDKs

- Python: `https://github.com/wzfukui/ani-agent-sdk-python`
- JavaScript: `https://github.com/wzfukui/ani-agent-sdk-js`

