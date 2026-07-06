---
name: migrating-from-beads
description: Use to migrate durable project memories out of a beads memory store (`bd remember` / `bd memories`) into the engram spine, one time, when adopting engram in a repo that already used beads for memory. Trigger on "migrate beads memories to engram", "import bd remember into engram", "adopt engram in this repo", "move bd memories to engram", or when you notice a repo carries durable facts in both `bd remember` and engram and the split should be reconciled. Migrates *memories* only — never beads *issues*. Pairs with curating-memory (routing + junk taxonomy) and promoting-memory (the overlay analogue).
---

# Migrating from Beads

engram is the preferred durable-memory store; beads memory (`bd remember` /
`bd memories`) is a read-only fallback (see the engram-vs-beads routing gate in
`curating-memory`). When a repo already kept durable facts in beads, this skill
**moves** them into the engram spine once and removes the beads copy, so durable
memory stops being split across two stores. It is the beads analogue of
`promoting-memory` (which graduates a workspace overlay into the spine).

Migration is deliberate and user/model-mediated — there is no automatic sync.
Run it once per repo at adoption, not on a schedule.

## Scope: what migrates

- **Migrate:** `bd remember` memories — durable decisions, preferences,
  conventions, gotchas, project facts that outlive a session.
- **Never migrate:** beads *issues* (`bd create` / `bd ready`). Those track work
  to *do*, not memory; they stay in beads. engram does not track work.
- **Drop, do not migrate:** transient state, progress notes, secrets, or
  anything trivially re-derivable — apply the `curating-memory` junk taxonomy.

## Workflow

1. Confirm the repo has **both** stores. Use the `Memory spine scope` (and any
   `Memory workspace scope`) line from session start for the engram target. If
   engram returns 401/403, it is unauthenticated — tell the user to `/mcp`
   (engram → Authenticate) and stop; do not delete any beads memory.
2. Enumerate every beads memory: `bd memories` with no filter (or read the
   "Persistent Memories" block already printed by `bd prime`). Note each
   record's `--key` — you need it for the `bd forget` step.
3. For each beads memory, decide with the user:
   - **Migrate** — durable and repo-wide. First
     `mcp__engram__search_memory(<spine>, "<natural-language description>")` to
     dedup (it is semantic — a paraphrase surfaces a near-duplicate). If a
     near-duplicate exists, `mcp__engram__update_memory` that record (merge the
     two) instead of adding a new one; otherwise
     `mcp__engram__store_memory(<spine>, …)`. Default to the **spine**; use the
     overlay only for a fact genuinely local to this line of work.
   - **Drop** — transient, stale, or junk; do not store it.
4. **Complete the move.** Only after the engram write succeeds, `bd forget <key>`
   the original so the fact lives in one store, not two. If the engram write
   failed, leave the beads copy in place and report it — never `bd forget` a
   record you could not store.
5. Report a summary: each beads key → engram id (its `short_id` is the
   compact handle to cite, or "deduped into <id>" / "dropped"), and confirm
   which beads keys were forgotten.

## Why move, not copy

Keeping both copies recreates the exact split the engram-vs-beads precedence
rules forbid: `bd prime` keeps auto-injecting the beads copy at session start, so
the two records drift and a reader can't tell which is authoritative. Moving
makes engram the single source of truth; the strengthened session-start recall
hook ensures the migrated facts still load every session via `list_memory`.

Keep the spine zero-junk: migrate only facts that are genuinely durable and
repo-wide, applying the same junk taxonomy as `curating-memory`.
