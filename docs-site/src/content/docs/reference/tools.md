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
| `supersede_memory` | Correct a memory with a new record, preserving history |
| `update_memory` | Replace a memory's content in place (re-embeds) |
| `delete_memory` | Delete one memory by id |
| `delete_all` | Delete your own memories in a scope (teardown) |
| `store_discovery` | Cache citation-backed codebase understanding |
| `search_discovery` | Semantic search over the discovery pool |
| `set_visibility` | Share or unshare a memory you own |
| `store_rule` | Persist a normative, user-blessed rule (ground truth) |
| `list_rules` | List the complete rule set for one or more scopes |

**`actor` and `owner` are always server-set.** They come from the validated OIDC
token and are never accepted as client input.

**A rejected call** — a missing or malformed argument, a bound exceeded — returns a
field-and-hint envelope rather than a plain message; see the
[error reference](/reference/errors/) for the full grammar and hint-code vocabulary.

## Blast radius

Every tool advertises four MCP `ToolAnnotations` hints — `readOnlyHint`,
`destructiveHint`, `idempotentHint`, `openWorldHint` — so an agent can read a
tool's blast radius before calling it, never by triggering it first. Values
come from one shared table (`internal/surfaces`), generated here rather than
hand-maintained per tool; `openWorldHint` is `false` on every tool, since
engram is a closed memory domain. These are hints, not an authorization
mechanism — never make a tool-use decision based on annotations from an
untrusted server.

<!-- engram:rule:start tool-blast-radius -->
| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |
|------|----------------|--------------------|-------------------|------------------|
| `store_memory` | false | false | false | false |
| `schedule_memory` | false | false | false | false |
| `search_memory` | true | false | true | false |
| `list_memory` | true | false | true | false |
| `list_scheduled` | true | false | true | false |
| `get_memory` | true | false | true | false |
| `update_memory` | false | true | true | false |
| `delete_memory` | false | true | true | false |
| `delete_all` | false | true | true | false |
| `store_discovery` | false | false | false | false |
| `search_discovery` | true | false | true | false |
| `set_visibility` | false | false | true | false |
| `supersede_memory` | false | false | true | false |
| `store_rule` | false | false | false | false |
| `list_rules` | true | false | true | false |
<!-- engram:rule:end tool-blast-radius -->

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
| `summary` | string | no | Short human-readable summary (caller-authored, `summary_source=client`). Max `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` bytes (default 512; see [Configuration](/guides/configure/)). Omit for no summary. |
| `citations` | citation[] | no | Optional structured source anchors (same shape as [`store_discovery`](#store_discovery)'s `citations`, max 50); never inferred — only what you explicitly supply. Omit for none. |

Returns the stored record's `id` and `short_id`.

---

## schedule_memory

Persist a memory with a temporal validity window.
<!-- engram:rule:start schedule-window-at-least-one-bound -->schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records)<!-- engram:rule:end schedule-window-at-least-one-bound -->.
<!-- engram:rule:start window-not-before-before-not-after -->not_before must be strictly before not_after<!-- engram:rule:end window-not-before-before-not-after -->.
<!-- engram:rule:start discovery-not-schedulable -->discovery is not schedulable; use store_discovery<!-- engram:rule:end discovery-not-schedulable -->.
A future `not_before` hides the record from recall until then;
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
| `summary` | string | no | Short human-readable summary (caller-authored, `summary_source=client`). Max `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` bytes (default 512; see [Configuration](/guides/configure/)). Omit for no summary. |
| `citations` | citation[] | no | Optional structured source anchors (same shape as [`store_discovery`](#store_discovery)'s `citations`, max 50); never inferred — only what you explicitly supply. Omit for none. |
| `not_before` | string | no | RFC3339; hide from recall until this time |
| `not_after` | string | no | RFC3339; drop from recall at this time |

Returns the scheduled record's `id` and `short_id`. At least one bound is required. Operators
reclaim lapsed records with the `engram prune-expired --apply` CLI command (preview by default
without `--apply`; add `--older-than DUR` for a grace period).

Operators can also permanently delete purge-eligible records with `engram spine-review purge
--apply` (preview by default without `--apply`), gated on an extract-before-delete precondition.
Its structural classes (`superseded`, `expired`, `archived`) need only that gate; the free-form
filter path (`--category`, `--tags`, or `--older-than` with no `--class`) additionally requires:
<!-- engram:rule:start purge-filter-requires-scope -->the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected<!-- engram:rule:end purge-filter-requires-scope -->.
See the [CLI guide](/guides/cli/#spine-review-purge) for the full contract.

---

## search_memory

Semantic (vector) search within a scope. Embeds `query` and returns the nearest
memories. By default returns compact summaries; pass `full=true` for complete content.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `query` | string | yes | Natural-language search query |
| `scope` | string | conditional | Scope to search within; <!-- engram:rule:start scope-required-unless-cross-spine -->scope is required unless cross_spine is true<!-- engram:rule:end scope-required-unless-cross-spine --> |
| `k` | uint64 | no | Number of results to return (default 8) |
| `tags` | string[] | no | Restrict to records carrying **all** listed tags (AND). Omit for no tag filter. Applied as a hard pre-filter, then results are ranked by vector similarity and reranking (see below) |
| `categories` | string[] | no | Restrict to records in **any** of the listed categories (OR) — the opposite of `tags`' ALL/AND semantics, since a record carries exactly one category. Omit or pass an empty array for no category filter. An unmatched value returns zero results, never an error; any stored category is accepted, including `discovery` and `rule`, not just the four `store_memory` write values. Applied as a hard pre-filter, before vector ranking. The same filter is available over the Connect read API on `SearchMemories`. |
| `created_after` | string | no | RFC3339 timestamp — include only records with `created_at >= created_after` (inclusive lower bound) |
| `created_before` | string | no | RFC3339 timestamp — include only records with `created_at < created_before` (exclusive upper bound). Half-open window: `[created_after, created_before)` |
| `full` | bool | no | Return full `content` instead of compact summaries (default `false`) |
| `cross_spine` | bool | no | Span all scopes the caller can read; ignores `scope` when true |

Returns a list of matching memory records. Each result carries a `score`: the
raw Qdrant cosine similarity for this query (higher = closer), present when
non-zero. Unranked `list_memory`/`get_memory` results have a zero/omitted score.
Final order may include reranking; `score` remains first-stage dense
similarity and may be non-monotonic after rerank. `citations` are omitted from
the default compact view; pass `full=true` to include them.

On a cross-spine call (`cross_spine=true`), the response also carries
`searched_scopes` — every scope you can read that the search spanned, not the
scopes that produced hits — and `scopes_truncated`, true when that scope
enumeration hit its bounded ceiling and the list may be incomplete. Both keys
are omitted entirely on a scope-confined call, so an existing consumer's
response shape is unchanged.

---

## list_memory

List recent memories in a scope without a query. Intended for session-start
bootstrap. Results are most-recent first. By default returns compact summaries;
pass `full=true` for complete content.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `scope` | string | conditional | The scope to list memories from; required unless `cross_spine` is true |
| `limit` | uint64 | no | Maximum memories to return (default 20) |
| `tags` | string[] | no | Restrict to records carrying **all** listed tags (AND). Omit for no tag filter |
| `categories` | string[] | no | Restrict to records in **any** of the listed categories (OR) — the opposite of `tags`' ALL/AND semantics, since a record carries exactly one category. Omit or pass an empty array for no category filter. An unmatched value returns zero results, never an error; any stored category is accepted, including `discovery` and `rule`, not just the four `store_memory` write values. The same filter is available over the Connect read API on `ListMemories`. |
| `created_after` | string | no | RFC3339 timestamp — include only records with `created_at >= created_after` (inclusive lower bound) |
| `created_before` | string | no | RFC3339 timestamp — include only records with `created_at < created_before` (exclusive upper bound). Half-open window: `[created_after, created_before)` |
| `cursor` | string | no | Opaque pagination cursor from a previous response's `next_cursor`. Omit for the first page. Mutually exclusive with `offset` |
| `full` | bool | no | Return full `content` instead of compact summaries (default `false`) |
| `cross_spine` | bool | no | Span all scopes the caller can read; ignores `scope` when true |

Returns `{ "memories": [...], "next_cursor": "<token>" }`. An empty or absent `next_cursor` indicates the last page.
`citations` are omitted from the default compact view; pass `full=true` to include them.

On a cross-spine call (`cross_spine=true`), the response also carries
`searched_scopes` — every scope you can read that the list spanned, not the
scopes that produced results — and `scopes_truncated`, true when that scope
enumeration hit its bounded ceiling and the list may be incomplete. Both keys
are omitted entirely on a scope-confined call.

Pass an explicit `limit` on a cross-spine list. The underlying total becomes
an exact count across every readable scope rather than one scope (visible as
the Connect API's `total` field), and on the Connect lane an unset limit means
"all" — a caller flipping `cross_spine` on an existing workflow will see the
result count jump and, on Connect, may pull far more than intended.

---

## list_scheduled

List your windowed memories the recall gate is hiding. Active windowed records
surface via `list_memory`/`search_memory`, not here.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `scope` | string | yes | The scope to list scheduled/expired memories from |
| `state` | string | no | `scheduled` (default, not yet active), `expired`, or `all` |
| `limit` | uint64 | no | Maximum memories to return (default 20) |
| `created_after` | string | no | RFC3339 timestamp — include only records with `created_at >= created_after` (inclusive lower bound) |
| `created_before` | string | no | RFC3339 timestamp — include only records with `created_at < created_before` (exclusive upper bound). Half-open window: `[created_after, created_before)` |

Returns the matching hidden windowed records.

---

## get_memory

Fetch one memory by id.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID **or `short_id`** of the memory to fetch |

Returns the full memory record. Authenticated callers can read their own records
plus any `shared` records. Anonymous callers can only read ownerless records.
Fetch-by-id is **not** recall-gated: it returns every state `search_memory`/
`list_memory`/`search_discovery`/`list_scheduled` hide —

- **scheduled** — not yet active (`not_before` in the future)
- **expired** — past its validity window (`not_after` in the past)
- **superseded** — corrected away by [`supersede_memory`](#supersede_memory);
  see that section for the full soft-hide contract
- **archived** — explicitly retired via the `engram spine-review archive` CLI
  command (reversible via `restore`); see
  [`reference/memory-record`](/reference/memory-record/#archiving) for the
  `archived_at` field's contract

`get_memory` always returns `citations` in full — unlike `search_memory`/
`list_memory`, it has no compact view to omit them from.

---

## supersede_memory

Correct one or more memories you own by **superseding** them: stores a single new,
correcting record and marks each target `superseded_by` that new record.
Correction is explicit and preserves history — nothing is deleted or overwritten.
A merge may span different scopes, categories, and visibilities; there is no
requirement that targets be alike.

Takes the full `store_memory` field set for the **new, correcting** record, plus:

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `supersedes` | array of string | yes | One or more ids — each a full UUID or a `short_id` — of the memories this new record corrects. A one-element array is the ordinary single-target case. |

There is no maximum target count — the set is unbounded. Duplicate targets — the
same id given twice, or two spellings of the same record (a short id and its
matching UUID) — collapse to one target; you do not need to dedupe your input
before calling. Each individual entry is bounded at 256 bytes — generous for a
real UUID or `short_id`, which never approaches that length — and an
oversized entry rejects the call before any target is resolved; this bounds
the LENGTH of one entry only and does not reintroduce a cap on how many
targets the set may contain.

Everything else (`content`, `scope`, `category`, `tags`, `summary`, `citations`,
repo/workspace/worktree/base_dir, `source`) describes the new record and behaves
exactly as in [`store_memory`](#store_memory), including `citations` — an
optional array of structured source anchors, never inferred.

**Idempotency.** `idempotency_key` is accepted on this verb — the replay
fingerprint covers the new record's content and the target set together, so the
same `idempotency_key` against a different target set is a conflict, not a
replay. A retry with the same key and the same target set — in any order, with
any duplicates — returns the original result instead of merging again. Because
the replay check runs after target existence and ownership are checked, a retry
whose targets were deleted, or whose ownership changed, since the first call
does **not** replay — it is rejected as not found, the same as any other
addressability failure.

**What changes.** The new record is stored normally and carries a `supersedes`
link to every target; each target gains a `superseded_by` link back to the new
record. All of these are additive payload links — no target's content, tags, or
vector is touched.

**Recall behavior.** A superseded record is soft-hidden from `search_memory`,
`list_memory`, `search_discovery`, and `list_scheduled`, so recall returns only the
current truth. It stays **fully fetchable by id** via [`get_memory`](#get_memory),
which is not recall-gated — so the superseded history remains auditable. This is
one of the four states `get_memory`'s own section lists as recall-hidden-but-fetchable
(scheduled, expired, superseded, archived); **archived** is a separate, independently
maintained state — see [`get_memory`](#get_memory) and
[`reference/memory-record`](/reference/memory-record/#archiving) — never entered or
cleared by this verb.

**If the merge fails partway through.** When stamping `superseded_by` onto every
target fails partway through, the server compensates by removing the new record
and clearing whatever links it managed to leave behind, so a failed merge is
*usually* observably a no-op. Compensation talks to the same store that just
failed, though, so when it cannot complete, the call can leave an orphaned new
record, one or more targets still carrying a link to it, or both — there is no
retry queue and no recovery command; the server logs the affected ids for the
operator, and that log is the only remediation path. Once a merge attempt has
returned and its cleanup succeeded, no partial state remains — but that is a
statement about the state after the call, not about every instant during it: a
concurrent, lock-free read can briefly see a target hidden while the merge is
still in flight.

**Constraints.**

- **Owner-only.** Routes through the ownership *write* gate, per target. A
  `shared` record you can read is **not** one you can supersede; a target you
  do not own, a target that does not exist, and a target whose short id is
  ambiguous (matches more than one record) are all the same rejection —
  indistinguishable from each other — and the response names every offending
  target of the set, not just the first.
- **Single live head.** Superseding an already-superseded record is rejected
  (Connect: `failed_precondition`). This rule applies per target: naming even
  one already-superseded id in the set rejects the whole call, naming every
  such target. Always target the current head — forward chains (C supersedes
  B supersedes A) are how history accumulates, and this makes cycles and
  self-supersession structurally impossible.
- **Never automatic.** No similarity threshold or write-through path ever
  supersedes a record; it is only ever this explicit call.
- **Rules are immutable.** A `store_rule` record cannot be superseded — naming
  one anywhere in the target set rejects the whole call, naming every rule
  target — delete the rule instead (same restriction as
  [`set_visibility`](#set_visibility)).

Returns the new record's `id` and `short_id`.

---

## update_memory

Replace a memory's content in place. The content is re-embedded. Optionally
toggle visibility, replace the tag set, or update the summary. The record's `id`,
`created_at`, and ownership are preserved across the update. **Important:** if
the record has a caller-authored summary (`summary_source=client`), you must
address it when changing content — re-send it (unchanged), update it (revised
summary), or clear it (empty `summary`) — or the update is rejected.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID **or `short_id`** of the memory to update |
| `content` | string | yes | The replacement text (re-embedded) |
| `shared` | bool | no | `true` = shared, `false` = private; omit to keep current visibility |
| `tags` | string[] | no | Replaces the full tag set; an empty array clears all tags. Omit to keep the current tags |
| `summary` | string | no | Replace the summary; empty string clears it. Omit to keep the current summary. When changing `content`, must be addressed if `summary_source=client`. Max `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` bytes (default 512). |

Only the record owner can update. Returns `"updated"` on success.

---

## delete_memory

Delete one memory by id.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `id` | string | yes | The UUID **or `short_id`** of the memory to delete |

Only the record owner can delete. Returns `"deleted"` on success.

Deleting a rule is permitted — this is the only path that retires one, since
`supersede_memory` and un-sharing are both rejected for rules — and no
server-side guard here distinguishes a rule from any other record. Agents
following the `curating-memory` skill are instructed to *propose* a rule's
removal and never perform it unasked; that is an instruction-level gate, not
an enforced one.

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
| `id` | string | no | Omit to create; supply the UUID **or `short_id`** to replace in place |

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

Returns the stored discovery's `id` and `short_id`.

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
| `id` | string | yes | The UUID **or `short_id`** of the memory |
| `shared` | bool | yes | `true` = readable by any authenticated caller; `false` = private |

Only the record owner can change visibility. Sharing grants read, never write.
Returns `"visibility updated"` on success.

---

## store_rule

Persist a **normative rule** (repository/project ground truth) that agents must
follow. An agent may notice a rule candidate and propose it to the user;
`store_rule` is called only after the user says yes — never promote a rule
unilaterally. See the `curating-memory` skill for the recognition triggers and
the proposal protocol. Rules live in a dedicated `rule:repo:*` /
`rule:project:*` scope, are always shared, and surface as a session-start
index.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `content` | string | yes | The full rule text (normative constraint); max 8 KiB |
| `scope` | string | yes | `rule:repo:<repo>` or `rule:project:<project>` |
| `summary` | string | yes | One-line index entry: a single physical line (no newlines), max 256 bytes |
| `tags` | string[] | no | Concern-area labels, e.g. `vcs`, `deploy`, `authz` |
| `id` | string | no | Omit to create; supply the UUID **or `short_id`** to replace in place |

`category` (`rule`), `source` (`user-said`), and `visibility` (`shared`) are all
server-set; do not supply them. The summary is stored as a client-authored
summary. A replace (`id` set to the existing UUID or `short_id`) preserves the
record's existing `short_id`, so handles cited elsewhere keep resolving.
`set_visibility` is rejected for rules — they are always shared — and so is
`supersede_memory`; both correction paths are closed, so retiring a rule
means deleting it.

Returns the stored rule's `id` and `short_id`.

---

## list_rules

List the **complete** rule set for one or more `rule:*` scopes, oldest-first.
Rules are the repository/project's normative ground truth.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `scopes` | string[] | yes | One or more `rule:*` scopes to fetch the complete rule set from |
| `tags` | string[] | no | Restrict to rules carrying **all** listed tags (AND) |
| `full` | bool | no | `true` adds full content; default returns the compact index shape |

The default compact shape is a `ruleView` (`short_id`, `id`, `summary`, `tags`,
`scope`, `created_at`) — note it carries no `content`, so a contradiction or
duplication check needs `full=true`. `full=true` returns the full records.
Ordering is oldest-first (this ascending order is specific to `list_rules`).
A per-scope count above 50 adds a curation-smell advisory to the text result
only — the returned rules payload is unaffected. The advisory is a volume
signal only: it says nothing about duplication or contradiction, and it
cannot fire below 51 rules in a scope.

---

## CLI: summarize-missing

Fill summaries for memories that do not have one (`summary_source=auto`).
Auto-generated summaries are created offline using the configured model.

```bash
engram summarize-missing (--scope <scope> | --all-scopes) [flags]
```

Like the other sweep-style operator commands (`spine-review scan`, `spine-review verify`), this command enforces one constraint: <!-- engram:rule:start sweep-scope-or-all-scopes-required -->a sweep requires an explicit --scope or --all-scopes: name one scope, or opt into every scope<!-- engram:rule:end sweep-scope-or-all-scopes-required -->.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--scope` | string | `""` | Only summarize records in this scope |
| `--all-scopes` | bool | `false` | Sweep every scope (required if `--scope` is omitted) |
| `--older-than` | duration | `0` | Only records created at least this long ago (0 = any age) |
| `--limit` | int | `0` | Max records to scan (0 = no cap) |
| `--dry-run` | bool | `false` | Count eligible records without writing |
| `--timeout` | duration | `30m` | Max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C |

Requires `ENGRAM_SUMMARY_MODEL` environment variable (e.g. `gpt-4o-mini`); the command errors if it is unset.
Creates summaries with `summary_source=auto` and stores them back in place.
Respects `ENGRAM_SUMMARY_MAX_CHARS` (default 280) for summary length.
