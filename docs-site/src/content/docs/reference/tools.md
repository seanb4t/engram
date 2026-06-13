---
title: MCP Tools
description: Complete reference for all engram MCP tools — arguments, return values, and usage notes.
---

engram exposes its memory API as Model Context Protocol (MCP) tools. All tools
require an active MCP session. When OIDC is enabled, every call must carry a
valid bearer token; the verified identity becomes the `actor` and `owner` of any
records created.

## Tool summary

| Tool | Purpose |
|------|---------|
| `store_memory` | Persist a deliberate, well-formed memory |
| `schedule_memory` | Persist a memory with a validity window (deferred reveal / expiry) |
| `search_memory` | Semantic search within a scope |
| `list_memory` | Most-recent memories in a scope (no query — session bootstrap) |
| `list_scheduled` | List windowed memories the recall gate is hiding |
| `get_memory` | Fetch one memory by id |
| `update_memory` | Replace a memory's content in place (re-embeds) |
| `delete_memory` | Delete one memory by id |
| `delete_all` | Delete your own memories in a scope (teardown) |
| `store_discovery` | Cache citation-backed codebase understanding |
| `search_discovery` | Semantic search over the discovery pool |
| `set_visibility` | Share or unshare a memory you own |

**`actor` and `owner` are always server-set.** They come from the validated OIDC
token and are never accepted as client input.

---

## store_memory

Persist a deliberate, well-formed memory. Do not store transient state, secrets,
or timestamps.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `content` | string | yes | The memory text to persist |
| `scope` | string | yes | `run:tier:repo` identifier, e.g. `eval-2026-05:project:selfhosted-cluster` |
| `source` | string | yes | `user-said` or `agent-inferred` |
| `category` | string | yes | `decision`, `preference`, `convention`, or `gotcha` |
| `tags` | string[] | no | Free-form labels |
| `repo` | string | no | Repository name or URL |
| `workspace` | string | no | Workspace identifier |
| `worktree_path` | string | no | Path to the git worktree |
| `base_dir` | string | no | Base directory for the project |

Returns the stored record's `id`.

---

## schedule_memory

Persist a memory with a temporal validity window. At least one of `not_before` /
`not_after` is required (use `store_memory` for unscheduled records). `discovery`
is not schedulable. A future `not_before` hides the record from recall until then;
`not_after` drops it from recall at that time. Active windowed records surface
normally via `search_memory`/`list_memory`.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `content` | string | yes | The memory text to persist |
| `scope` | string | yes | `run:tier:repo` identifier |
| `source` | string | yes | `user-said` or `agent-inferred` |
| `category` | string | yes | `decision`, `preference`, `convention`, or `gotcha` |
| `tags` | string[] | no | Free-form labels |
| `repo` | string | no | Repository name or URL |
| `workspace` | string | no | Workspace identifier |
| `worktree_path` | string | no | Path to the git worktree |
| `base_dir` | string | no | Base directory for the project |
| `not_before` | string | no | RFC3339; hide from recall until this time |
| `not_after` | string | no | RFC3339; drop from recall at this time |

Returns the scheduled record's `id`. At least one bound is required. Operators
reclaim lapsed records with the `engram prune-expired [--older-than DUR]` CLI command.

---

## search_memory

Semantic (vector) search within a scope. Embeds `query` and returns the nearest
memories.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `query` | string | yes | Natural-language search query |
| `scope` | string | yes | Scope to search within |
| `k` | uint64 | no | Number of results to return (default 8) |

Returns a list of matching memory records.

---

## list_memory

List recent memories in a scope without a query. Intended for session-start
bootstrap. Results are most-recent first.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `scope` | string | yes | The scope to list memories from |
| `limit` | uint64 | no | Maximum memories to return (default 20) |

Returns a list of memory records.

---

## list_scheduled

List your windowed memories the recall gate is hiding. Active windowed records
surface via `list_memory`/`search_memory`, not here.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `scope` | string | yes | The scope to list scheduled/expired memories from |
| `state` | string | no | `scheduled` (default, not yet active), `expired`, or `all` |
| `limit` | uint64 | no | Maximum memories to return (default 20) |

Returns the matching hidden windowed records.

---

## get_memory

Fetch one memory by id.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID of the memory to fetch |

Returns the full memory record. Authenticated callers can read their own records
plus any `shared` records. Anonymous callers can only read ownerless records.
Fetch-by-id is **not** recall-gated: a windowed record (set via `schedule_memory`)
that `search_memory`/`list_memory` hide because it is scheduled or expired is still
retrievable directly by id here.

---

## update_memory

Replace a memory's content in place. The content is re-embedded. Optionally
toggle visibility.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID of the memory to update |
| `content` | string | yes | The replacement text (re-embedded) |
| `shared` | bool | no | `true` = shared, `false` = private; omit to keep current visibility |

Only the record owner can update. Returns `"updated"` on success.

---

## delete_memory

Delete one memory by id.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID of the memory to delete |

Only the record owner can delete. Returns `"deleted"` on success.

---

## delete_all

Delete your own memories in a scope (teardown). Never deletes another caller's
records.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `scope` | string | yes | The scope to clear |

Returns `"scope cleared"` on success.

---

## store_discovery

Cache agent-earned codebase understanding with source citations. Discoveries
live in a separate `discovery:repo:*` scope and are recalled on demand, never
at session start.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `content` | string | yes | The understanding to cache (embedded and searched); max 64 KiB |
| `kind` | string | yes | `map` (orientation/structure) or `fact` (pinned checkable claim) |
| `citations` | citation[] | yes | At least one source anchor (max 50) |
| `scope` | string | yes | Must start with `discovery:`, e.g. `discovery:repo:my-repo` |
| `tags` | string[] | no | Free-form labels |
| `summary` | string | no | Short human-readable summary |
| `id` | string | no | Omit to create; supply to replace in place |

Each **citation** object:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `kind` | string | yes | `file`, `commit`, `url`, or `repo` |
| `ref` | string | yes | Path, repo URL, or doc URL |
| `locator` | string | no | E.g. `200-240` line range |
| `pin` | string | no | Commit SHA, content-hash, `@rev`, or `fetched-at` (aging anchor) |
| `excerpt` | string | no | Cached substance (max 16 KiB, soft cap ~50 lines) |

The `source` field is always `agent-inferred` and `category` is always
`discovery` — both are server-set; do not supply them.

Returns the stored discovery's `id`.

---

## search_discovery

Semantic search over the discovery pool. Scope is required unless
`cross_spine=true`.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `query` | string | yes | Natural-language search query |
| `scope` | string | conditional | Discovery scope; required unless `cross_spine` is true |
| `kind` | string | no | `map` or `fact` filter |
| `k` | uint64 | no | Number of results to return (default 8) |
| `cross_spine` | bool | no | Span all discovery scopes; ignores `scope` when true |

Results carry `citations` and `created_at` (useful as aging signals).

---

## set_visibility

Share or unshare a memory you own. Does not re-embed; only flips the visibility
flag.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID of the memory |
| `shared` | bool | yes | `true` = readable by any authenticated caller; `false` = private |

Only the record owner can change visibility. Sharing grants read, never write.
Returns `"visibility updated"` on success.
