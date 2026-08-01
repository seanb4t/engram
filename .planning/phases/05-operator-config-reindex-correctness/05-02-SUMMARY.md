<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 05-operator-config-reindex-correctness
plan: 02
subsystem: store
tags: [qdrant, reindex, resume, dry-run, tags]

requires: []
provides:
  - "tagsFromPayload(map[string]*qdrant.Value) []string — the single tag decoder, called by fromPayload (source) and reindexTargetContents (target)"
  - "tagsEqual(a, b []string) bool — order-independent, multiplicity-preserving, nil==empty"
  - "reindexTarget.tags []string — target-side tag snapshot for the resume skip predicate"
  - "resume skip predicate: content == && tagsEqual(...) && identity guard — three conjuncts"
  - "ReindexResult.WouldUpsert uint64 — populated only under DryRun"
  - "reindexSummary(res, target, dim, dryRun, resume bool) string — dry-run-with-resume sizing line, dry-run-without-resume byte-identical to pre-plan"
affects: [05-03]

actuals:
  tokens: 6634
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Single shared decode/compare helpers (tagsFromPayload, tagsEqual) referenced from both sides of a resume comparison, making 'same path' a structural grep fact rather than an incidental consequence"
    - "Single per-point skip predicate backing both the dry-run and real arms of Reindex, avoiding predicate duplication/drift"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/reindex_test.go
    - cmd/engram/reindex.go
    - cmd/engram/reindex_test.go

key-decisions:
  - "tagsFromPayload and the new tagsEqual/reindexTarget.tags code were placed together, away from payload()'s closing brace, specifically to keep git's hunk-header funcname heuristic from naming func payload( as touched — a hunk header naming a function is not the same as that function's body changing, and the plan's verification gate checks the header text literally."
  - "TestReindexDryRunResume was placed at the end of reindex_test.go, not immediately after TestReindexDryRunWritesNothing, for the same hunk-header reason — the gate requiring that test 'stay passing unmodified' also checks for its name appearing in any diff hunk header."
  - "WouldUpsert is a new ReindexResult field, not a repurposing of Upserted — preserves TestReindexDryRunWritesNothing's Upserted==0 invariant unmodified, per the plan's explicit reversibility note."

patterns-established: []

requirements-completed: [REQ-reindex-resume-tags]

coverage:
  - id: EDGE1
    description: "A record whose tags changed while its content did not is re-embedded by reindex --resume, not counted Unchanged."
    requirement: "REQ-reindex-resume-tags"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexResumeTags/EDGE_1"
        status: pass
    human_judgment: false
  - id: EDGE2
    description: "Paired positive control: a record whose content AND tags both match the target is still skipped."
    requirement: "REQ-reindex-resume-tags"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexResumeTags/EDGE_2"
        status: pass
    human_judgment: false
  - id: EDGE3
    description: "Same tag elements in a different order compare equal; a purely reordered record skips."
    requirement: "REQ-reindex-resume-tags"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexResumeTags/EDGE_3"
        status: pass
    human_judgment: false
  - id: EDGE4
    description: "A nil tag slice and an empty tag slice compare equal; an untagged record does not re-embed forever."
    requirement: "REQ-reindex-resume-tags"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexResumeTags/EDGE_4"
        status: pass
    human_judgment: false
  - id: EDGE5
    description: "--dry-run still writes nothing and creates no target collection after the D-14 change."
    requirement: "REQ-reindex-stale-repair"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexDryRunWritesNothing"
        status: pass
    human_judgment: false
  - id: DRYRUNRESUME
    description: "--dry-run --resume against an existing target reports a would-re-embed/would-skip split sizing the repair; against a nonexistent target it reports every record as would-re-embed and creates nothing."
    requirement: "REQ-reindex-stale-repair"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexDryRunResume"
        status: pass
    human_judgment: false
  - id: TAGSEQUAL
    description: "tagsEqual helper: equal, reordered-equal, different-element-unequal, differing-length-unequal, nil-vs-empty-equal, duplicate-multiplicity-unequal."
    requirement: "REQ-reindex-resume-tags"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestTagsEqual"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-08-01
status: complete
---

# Phase 5 Plan 02: Reindex Resume Tag-Awareness Summary

**`reindex --resume` now re-embeds tags-only edits while a paired positive control proves it still skips genuinely unchanged records, and `--dry-run --resume` sizes the repair before it runs — both through one shared tag decoder and one shared skip predicate.**

## Performance

- **Duration:** ~12 min (18:16:41 to 18:28:30, from prior commit to last task commit)
- **Started:** 2026-08-01T18:16:41-04:00
- **Completed:** 2026-08-01T18:28:30-04:00
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Extracted `fromPayload`'s inline tags decode into `tagsFromPayload`, the single tag decoder now
  called by both the source side (`fromPayload`) and the target side (`reindexTargetContents`) —
  making "both sides decode through the same path" a one-grep structural fact (D-08).
- Added `tagsEqual` (order-independent via `slices.Clone` + `sort.Strings` + `slices.Equal`,
  multiplicity-preserving, nil-vs-empty equal by construction) and wired it as the third conjunct
  of the resume skip predicate, between content equality and the identity guard.
- Rewrote the predicate's stale comment (D-11): it no longer claims equal content implies equal
  tags from the same source payload; it states `ti` is a target-side snapshot, content/tags are
  independently mutable, and states D-09's reordering residual explicitly.
- Added `TestReindexResumeTags` (4 labelled subtests pinning EDGE 1-4) and `TestTagsEqual` (6-case
  table test, Qdrant-free).
- Restructured `Reindex`'s scroll body into a single per-point loop (no dry-run-only predicate
  copy): added `ReindexResult.WouldUpsert`, wired the resume target lookup into the dry-run arm
  (guarded by `CollectionExists` so a missing target yields "everything would re-embed" rather than
  an error), and kept `ensureCollection` gated behind `!opts.DryRun` unchanged.
- Gave `reindexSummary` a `resume bool` parameter; the non-resume dry-run string is byte-identical
  to the pre-plan format, and the new dry-run-with-resume line names both the would-re-embed and
  would-skip counts. Added `TestReindexDryRunResume` (two cases: existing target, nonexistent
  target) and a third `reindexSummary` subtest for the new wording.

## Premise Checks

1. **Task 1 — `qdrant.NewWithPayload(true)` is an all-fields alias, not a selector.** Confirmed via
   `go doc github.com/qdrant/go-client/qdrant.NewWithPayload`: "Creates a *WithPayloadSelector
   instance with payload enabled/disabled. This is an alias for NewWithPayloadEnable()." No
   defensive widening needed; tags were already fetched from the target and merely never read out.
2. **Task 1 — `reindexTargetContents` does not call `fromPayload`.** Confirmed by reading: it
   decoded `content` and `identity` directly off the raw payload map via `GetStringValue()`. The
   shared-helper route (not `fromPayload`-wholesale) was taken, as the plan specified.
3. **Task 2 — exactly three `reindexSummary` call sites.** `rg -n 'reindexSummary\(' -g '*.go'`
   returned the production call site (`cmd/engram/reindex.go:82`), the definition
   (`cmd/engram/reindex.go:90`), and two test call sites in `cmd/engram/reindex_test.go`. Three
   call sites (excluding the definition itself), matching the plan's stated count; all three
   updated for the new `resume bool` parameter.

## RED/GREEN Transcripts

### Task 1 — RED for the defect (delete the tag conjunct)

Mutation: removed `tagsEqual(ti.tags, m.Tags) &&` from the resume skip predicate.

```
=== RUN   TestReindexResumeTags/EDGE_1:_tags-only_edit_re-embeds,_content-only-match_record_still_skips
    reindex_test.go:462: want upserted=1 unchanged=1, got {Scanned:2 Upserted:0 Skipped:0 Unchanged:2}
    reindex_test.go:467: target tags after tags-only re-embed: want [x z], got [x y] (proves the record was actually rewritten, not merely counted)
--- FAIL: TestReindexResumeTags (0.28s)
    --- FAIL: TestReindexResumeTags/EDGE_1 (0.01s)
    --- PASS: TestReindexResumeTags/EDGE_2 (0.01s)
    --- PASS: TestReindexResumeTags/EDGE_3 (0.01s)
    --- PASS: TestReindexResumeTags/EDGE_4 (0.01s)
```

Exactly the predicted failure: EDGE 1's tags-only edit was counted `Unchanged` instead of
re-embedded. Restored the conjunct; re-ran — all four subtests `--- PASS` again.

### Task 1 — RED for the positive control (force `tagsEqual` to always return `false`)

Mutation: `func tagsEqual(a, b []string) bool { return false }`.

```
=== RUN   TestReindexResumeTags/EDGE_1
    reindex_test.go:462: want upserted=1 unchanged=1, got {Scanned:2 Upserted:2 Skipped:0 Unchanged:0}
=== RUN   TestReindexResumeTags/EDGE_2
    reindex_test.go:480: want scanned=2 upserted=0 unchanged=2, got {Scanned:2 Upserted:2 Skipped:0 Unchanged:0}
=== RUN   TestReindexResumeTags/EDGE_3
    reindex_test.go:499: want upserted=0 unchanged=2 (reorder is a skip), got {Scanned:2 Upserted:2 Skipped:0 Unchanged:0}
=== RUN   TestReindexResumeTags/EDGE_4
    reindex_test.go:516: want upserted=0 unchanged=2 (rawID and fullID both stable), got {Scanned:2 Upserted:2 Skipped:0 Unchanged:0}
--- FAIL: TestReindexResumeTags (0.31s)
    --- FAIL: TestReindexResumeTags/EDGE_1 (0.01s)
    --- FAIL: TestReindexResumeTags/EDGE_2 (0.01s)
    --- FAIL: TestReindexResumeTags/EDGE_3 (0.01s)
    --- FAIL: TestReindexResumeTags/EDGE_4 (0.01s)
```

Exactly the predicted failure: EDGE 2 (the positive control) failed — nothing was counted
unchanged and everything re-upserted, cascading into all four subtests since every record
re-embeds on every run. This is the reading that proves the positive control actually catches a
fix that stops skipping anything. Restored `tagsEqual`'s real body; re-ran — all four subtests
`--- PASS` again, plus `TestTagsEqual`, `TestReindexResumeSkipsUnchanged`, and
`TestReindexResumeRestampsStaleIdentity` all green.

### Task 2 — RED for the dry-run-resume wiring (restore wholesale tally)

Mutation: forced `lookup = false` under `opts.DryRun` in the resume-lookup block, restoring the
pre-task behavior of skipping the target lookup entirely under a dry run.

```
=== RUN   TestReindexDryRunResume
    reindex_test.go:966: want would-re-embed=1 unchanged=1 upserted=0, got {Scanned:2 Upserted:0 Skipped:0 Unchanged:0 WouldUpsert:2}
--- FAIL: TestReindexDryRunResume (0.28s)
```

Exactly the predicted failure: the would-re-embed count covered every scanned record (2) instead
of just the mutated one (1), because the resume lookup never ran. Restored the `CollectionExists`
guard; re-ran — `TestReindexDryRunResume`, `TestReindexDryRunWritesNothing`, and
`TestReindexResumeTags` all `--- PASS` again.

## Live Qdrant Availability

Docker was running throughout; `internal/store`'s `TestMain` provisioned an ephemeral Qdrant via
testcontainers on the first run, and a long-lived `qdrant/qdrant:v1.18.2` container (started
manually, `ENGRAM_QDRANT_TEST_ADDR` set) was used for all subsequent iterations to avoid repeated
container-boot overhead. Every store-test gate in this plan was confirmed via the mandated
`^--- PASS: <TestName> \(` grep, not exit status — no gate in this plan's execution ever showed
`--- SKIP:`. The full `internal/store` suite (160 subtests via `-v`) ran clean with zero SKIP and
zero FAIL in the final plan-wide verification pass.

## Task Commits

1. **Task 1: End-to-end tags-only-edit re-embed + paired positive control (tracer/TDD)** -
   `b59a30b6` (fix)
2. **Task 2: `--dry-run` honors `--resume` for repair sizing** - `5fd8b051` (feat)

**Plan metadata:** (this commit, docs)

## Files Created/Modified

- `internal/store/store.go` - `tagsFromPayload`, `tagsEqual`, `reindexTarget.tags`, the
  three-conjunct resume skip predicate with rewritten comment, `ReindexResult.WouldUpsert`,
  the unified per-point loop backing both the dry-run and real arms
- `internal/store/reindex_test.go` - `TestReindexResumeTags` (4 subtests), `TestTagsEqual`
  (6-case table test), `TestReindexDryRunResume` (2 cases)
- `cmd/engram/reindex.go` - `reindexSummary`'s new `resume bool` parameter and dry-run-with-resume
  wording
- `cmd/engram/reindex_test.go` - both existing call sites updated; a third subtest asserting the
  dry-run-with-resume wording

## Decisions Made

- Placed `tagsFromPayload`/`tagsEqual`/`reindexTarget.tags` together, away from `payload()`'s
  closing brace, and placed `TestReindexDryRunResume` at the end of the test file rather than
  immediately after `TestReindexDryRunWritesNothing` — both moves were required to satisfy this
  plan's own verification gates, which check for the ABSENCE of those two function names in any
  git diff hunk header. Git's hunk-header heuristic names the nearest preceding function signature
  even when only inserting unrelated new code directly after it, so proximity alone (not an actual
  body edit) was tripping the gate on the first attempt; both were corrected before committing.
- `WouldUpsert` added as a new field rather than repurposing `Upserted`, per the plan's explicit
  Claude's-Discretion resolution — keeps `TestReindexDryRunWritesNothing`'s `Upserted == 0`
  assertion meaningful and that test unmodified.

## Deviations from Plan

None - plan executed exactly as written, including both mandated RED readings.

## Issues Encountered

- Two verification-gate false trips during development (not plan deviations, self-corrected before
  any task commit): git's hunk-header funcname heuristic named `func payload(` and
  `func TestReindexDryRunWritesNothing(` respectively when new code was inserted immediately after
  each, even though neither function's body changed. Both resolved by relocating the new code away
  from those insertion points (see Decisions Made above). No functional impact — caught entirely
  during the `<verify>` gate runs before commit.

## User Setup Required

None - no external service configuration required. Operators use the existing
`engram reindex --resume` and `engram reindex --dry-run --resume` flags; no new flags were added.

## Next Phase Readiness

- REQ-reindex-resume-tags is fully satisfied by this plan (marked Complete in REQUIREMENTS.md).
- REQ-reindex-stale-repair's code-level support (patched resume + dry-run sizing, D-13/D-14) is
  complete, but the requirement text mandates "via a **documented** repair path" — D-16's
  documentation lands in plan 05-03. Left Pending in REQUIREMENTS.md; 05-03 should mark it Complete
  once `docs-site/src/content/docs/guides/reindex.md`'s repair section and the `guides/upgrade.md`
  v0.12.0 entry land.
- `internal/authz/...` and `go.mod`/`go.sum` confirmed zero diff from phase base commit `dc98ec0c`.
- Plan 05-03 (wave 2) owns the full phase-close gate set (`task`, `go vet`, `chart:validate`, etc.)
  and the docs work; it can now run bare `task` since 05-01 and 05-02 have both landed cleanly on
  the same working directory.

## Self-Check: PASSED

All 4 modified files and this SUMMARY.md confirmed present on disk; both task commit hashes
(`b59a30b6`, `5fd8b051`) confirmed in `git log --oneline`.

---
*Phase: 05-operator-config-reindex-correctness*
*Completed: 2026-08-01*
