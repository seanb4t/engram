---
phase: 05-validation-debt-reconciliation
plan: 01
subsystem: testing
tags: [validation-debt, go-test, planning-artifacts, nyquist]

# Dependency graph
requires:
  - phase: 03.1-merge-supersession-supersede-memory-accepts-multiple-targets
    provides: "REQ-merge-idempotency store-side tests (TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand, TestPayloadRoundTripsIdempotencyFingerprint)"
  - phase: 01-interface-enforceability
    provides: "shipped CLI exit-code / flag / timeout / client-config test suites"
  - phase: 02-interface-discoverability
    provides: "shipped surfaces registry / MCP tool annotation / catalog test suites"
  - phase: 04-spine-curation-semantic-skill
    provides: "shipped curating-spine SKILL.md and its cold-read record"
provides:
  - "Reconciled status: validated / nyquist_compliant: true frontmatter for 03.1, 01, 02"
  - "Reconciled status: validated / nyquist_compliant: false (documented PARTIAL) frontmatter for 04"
  - "A repointed REQ-merge-idempotency store-side -run command (no more fictional test name)"
  - "A pinned-both-ends no-new-server-code diff command for 04, replacing an open-ended range"
affects: [05-02-citation-fixture-355]

# Actuals (#2632)
actuals:
  tokens: 9421
  tasks: 4
  commits: 3

tech-stack:
  added: []
  patterns: ["Tests Matched count column + dated Validation Audit section, reusing 03-VALIDATION.md's reconciled shape verbatim (no invented marker glyph)"]

key-files:
  created: []
  modified:
    - .planning/phases/03.1-merge-supersession-supersede-memory-accepts-multiple-targets/03.1-VALIDATION.md
    - .planning/phases/01-interface-enforceability/01-VALIDATION.md
    - .planning/phases/02-interface-discoverability/02-VALIDATION.md
    - .planning/phases/04-spine-curation-semantic-skill/04-VALIDATION.md

key-decisions:
  - "Task 3 checkpoint (blocking): applied narrow-and-record to 04's no-new-server-code row — human-selected after live re-verification showed the row's open-ended 72a32c58..HEAD range was already red (internal/server/verbtabledocs_test.go, commit a2599027, landed during phase 04's own review-fix pass). Repointed to the pinned 72a32c58..b992929b range with test files excluded, confirmed empty live, recorded as the file's 4th numbered correction."
  - "02's rows with no -run pattern (whole-package/regen/lint runs) get Tests Matched: n/a with an automated-test ✅ green Status, since they do have an automated command; the checkpoint:decision and manual/checkpoint:human-action rows get Tests Matched: n/a AND Status: N/A (not the automated-test green marker), since no automated test ran for them."
  - "03.1's REQ-merge-idempotency row (two separate go test invocations) gets a single summed Tests Matched integer (3+2=5) rather than a compound cell, to satisfy the per-row single-integer expectation."
  - "REQUIREMENTS.md intentionally NOT touched — the orchestrator owns it this wave because plan 05-02 substantively rewrites REQ-nyquist-reconciled's text there; this plan's update_requirements write step was skipped per explicit instruction."

requirements-completed: [REQ-nyquist-reconciled]

duration: ~40min (includes a blocking checkpoint pause for human decision)
completed: 2026-08-12
status: complete
---

# Phase 5 Plan 1: Validation Debt Reconciliation Summary

**Reconciled four v0.13.x VALIDATION.md records against live HEAD facts — repointed one fictional test name, flipped three files to fully validated, and partially reconciled a fourth (04) whose one genuinely unproven requirement stays visibly unproven.**

## Performance

- **Duration:** ~40 min (includes a blocking checkpoint pause for a human decision)
- **Started:** 2026-08-12T11:44:32Z (approx, per STATE.md)
- **Completed:** 2026-08-12T12:25:30Z
- **Tasks:** 4 (Task 3 was a blocking `checkpoint:decision`)
- **Files modified:** 4 (plus this SUMMARY.md)

## Accomplishments

- **03.1**: repointed the REQ-merge-idempotency store-side command, which named `TestSupersedeIdempotency` — a test that never shipped (confirmed via `go test -list` resolving to zero at HEAD) — to the two funcs that actually shipped in `internal/store/store_test.go`: `TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand` and `TestPayloadRoundTripsIdempotencyFingerprint`. All five pattern elements in the file now re-resolve to real tests; frontmatter flipped to `status: validated`, `nyquist_compliant: true`.
- **01 and 02**: all 18 `-run` pattern elements across both files re-resolved live against `go test -list '.*' ./...` at HEAD (none unresolved); both files renamed their `File Exists` column to `Tests Matched`, recorded the resolved count per row, and flipped to `status: validated`, `nyquist_compliant: true`.
- **04 (partial, per D-03)**: re-verified live that the forbidden-tool row's paired negative/positive grep genuinely holds (negative = 0, positive = 13 references across all 6 allowed tools). Discovered live that the no-new-server-code row's premise ("both structural rows have commands and passed") was false as stated — its `72a32c58..HEAD` range had already gone red on a doc-binding test file the phase's own review-fix pass added. Surfaced this as a blocking checkpoint (Task 3) rather than silently resolving it.
- **Task 3 checkpoint**: human selected `narrow-and-record` on the live evidence. Repointed the row to a pinned `72a32c58..b992929b` range excluding test files, confirmed empty, and recorded it as the file's fourth numbered correction in the same voice as the existing three.
- **04's REQ-consent-adversarial-proof row, its explanatory paragraph, and its Manual-Only row were left completely untouched** — that requirement's cold read genuinely terminated at NOT-OBTAINED and stays visibly unproven, as D-03 requires.

## Task Commits

Each task was committed atomically:

1. **Task 1: Repoint 03.1's fictional test name and reconcile the file end-to-end** - `eaabbf2d` (docs)
2. **Task 2: Reconcile 01 and 02 the same way** - `44cb4892` (docs)
3. **Task 3: checkpoint:decision (blocking)** - no separate commit; decision applied together with Task 4
4. **Task 4: Partially reconcile 04 — resolve the structural rows, leave the open requirement open** - `1cbf9ebb` (docs, includes the Task 3 decision)

**Plan metadata:** committed separately per the final-commit step (SUMMARY.md + STATE.md + ROADMAP.md).

## Files Created/Modified

- `.planning/phases/03.1-merge-supersession-supersede-memory-accepts-multiple-targets/03.1-VALIDATION.md` - repointed fictional test name, added Tests Matched counts + Validation Audit, flipped to validated
- `.planning/phases/01-interface-enforceability/01-VALIDATION.md` - added Tests Matched counts + Validation Audit, flipped to validated
- `.planning/phases/02-interface-discoverability/02-VALIDATION.md` - added Tests Matched counts (incl. `n/a` for non-`-run` and checkpoint/manual rows) + Validation Audit, flipped to validated
- `.planning/phases/04-spine-curation-semantic-skill/04-VALIDATION.md` - resolved the two structural rows, re-anchored the no-new-server-code range, extended the corrections list to four, flipped to validated with `nyquist_compliant: false` (documented PARTIAL); REQ-consent-adversarial-proof left pending

## Decisions Made

- **Task 3 (blocking checkpoint, human-decided):** `narrow-and-record` — re-anchor 04's no-new-server-code row to the pinned `72a32c58..b992929b` range with test files excluded, mark it green, and record the correction, rather than marking it red (which would misdescribe a doc-binding test as new server behavior) or leaving it pending (which would restate the drift this phase exists to clear).
- Rows without any automated command (a `checkpoint:decision` row and two manual/`checkpoint:human-action` rows in 02) were marked `Tests Matched: n/a` **and** `Status: N/A` — not the `✅ green` automated-test marker — since no automated test ran for them; marking them green would have misrepresented a manual/gate step as a passing test.
- 03.1's REQ-merge-idempotency row (server-side + store-side commands in one cell) got a single summed integer (5) in Tests Matched rather than a compound "3+2" string, to keep the column's per-row value an unambiguous integer.
- `.planning/REQUIREMENTS.md` was intentionally left untouched, per explicit orchestrator instruction — plan 05-02 substantively rewrites `REQ-nyquist-reconciled`'s text there this wave, and both plans declare that requirement.

## Deviations from Plan

None — plan executed exactly as written, including the blocking checkpoint at Task 3, which surfaced a real finding (D-03's premise was false for one row) rather than deciding it autonomously.

## Issues Encountered

None beyond the Task 3 finding itself, which was the plan's own designed decision point, not an unplanned issue.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four `-run`-bearing v0.13.x VALIDATION.md files under `.planning/phases/` now state what is actually true at HEAD: 01, 02, 03.1 fully validated; 04 partially validated with its one genuinely unproven requirement (`REQ-consent-adversarial-proof`) still visibly open.
- `.planning/milestones/**` confirmed untouched throughout (`git status --porcelain .planning/milestones/` empty at every checkpoint).
- `.planning/REQUIREMENTS.md` is unchanged by this plan; plan 05-02 (same wave) owns the D-05/D-06 wording corrections there.
- No CI gate, `task` target, committed script, or reconciliation report artifact was introduced — matching D-09/D-11.

---
*Phase: 05-validation-debt-reconciliation*
*Completed: 2026-08-12*
