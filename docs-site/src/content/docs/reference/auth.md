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

## Service principals (machine-to-machine auth)

Beyond the human/no-issuer OIDC lane above, engram supports headless service
principals — CI runners, batch jobs, other backend services — authenticating
over a third, independently-configurable lane. All three mechanisms compose
into a single verifier chain in front of the MCP bearer endpoint (`withAuth`,
`cmd/engram/serve.go`):

1. **Human OIDC** (`ENGRAM_OIDC_ISSUER`, above) — tried first for any
   JWT-shaped bearer.
2. **OIDC client-credentials** — tried second, only for a JWT-shaped bearer
   the human lane didn't accept.
3. **Static token** — tried for any non-JWT (opaque) bearer.

A bearer is routed to lane 1+2 or lane 3 by shape alone (three dot-separated
base64url segments = JWT-shaped, tried against the OIDC lanes; anything else
is opaque, tried against the static-token lane) *before* any verifier runs —
the two mechanism families never both attempt the same bearer.

Each mechanism activates independently, based on its own config being
present:

| Environment variable | Default | Description |
|----------------------|---------|--------------|
| `ENGRAM_SERVICE_AUTH_OIDC_ISSUER` | _(empty — lane off)_ | Client-credentials OIDC issuer URL. May be the same IdP as `ENGRAM_OIDC_ISSUER` or a distinct one. |
| `ENGRAM_SERVICE_AUTH_OIDC_AUDIENCE` | _(empty — audience not checked)_ | Expected `aud` claim for the service lane — configured **independently** of `ENGRAM_OIDC_AUDIENCE`; tightening or loosening one never affects the other. |
| `ENGRAM_SERVICE_AUTH_OWNER_CLAIMS` | `client_id,azp` | Ordered, comma-separated claim list the service lane resolves an owner from. **Never** defaults to `email` — a client-credentials token has no human identity to protect from an accidental email-shaped owner collision. |
| `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` | _(empty — lane off)_ | Comma-separated `owner=token` pairs, e.g. `ci=tok-abc123,batch=tok-def456`. Each token maps to its own distinct owner — never a single shared "service" bucket. |

A deployment with none of these set behaves byte-for-byte like today: only
the human OIDC lane (or no auth at all) is active.

### Owner resolution and fail-closed guarantee

Every mechanism resolves to the SAME `store.Subject` contract the human lane
uses — a service principal is isolated to its own `owner` bucket exactly like
any other authenticated caller (see [Isolation model](#isolation-model)
below), via the same namespaced-owner encoding used for any non-`email`
claim.

The client-credentials and static-token lanes are **fail-closed on an empty
owner**: an authenticated service principal whose configured owner claim
(`client_id`/`azp` by default) is absent from the token is **rejected**
(401) at the verifier boundary — it never falls through to the anonymous
`owner == ""` bucket the way the human lane does when auth is disabled. This
is the opposite of the human lane's fail-open-to-anonymous behavior, and it
is deliberate: an authenticated-but-unidentifiable service principal must
never silently share the anonymous bucket with unauthenticated traffic.

Before enabling the client-credentials lane, verify which claim your IdP
actually emits for the client-credentials grant — some IdPs emit `client_id`,
others `azp`, and some emit neither by default. `ENGRAM_SERVICE_AUTH_OWNER_CLAIMS`
accepts an ordered list (default `client_id,azp`) so both are tried; if your
IdP emits a different claim, add it to the list.

### Static-token safety

Static tokens are compared using a constant-time comparison
(`crypto/subtle.ConstantTimeCompare`) over the full token value — never a
prefix or substring match — and the raw token value is never written to a
log line, error message, or trace span, on either the accept or reject path.

Static tokens are stored as **plaintext** in config (consistent with how
`ENGRAM_OIDC_CLIENT_SECRET` and `ENGRAM_UI_COOKIE_KEY` are already handled) —
not hashed at rest. Treat `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` as a secret the
same way you already treat those variables.

:::caution[No revocation — the kill-switch is config rotation]
There is no revocation list for static tokens. The **only** way to invalidate
a compromised or retired static token is to remove or rotate it out of
`ENGRAM_SERVICE_AUTH_STATIC_TOKENS` and restart the server — the same
limitation already documented for the web-console session cookie key
(`ENGRAM_UI_COOKIE_KEY` rotation). Rotating a single principal's token means
minting a NEW `owner=token` entry and removing the old one; because multiple
tokens may map to the same owner, you can add the new token before removing
the old one for a zero-downtime rotation.
:::

### Cross-tenant `shared` reads

A `shared`-visibility record remains readable by **any** authenticated
caller — including a service principal from a *different* service tenant —
exactly as it already is for human callers. This is a deliberate, documented
decision for v0.11.x, not an oversight: see
[`docs/adr/engram-svct-service-tenant-global-shared-read.md`](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-svct-service-tenant-global-shared-read.md)
for the full rationale. Per-tenant `shared`-read scoping is deferred to a
future full tenant/group/role ABAC milestone. The tenancy-isolation guarantee
below applies to **private / owner-scoped** records only.

---

## Isolation model

Each authenticated caller is identified by the value of the configured owner
claim, stored as the record's `owner`. The owner claim is set via
`ENGRAM_OWNER_CLAIM` / `--owner-claim` (default `email`) and accepts an
**ordered, comma-separated list** of claims tried in order — e.g. `email,sub`
tries `email` first and falls back to `sub` for a service/machine token that
has no `email` claim, instead of failing closed. Authentication fails closed
when every claim in the list is absent from the token. A winning `email`
claim always requires `email_verified`; a winning non-`email` claim is
written as a namespaced owner string (see
[Upgrading to namespaced service owners](#upgrading-to-namespaced-service-owners)
below) so it can never collide with an `email` owner or another claim's
owner.

:::caution[Operator lockout risk]
Because the default claim is `email`, every token **must** carry
`email_verified: true` (a JSON boolean — a string `"true"` does **not** count and
fails closed). An IdP that omits `email_verified`, emits it as a string, or does
not issue the configured claim at all will cause **every** request to be rejected
(401) after upgrade. Before enabling, confirm your IdP emits the configured claim
with `email_verified` as a boolean, or set `ENGRAM_OWNER_CLAIM` to a different
stable claim your IdP does emit (e.g. `preferred_username` or `sub`).
:::

### Read access

| Caller type | What is readable |
|-------------|-----------------|
| Authenticated (owner claim present) | Own records **plus** records where `visibility == "shared"` |
| Anonymous (no issuer, or auth disabled) | Only ownerless records (`owner == ""`) — **cannot** read shared records |

The shared read grant explicitly requires an authenticated caller with a
resolved owner claim. Anonymous callers cannot read other actors' shared records
even when `visibility` is set to `"shared"`.

### Write access

Writes (update, delete, set_visibility) always require **ownership**:

- Authenticated callers: must be the record's `owner`
- Anonymous callers: may only mutate records where `owner == ""`

Sharing (`visibility == "shared"`) grants **read only** — it never grants write
access to any other caller.

### Fail-closed design

- A record that exists but is not readable by the caller returns `not found`
  (ownership never leaks across actors)
- An unknown or nil subject fails closed, returning zero results rather than
  over-returning
- A validated token with a missing or absent owner claim is rejected (fails
  closed rather than collapsing to anonymous)

---

## Upgrading an existing deployment

Records written before per-actor isolation was introduced carry no `owner` key
(distinct from an empty-string `owner`). Once the new binary starts, these
**pre-isolation records are invisible to every read** and cannot be cleared by
`delete_all`.

The server logs a startup warning when owner-less records exist. Claim them with
`migrate-remap-owner` (`migrate-set-owner` is a deprecated alias for
`--from-missing`):

```sh
# Backfill pre-isolation records that have no owner set
engram migrate-remap-owner --from-missing --to <owner-claim-value> --apply

# Re-stamp records that were written when owner was derived from sub
# (e.g. after switching ENGRAM_OWNER_CLAIM from "sub" to "email")
engram migrate-remap-owner --from <old-sub-value> --to <email-value> --apply
```

Both forms preview by default — drop `--apply` from either command above to
see the eligible count without writing anything — and accept `--timeout` to
cap the operation. The command is **idempotent** — re-running when no
matching records remain reports `0`.

> **Note:** changing `ENGRAM_OWNER_CLAIM` (including upgrading from an older
> binary that always used `sub`) invalidates existing web-console session
> cookies. Users see a one-time re-login prompt on their next console visit.

### Disabling auth after it was enabled

Records written while authenticated carry a non-empty `owner`. If you remove
`--oidc-issuer`, callers fall into the anonymous bucket and can no longer read
those records — including ones marked `shared` (the shared read grant requires
an authenticated caller). The records are **not lost and not deleted**; they
become readable again once authentication is re-enabled.

`migrate-remap-owner --from-missing` only backfills pre-isolation records (those
missing an `owner` key entirely). It requires a non-empty `--to`, so it is safe
to re-run.

### Upgrading to namespaced service owners

`ENGRAM_OWNER_CLAIM` accepts an ordered, comma-separated claim list (default
`email`) so a service/machine token with no `email` claim can resolve to a
stable owner instead of failing closed — e.g. `email,sub` tries `email` first
and falls back to `sub` when `email` is absent or an empty string. A winning
non-`email` claim's value is encoded into a **namespaced** owner string using
a length-prefixed scheme,
`<len(claim)>:<claim>:<len(value)>:<value>`, so two different (claim, value)
pairs — and a bare `email` owner — can never collide.

Moving from a single-value `email`-only deployment to a multi-claim list is a
rollout **migration**, not just a config change, because the owner string
**is** the authorization key compared directly on every read/write:

- **Web-console sessions are invalidated automatically.** The session cookie
  payload carries a version; upgrading to a release with this change rejects
  every pre-upgrade cookie and forces a one-time re-login, so no bare-owner
  cookie can be silently forwarded into the new namespaced owner space. This
  is the same automatic re-login behavior already documented above for an
  `ENGRAM_OWNER_CLAIM` change — no manual `--ui-cookie-key` rotation is
  required.
- **Existing non-`email` owner records need remapping.** If you previously ran
  with `ENGRAM_OWNER_CLAIM` set to a non-`email` claim (`sub`, `client_id`,
  etc.), those records still carry the OLD bare claim value as `owner`. Remap
  them to the new encoded form with `engram migrate-remap-owner`, deriving the
  encoded target yourself as `<len(claim)>:<claim>:<len(value)>:<value>`:

  ```sh
  # sub owner "svc-1" -> encoded target "3:sub:5:svc-1"
  engram migrate-remap-owner --from svc-1 --to 3:sub:5:svc-1              # preview (no --apply)
  engram migrate-remap-owner --from svc-1 --to 3:sub:5:svc-1 --apply

  # client_id owner "app42" -> encoded target "9:client_id:5:app42"
  engram migrate-remap-owner --from app42 --to 9:client_id:5:app42        # preview (no --apply)
  engram migrate-remap-owner --from app42 --to 9:client_id:5:app42 --apply
  ```

  `email`-owned records need **no** migration — a winning `email` claim is
  still written bare, byte-for-byte unchanged.

  :::caution[Global, non-transactional rewrite]
  `migrate-remap-owner` matches and rewrites **every** record carrying the
  exact `--from` owner string together, in one non-transactional pass — it
  cannot distinguish which claim originally produced that value if historical
  claim configurations collided. It previews by default and writes nothing
  until you pass `--apply` (see the [upgrade guide](/guides/upgrade/)); review
  the preview count and take a backup of your Qdrant collection before
  applying the mapping.
  :::

Pre-1.0 self-hosted deployments that have never set `ENGRAM_OWNER_CLAIM` to a
non-`email` claim have zero affected records — this section applies only to
fleets that opted into a non-`email` owner claim before this release.
