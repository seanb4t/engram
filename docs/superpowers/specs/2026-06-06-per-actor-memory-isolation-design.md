<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Per-actor memory isolation — authentication → authorization

**Date:** 2026-06-06
**Status:** Design
**Design bead:** engram-99z

## Context

engram persists durable memory for coding agents as vectors in a single Qdrant
collection. A `Memory` carries `content`, `scope`, repo/workspace/worktree/
base_dir, `source`, `category`, `tags`, `actor`, and `created_at`; discoveries
add `kind`, `citations`, and `summary` in a separate `discovery:repo:*` scope.

engram today **authenticates but does not authorize.** When `--oidc-issuer` is
set, every request must carry a valid IdP-signed bearer token (signature via
JWKS, issuer, expiry, optional audience). But once a token validates, the caller
has full read/write access to **every scope and every record**. The verified
identity is recorded as the `actor` field purely for audit/display — it is
*never* used as an access-control filter, on any tool (`store`/`search`/`list`/
`get`/`update`/`delete_memory`, `delete_all`, `store_discovery`,
`search_discovery`).

`scope` (`repo:<repo>` / `discovery:repo:<repo>`) is the only isolation axis,
and it is a *labeling* axis, not an authorization one: a client supplies the
scope string, the server never ties a scope to who may access it. The operative
trust boundary is therefore "holding a token Authentik issued = fully trusted."

This is not yet exploitable because engram is single-user (just Sean). But the
intended model is **private-per-author with opt-in sharing**, and reaching it
requires an engram-wide authorization layer. Two known bugs are direct
consequences of its absence and are subsumed by this design:

- **engram-2kw** — reads are not actor-filtered: scope reads (including
  `search_discovery` `cross_spine=true`) expose every actor's records.
- **engram-ir1** — writes are not actor-gated: a client-supplied `id` on
  `store_discovery` (and the `update_memory` replace path) lets one actor
  overwrite another actor's record.

This design adds the authorization layer: read isolation, write gating, a
sharing model, a stable authorization key, and a migration for existing records.

## Decisions

These were settled during brainstorming (see bead engram-99z notes):

| Axis | Decision | Rationale |
|------|----------|-----------|
| Sharing model | Private-default + opt-in sharing | A repo is a shared artifact; discoveries especially are worth pooling. Future-proofs team use without a re-migration. |
| Meaning of "shared" | Readable by **any authenticated caller** | No membership model. Fits a self-hosted single-org (one Authentik) deployment with mutual trust. `scope` stays a query filter, not an access boundary. |
| Granularity | **Per-record** now; per-scope designed-for, not built | Maximum control with the least new state. `visibility` is a string (not a bool) so a future `scope-shared` default needs no re-migration. |
| Authorization key | Stable OIDC **`sub`** → new `owner` field | `email`/`preferred_username` are mutable; `sub` is the spec-guaranteed stable identifier. Keying authz on a display string would make an email change silently revoke a user's access to their own memories. `auth.go` already exposes `sub` via `TokenInfo.Extra["sub"]`. |
| `actor` field | Unchanged — stays human-readable audit/display | "Who recorded this, for humans" is a different concern from "who owns this, for authz". |
| Deny semantics | **404 not-found** | Not-owned is indistinguishable from not-exists; no cross-actor existence leak. |
| Enforcement layer | **In the store** | Isolation becomes a property of the data layer — no code path can return or mutate another owner's record. The read filter must live in the query regardless (post-filtering in a handler would over-fetch other owners' vectors into memory). |

## Data model

Two additive payload keys on the existing `Memory` record. Qdrant payloads are
schemaless and `fromPayload` skips absent keys, so this is zero-migration for
the *schema* (records written before this change simply lack the keys); a
*data* migration backfills `owner` (see Migration).

```go
type Memory struct {
    // …existing fields…
    Actor      string `json:"actor"`               // unchanged: display/audit (email→username→sub)
    Owner      string `json:"owner"`               // NEW: stable OIDC sub — the authorization key
    Visibility string `json:"visibility,omitempty"` // NEW: "" = private (default) | "shared"
}
```

- `Owner` is server-set from the verified token's `sub`, **never
  client-supplied** — the same rule that already governs `actor`.
- `Visibility` empty ⇒ private. The only value initially is `"shared"`. Keeping
  it a string (not a bool) reserves room for future values (e.g.
  `"scope-shared"`) without a re-migration.
- `payload(m)` writes both keys; `fromPayload` reads them with the existing
  absent-key tolerance.

### Context accessors (`tools.go`)

`actorFromContext` is unchanged. A sibling is added:

```go
// ownerFromContext returns the stable OIDC subject (authz key) injected by the
// RequireBearerToken middleware, or "" when auth is disabled. Never client-supplied.
func ownerFromContext(ctx context.Context) string {
    if ti := mcpauth.TokenInfoFromContext(ctx); ti != nil {
        if sub, ok := ti.Extra["sub"].(string); ok {
            return sub
        }
    }
    return ""
}
```

## Enforcement (all in `store.go`)

### Read filter

Reads compose an owner/shared subclause into the Qdrant `Must`, expressing
`scope == X AND (owner == caller OR visibility == "shared")` via
`NewFilterAsCondition` (a `Should` subclause nested as one `Must` condition):

```go
// ownerScopeFilter: scope==scope AND (owner==sub OR visibility=="shared").
func ownerScopeFilter(scope, sub string) *qdrant.Filter {
    return &qdrant.Filter{Must: []*qdrant.Condition{
        qdrant.NewMatch("scope", scope),
        ownerOrSharedCondition(sub),
    }}
}

func ownerOrSharedCondition(sub string) *qdrant.Condition {
    return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
        qdrant.NewMatch("owner", sub),
        qdrant.NewMatch("visibility", "shared"),
    }})
}
```

| Path | Change |
|------|--------|
| `Search(scope, sub, vec, k)` | `scopeFilter` → `ownerScopeFilter` |
| `List(scope, sub, limit)` | `scopeFilter` → `ownerScopeFilter` |
| `SearchDiscovery(scope, kind, sub, vec, k)` | append `ownerOrSharedCondition(sub)` to the existing `category=discovery` `Must`. **`cross_spine` drops the `scope` condition but keeps the owner/shared subclause** — so `cross_spine` = *my* discoveries across all my scopes (plus anyone's shared discoveries), never everyone's private discoveries. Fixes engram-2kw / f7h.3. |

### Id-path gates

Id-addressed operations gate on owner inside the store, so no handler can bypass
them. The single primitive is an owner-aware fetch:

```go
// getOwned returns the record only if the caller may read it (owner match or
// shared); otherwise the same not-found error as a missing id. No existence leak.
func (s *Store) getOwned(ctx context.Context, id, sub string) (Memory, error) {
    m, err := s.Get(ctx, id) // raw fetch (unexported use)
    if err != nil { return Memory{}, err }
    if m.Owner != sub && m.Visibility != "shared" {
        return Memory{}, fmt.Errorf("not found: %s", id)
    }
    return m, nil
}
```

| Path | Gate |
|------|------|
| `get_memory` | `getOwned(id, sub)` — read allows owner **or** shared |
| `update_memory` (content replace, and `shared` flag) | **write gate is stricter than read**: requires `owner == sub` (a shared record is readable by all but writable only by its owner). Mismatch → not found, never reaches `Upsert`. |
| `delete_memory` | owner-only fetch-and-check before delete; mismatch → not found |
| `store_discovery` with client-supplied `id` | before the upsert-replace, owner-only check on the existing record; if it exists and `owner != sub` → not found (refuse cross-owner overwrite). Fixes engram-ir1 / f7h.2. A brand-new `id` (no existing record) proceeds and stamps `owner` from ctx. |
| `set_visibility` (new) | owner-only check, then flip `visibility` |
| `store_memory` | unchanged — random UUID, no overwrite is possible; stamps `owner` from ctx |

**Write vs read asymmetry (load-bearing):** read paths admit `owner == sub OR
shared`; write/mutate paths (`update`, `delete`, `set_visibility`, discovery
overwrite) require `owner == sub`. Sharing grants *read*, never *write*.

### `DeleteAll`

`DeleteAll(scope, sub)` becomes owner-scoped: the delete filter is `scope == X
AND owner == sub`, so a teardown removes only the caller's own records in the
scope and never another owner's (and never another owner's shared records).

## MCP tool surface

The tool *contract* is unchanged except for two additions; `owner` is always
server-set and never a tool argument:

- **`update_memory`** gains an optional `shared bool` argument: replace content
  and (if present) set visibility in one owner-gated call.
- **`set_visibility(id, shared)`** — new dedicated tool to flip a record's
  visibility without rewriting content. Owner-gated. Both mechanisms exist by
  design (a content-replace can also re-share; a pure re-share need not
  re-embed).

`search` / `list` / `get` / `delete` / `delete_all` / `search_discovery`
signatures are unchanged at the MCP boundary — the `sub` they enforce on comes
from the verified token via context, not from arguments.

## Migration

Existing records have no `owner`; under the new read filter they would become
invisible to everyone (empty owner matches no real `sub`, and they are not
shared). Because engram is single-user today, every existing record is Sean's.

A one-time CLI subcommand stamps them:

```
engram migrate-set-owner --owner <sub>     # env: MEM_MIGRATE_OWNER
```

It scrolls the collection and, for every record with **empty** `owner`, sets
`owner = <sub>` (leaving `visibility` empty = private). It is **idempotent**:
records that already have a non-empty `owner` are skipped, so re-running is safe.
Run once by the operator with their real `sub`; multi-user then starts from a
clean, fully-owned state.

## Auth-disabled invariant

With no `--oidc-issuer`, there is no token, so `sub == ""`. New records get
`owner == ""`, and the read filter matches `owner == ""` — i.e. a **single
shared anonymous bucket; isolation is effectively off.** This is the only
coherent behavior and matches today's single-user reality. It is documented as
an explicit invariant: **isolation requires authentication.** Corollary: the
migration's configured `sub` should be the operator's *real* `sub`, so that
enabling auth later keeps their records theirs rather than orphaning them in the
anonymous bucket.

## Error handling

- Any id-path owner/visibility mismatch returns engram's existing
  `fmt.Errorf("not found: %s", id)` — identical to a genuinely missing id. No
  403, no ownership/existence disclosure across actors.
- `update_memory` / `delete_memory` / `set_visibility` / discovery-overwrite on
  a not-owned id therefore look exactly like operating on a non-existent id.
- Embedding and Qdrant transport errors surface unchanged.

## Testing

- **Store isolation (two owners A, B):**
  - `Search` / `List` / `SearchDiscovery` for A return A's records plus any
    `shared` records, and never B's private records.
  - `get_memory` of B's private id as A → not found; of B's shared id as A → ok.
  - `update` / `delete` / `set_visibility` / discovery-overwrite of B's id as A
    (even when shared) → not found; record unchanged.
  - `DeleteAll(scope, A)` leaves B's records in the scope intact.
  - `cross_spine` discovery search as A returns A's discoveries across scopes
    (plus shared), never B's private discoveries.
- **Sharing:** A marks a record shared (via both `set_visibility` and the
  `update_memory shared` flag); B can read it but cannot update/delete it.
- **Auth-disabled:** `sub == ""` round-trips through store + write in the single
  anonymous bucket.
- **Migration:** owner-less records get stamped to the configured `sub`;
  already-owned records are skipped (idempotency).

## Scope of change

- `internal/store/store.go` — `Memory` fields, `payload`/`fromPayload`,
  `ownerScopeFilter` / `ownerOrSharedCondition`, `getOwned`, owner-aware
  `Search`/`List`/`SearchDiscovery`/`Get`/`Delete`/`DeleteAll`, discovery
  overwrite gate, `SetVisibility`.
- `internal/server/tools.go` — `ownerFromContext`, thread `sub` into store
  calls, `update_memory` `shared` flag, `set_visibility` tool registration.
- `cmd/engram/` — `migrate-set-owner` subcommand.
- `README.md` / `CLAUDE.md` — memory contract: document `owner`/`visibility`,
  the private-default + opt-in-shared model, and the isolation-requires-auth
  invariant.

## Out of scope (explicit)

- **Membership / per-repo access control** — "shared" is server-wide-authenticated,
  not team-scoped. No membership store.
- **Per-scope visibility defaults** — designed-for (string `visibility`) but not
  built.
- **Admin/superuser cross-owner access** — none. Even `delete_all` is owner-scoped.
- **Migrating by `actor`→`sub` mapping** — unnecessary while single-user; the
  flat stamp suffices.
