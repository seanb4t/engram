---
phase: 03-curation-verdicts
plan: 04
subsystem: testing
tags: [curationeval, blind-labeling, D-01, D-02, tdd]

# Dependency graph
requires:
  - phase: 03-curation-verdicts
    provides: "plan 03-01's verdict.Relations(), verdict.RelationCriteria(), verdict.RelationInstructions, verdict.StateContext — the criteria text shown to the blind labeler and the label vocabulary the corpus validates against"
provides:
  - "internal/curationeval.syntheticPairs: a committed, 70-pair, five-class, blind-agreed labeled pair corpus (D-01) for CUR-03's live curation-verdict metrics (plan 03-07/03-08)"
  - "blindLabelPrompt / blindLabelMarker: a proven label-independent prompt renderer, reusable for any future blind-labeling round on this corpus"
  - "TestPairFixtureIntegrity: a hermetic mechanical gate (ids, labels, text shape, interleave, size, denylist) that keeps the corpus from silently regressing"
  - "the D-02 authored/blind-labeled/kept procedure, with per-class counts and dropped IDs, recorded in pairs.go's file comment for future auditing"
affects: ["03-07 (curation-verdict metrics computed over syntheticPairs)", "03-08 (live Jev eval run against this corpus)"]

# Actuals (#2632)
actuals:
  tokens: 13876
  tasks: 3
  commits: 5
  plan_head_before: eeb33de24b526482bcb95068fa52597ebe74a15f

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Author-with-label / blind-check independence (D-02): the authoring executor never blind-labels its own corpus; a fresh, tool-less subagent labels from blindLabelPrompt's text alone, and only pairs where both labels agree are kept."
    - "Label-independence proof by rotation: TestBlindLabelPromptIsLabelIndependent renders the corpus once as-authored and once with every label rotated to the next relation, asserting byte-identical output — a renderer that ever reads label would fail this immediately."
    - "Originally-adjacent interleave check: after filtering removes pairs without renumbering IDs, two never-adjacent agreed pairs can become slice-adjacent by coincidence. The interleave subtest now resets its same-label run counter on any numeric ID gap, so it measures leakage against the original authored P01..P80 sequence rather than the post-filter slice."

key-files:
  created:
    - internal/curationeval/doc.go
    - internal/curationeval/pairs.go
    - internal/curationeval/pairs_test.go
    - .planning/phases/03-curation-verdicts/03-BLIND-LABEL-PROMPT.md
    - .planning/phases/03-curation-verdicts/03-BLIND-LABELS.md
    - .planning/phases/03-curation-verdicts/03-BLIND-LABELS-ROUND1.md
  modified: []

key-decisions:
  - "Round 1's blind pass agreed 80/80 but was rejected by the user: a fixed authoring interleave order plus recordB cue phrases ('was wrong', 'as previously stated', 'now also', etc.) let the labeler infer class from wording/position rather than content. The corpus was hardened (cue phrases stripped, contradicts/updates pairs rewritten to share a sentence frame, several related/duplicate pairs rewritten as harder near-misses) and shuffled with a fixed-seed permutation before a second blind round — no external/OSS dataset was substituted; synthetic-only per D-01."
  - "Round 2's fresh, tool-less subagent (0 tool calls, no repository context) agreed on 70 of 80 pairs. Every one of the 10 disagreements was a contradicts/updates confusion in one direction or the other (P03, P07, P61, P62: intended contradicts, blind updates; P06, P25, P36, P57, P60, P72: intended updates, blind contradicts) — matching this plan's flagged D-06 criteria-overlap risk, just landing on a different class pair than the one named in the plan text (contradicts/updates, not duplicate/updates)."
  - "Fixed TestPairFixtureIntegrity's interleave subtest (Rule 1 — test bug surfaced by the plan's own explicit 'do not renumber IDs' instruction): the pre-filter version counted slice adjacency, which after deletion could count two pairs that were never adjacent in the authored P01..P80 sequence (e.g. P56/P58/P59, adjacent only because P57 was dropped) as a leaking run of 3. The fix resets the run on any numeric ID gap, preserving the test's actual intent — no more than 2 consecutive pairs sharing a label in the sequence the blind labeler actually saw — while correctly tolerating filter-induced slice adjacency that carries no such leak."

requirements-completed: []  # CUR-03 declared by 03-04, 03-07 and 03-08 (shared-ID gate, #2388) — not yet all-finished; marked complete once the last declaring plan's SUMMARY exists.

coverage:
  - id: D1
    description: "internal/curationeval.syntheticPairs is a committed, synthetic, spine-shaped 70-pair corpus (D-01) covering all five verdict.Relations() classes, each pair's label agreed by an independent blind pass (D-02)"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/pairs_test.go#TestPairFixtureIntegrity"
        status: pass
    human_judgment: false
  - id: D2
    description: "blindLabelPrompt is provably label-independent and carries verdict.RelationCriteria() verbatim, so the blind labeler saw exactly the criteria Jev sees and never saw an intended label"
    requirement: CUR-03
    verification:
      - kind: unit
        ref: "internal/curationeval/pairs_test.go#TestBlindLabelPromptIsLabelIndependent"
        status: pass
    human_judgment: false
  - id: D3
    description: "The D-02 blind-labeling round was genuinely independent (fresh, tool-less, no repository context subagent) and every kept pair's intended label matches the recorded blind label verbatim"
    verification: []
    human_judgment: true
    rationale: "Independence of the labeling context (no access to pairs.go, 0 tool calls) is a dispatch-process property no automated check inside this repo can observe; the plan's own prohibitions section flags this as judgment-verified by design. The label-agreement half is mechanically checked (D1's verification), but the independence claim itself requires trusting the orchestrator's dispatch record in 03-BLIND-LABELS.md's provenance header."
---

# Phase 3 Plan 4: Curation Verdicts Blind-Labeled Pair Corpus Summary

**70-pair, five-class, blind-agreed labeled corpus for CUR-03's curation-verdict metrics — authored in one context, labeled blind in another, with only agreed pairs kept.**

## Performance

- **Duration:** spanned two sessions (tracer + full corpus authored 2026-09-23 daytime; blind-labeling checkpoint resumed and closed 2026-09-24)
- **Tasks:** 3/3 complete
- **Commits:** 5 (plan_head_before `eeb33de2`)
- **Files touched:** 6 (3 created in `internal/curationeval`, 3 created under `.planning/phases/03-curation-verdicts/`)

## Accomplishments

- Task 1 (tracer, TDD): proved the corpus shape on 5 pairs — `labeledPair`, `syntheticPairs`, `blindLabelPrompt`, `blindLabelMarker`, and `TestPairFixtureIntegrity`'s five integrity subtests, RED-then-GREEN.
- Task 2: extended to the full 80-pair, 16-per-class, interleaved corpus and generated `03-BLIND-LABEL-PROMPT.md` via the golden-flag writer test.
- Deviation (user-directed): round 1's 80/80 blind agreement was rejected as unearned (fixed interleave order + wording cues leaked class); the corpus was hardened and reshuffled, and round 1's reply preserved for provenance.
- Task 3 (checkpoint, this session): recorded round 2's blind reply verbatim, kept the 70 pairs that agreed, updated `pairs.go`'s procedure comment, and fixed a test-invariant gap the filtering step exposed.

## Task Commits

Each task was committed atomically:

1. **Task 1: Corpus shape end to end on five pairs** — `cc058957` (test, TDD RED+GREEN)
2. **Task 2: Full 80-pair corpus and generated prompt** — `ee973136` (test)
3. **Deviation: preserve round-1 blind labels (superseded)** — `0d2e7e70` (docs)
4. **Deviation: harden corpus — strip cue phrases, shuffle order** — `ee010289` (test)
5. **Task 3: record blind labels and keep only agreed pairs** — `43830948` (test)

_No separate plan-metadata commit yet — this SUMMARY and STATE/ROADMAP updates are committed next._

## Files Created/Modified

- `internal/curationeval/doc.go` — package doc: measures advisory curation-verdicts on labeled pairs; corpus is synthetic and blind-checked
- `internal/curationeval/pairs.go` — `labeledPair`, `syntheticPairs` (70 kept pairs), `blindLabelMarker`, `blindLabelPrompt`; file comment documents the full D-01/D-02 procedure through round 2
- `internal/curationeval/pairs_test.go` — `TestPairFixtureIntegrity` (6 subtests), `TestBlindLabelPromptIsLabelIndependent`, `TestWriteBlindLabelPrompt`; interleave subtest fixed to be originally-adjacency-aware
- `.planning/phases/03-curation-verdicts/03-BLIND-LABEL-PROMPT.md` — generated, label-free prompt sent to the blind labeler (commit `ee010289`)
- `.planning/phases/03-curation-verdicts/03-BLIND-LABELS.md` — round 2's verbatim reply, 80 lines, kept as the record of agreement
- `.planning/phases/03-curation-verdicts/03-BLIND-LABELS-ROUND1.md` — round 1's superseded reply, preserved for provenance only

## Decisions Made

See `key-decisions` in frontmatter: round-1 rejection and hardening (user-directed), round-2 disagreement set (all contradicts/updates confusions), and the interleave-subtest fix (Rule 1).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Interleave subtest failed after Task 3's ID-preserving deletion**
- **Found during:** Task 3, post-deletion verification (`go test ./internal/curationeval/ -run '^TestPairFixtureIntegrity$'`)
- **Issue:** Deleting the 10 disagreed pairs without renumbering (as the plan instructs) made P56 and P58 slice-adjacent (P57 was dropped), producing a run of 3 consecutive `duplicate`-labeled pairs. The original subtest counted plain slice adjacency, which conflated "adjacent after filtering" with "adjacent in the sequence the blind labeler actually saw" — the property the invariant exists to protect.
- **Fix:** Changed the subtest to reset its run counter whenever two consecutive slice entries have a numeric-ID gap greater than 1 (i.e., they were never adjacent in the authored P01..P80 sequence). Verified this makes the max run 2 for the actual kept corpus, both by a standalone `awk` check and by the Go test.
- **Files modified:** `internal/curationeval/pairs_test.go`
- **Verification:** `go test ./internal/curationeval/ -run '^TestPairFixtureIntegrity$' -count=1 -v` — all 6 subtests PASS.
- **Commit:** `43830948`

**Total deviations:** 1 auto-fixed (test invariant gap), plus 1 user-directed procedural deviation carried forward from Task 2 (round-1 rejection/hardening, already committed at `0d2e7e70`/`ee010289` before this session). **Impact:** none on the shipped corpus's correctness; both changes make the fixture's own tests actually measure what they claim to measure.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None — this plan's threat register entries (T-03-06, T-03-07, T-03-15) are all satisfied by artifacts already in place; no new surface introduced.

## Self-Check: PASSED

- `internal/curationeval/doc.go`: FOUND
- `internal/curationeval/pairs.go`: FOUND
- `internal/curationeval/pairs_test.go`: FOUND
- `.planning/phases/03-curation-verdicts/03-BLIND-LABEL-PROMPT.md`: FOUND
- `.planning/phases/03-curation-verdicts/03-BLIND-LABELS.md`: FOUND
- `.planning/phases/03-curation-verdicts/03-BLIND-LABELS-ROUND1.md`: FOUND
- Commit `cc058957`: FOUND in `git log --oneline --all`
- Commit `ee973136`: FOUND in `git log --oneline --all`
- Commit `0d2e7e70`: FOUND in `git log --oneline --all`
- Commit `ee010289`: FOUND in `git log --oneline --all`
- Commit `43830948`: FOUND in `git log --oneline --all`
- `go test ./internal/curationeval/ -count=1`: ok
- `golangci-lint run ./internal/curationeval/...`: 0 issues
- `task license:check`: 0 invalid

Ready for 03-05.
