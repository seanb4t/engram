---
name: curating-memory
description: Use when storing or updating durable project memory via the engram MCP tools — enforces the engram-vs-beads routing gate (engram is preferred over `bd remember`/`bd memories` for durable facts), durable-only capture, search-before-store, supersede-on-contradiction, and the two-tier spine/overlay scope. Trigger when the user states a durable decision/preference/convention/gotcha, when the user explicitly asks to remember something (including a time-bound reminder, due date, or "not before"/expiry — even if it looks task-shaped), whenever you are about to record a durable fact and the repo also has a beads memory store (prefer engram; do not write it to `bd remember`), on the session-start recall and capture nudges, and before any mcp__engram__store_memory / schedule_memory / update_memory / delete_memory call.
---

# Curating Memory

The memory store is **explicit and zero-junk**: only deliberately chosen durable
facts live in it, and it stays correct over time. Apply this discipline before
every memory write.

## Routing: is this an engram memory at all?

Decide *where* the fact belongs **before** applying the taxonomy below:

- **A task to *do*** — a work item or action ("drain hl-kt1r", "fix the flaky
  test", "ship the release") → that is a **bead**, not a memory. Route it to the
  issue tracker; engram does not track work.
- **A durable fact about the repo/project** — a decision, preference,
  convention, or gotcha → engram memory. Continue below.
- **A durable fact in a repo that *also* has beads memory** (`bd remember` /
  `bd memories`) → **still engram.** engram is the preferred durable-memory store;
  it wins over beads memory. Write the fact to engram, search engram first, and
  do **not** split durable facts across both stores or mirror them into
  `bd remember`. Beads memory is a read-only fallback for *recall* only when
  engram is unavailable (401/403). Note the asymmetry that makes this easy to get
  wrong: `bd prime` *auto-injects* its memories at session start, so memory can
  *look* covered before you have read engram at all — it is not. Pull engram
  explicitly, and capture into engram the moment a fact is established rather than
  storing in beads and "lifting" later.
- **An explicit ask to remember with a time bound** — "remember X by/until/after
  `<when>`", a due date, a deferred reveal → engram, via `schedule_memory`. The
  explicit *remember* plus the time window is the durability signal; do **not**
  drop it as transient. See Scheduling.
- **Unclear** — task-shaped *and* an explicit timed "remember" (e.g. "remember
  to drain hl-kt1r tonight or tomorrow"), or the scope is ambiguous → **ask /
  offer the choice** (bead vs scheduled memory). Never silently pick one.

## Junk taxonomy

**STORE (durable):** decisions, preferences, conventions, gotchas, and
project-specific facts that outlive the session.

**DO NOT STORE:** transient state, current activity/progress, secrets or API
keys, timestamps, one-off tool output, or anything trivially re-derivable.

## Tagging

`tags` are now a recall dimension, not just display metadata: `search_memory`
and `list_memory` accept an optional `tags` filter that narrows results to
records carrying **all** listed tags (AND), and `update_memory` can correct a
record's tag set after the fact. So tag deliberately — a tag is only useful for
recall if it is applied **consistently** across the records that share it.

`search_memory`, `list_memory`, and `list_scheduled` also accept optional
`created_after` / `created_before` (RFC3339, half-open `[after, before)`) to
window recall by creation time. `list_memory` additionally accepts a `cursor`
arg (opaque, from `next_cursor`) for deterministic pagination and returns
`{ "memories": [...], "next_cursor": "<token>" }` — an empty `next_cursor`
signals the last page.

Stay semantic-first, though: tags **complement** vector recall, they don't
replace it. Reach for a tag when you want a precise, boolean axis the content
text can't express (e.g. a project/component label, a `decision` vs `gotcha`
distinction already covered by `category`, a transient-vs-durable marker) — not
to rebuild a folksonomy the embedder already handles. Prefer a handful of stable,
low-cardinality tags over an ever-growing taxonomy; when in doubt, rely on
content + semantic search and leave the record untagged.

## Discipline

1. **Search before store.** Call `mcp__engram__search_memory` across both
   the spine and (if present) the workspace overlay first. `search_memory` is
   backed by a semantic/vector engine, so query it with a natural-language
   description of the fact (not keyword fragments) — it surfaces conceptually
   related records even when they share no exact wording. If a near-duplicate
   exists, update it instead of adding a new record.
2. **Supersede on contradiction — within a tier.** When new info conflicts with
   an existing memory, `update_memory` (preferred) or `delete_memory` the stale
   record. Do **not** treat a spine fact and a divergent workspace-overlay fact
   as a contradiction — they are parallel truths by design.
3. **Tier selection.** Default to the **spine** (`Memory spine scope` from
   session start) — most durable facts are repo-wide and should follow the user
   into every workspace. Store to the **overlay** (`Memory workspace scope`)
   only when a fact is genuinely local to this line of work and would be wrong
   or premature elsewhere (e.g. an in-flight decision that contradicts main
   until merged). Promotion of overlay facts to the spine when work merges is
   the `promoting-memory` skill.
4. **Provenance.** Set `source` honestly (`user-said` vs `agent-inferred`). Do
   not set `actor` — it is assigned server-side from the validated OAuth token.

## Scheduling (temporal validity windows)

Use `schedule_memory` instead of `store_memory` when a durable fact should not be
recalled *yet*, or should stop being recalled *after* a point in time. It takes
the same fields as `store_memory` plus a validity window — at least one of
`not_before` (RFC3339; hidden from recall until then, a deferred reveal) or
`not_after` (RFC3339; dropped from recall at then, an expiry). Search-before-store
still applies. The junk-taxonomy "transient" exclusion targets *incidental* state
the user never asked to keep — an explicit request to remember something
by/until/after a time is itself durable (the ask is the signal), so schedule it
rather than discarding it. Scheduling controls *when* a durable fact is active,
not *whether* it is durable. Discoveries are not schedulable.

A windowed record inside its active window surfaces normally through
`search_memory` / `list_memory`. Outside that window the recall tools hide it,
and `list_scheduled` lists only those hidden records (`state`: `scheduled`
default | `expired` | `all`) — never the active ones, so an in-window record
absent from `list_scheduled` is reached through ordinary recall, not missing.
Recall is gated, but fetch-by-id (`get_memory`) is not. Operators reclaim lapsed
records with the `engram prune-expired [--older-than DUR]` CLI.

## Summaries

Pass `summary` on `store_memory` or `update_memory` when you have explicit
context on the caveat-bearing essence of the fact — `summary_source=client`.
Recall returns compact summaries by default to keep the spine small; before
acting on caveats or edge cases visible only in the summary, call `get_memory`
(id fetch, always returns full content) or pass `full=true` on `search_memory`/`list_memory`.
When `summary_source=auto` (offline-generated), the summary is lossy — rely
on it only as orientation. Changing a memory's content while a caller-authored
summary (`summary_source=client`) is present requires you to address it:
re-send it (unchanged), update it (revised summary), or clear it (send empty
`summary`) — otherwise the update is rejected.

## Tools and auth

All tools are on the `engram` server: `mcp__engram__store_memory`,
`…__schedule_memory`, `…__search_memory`, `…__update_memory`,
`…__delete_memory`, `…__list_memory`, `…__list_scheduled`, `…__get_memory`. If a
call returns 401/403 the server is not authenticated —
tell the user to authenticate via `/mcp` (engram → Authenticate), and
restate the durable fact so they can re-store it after authenticating; never
drop it silently.
