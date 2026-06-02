---
name: promoting-memory
description: Use when a line of work completes (merges, lands, or is abandoned) to graduate a workspace's overlay memories into the repo spine and clean up. Trigger on "promote memories", "merge workspace memories", "clean up this workspace's memories", or when finishing/merging a branch. Pairs with dev-flow:finishing-a-development-branch.
---

# Promoting Memory

Capture-time tier selection (the `curating-memory` skill) decides where a *new*
fact goes. This skill reconciles an *existing* workspace overlay against the
repo spine at the natural end of the work. Promotion is deliberate and
user/model-mediated — there is no automatic merge-triggered migration.

## Workflow

1. Use the `Memory spine scope` and `Memory workspace scope` lines from session
   start. If there is **no** workspace scope (primary checkout), there is
   nothing workspace-local to promote — stop.
2. `mcp__memory_oauth__list_memory(<overlay scope>)` to enumerate this
   workspace's local memories. (401/403 → server unauthenticated; tell the user
   to `/mcp` Authenticate and stop.) If empty, report "nothing to promote".
   All tools are on the `memory_oauth` server; below, `…__` abbreviates the
   `mcp__memory_oauth__` prefix.
3. For each overlay memory, decide with the user:
   - **Promote** — now true repo-wide. `…__search_memory(<spine>, …)` first for a
     duplicate/contradiction; then `…__store_memory(<spine>, …)` (or
     `…__update_memory` the spine record on contradiction); then
     `…__delete_memory` the overlay copy.
   - **Keep** — still genuinely work-local (rare once merged); leave it.
   - **Drop** — no longer relevant; `…__delete_memory`.
4. Once the workspace is being retired, offer `…__delete_all(<overlay scope>)` as
   a teardown after promotions are done.

Keep the spine zero-junk: promote only facts that are genuinely durable and
repo-wide, applying the same junk taxonomy as `curating-memory`.
