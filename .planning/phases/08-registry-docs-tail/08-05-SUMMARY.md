---
phase: 08-registry-docs-tail
plan: 05
subsystem: docs
tags: [claude-md, routing-doc, cobra, deprecation, recall-gate, gap-closure]

# Dependency graph
requires:
  - phase: 08-registry-docs-tail (plan 04)
    provides: CLAUDE.md's migrations convention, Layout row inventory, and Archived-state paragraph as landed pre-repair
provides:
  - CLAUDE.md's cmd/engram/ Layout row annotates every deprecated command the goldens derive (backfill-short-ids and migrate-set-owner), not just one
  - CLAUDE.md's Archived-state paragraph names all four soft-hide recall surfaces (search_memory/list_memory/search_discovery/list_scheduled), matching the Supersession paragraph and the four live archived_at gate sites in internal/store/store.go
affects: [08-registry-docs-tail verification, future CLAUDE.md edits to the Layout row or Memory-contract section]

actuals:
  tokens: 900
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Derived-set verification gates for markdown routing docs: read the deprecated-command set from committed cobra goldens (catalog.golden vs help.golden) rather than hardcoding names, and cross-check paragraph surface lists against a sibling paragraph plus a live grep-count of the enforcing code, so future drift trips the gate instead of a hand-written literal going stale."

key-files:
  created: []
  modified:
    - CLAUDE.md

key-decisions:
  - "Left the migrate-remap-owner / migrate-set-owner \"alias\" wording untouched — 08-VERIFICATION.md records it as info-level-only and correct (the deprecated-supersession relationship is real and pinned by TestMigrateSetOwnerEquivalentToRemapOwnerMissing); two prior reviewers wrongly flagged it by comparing flag shapes."
  - "Kept both edits to single sentences/phrases within their existing paragraphs rather than rewriting either section — the plan's own scope-discipline truth required 08-04's landed prose to survive byte-identical apart from the two named corrections."

requirements-completed: [REQ-claude-md-migrations-convention]

coverage:
  - id: D1
    description: "cmd/engram/ Layout row marks every deprecated command derived from the committed goldens (backfill-short-ids and migrate-set-owner), not just one"
    requirement: "REQ-claude-md-migrations-convention"
    verification:
      - kind: other
        ref: "bash gate deriving deprecated set from catalog.golden/help.golden diff, asserting zero unannotated names in the Layout row"
        status: pass
      - kind: other
        ref: "08-04's row-scoped inventory gate (per-word backtick presence check against catalog.golden)"
        status: pass
      - kind: other
        ref: "08-04's no-changelog-phrasing gate (rejects 'new in'/'as of'/'shipped in'/'this release' in the diff)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Archived-state paragraph names all four soft-hide recall surfaces, agreeing with the Supersession paragraph and the store's live archived_at gate-site count"
    requirement: "REQ-claude-md-migrations-convention"
    verification:
      - kind: other
        ref: "bash gate: symmetric set difference between Archived and Supersession paragraph surface lists, plus count match against comment-stripped qdrant.NewIsEmpty(\"archived_at\") occurrences in internal/store/store.go"
        status: pass
      - kind: other
        ref: "no-changelog-phrasing gate"
        status: pass
      - kind: other
        ref: "git diff --name-only scoped outside .planning/ reports CLAUDE.md alone"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-08-21
status: complete
---

# Phase 08 Plan 05: CLAUDE.md Gap Closure (Deprecation Marking + Archived-State Recall Surfaces) Summary

**Repaired two confirmed factual defects in CLAUDE.md: an incomplete deprecation marking in the `cmd/engram/` Layout row and an understated four-surface recall gate in the Archived-state paragraph, each closed with a derived (not hardcoded) verification gate.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-08-22T00:32:00Z (approx.)
- **Completed:** 2026-08-22T00:36:11Z
- **Tasks:** 2
- **Files modified:** 1 (`CLAUDE.md`)

## Accomplishments

- `cmd/engram/` Layout row's `backfill-short-ids` item now carries `(deprecated, use `migrate`)`, matching the annotation density already used for `migrate-set-owner`. Repairs 08-VERIFICATION.md truth 11 / D-05.2.
- Archived-state paragraph's soft-hide surface list expanded from two (`search_memory`/`list_memory`) to all four (`search_memory`/`list_memory`/`search_discovery`/`list_scheduled`), matching the adjacent Supersession paragraph and the four live `qdrant.NewIsEmpty("archived_at")` gate sites in `internal/store/store.go`. Repairs 08-VERIFICATION.md truth 12 / D-05.3.
- Both gates were observed RED against `HEAD` before their respective edits, confirming the defects were real rather than assumed.

## Observed RED Pre-States (recorded before each edit)

**Task 1 gate (deprecated-set derivation):**
```
deprecated set:backfill-short-ids migrate-set-owner
unannotated: backfill-short-ids
```
Derived set matched the plan's prediction exactly. `backfill-short-ids` was the sole unannotated item.

**Task 1 regression gates (08-04's, both already green pre-edit and stayed green):**
```
inventory misses: []
no-changelog-phrasing: 0
```

**Task 2 gate (set-difference + gate-site count):**
```
archived paragraph: list_memory search_memory
supersession paragraph: list_memory list_scheduled search_discovery search_memory
symmetric difference: [list_scheduledsearch_discovery]
store gate sites: 4 / surfaces named: 2
```
Confirmed the paragraph named only 2 of the 4 surfaces the store actually gates, matching the plan's predicted pre-state.

## Post-Edit GREEN States

**Task 1:**
```
deprecated set:backfill-short-ids migrate-set-owner
unannotated:
GATE1_PASS
inventory misses: []
GATE2_PASS
GATE3_PASS
```

**Task 2:**
```
archived paragraph: list_memory list_scheduled search_discovery search_memory
supersession paragraph: list_memory list_scheduled search_discovery search_memory
symmetric difference: []
store gate sites: 4 / surfaces named: 4
GATE1_PASS
GATE2_PASS
GATE3_PASS (scoped outside .planning/ — see Deviations)
```

## Corrected Sentences (quoted whole)

**Layout row** (`CLAUDE.md` line 15, full row unchanged apart from the one insertion):
> `| \`cmd/engram/\` | cobra CLI: \`root\`, \`serve\`, \`version\` + client-tier commands reaching a running server over Connect (\`get\`, \`search\`, \`list\`, \`store\`, \`migration-status\`) + operator-tier commands acting on Qdrant directly (\`reindex\` embedder migration — see docs-site \`guides/reindex\`; \`migrate\` (\`status\`, \`revert\`) schema-version sweep — see docs-site \`guides/migrate\`; \`migrate-remap-owner\` (alias: \`migrate-set-owner\`, deprecated); \`prune-expired\`; \`summarize-missing\`; \`backfill-short-ids\` (deprecated, use \`migrate\`); \`spine-review\` (\`scan\`, \`verify\`, \`consolidate\`, \`purge\`, \`archive\`, \`restore\`)) (entrypoint only) |`

**Archived-state paragraph** (`CLAUDE.md` lines 171-176, whole paragraph, tail sentence corrected):
> Archived state: `engram spine-review archive` stamps `archived_at` on one or more records by id; `engram spine-review restore` deletes it, returning the record to normal recall — always reversible, and never a delete, content erasure, or vector removal. `archived_at` shares supersession's soft-hidden-but-still-fetchable-by-id contract: an archived record drops out of `search_memory`/`list_memory`/`search_discovery`/`list_scheduled` but stays reachable by id via `get_memory`. Archiving is an orthogonal key — it never writes an expiry and never writes a supersession link, and each of a record's derived states clears independently of the others. Every surface renders a record's derived state as up to four words, in canonical order: `archived`, `superseded`, `expired`, `scheduled` — descending by finality. `expired` is evaluated first and, when present, suppresses `scheduled`; the window-boundary rule that decides each state lives on `reference/memory-record.md`, not restated here.

Both paragraphs read as natural routing-doc prose in the file's existing idiom — compressions of the current contract, not changelog entries or section expansions.

## Task Commits

Each task was committed atomically, with an explicit pathspec (`-- CLAUDE.md`):

1. **Task 1: Mark every deprecated command in the cmd/engram/ Layout row, derived from the goldens** — `253e20c3` (docs)
2. **Task 2: State all four soft-hide recall surfaces in the Archived-state paragraph** — `cb8c3144` (docs)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `CLAUDE.md` — two sentence-level corrections: `backfill-short-ids` deprecation annotation in the Layout row; four-surface recall-gate list in the Archived-state paragraph.

## Decisions Made

- Left `migrate-remap-owner (alias: \`migrate-set-owner\`, deprecated)` byte-identical to `HEAD` per the plan's explicit instruction and 08-VERIFICATION.md's info-level-only classification of that wording.
- Left `CLAUDE.md` line 97's Memory-contract mention of `engram backfill-short-ids` untouched — outside this gap and outside this plan's file scope.
- Confirmed `task lint` (rumdl) stays clean after both edits.

## Deviations from Plan

None in the production edits — plan executed exactly as written, both gates verified RED-then-GREEN as the plan required.

**One scoping note, not a deviation from CLAUDE.md's edit but from the literal `git diff --name-only` gate text:** at the time this plan executed, `.planning/STATE.md` and `.planning/config.json` were already modified in the working tree by the orchestrator (phase-08 gap-closure plan setup: `total_plans` bumped 42→44, `status` toggled `verifying`→`executing`, `use_worktrees` toggled `true`→`false`) before this executor started any edit. These predate and are unrelated to this plan's two tasks. `git diff --name-only` scoped outside `.planning/` (`git diff --name-only -- . ':!.planning'`) reports `CLAUDE.md` alone across both commits, matching the plan's success criterion in spirit — production/doc file scope, not GSD tracking bookkeeping (which this plan's own STATE.md update below further modifies, as expected).

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Self-Check: PASSED

- `CLAUDE.md` exists and both edits verified present via `git diff -- CLAUDE.md` inspection: FOUND.
- Commit `253e20c3` found in `git log --oneline --all`: FOUND.
- Commit `cb8c3144` found in `git log --oneline --all`: FOUND.
- `task lint` clean after both edits: PASSED.
- `git diff --name-only -- . ':!.planning'` reports `CLAUDE.md` alone: PASSED.

## Next Phase Readiness

- Both 08-VERIFICATION.md gaps (truth 11 / D-05.2, truth 12 / D-05.3) closed. The remaining two gaps this phase's gap-closure wave targets (per 08-04-SUMMARY.md and 08-VERIFICATION.md truths 5 and truth-25-style rules.go comment defect) are out of this plan's scope — this plan covered only the CLAUDE.md pair.
- `REQ-claude-md-migrations-convention` requirement now fully satisfied without gaps.
- No blockers for phase completion from this plan's side.

---
*Phase: 08-registry-docs-tail*
*Completed: 2026-08-21*
