---
title: Auth & Isolation
description: How engram enforces OIDC bearer-token authentication and per-actor memory isolation.
---

engram enforces per-actor memory isolation backed by OIDC bearer tokens. When
authentication is enabled, each caller sees and mutates only their own records.
When it is disabled, all callers share a single anonymous bucket.

## Enabling authentication

Set `--oidc-issuer` (or its env equivalent `MEM_OIDC_ISSUER`) to the OIDC
issuer URL. This is the **only configuration required** to enable bearer-token
enforcement.

```sh
engram serve \
  --oidc-issuer https://idp.example/application/o/engram/ \
  --oidc-audience engram
```

The four serve flags that have both a `--flag` and a `MEM_*` env equivalent:

| Flag | Env | Default |
|------|-----|---------|
| `--listen-addr` | `MEM_LISTEN_ADDR` | `:8080` |
| `--oidc-issuer` | `MEM_OIDC_ISSUER` | _(unset — auth disabled)_ |
| `--oidc-audience` | `MEM_OIDC_AUDIENCE` | _(unset — audience not checked)_ |
| `--oidc-resource-metadata` | `MEM_OIDC_RESOURCE_METADATA` | _(unset)_ |

Storage and embedding are configured via env-only variables (`MEM_QDRANT_ADDR`,
`MEM_QDRANT_COLLECTION`, `MEM_LITELLM_URL`, `MEM_LITELLM_KEY`,
`MEM_EMBED_MODEL`, `MEM_EMBED_DIM`) — these do not have `--flag` equivalents.

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

The `--owner` flag also reads from `MEM_MIGRATE_OWNER`. The command is
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
