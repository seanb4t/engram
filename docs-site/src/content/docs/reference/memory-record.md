---
title: Memory Record
description: Field-by-field reference for the engram memory record — types, allowed values, and isolation semantics.
---

Every piece of information stored in engram is a **memory record**. Records are
stored as vectors in Qdrant and surfaced through the MCP tools. This page
documents every field, its serialized JSON name, allowed values, and who sets it.

## Field reference

| Field | JSON key | Type | Set by | Description |
|-------|----------|------|--------|-------------|
| ID | `id` | string (UUID) | server | Unique record identifier, generated on creation |
| Short ID | `short_id` | string (Crockford base32) | server | Short, case-insensitive handle; accepted anywhere an id is accepted; minted on creation |
| Content | `content` | string | client | The memory text; also the text that is embedded |
| Scope | `scope` | string | client | `run:tier:repo` identifier, e.g. `eval-2026-05:project:selfhosted-cluster` |
| Repo | `repo` | string | client | Repository name or URL (optional context) |
| Workspace | `workspace` | string | client | Workspace identifier (optional context) |
| Worktree path | `worktree_path` | string | client | Path to the git worktree (optional context) |
| Base dir | `base_dir` | string | client | Base directory for the project (optional context) |
| Source | `source` | string | client | How the memory was produced — see [Source values](#source-values) |
| Category | `category` | string | client | What kind of memory — see [Category values](#category-values) |
| Tags | `tags` | string[] | client | Free-form labels |
| Summary | `summary` | string | client/server | Short human-readable summary; omit or empty for none — see [Summary fields](#summary-fields) |
| Summary Source | `summary_source` | string | client/server | How the summary was produced: `client` (caller-authored), `auto` (offline-generated), or `""` (none) |
| Summary Model | `summary_model` | string | server | Name of the model used when `summary_source=auto` (e.g. `gpt-4o-mini`); empty when source is `client` or none |
| Actor | `actor` | string | **server** | Verified caller identity extracted from the OIDC token (email, username, or subject); never client-supplied; empty when auth is disabled |
| Owner | `owner` | string | **server** | Value of the configured owner claim (`ENGRAM_OWNER_CLAIM`, default `email`) — the authorization key; never client-supplied; empty string when auth is disabled (anonymous bucket) |
| Visibility | `visibility` | string | client/server | `""` (private, default) or `"shared"` — see [Visibility](#visibility) |
| Created at | `created_at` | string (RFC3339) | server | UTC timestamp of creation |
| Supersedes | `supersedes` | string (optional) | server | Set on a *correcting* record: the id of the memory it replaced — see [Supersession](#supersession) |
| Superseded by | `superseded_by` | string (optional) | server | Set on a *corrected* record: the id of the memory that replaced it; its presence soft-hides the record from recall |

### Supersession

`supersede_memory` corrects a record without losing history. It stores the new
record with a `supersedes` link to the target, and stamps `superseded_by` onto the
target — both additive; the target's content, tags, and vector are untouched.

A record carrying `superseded_by` is **soft-hidden from recall** (`search_memory`,
`list_memory`, `search_discovery`, `list_scheduled`) but remains **fetchable by id**
via `get_memory`, so the prior state is always auditable. Absent on every record
that has never participated in a supersession — pre-feature records are unaffected.

Chains run forward (C supersedes B supersedes A) with exactly one live head:
superseding an already-superseded record is rejected, which makes cycles and
self-supersession structurally impossible. See
[`supersede_memory`](/reference/tools/#supersede_memory) for the full contract.

### Source values

The `source` field describes how the memory was produced. Exactly two values are
accepted by the store:

| Value | Meaning |
|-------|---------|
| `user-said` | The user stated this explicitly |
| `agent-inferred` | The agent derived or inferred this |

Discovery records always have `source` set to `agent-inferred` by the server.

### Category values

The `category` field classifies what kind of memory is stored:

| Value | Meaning |
|-------|---------|
| `decision` | An architectural or design decision |
| `preference` | A stated user or team preference |
| `convention` | A coding or workflow convention |
| `gotcha` | A known pitfall or non-obvious behaviour |
| `discovery` | Agent-earned codebase understanding (see [Discovery fields](#discovery-fields)) |

The `discovery` category is set by the server for records created via
`store_discovery`; client callers use the other four values with `store_memory`.

### Visibility

The `visibility` field controls cross-actor reads:

| Value | Meaning |
|-------|---------|
| `""` (empty string) | Private — only the owner can read and write |
| `"shared"` | Readable by any authenticated caller; writable only by owner |

Toggle visibility with `set_visibility` or `update_memory`'s `shared` argument.
Sharing grants **read only** — another actor can never write a record they do not
own, even when it is shared.

### Summary fields

Summaries help agents work with stored memories efficiently. Recall (via
`search_memory` / `list_memory`) returns compact summaries by default, keeping
the spine bootstrap small; full `content` is always accessible via `get_memory`
(id fetch) or by passing `full=true` to recall tools.

- **`summary`**: Short human-readable digest of the memory. Optional.
- **`summary_source`**: Describes how the summary was produced:
  - `"client"` — caller-authored and explicit; important for content matching
    and stable, treat as authoritative
  - `"auto"` — offline-generated by `engram summarize-missing`; lossy but useful
    for orientation; always verify against full content before acting on caveats
  - `""` (empty string) — no summary present

- **`summary_model`**: Name of the model used when `summary_source=auto` (e.g.
  `gpt-4o-mini`). Set by the server; empty for client-authored or absent
  summaries.

**Updating with a stale summary:** If `summary_source=client` and you change the
memory's `content`, you must address the summary: re-send it unchanged, update
it to reflect the new content, or clear it. Failing to address a stale
caller-authored summary causes the update to be rejected.

---

## Discovery fields

Records in the `discovery` category carry additional fields that are absent (or
zero-valued) on regular memory records.

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Kind | `kind` | string | yes | `map` (orientation/structure) or `fact` (pinned checkable claim) |
| Citations | `citations` | Citation[] | yes | At least one source anchor; max 50 |
| Summary | `summary` | string | no | Short human-readable summary |

Discovery records live in scopes starting with `discovery:`, typically
`discovery:repo:<repo>`. They are recalled on demand via `search_discovery` and
are never returned by `list_memory` session bootstrap.

### Citation fields

Each citation anchors a discovery claim to a verifiable source:

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Kind | `kind` | string | yes | `file`, `commit`, `url`, or `repo` |
| Ref | `ref` | string | yes | Path, repo URL, or doc URL |
| Locator | `locator` | string | no | E.g. `200-240` line range |
| Pin | `pin` | string | no | Aging anchor: commit SHA, content-hash, `@rev`, or `fetched-at` |
| Excerpt | `excerpt` | string | no | Cached substance from the source; max 16 KiB, soft cap ~50 lines |

---

## Field name notes

The serialized JSON keys match the Go struct tags in `internal/store/store.go`
exactly:

- `worktree_path` (not `worktree`) — the struct field is `Worktree string \`json:"worktree_path"\``
- `base_dir` (not `baseDir`) — snake_case throughout
- `created_at` — RFC3339 string in the Qdrant payload; deserialized as `time.Time`

These match the README's description (`repo`/`workspace`/`worktree_path`/`base_dir`).

---

## Isolation and ownership

`actor` and `owner` are always server-set. The `actor` is extracted from the
token's `UserID` (email, username, or subject claim, in priority order). The
`owner` is the value of the configured owner claim (`ENGRAM_OWNER_CLAIM`,
default `email`); it changes only if the chosen claim is not stable across IdP
profile updates (e.g. `email` changes when the user renames their account; `sub`
never changes).

When authentication is disabled (no `--oidc-issuer`), both `actor` and `owner`
are empty strings, and all callers share one anonymous bucket.

Pre-isolation records — those written before per-actor ownership was added —
carry no `owner` key (distinct from an empty-string `owner`). They are invisible
to every owner-scoped read. See [Auth and Isolation](/reference/auth/) for
migration details.
