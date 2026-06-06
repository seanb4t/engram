---
name: discovering
description: Use when mapping or investigating a repository/codebase to cache agent-earned understanding as citation-backed discoveries via engram's store_discovery tool. Trigger on "map this repo", "help me understand this codebase", onboarding to unfamiliar third-party code, or before substantial work in an unmapped area. Pairs with search_discovery for on-demand recall.
---

# Discovering

A **discovery** caches understanding you earned by reading code — the expensive
re-derivation you would otherwise repeat next session. Its value is the work it
saves, so the bar is simple: **store a discovery only when re-deriving it would
cost meaningful tokens.** Discoveries are separate from the curated four memory
types (decision / preference / convention / gotcha) and never load at session
start — they are pulled on demand.

## When to capture

- Tracing how an unfamiliar subsystem or third-party dependency works.
- Orientation worth keeping: where things live, how a flow connects.
- A behavioral fact that is costly to re-derive and risky to get wrong.

Do **not** capture: anything trivially re-read in one file, transient state,
secrets, or restating the curated four. Capture is explicit — never
auto-extract.

## kind: map vs fact

- **map** — orientation: structure, where things live, how flows connect.
  Broader; commit-SHA pins are enough.
- **fact** — a pinned, checkable behavioral claim. Tighter; pin a content-hash
  of the cited region so a later reader can detect that *those exact lines*
  changed.

## Citations are mandatory

Every discovery carries **>= 1 citation** — that is what makes it trustworthy
and ageable. For each citation capture:

- `kind`: file | commit | url | repo
- `ref`: path / repo URL / doc URL
- `locator`: line range for files
- `pin`: the aging anchor captured now — content-hash (fact files), commit SHA
  (map files), `@rev` (repo), or fetched-at (url)
- `excerpt`: the cached substance — keep the few lines worth not re-fetching.
  Soft cap **~50 lines**; exceed only with explicit reason.

## Workflow

1. **search-before-store.** Run `search_discovery` for the area first (a
   natural-language description — it is semantic). If a near-duplicate exists,
   call `store_discovery` with that record's `id` to replace it rather than
   adding a duplicate.
2. Explore breadth-first; for each meaningful unit decide map vs fact.
3. Capture citations (pins + excerpts) as you read.
4. `store_discovery(content, kind, citations[], scope="discovery:repo:<repo>", summary?, tags?, id?)`
   — omit `id` to create; pass the near-duplicate's `id` (from step 1) to replace
   it in place rather than adding a new record.

## Recall (the other half)

When entering mapped territory later, issue a targeted `search_discovery` scoped
to `discovery:repo:<repo>`. Pass `cross_spine=true` only when you deliberately
want to span every discovery scope. The result carries each citation's `pin` and
the record's `created_at` — render trust from those (age, pinned commit, whether
the cited code has since moved); the server stores signals, never a verdict.
