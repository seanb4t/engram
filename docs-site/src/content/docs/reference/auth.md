---
title: Auth & Isolation
description: How engram enforces OIDC bearer-token authentication and per-actor memory isolation.
---

engram enforces per-actor memory isolation backed by OIDC bearer tokens. When
authentication is enabled, each caller sees and mutates only their own records.
When it is disabled, all callers share a single anonymous bucket.

## Enabling authentication

Set `--oidc-issuer` (or its env equivalent `ENGRAM_OIDC_ISSUER`) to the OIDC
issuer URL. This is the **only configuration required** to enable bearer-token
enforcement.

```sh
engram serve \
  --oidc-issuer https://idp.example/application/o/engram/ \
  --oidc-audience engram
```

The MCP bearer-token lane's serve flags that have both a `--flag` and an `ENGRAM_*` env equivalent (the web-UI lane's flags are in the console section below):

| Flag | Env | Default |
|------|-----|---------|
| `--listen-addr` | `ENGRAM_LISTEN_ADDR` | `:8080` |
| `--oidc-issuer` | `ENGRAM_OIDC_ISSUER` | _(unset — auth disabled)_ |
| `--oidc-audience` | `ENGRAM_OIDC_AUDIENCE` | _(unset — audience not checked)_ |
| `--oidc-resource-metadata` | `ENGRAM_OIDC_RESOURCE_METADATA` | _(unset)_ |

Storage and embedding are configured via env-only variables (`ENGRAM_QDRANT_ADDR`,
`ENGRAM_QDRANT_COLLECTION`, `ENGRAM_OPENAI_BASE_URL`, `ENGRAM_OPENAI_API_KEY`,
`ENGRAM_EMBED_MODEL`, `ENGRAM_EMBED_DIM`) — these do not have `--flag` equivalents.

### What is verified

On every MCP request, engram verifies:

- **Signature** — validated against the issuer's JWKS endpoint
- **Issuer** — must match `--oidc-issuer`
- **Expiry** — expired tokens are rejected
- **Audience** — checked only when `--oidc-audience` is set

The verified identity is extracted from the token (email address preferred, then
username, then subject) and recorded as the record's `actor` field.

### No issuer configured

When `--oidc-issuer` is not set, validation is **disabled**. Every request is
accepted. The server logs a loud warning at startup so this state is never
silently open. All callers share a single anonymous bucket (`owner == ""`).

---

## Two auth lanes: agents vs. web console

engram authenticates over two independent lanes, each verifying the issuer and
token signature on its own (and the audience where configured):

- **MCP bearer lane** — agents forward an OIDC bearer token (issued to a
  public PKCE client). Verified against `ENGRAM_OIDC_ISSUER`; the audience is
  checked only when `ENGRAM_OIDC_AUDIENCE` is set.
- **Web console login lane** — the operator console runs the OIDC
  authorization-code flow as a confidential client. It verifies ID tokens
  against its own issuer and pins the audience to the client ID.

The console lane is configured by these env vars (all have `--flag` equivalents)
and only activates when its credentials are present (or forced via
`ENGRAM_UI_ENABLED=true`):

| Flag | Env | Purpose |
|------|-----|---------|
| `--ui-enabled` | `ENGRAM_UI_ENABLED` | `""` implies-from-creds, `"true"` forces on, `"false"` hard off |
| `--ui-issuer` | `ENGRAM_UI_ISSUER` | Console OIDC issuer — **empty defaults to `ENGRAM_OIDC_ISSUER`** |
| `--oidc-client-id` | `ENGRAM_OIDC_CLIENT_ID` | Confidential-client ID |
| `--oidc-client-secret` | `ENGRAM_OIDC_CLIENT_SECRET` | Confidential-client secret |
| `--ui-redirect-url` | `ENGRAM_UI_REDIRECT_URL` | Auth-code callback URL |
| `--ui-cookie-key` | `ENGRAM_UI_COOKIE_KEY` | 32-byte AES-256 session-cookie key |

### Split-issuer (per-application IdP) topology

On an IdP that mints a distinct issuer per application (for example
Authentik's default `issuer_mode`), the security-preferred split — agents on a
**public** PKCE client and the console on a **confidential** client, i.e. two
apps with two different `iss` values — needs the two lanes to trust different
issuers. Set `ENGRAM_UI_ISSUER` to the console app's issuer; the MCP bearer lane
keeps `ENGRAM_OIDC_ISSUER` (the agents' app):

```sh
engram serve \
  --oidc-issuer https://idp.example/application/o/engram-agents/ \
  --ui-issuer   https://idp.example/application/o/engram-console/ \
  --oidc-client-id engram-console \
  --oidc-client-secret "$SECRET" \
  --ui-redirect-url https://engram.example/auth/callback \
  --ui-cookie-key "$COOKIE_KEY"
```

When `ENGRAM_UI_ISSUER` is unset, the console lane reuses `ENGRAM_OIDC_ISSUER`, so
single-application deployments need no extra configuration. An enabled console
with neither issuer set is a fail-fast startup error.

---

## Isolation model

Each authenticated caller is identified by the stable OIDC `sub` claim, stored
as the record's `owner`. The `sub` does not change when the user's email changes,
so access is never revoked by an identity provider profile update.

### Read access

| Caller type | What is readable |
|-------------|-----------------|
| Authenticated (`sub` present) | Own records (`owner == sub`) **plus** records where `visibility == "shared"` |
| Anonymous (no issuer, or auth disabled) | Only ownerless records (`owner == ""`) — **cannot** read shared records |

The shared read grant explicitly requires an authenticated `sub`. Anonymous
callers cannot read other actors' shared records even when `visibility` is set
to `"shared"`.

### Write access

Writes (update, delete, set_visibility) always require **ownership**:

- Authenticated callers: must be the record's `owner` (`owner == sub`)
- Anonymous callers: may only mutate records where `owner == ""`

Sharing (`visibility == "shared"`) grants **read only** — it never grants write
access to any other caller.

### Fail-closed design

- A record that exists but is not readable by the caller returns `not found`
  (ownership never leaks across actors)
- An unknown or nil subject fails closed, returning zero results rather than
  over-returning
- A validated token with a missing or empty `sub` is rejected (fails closed
  rather than collapsing to anonymous)

---

## Upgrading an existing deployment

Records written before per-actor isolation was introduced carry no `owner` key
(distinct from an empty-string `owner`). Once the new binary starts, these
**pre-isolation records are invisible to every read** and cannot be cleared by
`delete_all`.

The server logs a startup warning when owner-less records exist. Claim them
once with:

```sh
engram migrate-set-owner --owner <your-oidc-sub>
```

The `--owner` flag also reads from `ENGRAM_MIGRATE_OWNER`. The command is
**idempotent** — re-running when no owner-less records remain reports `0`.

The `migrate-set-owner` subcommand is implemented in `cmd/engram/migrate.go` and
is registered as a cobra subcommand (`Use: "migrate-set-owner"`).

### Disabling auth after it was enabled

Records written while authenticated carry a non-empty `owner`. If you remove
`--oidc-issuer`, callers fall into the anonymous bucket and can no longer read
those records — including ones marked `shared` (the shared read grant requires
an authenticated `sub`). The records are **not lost and not deleted**; they
become readable again once authentication is re-enabled.

`migrate-set-owner` only backfills pre-isolation records (those missing an
`owner` key entirely). It cannot move owner-stamped records into the anonymous
bucket and requires a non-empty `--owner`, so it is safe to re-run.
