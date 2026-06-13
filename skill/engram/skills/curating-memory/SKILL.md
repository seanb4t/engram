---
name: curating-memory
description: Use when storing or updating durable project memory via the engram MCP tools — enforces durable-only capture, search-before-store, supersede-on-contradiction, and the two-tier spine/overlay scope. Trigger when the user states a durable decision/preference/convention, on the session-end capture nudge, and before any mcp__engram__store_memory / schedule_memory / update_memory / delete_memory call.
---

# Curating Memory

The memory store is **explicit and zero-junk**: only deliberately chosen durable
facts live in it, and it stays correct over time. Apply this discipline before
every memory write.

## Junk taxonomy

**STORE (durable):** decisions, preferences, conventions, gotchas, and
project-specific facts that outlive the session.

**DO NOT STORE:** transient state, current activity/progress, secrets or API
keys, timestamps, one-off tool output, or anything trivially re-derivable.

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
`not_after` (RFC3339; dropped from recall at then, an expiry). The junk taxonomy
and search-before-store discipline still apply: scheduling controls *when* a
durable fact is active, not *whether* it is durable. Discoveries are not
schedulable.

A windowed record inside its active window surfaces normally through
`search_memory` / `list_memory`. Outside that window the recall tools hide it,
and `list_scheduled` lists only those hidden records (`state`: `scheduled`
default | `expired` | `all`) — never the active ones, so an in-window record
absent from `list_scheduled` is reached through ordinary recall, not missing.
Recall is gated, but fetch-by-id (`get_memory`) is not. Operators reclaim lapsed
records with the `engram prune-expired [--older-than DUR]` CLI.

## Tools and auth

All tools are on the `engram` server: `mcp__engram__store_memory`,
`…__schedule_memory`, `…__search_memory`, `…__update_memory`,
`…__delete_memory`, `…__list_memory`, `…__list_scheduled`, `…__get_memory`. If a
call returns 401/403 the server is not authenticated —
tell the user to authenticate via `/mcp` (engram → Authenticate), and
restate the durable fact so they can re-store it after authenticating; never
drop it silently.
