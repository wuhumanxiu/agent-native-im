# ANI Account and External Identity Upgrade

Date: 2026-05-10

## Background

ANI already has first-party username/password accounts and browser login through 1Pass. 1Pass is the real external authentication provider from ANI's perspective. WeChat is an upstream identity behind 1Pass, not the direct provider integrated by ANI.

The current 1Pass login implementation creates local users with synthetic names like `1pass_<hash>`. This works for browser login, but it creates three product issues:

- The local ANI username is overloaded with an external login identifier.
- 1Pass-only users cannot set a first password because the password API requires an old password.
- Mobile users cannot reliably continue after browser-only QR login unless they can set an ANI username/password or bind/unbind external login methods.

## Target Model

ANI must treat its own entity as the primary account and external login methods as bound identities.

```
ANI entity
  id/public_id/name/email/display_name
  |
  +-- credentials
  |     password
  |     api_key
  |
  +-- auth_external_identities
        provider = 1pass
        provider_subject = stable 1Pass subject
        upstream_provider = wechat
        upstream_subject = unionid/openid when available
```

## Data Ownership

- `entities.id` remains the internal database primary key.
- `entities.public_id` remains the public ANI account identifier.
- `entities.name` remains the first-party username that users can type on web/mobile login.
- `credentials` remains the table for first-party secrets.
- `auth_external_identities` stores external login bindings.
- 1Pass fields are never used as the long-term ANI username contract.

## Provider Semantics

For the current integration:

- `provider`: `1pass`
- `provider_subject`: a deterministic subject built from `site_id` and 1Pass `openid`
- `upstream_provider`: `wechat`
- `upstream_subject`: 1Pass `unionid` if present, otherwise `openid`

This lets ANI add GitHub, Alipay, enterprise SSO, or another 1Pass upstream later without changing the account model.

## API Changes

### `GET /api/v1/me/auth-methods`

Returns the current account's authentication methods.

Response shape:

```json
{
  "has_password": true,
  "password_can_set": false,
  "external_identities": [
    {
      "id": 1,
      "provider": "1pass",
      "upstream_provider": "wechat",
      "display_name": "Chris",
      "avatar_url": "https://...",
      "linked_at": "2026-05-10T00:00:00Z",
      "last_used_at": "2026-05-10T00:00:00Z"
    }
  ]
}
```

### `PUT /api/v1/me/password`

For normal password users, `old_password` is still required.

For external-login-only users, `old_password` can be omitted and the API creates the first password credential. The request may also include `username` and `email` so users created by 1Pass can set a usable mobile login name in the same operation.

### `POST /api/v1/me/external-identities/1pass/link`

Authenticated users can bind a 1Pass identity by submitting the callback ticket. The backend exchanges the ticket with 1Pass, checks conflicts, and links the identity to the current ANI entity.

### `DELETE /api/v1/me/external-identities/:id`

Users can unbind an external identity when another login method remains. The API must reject deleting the last available auth method.

## Migration Strategy

Create `auth_external_identities` and backfill existing 1Pass-created users from `entities.metadata.onepass`.

If a legacy row cannot be backfilled because metadata is incomplete, it remains usable through the compatibility path:

1. next 1Pass login computes the old synthetic username;
2. loads the existing entity;
3. creates the missing external identity binding.

This avoids blocking deployment on imperfect historical data.

## Security Rules

- Do not expose raw 1Pass `openid` or `unionid` in public API responses.
- Do not allow binding a 1Pass identity already linked to another ANI entity.
- Do not allow removing the last auth method from an active user.
- Do not allow first-password setup for non-user entities.
- Keep first-party password complexity validation unchanged.

## Frontend Requirements

Settings > Security should show:

- whether a password is set;
- bound external login providers;
- a "Set username and password" flow for 1Pass-only users;
- a "Bind 1Pass" action for password users;
- an "Unbind" action with safety guardrails.

The 1Pass callback page should support both login mode and link mode.

## Test Plan

Backend:

- 1Pass login creates and reuses a first-class external identity.
- Legacy `1pass_<hash>` users are linked on next login.
- A 1Pass-only user can set username and first password without old password.
- Existing password users must provide the old password.
- 1Pass link rejects conflicts.
- External unlink rejects deleting the last auth method.

Frontend:

- Security settings render first-password and existing-password modes.
- 1Pass callback handles login mode and link mode.

Deployment:

- Apply migration before starting the new backend binary.
- Verify browser 1Pass login still works.
- Verify a 1Pass-created user can set username/password and then log in on mobile.
