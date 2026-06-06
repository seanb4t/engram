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
them. `Get(ctx, id)` stays as the raw, package-internal fetch (no policy). Two
**owner-aware store primitives** wrap it — one read-permissive, one write-strict
— and the destructive store methods are rewritten to take `sub` and enforce
internally. This is the load-bearing fix for the read/write asymmetry: a single
`getOwned` would wrongly let a non-owner mutate a `shared` record, so the two
sides get distinct primitives.

```go
// getReadable returns the record only if the caller may READ it (owner match OR
// shared); otherwise the same not-found error as a missing id. No existence leak.
func (s *Store) getReadable(ctx context.Context, id, sub string) (Memory, error) {
    m, err := s.Get(ctx, id) // raw internal fetch
    if err != nil { return Memory{}, err }
    if m.Owner != sub && m.Visibility != "shared" {
        return Memory{}, fmt.Errorf("not found: %s", id)
    }
    return m, nil
}

// getWritable returns the record only if the caller OWNS it (shared does NOT
// grant write); otherwise not-found. The mutate primitive.
func (s *Store) getWritable(ctx context.Context, id, sub string) (Memory, error) {
    m, err := s.Get(ctx, id) // raw internal fetch
    if err != nil { return Memory{}, err }
    if m.Owner != sub {
        return Memory{}, fmt.Errorf("not found: %s", id)
    }
    return m, nil
}
```

The destructive store methods change signature to enforce internally (so a
store-level caller cannot bypass the gate):

- `Delete(ctx, id, sub)` → `getWritable` then delete; mismatch → not found.
- `Update(ctx, id, sub, content, shared *bool)` → `getWritable`, apply content
  (+visibility if `shared != nil`), re-embed, `Upsert`. (Replaces the handler's
  current `Get`-then-`Upsert` two-step.)
- `SetVisibility(ctx, id, sub string, shared bool)` → `getWritable`, set
  `visibility`, `Upsert` (no re-embed).
- For the discovery client-supplied-`id` overwrite, write-strict is *not*
  sufficient because a brand-new `id` legitimately has no existing record.
  The store gains `ownedOrAbsent(ctx, id, sub)`: raw `Get`; if **not found** →
  ok (proceed as a new record); if **found and `owner == sub`** → ok (replace);
  if **found and `owner != sub`** → not found (refuse cross-owner overwrite).

| Path | Store gate |
|------|-----------|
| `get_memory` | `getReadable(id, sub)` — owner **or** shared |
| `update_memory` (content, and optional `shared`) | `Update(id, sub, …)` → `getWritable`; owner-only, never reaches `Upsert` on mismatch |
| `delete_memory` | `Delete(id, sub)` → `getWritable`; owner-only |
| `set_visibility` (new) | `SetVisibility(id, sub, shared)` → `getWritable`; owner-only |
| `store_discovery` with client-supplied `id` | `ownedOrAbsent(id, sub)` — replace own / create new / refuse other's. Fixes engram-ir1 / f7h.2. |
| `store_memory` | unchanged — random UUID, no overwrite possible; stamps `owner` from ctx |

**Write vs read asymmetry (load-bearing):** `getReadable` admits `owner == sub
OR shared`; `getWritable` (used by update, delete, set_visibility) and
`ownedOrAbsent` (discovery overwrite) require `owner == sub`. Sharing grants
*read*, never *write*.

### `DeleteAll`

`DeleteAll(scope, sub)` becomes owner-scoped: the delete filter is `scope == X
AND owner == sub`, so a teardown removes only the caller's own records in the
scope and never another owner's (and never another owner's shared records).

## MCP tool surface

The tool *contract* is unchanged except for two additions; `owner` is always
server-set and never a tool argument:

- **`update_memory`** gains an optional `shared *bool` argument (`Shared *bool`
  on `updateArgs`). **Pointer, not plain `bool`**: nil = leave visibility
  untouched (content-only update), so a routine content replace does *not*
  silently un-share a record; non-nil sets visibility explicitly.
- **`set_visibility(id, shared bool)`** — new dedicated tool to flip a record's
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

**Empty-owner guard (footgun prevention):** the command **refuses to run with an
empty `--owner`/`MEM_MIGRATE_OWNER`** and exits non-zero. Stamping records to
`owner == ""` would place them in the anonymous bucket (see Auth-disabled
invariant), defeating the migration. The operator must supply their real OIDC
`sub` — obtainable from a decoded token or the IdP — and should run the
migration *with OIDC enabled* so the value matches what the server will compare
against.

## Auth-disabled invariant

With no `--oidc-issuer`, there is no token, so `sub == ""`. New records get
`owner == ""`, and the read filter matches `owner == ""` — i.e. a **single
shared anonymous bucket; isolation is effectively off.** This is the only
coherent behavior and matches today's single-user reality. It is documented as
an explicit invariant: **isolation requires authentication.** Corollary: the
migration's configured `sub` should be the operator's *real* `sub`, so that
enabling auth later keeps their records theirs rather than orphaning them in the
anonymous bucket.

Behavior note for tests: in auth-disabled mode `cross_spine` discovery search
returns *all* discoveries (every record has `owner == ""`, which the filter
admits) — consistent with the single-bucket invariant, but a deliberate change
from today's unconditionally-unfiltered `cross_spine`. The filter is always
applied; it is simply permissive when everyone shares the empty owner.

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
  `update_memory shared` flag); B can read it but cannot update/delete it
  (`getWritable` denies). A content-only `update_memory` (`shared == nil`) on an
  already-shared record **preserves** `visibility` (regression guard for the
  pointer semantics).
- **Discovery overwrite (`ownedOrAbsent`):** A's client-supplied **new** id
  creates a record owned by A; A re-supplying that id replaces in place;
  B supplying A's id → not found, A's record unchanged.
- **Auth-disabled:** `sub == ""` round-trips through store + write in the single
  anonymous bucket.
- **Migration:** owner-less records get stamped to the configured `sub`;
  already-owned records are skipped (idempotency).

## Scope of change

- `internal/store/store.go` — `Memory` fields (`Owner`, `Visibility`),
  `payload`/`fromPayload`, `ownerScopeFilter` / `ownerOrSharedCondition`, the
  `getReadable` / `getWritable` / `ownedOrAbsent` primitives (raw `Get` is left
  unchanged, used internally by them), owner-aware
  `Search`/`List`/`SearchDiscovery` (filter) and `Delete`/`Update`/`DeleteAll`
  (signature gains `sub`), new `SetVisibility`, discovery-overwrite gate.
- `internal/server/tools.go` — `ownerFromContext`, thread `sub` into store
  calls, `updateArgs.Shared *bool`, `set_visibility` tool registration.
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
