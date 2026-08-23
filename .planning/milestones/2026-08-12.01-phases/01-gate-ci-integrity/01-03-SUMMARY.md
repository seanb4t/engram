---
phase: 01-gate-ci-integrity
plan: 03
subsystem: testing
tags: [go, regexp, re2, conformance-gate, gsd-key-links, planning-artifacts, one-time-reassessment]

# Dependency graph
requires: ["internal/keylinks (01-01)", "escape-free patterns + recurring gates (01-02)"]
provides:
  - "Every v0.13.x Phase 1-2 key-link re-resolved against HEAD exactly once, with a recorded verdict from a closed set — REQ-keylink-past-gates-reassessed satisfied."
  - ".planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md: the phase-owned verdict table plus the D-01 upstream-gap disposition record."
affects: []

# Actuals (#2632)
actuals:
  tokens: 6386
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "one-time sweep reuses the recurring guard's own CheckSatisfiable rather than a second matcher (D-12); which of the two legs (from, then to) actually matched is read off via the same compiled *regexp.Regexp CheckSatisfiable already validated, not a re-implementation of pattern resolution"
    - "a from-and-to-both-missing link is classified 'unreadable' by a plain os.ReadFile existence check performed BEFORE CheckSatisfiable is consulted — CheckSatisfiable's own 'from missing → silent nil' shortcut (meant for an un-executed plan, D-04) is intentionally never allowed to masquerade as 'pinned' for an already-shipped plan's link"
    - "unpinned reasons are hand-recorded by inspection in a file:line-keyed map, with a loud placeholder for any verdict that lands unpinned with no recorded reason, rather than an empty/best-effort string"
    - "a phase-owned standalone .md document (not this phase's own VERIFICATION.md) as the destination for a one-time audit's output, when the natural GSD-parsed home for that output is a tool-owned generated file (rule 8dfdhfs5nn)"

key-files:
  created:
    - internal/keylinks/sweep_test.go
    - .planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md
  modified: []

key-decisions:
  - "D-01 checkpoint resolved by Sean: spine-track, not file-upstream and not both. The gsd-core parseMustHavesBlock unescaping gap is recorded as durable engram memory in this repo's spine; no GitHub issue is filed against the gsd-core tool, and no other outward-facing action is taken (precedent: memory cvvrwjbsnz). Stated plainly in 01-KEYLINK-REASSESSMENT.md: because the gap stays unreported upstream, internal/keylinks is a PERMANENT guard, not a transitional one — there is no upstream fix in flight that could ever make it redundant, because none was requested. The spine memory record itself is written by the orchestrator at phase close (search-before-store / supersede-on-contradiction is the orchestrator's responsibility, per the division of labor at the checkpoint), not by this plan's Task 3."
  - "The sweep's per-link classification logic checks from/to file EXISTENCE (a plain os.ReadFile error check) before consulting CheckSatisfiable, specifically so a from-and-to-both-missing link is recorded 'unreadable' rather than silently absorbed into CheckSatisfiable's un-executed-plan shortcut (which would return nil — 'pinned' — for a missing from file regardless of the to file). Every link in this sweep's 30-link input set turned out to have at least one of its two files present, so this branch never actually fired against real data, but the ordering is deliberate defense against the one shape D-11's prohibition block explicitly forbids: 'unreadable' collapsed into 'pinned'."
  - "The single unpinned link (02-interface-discoverability/02-04-PLAN.md:48, pattern surfaces[.]ClassForTool) was investigated by inspection rather than left as a generic 'pattern matched neither file' placeholder: git log --all -S\"ClassForTool\" -- internal/server/tools.go confirms the from file has never contained the literal call site, because the real call is routed through a wrapper (annotationsFor(name) in internal/server/toolannotations.go, itself calling surfaces.ClassForTool). The underlying behavior is real and correctly wired; only the key-link's from-file assumption is stale. Per D-14, this is recorded and NOT repaired in this phase."
  - "Input-set link count (30) matched the plan's planning-time estimate exactly, unlike plan 01-02's 39-vs-38 discrepancy — no reconciliation note was needed for the count itself."

requirements-completed: [REQ-keylink-past-gates-reassessed]

coverage:
  - id: D1
    description: "Every v0.13.x Phase 1-2 key-link carries exactly one verdict from the closed set {pinned, pinned-via-target, unpinned, unreadable, invalid}; a link producing no verdict, or a verdict outside the set, fails the completeness assertion"
    requirement: "REQ-keylink-past-gates-reassessed"
    verification:
      - kind: unit
        ref: "internal/keylinks/sweep_test.go#TestReassessmentTableIsComplete"
        status: pass
      - kind: manual-red-proof
        ref: "SUMMARY Fail-First Observations, completeness assertion"
        status: pass
    human_judgment: false
  - id: D2
    description: "A verdict of pinned means the corrected pattern resolves against its from file, or its to file by the tool's own fallback, at HEAD (D-11); resolution reuses the guard's own CheckSatisfiable rather than a second matcher (D-12)"
    requirement: "REQ-keylink-past-gates-reassessed"
    verification:
      - kind: unit
        ref: "internal/keylinks/sweep_test.go#TestReassessV013Phase12"
        status: pass
      - kind: source-read
        ref: "internal/keylinks/sweep_test.go — classifyV013Link calls CheckSatisfiable and derives pinned/pinned-via-target from which leg's re.Match succeeded"
        status: pass
    human_judgment: false
  - id: D3
    description: "unreadable is its own verdict class, never absorbed into pinned or unpinned, for a link whose from and to files could not be read"
    requirement: "REQ-keylink-past-gates-reassessed"
    verification:
      - kind: source-read
        ref: "internal/keylinks/sweep_test.go — classifyV013Link checks fromExists/toExists before ever calling CheckSatisfiable, returning verdictUnreadable when both are false"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every unpinned verdict names a non-empty reason, recorded by inspection"
    requirement: "REQ-keylink-past-gates-reassessed"
    verification:
      - kind: unit
        ref: "internal/keylinks/sweep_test.go#TestReassessmentTableIsComplete (non-empty-reason assertion for verdictUnpinned)"
        status: pass
      - kind: manual-inspection
        ref: "02-interface-discoverability/02-04-PLAN.md:48 — git log --all -S\"ClassForTool\" -- internal/server/tools.go confirmed zero commits, i.e. routed through the annotationsFor wrapper"
        status: pass
    human_judgment: true
  - id: D5
    description: "Nothing found unpinned is repaired in this phase (D-14); the record is the deliverable"
    requirement: "REQ-keylink-past-gates-reassessed"
    verification:
      - kind: manual-review
        ref: ".planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md §'What happens to the one unpinned gate (D-14)' — states explicitly no repair was made; git diff confirms zero changes to internal/server/tools.go, toolannotations.go, or toolclass.go"
        status: pass
    human_judgment: false
  - id: D6
    description: "The upstream unescaping gap's disposition (D-01) is a recorded human decision, not an executor default; the consequence of that choice (permanent, not transitional, guard) is stated plainly"
    requirement: "REQ-keylink-past-gates-reassessed"
    verification:
      - kind: manual-review
        ref: ".planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md §'Upstream unescaping gap disposition (D-01)' — records Sean's spine-track selection at the Task 3 checkpoint and the permanent-guard consequence"
        status: pass
    human_judgment: true

duration: ~55min
completed: 2026-08-13
status: complete
---

# Phase 01 Plan 03: v0.13.x Phase 1-2 Key-Link Reassessment Summary

**One-time sweep re-resolves all 30 v0.13.x Phase 1-2 key-links against HEAD via the recurring guard's own `CheckSatisfiable` — 26 pinned, 3 pinned-via-target, 1 unpinned (routed through a wrapper, recorded not repaired per D-14) — landing as a phase-owned record with a completeness test proven red-then-green, plus Sean's checkpoint decision to spine-track (not file upstream) the gsd-core unescaping gap that made these gates no-ops in the first place.**

## Performance

- **Duration:** ~55 min (includes a blocking `checkpoint:decision` pause awaiting Sean)
- **Completed:** 2026-08-13
- **Tasks:** 3
- **Files modified:** 2 created (`internal/keylinks/sweep_test.go`, `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md`)

## Accomplishments
- `TestReassessV013Phase12` — reassesses every key-link in every `-PLAN.md` under `.planning/milestones/v0.13.x-phases/01-interface-enforceability/` and `.../02-interface-discoverability/` (30 found, matching the plan's own estimate exactly), emitting one deterministic `t.Logf` line per link plus a per-verdict rollup.
- `classifyV013Link` — resolves each pattern through `ValidatePattern` (invalid bucket) then `CheckSatisfiable` (the recurring guard's own matcher, reused per D-12, never a second implementation), reading off which of the from/to legs actually matched to distinguish `pinned` from `pinned-via-target`. A plain file-existence check runs BEFORE `CheckSatisfiable` so a from-and-to-both-missing link is classified `unreadable` rather than silently absorbed by `CheckSatisfiable`'s un-executed-plan shortcut.
- `TestReassessmentTableIsComplete` — mechanically asserts verdict count equals parsed-link count and every verdict is a closed-set member, with a mandatory non-empty reason for `unpinned`. Its red direction was proven by a temporary early `continue` (0 verdicts vs. 30 links), then reverted before commit.
- Investigated the single `unpinned` finding by inspection rather than a generic placeholder: `internal/server/tools.go`'s `surfaces[.]ClassForTool` pattern never matched because the real call site is `internal/server/toolannotations.go`'s `annotationsFor(name)` wrapper, confirmed via `git log --all -S"ClassForTool" -- internal/server/tools.go` returning zero commits.
- `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md` — the phase-owned, reproducible verdict table (30 rows: plan file:line, from, to, pattern, verdict, reason), the D-11 resolution rule and resolved-at commit SHA, the per-verdict rollup, a re-run invocation, the D-14 not-repaired statement for the one unpinned gate, the D-13 placement rationale (rule `8dfdhfs5nn` forbids hand-adding structure to `VERIFICATION.md`, a tool-owned generated file), and — added after the Task 3 checkpoint resolved — the D-01 upstream-gap disposition record.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "resolve one v0.13.x key-link at HEAD and emit its verdict" — then all of them (tracer)** - `e3b38bb5` (test)
2. **Task 2: Write the verdict table as a phase-owned artifact** - `784cda46` (docs)
3. **Task 3: Decide how the upstream unescaping gap is reported (D-01)** - `540cccce` (docs) — after the blocking checkpoint resolved with Sean's `spine-track` selection

**Plan metadata:** committed separately by the orchestrator after wave merge (worktree mode — this executor does not write STATE.md/ROADMAP.md).

## Files Created/Modified
- `internal/keylinks/sweep_test.go` - `TestReassessV013Phase12`, `TestReassessmentTableIsComplete`, `collectV013Phase12Links`, `classifyV013Link`, `reassessV013Phase12`, `reassessLine`, `relKey`, `v013UnpinnedReasons`, `reassessVerdict`/`reassessVerdictSet`/`reassessRow` types
- `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md` - the full verdict table, rollup, D-14 not-repaired statement, D-13 placement rationale, and D-01 upstream-gap disposition (spine-track, with the permanent-guard consequence stated plainly)

## Decisions Made
- **D-01 checkpoint (Task 3), resolved by Sean:** `spine-track` — see frontmatter `key-decisions` for the full record, including the consequence that `internal/keylinks` is now a permanent guard (no upstream fix requested, none in flight). The spine memory record itself is written by the orchestrator at phase close, not by this executor — no `mcp__engram__*` write tool was called during this plan's execution.
- Existence-before-satisfiability ordering in `classifyV013Link`, so `unreadable` can never be masqueraded as `pinned` by `CheckSatisfiable`'s un-executed-plan shortcut — see frontmatter `key-decisions`.
- The one `unpinned` finding's reason was hand-derived from git history inspection, not left generic — see frontmatter `key-decisions`.

## Deviations from Plan

None. The plan's own Task 3 was a checkpoint by design; the pause for Sean's decision was expected flow, not a deviation. No Rule 1-4 auto-fixes were needed in any of the three tasks.

## Fail-First Observations (verbatim)

**Task 1 — TestReassessmentTableIsComplete, completeness assertion:**

RED (temporary early `continue` added to `reassessV013Phase12`'s classification loop, skipping every link before it could be appended to `rows`):
```
=== RUN   TestReassessmentTableIsComplete
    sweep_test.go:275: TestReassessmentTableIsComplete: parsed 30 key-links under [01-interface-enforceability 02-interface-discoverability] but produced 0 verdicts — at least one link was silently omitted from the sweep
--- FAIL: TestReassessmentTableIsComplete (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/internal/keylinks	0.074s
FAIL
```

GREEN (temporary `continue` removed, loop restored to unconditional append):
```
=== RUN   TestReassessmentTableIsComplete
--- PASS: TestReassessmentTableIsComplete (0.00s)
PASS
ok  	github.com/seanb4t/engram/internal/keylinks	0.061s
```

Confirmed clean revert via `git status --short` before the Task 1 commit: no residual diff, no stray files.

## Verdict Rollup (verbatim, from `TestReassessV013Phase12` at commit `e3b38bb5c86bae050162f8935180e1c746334c88`)

```
total v0.13.x Phase 1-2 key-links reassessed: 30 (pinned=26 pinned-via-target=3 unpinned=1 unreadable=0 invalid=0)
```

Full per-link table: `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md`.

## Issues Encountered
None beyond the expected checkpoint pause (Task 3 is a designed `checkpoint:decision` gate, not an issue).

## User Setup Required
None - no external service configuration required. The orchestrator still owes one follow-up action outside this plan's scope: writing the engram spine memory record for the D-01 disposition (search-before-store / supersede-on-contradiction, per the division of labor at the Task 3 checkpoint).

## Worktree Metadata Correction

The checkpoint return for Task 3 reported `expected_base` as `784cda46` (this plan's own second commit) rather than this worktree's actual fork base, `79258f3d6407184ca98355452502ca87a14273b6`. Noted per the orchestrator's correction; no further action taken on it beyond not repeating the wrong value here.

## Next Phase Readiness
- `REQ-keylink-past-gates-reassessed` is satisfied: every v0.13.x Phase 1-2 key-link has exactly one recorded verdict, completeness is mechanically enforced, and the record is reproducible from `sweep_test.go` at a named commit.
- The one gate found unpinned (`02-interface-discoverability/02-04-PLAN.md:48`) is recorded, not repaired (D-14) — its underlying behavior was confirmed correctly wired by inspection, so it is not a live defect, only a stale pin. If repinning is ever wanted, that is its own scoped work.
- D-01's upstream-gap disposition is now a recorded human decision (`spine-track`); the orchestrator still needs to write the actual engram spine memory at phase close for that decision to be durably recallable by future agents.
- No blockers for this phase's remaining plans (01-04 through 01-06, the Qdrant CI mitigation work).

---
*Phase: 01-gate-ci-integrity*
*Completed: 2026-08-13*

## Self-Check: PASSED

- FOUND: `internal/keylinks/sweep_test.go`
- FOUND: `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md`
- FOUND: `.planning/phases/01-gate-ci-integrity/01-03-SUMMARY.md`
- FOUND commit `e3b38bb5` (test)
- FOUND commit `784cda46` (docs — verdict table)
- FOUND commit `540cccce` (docs — D-01 disposition)
