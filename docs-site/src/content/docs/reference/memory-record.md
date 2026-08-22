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
| Summary Egress At | `summary_egress_at` | string (RFC3339) | **server** | Durable audit stamp: when this record's content was egressed to the summarizer model (`summary_source=auto` path only); zero if never egressed or the summary was client-authored/cleared |
| Actor | `actor` | string | **server** | Verified caller identity extracted from the OIDC token (email, username, or subject); never client-supplied; empty when auth is disabled |
| Owner | `owner` | string | **server** | Value of the configured owner claim (`ENGRAM_OWNER_CLAIM`, default `email`) — the authorization key; never client-supplied; empty string when auth is disabled (anonymous bucket) |
| Visibility | `visibility` | string | client/server | `""` (private, default) or `"shared"` — see [Visibility](#visibility) |
| Created at | `created_at` | string (RFC3339) | server | UTC timestamp of creation |
| Not before | `not_before` | string (RFC3339, optional) | client | Deferred-reveal lower bound: the record is hidden from recall until now is at or past `not_before`. Stored as an epoch-second integer, the same convention `archived_at` uses — see [The validity window](#the-validity-window) |
| Not after | `not_after` | string (RFC3339, optional) | client | Expiry upper bound: the record drops out of recall once now is at or past `not_after`. Stored as an epoch-second integer, the same convention `archived_at` uses — see [The validity window](#the-validity-window) |
| Supersedes | `supersedes` | string[] (optional) | server | Set on a *correcting* record: the id(s) of the memory or memories it replaced — see [Supersession](#supersession) |
| Superseded by | `superseded_by` | string (optional) | server | Set on a *corrected* record: the id of the memory that replaced it; its presence soft-hides the record from recall |
| Archived at | `archived_at` | string (RFC3339) | **server** | Set via `engram spine-review archive`; its presence soft-hides the record from recall but never from `get_memory` — see [Archiving](#archiving) |
| Access count | `access_count` | integer | **server** | Monotonic count of strong-signal touches (`get_memory` fetch + `update_memory`); never incremented by search/list result-set membership. Never read by the reranker or any recall filter |
| Last accessed at | `last_accessed_at` | string (RFC3339, optional) | **server** | Timestamp of the most recent strong-signal touch; absent when the record has never been accessed |
| Schema version | `schema_version` | integer | **server** | Schema-version discriminator; a record predating the key reads as `0` by absence, no backfill required; never participates in a recall or authorization filter — see [Schema version](#schema-version) |
| Kind | `kind` | string (optional) | client | Discovery discriminator: `map` or `fact`; present only on `discovery`-category records, set via `store_discovery` — see [Discovery fields](#discovery-fields) |
| Citations | `citations` | Citation[] | client | Optional structured source anchors on **any** category (required, min 1, only for `discovery`) — see [Citation fields](#citation-fields); never auto-populated |
| Score | `score` | number (optional) | server (query-time) | **Not a stored payload key.** The Qdrant similarity score for this result on `search_memory` (higher = closer); zero or omitted on unranked `list_memory`/`get_memory` results |

### Supersession

`supersede_memory` corrects one or more records without losing history. It stores
a single new, correcting record carrying a `supersedes` link to every target, and
stamps `superseded_by` onto each target — both additive; a target's content, tags,
and vector are untouched. One correcting record may replace several predecessors
in the same call; each predecessor still has exactly one successor.

A record carrying `superseded_by` is **soft-hidden from recall** (`search_memory`,
`list_memory`, `search_discovery`, `list_scheduled`) but remains **fetchable by id**
via `get_memory`, so the prior state is always auditable. Absent on every record
that has never participated in a supersession — pre-feature records are unaffected.

Chains still run forward (C supersedes B supersedes A) with exactly one live head
per chain: superseding an already-superseded record is rejected, which makes
cycles and self-supersession structurally impossible. See
[`supersede_memory`](/reference/tools/#supersede_memory) for the full contract.

### Archiving

`engram spine-review archive --id <id>` explicitly retires a record: it stamps
`archived_at`, an entirely **new, orthogonal** key — distinct from both `not_after`
expiry and `superseded_by` supersession. Archiving never writes `not_after` and
never writes `superseded_by`; a record can be archived, expired, and superseded
independently, and each state is cleared independently of the others.

A record carrying `archived_at` is **soft-hidden from recall** (`search_memory`,
`list_memory`, `search_discovery`, `list_scheduled`) but remains **fetchable by id**
via `get_memory`, exactly like `superseded_by` above. `engram spine-review restore
--id <id>` reverses it by deleting the `archived_at` key outright — never a
delete, content erasure, or vector removal on either verb. Archiving an
already-archived record, or restoring a never-archived one, is a no-op success.

Internally, `archived_at` is stored in Qdrant as an epoch-second integer (matching
`not_before`/`not_after`'s stored form) but round-trips through `get_memory` as an
RFC3339 string like every other `time.Time`-typed field in this table. It is
**visible on both lanes**: `get_memory` returns the `store.Memory` struct directly
as structured output on the MCP lane, and the Connect `Memory` message carries
`archived_at` alongside `superseded_by`, `supersedes`, `not_before`, `not_after`,
and `schema_version` — the full record-state contract is the same on either lane.

`archived_at` is deliberately **not** a Qdrant payload index: it is filtered with
`IsEmpty`, the identical access pattern `superseded_by` already has unindexed, so
the cost is already accepted for an equivalent predicate at equivalent
cardinality. See `engram spine-review scan`'s `archived` bucket (in
[the CLI guide](/guides/cli/)) for a spine-wide count of archived records,
separate from its `expired` bucket.

### The validity window

`not_before` and `not_after` together define a record's validity window — the same
mechanism `schedule_memory` writes and `list_scheduled` surfaces. Both are optional,
independent wire fields; a record carrying neither is unaffected, which is why the
feature needed no backfill.

- **`not_before`** gates deferred reveal with an **inclusive** lower bound: the
  record is hidden from recall until now is at or past `not_before`. A record
  whose `not_before` equals the current instant is already active — the moment of
  the boundary belongs to the active side, not the `scheduled` side.
- **`not_after`** gates expiry with an **exclusive** upper bound: the record drops
  out of recall once now is at or past `not_after`. A record whose `not_after`
  equals the current instant is already expired — the moment of the boundary
  belongs to the `expired` side, not the active side.

The window is therefore half-open, `[not_before, not_after)` — the same convention
[`search_memory`/`list_memory`'s](/reference/tools/#search_memory)
`created_after`/`created_before` already use. A caller who has internalized one
has internalized both; this is the house convention applied consistently, not a
special case.

`expired` and `scheduled` are mutually exclusive: `expired` is evaluated first
and, when present, suppresses `scheduled`. This precedence lives in the
derivation rather than in write-time validation — `not_before` and `not_after`
are independent wire fields, so an inverted window can in principle arrive from a
legacy or future path even though writing one is rejected today. The canonical
order in which state words are emitted, on every surface that renders them, is
`archived, superseded, expired, scheduled` — descending by finality. See
[`get_memory`](/reference/tools/#get_memory) for where a caller reads these
words back.

### Schema version

Every write stamps the current schema version onto the record's `schema_version`
key. A record written before the key existed has no such key at all and reads as
version `0` by absence — no backfill is required. The value is wire-visible on
both the MCP and Connect lanes; it is an observable field, not an internal audit
stamp, and it never appears in a recall or authorization filter.

The forward-compatibility guarantee has three parts and a boundary — read all of
them before assuming a rollback is always safe:

1. **Reads and recall are unconditionally safe.** A binary reading a record
   stamped newer than its own constant returns it normally — it never rejects,
   hides, or downgrades it. The version is not a filter key, so recall does not
   narrow on it either.
2. **The normal write path preserves the higher stamp.** An ordinary write
   computes the stored version as the greater of the current constant and the
   record's own decoded value, so a full write through the normal path cannot
   lower it.
3. **Editing a newer record with an older binary is lossy, and one write path
   sits outside the guarantee.** An older binary that edits a newer record keeps
   the newer stamp but rebuilds the payload from its own older struct, so keys
   only a newer version knows about are dropped. What is lost is re-derivable,
   because migration steps are additive-only — the recovery is re-running the
   migration sweep. Separately, one lower-level write path replaces a record's
   payload by id without reading the existing record first, so a caller holding
   a stale copy of a record's fields **can** lower its stored version through
   that path; this is a real, narrower boundary on the guarantee, not an edge
   case to wave away.

See the [`engram migrate`](/guides/migrate/) guide for the operator procedure
that advances a record's schema version, and the rollback hazard in
[`guides/upgrade.md`](/guides/upgrade/) for what a version downgrade means in
practice — the two pages describe the same hazard and must not disagree.

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

Records in the `discovery` category carry one additional field that is absent
(or zero-valued) on regular memory records.

| Field | JSON key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Kind | `kind` | string | yes | `map` (orientation/structure) or `fact` (pinned checkable claim); discovery-only, not settable on `store_memory` |
| Summary | `summary` | string | no | Short human-readable summary |

Discovery records live in scopes starting with `discovery:`, typically
`discovery:repo:<repo>`. They are recalled on demand via `search_discovery` and
are never returned by `list_memory` session bootstrap.

`citations` itself is **not** discovery-only — see the [Citations](#field-reference)
row in the main field reference above. The one asymmetry that remains: a
`discovery` record requires **at least one** citation, while a curated
`memory`-category record requires **none** (citations there are optional
provenance, added only when a claim benefits from a checkable anchor).

### Citation fields

Each citation anchors a claim to a verifiable source. The shape is identical
whether the citation lives on a `discovery` record or on any other category:

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
