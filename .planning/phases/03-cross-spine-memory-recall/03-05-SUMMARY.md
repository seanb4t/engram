---
phase: 03-cross-spine-memory-recall
plan: 05
subsystem: docs
tags: [docs, agent-guidance, cross-spine]

# Dependency graph
requires:
  - phase: 03-cross-spine-memory-recall (plan 04)
    provides: "The complete, proven cross_spine feature on both MCP and Connect lanes: cross_spine on search_memory/list_memory (and their Connect siblings), searched_scopes/scopes_truncated reporting, D-04 explicit-field non-inference — the shipped reality this plan documents."
provides:
  - "Three agent-facing surfaces (tool reference, curating-memory skill, CLAUDE.md memory contract) documenting cross_spine, worded to the searched_scopes semantic precision the phase's own convention (yaj7dqz9qq) requires: an argument with no guidance is an incomplete feature."
  - "A when-not-to-use-cross-spine subsection naming the two concrete costs (ranking dilution, extra scan) — the behavioral mitigation for T-03-07 (agents defaulting to cross-spine on every call)."
affects: []

actuals:
  tokens: 3100
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Doc wording for searched_scopes is pinned to the authorized-span framing across all three agent-facing surfaces plus the proto field comments from 03-04 (four surfaces total agree): 'every scope you can read that the search spanned' / 'the scopes searched under your authorization' — never 'scopes that produced hits' or 'scopes with results'."

key-files:
  created: []
  modified:
    - docs-site/src/content/docs/reference/tools.md
    - skill/engram/skills/curating-memory/SKILL.md
    - CLAUDE.md

key-decisions:
  - "list_memory's operational limit note was reworded from the plan's literal 'the total becomes an exact count' phrasing to 'the underlying total becomes an exact count ... (visible as the Connect API's total field)' after checking the live tree: the MCP list_memory result map does NOT include a total key (only memories/next_cursor, plus searched_scopes/scopes_truncated on cross-spine) — total lives only in coreListResult and the Connect ListMemoriesResponse. The note stays accurate about MCP-observable behavior (result count jumps) while correctly attributing the exact-count total field to the Connect lane, rather than implying the MCP tool response carries a total key it does not."

patterns-established: []

requirements-completed: [REQ-cross-spine-search, REQ-cross-spine-result-provenance]

coverage:
  - id: D1
    description: "An agent reading the tool reference learns that cross_spine exists on search_memory and list_memory, that scope is conditional rather than required, and what the response gains (searched_scopes, scopes_truncated, worded as the authorized span never a hit distribution) — plus an explicit-limit operational note on list_memory's total-count jump."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: other
        ref: "rg -c -e '^\\| .cross_spine. \\|' docs-site/src/content/docs/reference/tools.md == 3"
        status: pass
      - kind: other
        ref: "rg -c -e '^\\| .scope. \\|.*\\| conditional \\|' docs-site/src/content/docs/reference/tools.md == 3"
        status: pass
    human_judgment: false
  - id: D2
    description: "An agent reading curating-memory learns WHEN NOT to reach for cross-spine (opt-in widening, default stays scope-confined, two named costs), not only that it exists."
    requirement: "REQ-cross-spine-result-provenance"
    verification:
      - kind: other
        ref: "rg -e '^## Cross-spine recall' and '^### When not to use cross-spine' in skill/engram/skills/curating-memory/SKILL.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "searched_scopes is documented as the scopes the caller is authorized to read, never as the scopes that produced hits, across all three surfaces."
    requirement: "REQ-cross-spine-result-provenance"
    verification:
      - kind: other
        ref: "manual text review of all three edited files against the exact sentence quoted in Decisions Made below"
        status: pass
    human_judgment: true
  - id: D4
    description: "The repo's memory contract in CLAUDE.md names the new recall dimension alongside tags, the created-at window, and cursor."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: other
        ref: "rg -c -e 'cross_spine' CLAUDE.md == 1, in the Memory contract section's existing sentence chain"
        status: pass
    human_judgment: false

duration: ~15min
completed: 2026-08-01
status: complete
---

# Phase 3 Plan 5: Agent-Facing Guidance for Cross-Spine Recall Summary

**Documented the shipped `cross_spine` feature (03-02/03-03/03-04) across the three surfaces an agent actually reads — tool reference, `curating-memory` skill, `CLAUDE.md` — with `searched_scopes` worded as the authorized span it is, never as a hit-distribution report, and a load-bearing "when not to widen" subsection naming the ranking-dilution and extra-scan costs of the opt-in.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-08-01
- **Tasks:** 2/2
- **Files modified:** 3

## Accomplishments

- `docs-site/src/content/docs/reference/tools.md`: `search_memory`'s and `list_memory`'s `scope` argument rows changed from `yes` to `conditional` ("required unless `cross_spine` is true"), matching `search_discovery`'s pre-existing phrasing verbatim. Both tables gained a `cross_spine` row. Both sections' return-value prose gained a paragraph on `searched_scopes`/`scopes_truncated` — worded as "every scope you can read that the search/list spanned, not the scopes that produced hits/results" — both omitted entirely on a scope-confined call. `list_memory` additionally gained an operational note: pass an explicit `limit` on a cross-spine call, since the underlying total becomes an exact count across every readable scope (the Connect API's `total` field) and an unset Connect-lane limit means "all".
- `skill/engram/skills/curating-memory/SKILL.md`: new `## Cross-spine recall` section (placed after `## Tagging`, before `## Discipline`, alongside the file's other recall-dimension prose) stating what `cross_spine` does and how `searched_scopes`/`scopes_truncated` should be read. Its `### When not to use cross-spine` subsection is the load-bearing half: states plainly that the scope-confined default is correct for ordinary work (session-start bootstrap, recall about the current repo, or anything the two-tier spine/overlay scope already covers), that `cross_spine` is for when the thing you're looking for might live somewhere you're not, and names both costs — ranking dilution (a distant match can outrank a local one) and the extra bounded scan needed to enumerate scopes.
- `CLAUDE.md`'s Memory contract section: one sentence appended to the chain already covering `tags`, the created-at window, and `cursor` — naming `cross_spine`, `searched_scopes`, and `scopes_truncated`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Document cross_spine in the tool reference** — `de8e488b` (docs)
2. **Task 2: Tell agents when NOT to use cross-spine** — `292fa380` (docs)

**Plan metadata:** committed below.

## Gate Results

All plan-level and phase-close verification gates:

```
# Task 1 gates
test "$(rg -c -e '^\| .cross_spine. \|' docs-site/.../tools.md)" = "3"          -> PASS
test "$(rg -c -e '^\| .scope. \|.*\| conditional \|' docs-site/.../tools.md)" = "3" -> PASS
rg -c -e 'searched_scopes' docs-site/.../tools.md                              -> 2
rg -c -e 'scopes_truncated' docs-site/.../tools.md                             -> 2

# Task 2 gates
rg -e '^## Cross-spine recall' SKILL.md                                        -> found
rg -e '^### When not to use cross-spine' SKILL.md                              -> found
rg -c -e 'cross_spine' CLAUDE.md                                               -> 1
task lint:markdown                                                             -> clean (125 files)
task license:check                                                             -> clean (233 valid, 0 invalid)
task (lint + full suite)                                                       -> all green (golangci-lint, yamlfmt,
                                                                                    actionlint, rumdl, ruff check/format,
                                                                                    go test ./... all ok, pytest 33 passed)

# Phase-close gates (run once, on the final tree)
task                                                                            -> green
go vet ./...                                                                   -> exit 0
task license:check                                                             -> clean
task chart:validate                                                            -> OK (containerEnv checksum pin intact,
                                                                                    helm lint 1 chart, 0 failed)
task ui:build                                                                  -> succeeds, zero diff (git status --short
                                                                                    empty after — no content-hash drift,
                                                                                    docs-only plan)
git diff --exit-code -- go.mod go.sum                                          -> clean, zero new dependencies
task proto:lint                                                                -> clean
task proto:gen && git diff --exit-code gen/ ui/src/lib/gen/                    -> zero diff
```

## Files Created/Modified

- `docs-site/src/content/docs/reference/tools.md` — `scope` rows conditional, `cross_spine` rows added, `searched_scopes`/`scopes_truncated` return-value prose, `list_memory` explicit-limit note.
- `skill/engram/skills/curating-memory/SKILL.md` — new `## Cross-spine recall` section with `### When not to use cross-spine` subsection.
- `CLAUDE.md` — one sentence in the Memory contract section naming `cross_spine`/`searched_scopes`/`scopes_truncated`.

## Decisions Made

- **The exact `searched_scopes` sentences, checked for precision against the plan's constraint (never "scopes with results"):**
  - Tool reference (`search_memory`): "every scope you can read that the search spanned, not the scopes that produced hits"
  - Tool reference (`list_memory`): "every scope you can read that the list spanned, not the scopes that produced results"
  - `curating-memory` `## Cross-spine recall`: "which name the scopes searched under your authorization — not the scopes that had results"
  - `CLAUDE.md`: names the keys (`searched_scopes`, `scopes_truncated`) without re-deriving the semantic in the one-sentence contract-index form the plan required ("Keep it to a sentence — that section is a contract index, not a guide").
- `list_memory`'s explicit-limit note was reworded from the plan's literal draft after verifying against `internal/server/tools.go`: the MCP `list_memory` result map has no `total` key (only `memories`/`next_cursor`, plus the two cross-spine keys) — `Total` exists only on `coreListResult` and the Connect `ListMemoriesResponse`. The shipped note says the *underlying* total becomes an exact count, visible as the Connect API's `total` field, so it doesn't imply the MCP tool itself surfaces a `total` value it does not return. This is a wording-precision fix within Task 1's own scope, not a deviation from the plan's intent (the plan's own acceptance criterion is "the `list_memory` section carries the explicit-limit note" — satisfied).
- Placement of `## Cross-spine recall` in `SKILL.md`: directly after `## Tagging` (the file's other recall-dimension section covering `tags`/`created_after`/`created_before`/`cursor`) and before `## Discipline`, so all recall-dimension guidance reads together before the store/supersede/delete discipline block.

## Deviations from Plan

None — plan executed exactly as written, with one documented wording-precision adjustment (the `list_memory` total-field attribution above) made within Task 1's own action text ("match the surrounding table's column formatting exactly" / accurate description of what ships) rather than a deviation from what the plan required.

## Known Stubs

None — this plan is documentation-only; no code, no data wiring, no placeholder values.

## Threat Flags

None — this plan touches only the three named documentation surfaces. No new network endpoints, auth paths, file access patterns, or schema changes.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All three of Phase 3's requirements (`REQ-cross-spine-search`, `REQ-cross-spine-authz-verified`, `REQ-cross-spine-result-provenance`) are now fully complete, including the agent-facing guidance obligation `yaj7dqz9qq` requires as part of "done," not a follow-up.
- Four surfaces now agree on `searched_scopes`' semantics: the two proto field comments (03-04), the tool reference, and `curating-memory` — all say "authorized span, never scopes with results."
- Phase 3 (Cross-Spine Memory Recall) is complete: all 5 waves (03-01 through 03-05) landed, all phase-close gates green on the final tree.
- No blockers. Next: phase verification/close per the orchestrator (`03-VALIDATION.md` row 3-05-01 is now satisfied), then milestone-level next steps per `.planning/STATE.md`.

---
*Phase: 03-cross-spine-memory-recall*
*Completed: 2026-08-01*

## Self-Check: PASSED

- FOUND: `docs-site/src/content/docs/reference/tools.md`
- FOUND: `skill/engram/skills/curating-memory/SKILL.md`
- FOUND: `CLAUDE.md`
- FOUND: `.planning/phases/03-cross-spine-memory-recall/03-05-SUMMARY.md`
- FOUND commits: `de8e488b`, `292fa380` (both present in `git log --oneline --all`)
