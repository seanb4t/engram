<!--
SPDX-License-Identifier: Apache-2.0
-->
# Design: `rule` memory kind — normative, always-indexed ground truth

- **Bead:** `engram-3jo0`
- **Date:** 2026-07-06
- **Status:** Draft (pending design review)

## Problem

Engram's curated categories (`decision` / `preference` / `convention` /
`gotcha`) are all advisory: nothing distinguishes a binding invariant
("never push to main directly") from a soft preference. Three failure modes
follow:

1. **No normative weight.** An agent reading recall output cannot tell a
   MUST-follow rule from a nice-to-know note.
2. **Recency-windowed surfacing.** The session-start hook digests the most
   recent ~10 records per scope. An invariant stored months ago scrolls out
   of the bootstrap window and is silently absent from new sessions.
3. **No enumerable rule set.** `search_memory` is vector-ranked and
   `list_memory` is recency-paged; there is no way to ask for *the complete,
   authoritative rule set* for a repo or project.

## Goals

- A new record type for **normative rules**: user-blessed, always-shared
  ground truth attached to a repo or project scope.
- **Guaranteed surfacing with progressive disclosure**: every session sees a
  compact one-line-per-rule index (like CLAUDE.md rules or skill
  `description` frontmatter); full text is fetched on demand — never bulk
  injected. Context cost ≈ 1 line per rule, 0 when no rules exist.
- **Discrete enumeration**: one call returns the complete rule set for a
  scope, in stable order.
- **Discrete addressing**: each rule is referenced, fetched, and corrected by
  its `short_id` handle (engram-c0yl).
- Purely additive: no store schema change, no migration, curated tool
  contracts untouched.

## Non-goals

- **Ground-truth facts.** This feature is normative rules only. Factual
  invariants ("prod embeds via qwen3-embedding-8b") remain ordinary memories
  or discoveries. A `kind: rule|fact` enum was considered and rejected —
  revisit only if a real need appears.
- **Per-memory usage/hit tracking** — split to its own design (bead
  `engram-qx0d`).
- **Console Rules view.** Rules appear as `rule:*` scope cards in the
  operator console as-is; a dedicated tab is a follow-up bead.
- **Proto/Connect changes.** Connect `ListMemories` already reaches rules by
  scope filter.
- **Auto-promotion** of existing conventions into rules. Promotion is a
  manual re-store on user instruction.
- **Auto-extraction** of any kind (unchanged design intent).

## Decision: record shape

`rule` is the **6th `category`** value on the existing `Memory` record —
same single Qdrant collection, no new fields, no migration (precedent: ADR
`engram-2bv`, discovery as 5th category).

A dedicated scope prefix isolates rules from normal recall (precedent:
`discovery:repo:*`):

- `rule:repo:<repo>` — e.g. `rule:repo:github.com/seanb4t/engram`
- `rule:project:<project>` — e.g. `rule:project:selfhosted-cluster`

Rules never appear in `search_memory` / `list_memory` over ordinary scopes,
never occupy recent-10 digest slots, and never compete in vector ranking.
They are already guaranteed in context via the index (below), so semantic
findability adds nothing.

This isolation is enforced by **scope-prefix separation** — the same posture
`discovery:*` already relies on. Note that `category` is not a
server-enforced enum today (`store_memory`'s `category` is a jsonschema hint,
not a validated field; `schedule_memory` special-cases only
`category=="discovery"`), so the guarantee is "rules live in `rule:*` scopes
and callers query the scope they mean," not an airtight category firewall.
This is precedent-consistent with discovery, not a new gap; hardening
category into an enforced enum across all record types is out of scope here.

Server-set at store time (never client-supplied):

| Field | Value | Why |
|-------|-------|-----|
| `category` | `rule` | taxonomy |
| `source` | `user-said` | rules are user-blessed by contract |
| `visibility` | `shared` | a rule IS the scope's shared ground truth |

Field semantics:

| Field | Role |
|-------|------|
| `summary` | **Required.** The one-line index entry (the "frontmatter" line) |
| `content` | Full rule text; bounded (`maxRuleContentBytes` = 8 KiB) |
| `tags` | Concern-area labels (`vcs`, `deploy`, `authz`) — surfaced in the index, filterable |

**Authorship contract (user-blessed only).** The server cannot verify a
human said something; the invariant is enforced the same way `source`
semantics are enforced everywhere else in engram — by tool contract and
skill discipline. The `store_rule` tool description and the curating-memory
skill state: agents call `store_rule` only on explicit user instruction. An
agent that believes something should be a rule **proposes** it to the user;
it never promotes unilaterally.

**Visibility invariant (always shared).** `visibility=shared` is server-set
and immutable for rules. Anyone who can read the scope reads the same rule
set (existing `ownerOrSharedCondition` grant — zero new authz code); writes
remain owner-only (ADR `engram-kyz`). `set_visibility` on a rule is
**rejected** with a clear error ("rules are always shared — delete the rule
instead"): a privately-hidden rule would let two actors see different ground
truth in the same scope. Personal "rules to self" belong in ordinary
`preference` memories. Inherited caveat (documented, unchanged semantics):
with auth disabled, rules live in the anonymous `owner==""` bucket;
anonymous callers cannot read authenticated owners' shared records, so mixed
anonymous/authenticated deployments see different rule sets — the existing
shared-read posture, not a new behavior.

## Decision: tools

Dedicated tool pair, mirroring ADR `engram-0gy` (dedicated discovery tools):
the curated tool contracts stay frozen, and each tool's signature fully
describes a valid call.

### `store_rule`

```text
store_rule(content, scope, summary, tags?, id?) → {id, short_id}
```

Validation (`validateStoreRule`, pure function, unit-tested without Qdrant):

- `scope` must match `rule:repo:*` or `rule:project:*` (non-empty tail).
- `content` required; ≤ `maxRuleContentBytes` (8 KiB).
- `summary` required; **single physical line** — any `\n` / `\r` is
  rejected, never silently normalized (explicit/correctable ethos: munging
  user input hides the problem); ≤ `maxRuleSummaryBytes` (256 B) so one rule
  ≈ one terminal line in the index.
- `id` optional for replace-in-place (same idiom as `store_discovery`).

Handler: embeds `store.EmbedText(content, tags)` (rules live in the same
collection; points require vectors), assembles `Memory` with the server-set
fields above, `SummarySource=client`, and Upserts.

### `list_rules`

```text
list_rules(scopes[], tags?, full=false) → {rules}
```

- `scopes` — one or more `rule:*` scopes (repo + project fetched in one
  call). Each scope validated against the same prefix rule.
- Returns the **complete** rule set across the given scopes, ordered
  `created_at` ascending (stable, oldest-first like a numbered rule list).
  **This oldest-first ordering is an MCP-`list_rules` guarantee only.**
  Connect `ListMemories` / the operator console browse `rule:*` scopes with
  the store's default newest-first order (see "Store layer" below); the
  numbered-list reading is a `list_rules` property, not a global one.
- Compact by default: `{short_id, id, summary, tags, scope, created_at}`.
  `full=true` opt-in adds `content` (mirrors recall-summary convention, ADR
  `engram-ambu`).
- `tags` filter: AND semantics, same as `list_memory`.
- **No pagination.** A rule set is definitionally small. If a scope exceeds
  a soft threshold (50), the tool's human-readable `textResult` line carries
  a curation-smell advisory (e.g. "47 rules in <scope> — consider
  consolidating"). This is a `textResult` note only — the structured
  `{rules}` payload shape is unchanged, so no wire field is added.

### By-id operations: no new tools

`get_memory` / `update_memory` / `delete_memory` already serve any record
by id; short-id addressing arrives with engram-c0yl (`ResolvePointID` at
every by-id handler). The correction loop is: index shows `short_id` →
`get_memory(short_id)` → `update_memory(short_id, ...)`.

**Category-aware update guard.** The `update_memory` handler already
fetches the current record for the summary-addressing guard (ADR
`engram-ddiw`), so it knows `category==rule` at no extra cost. For rules it
additionally rejects:

- a replacement summary containing `\n` / `\r` or exceeding
  `maxRuleSummaryBytes`;
- **clearing** the summary (empty-string-clears is legal for ordinary
  memories; a rule without its index line is invalid).

The existing ddiw guard (content change must address a client summary)
applies to rules unchanged — and since every rule has a client summary, a
rule's content can never drift from its index line unnoticed.

**`set_visibility` rejection lives in the handler.** `store.SetVisibility`
(store.go) calls `getWritable` but discards the fetched `Memory` and issues
a Qdrant `SetPayload` — there is no pure, Qdrant-free validation seam in that
path (unlike `update_memory`, whose handler passes the fetched record to
`resolveSummaryUpdate`). The guard therefore lives in the **MCP
`set_visibility` handler** and requires a **new record read** the handler
does not perform today: `ResolvePointID` (engram-c0yl) returns only the
canonical UUID (`WithPayload(false)` — no `category`), and the handler
otherwise calls straight through to `store.SetVisibility`. So the handler
gains one `GetReadable(ctx, pid, subj)` before dispatch; if
`category==rule`, it rejects with a clear error ("rules are always shared —
delete the rule instead"). Because the check reads the record, it is
exercised by an **integration** test (needs Qdrant), not a pure unit test —
the Testing section reflects that. (Contrast `update_memory`, where the
fetched `Memory` is already in hand from the ddiw guard, so the rule check
there is genuinely free.)

*Implementation-order note for the plan author:* `GetReadable` is
owner-or-shared, so running the rule check first means a **non-owner**
calling `set_visibility` on another actor's rule gets the rule-specific
rejection rather than the owner-only `ErrNotFound` that `store.SetVisibility`
would otherwise return. This is not a leak (rules are unconditionally
readable already), but decide deliberately whether the `category==rule`
rejection runs before or after the write-ownership gate — the intended
behavior is category-first (the message "rules are always shared" is more
useful than a not-found).

## Decision: session-start surfacing (progressive disclosure)

The `session-start-memory-recall` hook gains one instruction block: call
`list_rules` once for `rule:<spine>` (plus the project scope when
configured — below) and render a **Rules index** as a distinct section above
the recent-10 digest:

```text
Rules (fetch full text via get_memory(<short_id>) before working in a rule's concern area):
- 0abc123xyz — never push to main directly; PRs only [vcs]
- 1def456uvw — all Go/MD files carry the Apache-2.0 SPDX header [license]
```

One terse line per rule: `short_id — summary [tags]`. Full text is pulled
on demand via `get_memory(short_id)` when the agent is about to act in a
rule's concern area. No bulk injection; storage guarantees the summary is
single-line, so the renderer needs no truncation logic. When `list_rules`
returns empty, the section is omitted entirely.

## Decision: project-scope configuration

The hook derives the repo spine from cwd but cannot infer project
membership (project scopes are a client-side naming convention; the server
does not model project→repo edges). The hook lib today
(`skill/engram/hooks/lib/`) reads **no** configuration — `scope.py` derives
the spine/workspace purely from `jj`/`git`, and there is no config-file or
env lookup anywhere in `skill/engram/hooks/`. V1 therefore introduces a
single env var, **`ENGRAM_PROJECT`**: when set, the hook includes
`rule:project:<name>` in the `list_rules` call. Unconfigured = repo rules
only. This is a **client-side hook env var, read directly by the Python
hook — not part of the `internal/config` (koanf) `ENGRAM_` registry** that
governs the Go server (same posture as the existing `ENGRAM_MCP_URL` /
`ENGRAM_MCP_PATH` client-side vars). A config-file mechanism (settings path,
plugin config) is **new work, explicitly out of scope**; so is modeling
project→repo membership server-side.

## Store layer: one additive option

`store.List` (engram-nx2t) already provides filtered, `order_by created_at`,
complete listing with the authz condition as outer Must; `payload()` /
`fromPayload()` need nothing (no new fields). `list_rules` is a thin handler
over `store.List` per scope with the category filter. The one store change:
`store.List` orders `created_at` **descending** today (recency-first
recall), so `ListOptions` gains an additive `Ascending bool` that
`list_rules` sets for stable oldest-first ordering.

## Sequencing

Depends on **engram-c0yl** (short-id handles): the index format and the
get/set-by-handle correction loop want `short_id` from day one. Rules work
is sequenced **after** c0yl lands. (The tools would function UUID-only, but
shipping the index with 36-char UUIDs defeats the one-line budget and would
change the rendered contract later.)

## Testing

- **Unit (no Qdrant):** `validateStoreRule` — scope prefix, required
  content/summary, newline rejection, byte caps; `update_memory` rule-guard
  cases (newline summary, oversize summary, summary clear) — these validate
  the fetched-record `cur` in a pure Go function, same seam as
  `resolveSummaryUpdate`.
- **Integration (Qdrant):** `store_rule` server-set fields round-trip
  (`category=rule`, `source=user-said`, `visibility=shared`);
  `set_visibility` rejection for `category==rule` (the handler pre-fetches the
  record, so this needs a live/fake store — not a pure unit test);
  `list_rules` completeness + ascending order + compact/full shapes +
  multi-scope + tags-AND + the >50 `textResult` curation advisory (structured
  `{rules}` payload unaffected); cross-actor read (actor B reads actor A's
  rules via shared); anon-bucket isolation (mirrors
  `TestAnonBucketDiscoveryReadIsolation`); scope isolation (rules invisible to
  `search_memory`/`list_memory` over the ordinary repo scope).
- **Hook:** rules-index instruction present with the spine-derived rule
  scope; project scope included iff configured; index section omitted when
  no rules exist.

## Alternatives considered

- **Rules in the ordinary repo/project scope (category filter only).**
  Searchable semantically and visible in existing console cards, but rules
  would occupy recent-10 digest slots (duplicating the always-injected
  index) and "discrete" degrades to "filtered." Rejected: scope-prefix
  isolation is strictly cleaner given the index guarantees presence.
- **`pinned` flag on existing memories.** Minimal machinery, but conflates
  surfacing with taxonomy, has no user-blessed contract, and grows
  `store_memory` conditionally — the exact shape ADR `engram-0gy` rejected.
- **`kind: rule|fact` enum (discovery-style).** Facts declared out of scope;
  an unused enum arm is junk. Additive later if needed.
- **Full-content injection at session start.** Rejected for context bloat;
  progressive disclosure (index + fetch-on-demand) chosen instead.
- **Default-shared (overridable) instead of always-shared.** Rejected: a
  private rule contradicts "one rule set per scope"; personal rules belong
  in `preference`.
- **Silently normalizing multi-line summaries.** Rejected: explicit,
  correctable — reject with a clear error instead of munging.
<!-- adr-capture: sha256=a5c1d39cedf53dd4; session=cli; ts=2026-07-06T22:18:41Z; adrs=engram-iedk,engram-d386,engram-m4s8 -->
