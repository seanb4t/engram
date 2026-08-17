---
phase: 06-typed-operator-renderer
plan: 07
subsystem: cli
tags: [go, testing, reflection, operator-cli, json, cobra]

# Dependency graph
requires:
  - phase: 06-typed-operator-renderer (06-01 through 06-06)
    provides: the view renderer, all 15 converted operator reports, and each plan's own group fixture function (pruneViewFixtures, flatViewFixtures, migrateViewFixtures, archivePurgeViewFixtures, spineViewFixtures)
provides:
  - Deletion of TestOperatorOutputParity and its hand-built operatorParityRows table, jsonScalarValues, and containsString
  - operatorViewFixtures() merging all five group fixture functions into one map, keyed by commandKey
  - TestOperatorViewFixturesCoverEveryOperatorCommand — both-directions enumeration gate against operatorCommands(), the retired parity test's inherited property
  - setDiff() + TestSetDiffDetectsDivergence — committed non-vacuity proof, independent of operatorCommands()/operatorViewFixtures()
  - TestOperatorViewIdentityAcrossEveryOperatorCommand — identity gate run once over the complete merged set
  - TestOperatorDocsAreHandDeclared — tier-wide reflection assertion of threat T-06-01 across all 15 reports
  - A live-demonstrated, reverted transcript proving SC3 (ROADMAP.md Phase 6)
affects: [phase-7-console-cli-state-surfacing]

actuals:
  tokens: 7413
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Both-directions enumeration gate derived from a live cobra tree (commandKeySet(operatorCommands())), never a hand-written command list"
    - "Merged multi-plan fixture map with panic-on-duplicate-key collision detection, rather than silent overwrite"
    - "Pure setDiff() helper kept independent of the gate's own data sources so its own test cannot be satisfied by the same evidence the gate reads"
    - "Reflection-based tier-wide invariant assertion (TestOperatorDocsAreHandDeclared) replacing a per-file doc-comment convention"

key-files:
  created: []
  modified:
    - cmd/engram/operator_output_test.go
    - cmd/engram/migrate_family.go
    - cmd/engram/operator_view_archive_purge_test.go
    - cmd/engram/operator_view_flat_test.go
    - cmd/engram/operator_view_migrate_test.go
    - cmd/engram/operator_view_scan_test.go

key-decisions:
  - "Reworded the retirement comment and every stale cross-plan comment referencing the retired operatorParityRows/TestOperatorOutputParity/facts identifiers (in operator_view_scan_test.go, operator_view_migrate_test.go, operator_view_flat_test.go, operator_view_archive_purge_test.go) so the plan's literal rg -c ... outputs 0 acceptance criteria are actually satisfied, rather than leaving dangling documentation references to deleted code."
  - "Removed the now-dead //nolint:unparam on migrateSummary (migrate_family.go): deleting the retired test's only call site dropped unparam's call-site evidence below its flagging threshold for that function specifically (migrateReportDoc keeps its own nolint, since its evidence count stays above threshold via remaining test call sites)."
  - "Ran the SC3 probe via json.Encoder + renderOperatorView directly in a scratch test rather than go run ./cmd/engram, since no live Qdrant/store was reachable in this environment; the plan explicitly permits this fallback."
  - "TestOperatorViewIdentityAcrossEveryOperatorCommand and TestOperatorDocsAreHandDeclared both run one subtest per commandKey (looping over that command's document variants inside the subtest) rather than one subtest per (command, document-index) pair, to satisfy the acceptance criteria's 'exactly 15 subtest PASS lines' requirement."

patterns-established:
  - "Multi-plan fixture merge with loud duplicate-key detection: any future group fixture function added to operatorViewFixtures() that collides with an existing commandKey panics at test time rather than silently losing coverage."

requirements-completed: [REQ-operator-renderer-typed]

coverage:
  - id: D1
    description: "TestOperatorOutputParity and its hand-built row table (operatorParityRow, operatorParityRows, jsonScalarValues, containsString) are deleted"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "rg -c 'operatorParityRow' cmd/engram/ outputs empty (0 matches)"
        status: pass
      - kind: unit
        ref: "go test ./cmd/engram/ -run 'TestOperatorOutput|TestRenderOperator' -v"
        status: pass
    human_judgment: false
  - id: D2
    description: "operatorViewFixtures() merges all five group fixture functions and is gated against operatorCommands() in both directions, derived from the live cobra tree"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorViewFixturesCoverEveryOperatorCommand"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestSetDiffDetectsDivergence"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorViewIdentityAcrossEveryOperatorCommand (exactly 15 subtest PASS lines)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The enumeration gate is proven able to fail in both directions by live mutation (extra fixture key not naming a command; a fixture group omitted), then reverted clean"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: manual_procedural
        ref: "Live mutation transcript recorded below (both directions observed RED, then git diff --exit-code clean)"
        status: pass
    human_judgment: true
    rationale: "The mutation-and-revert cycle was performed interactively in this session and is recorded as a transcript below rather than as a committed red-evidence patch (the harness is deferred per 06-CONTEXT.md); a human should confirm the transcript is a faithful account."
  - id: D4
    description: "Success Criterion 3 (ROADMAP.md Phase 6) demonstrated live: one field added to pruneOutputDoc appears in both --output json and --output text with no second call site edited, then reverted"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: manual_procedural
        ref: "SC3 probe transcript recorded below; git diff --exit-code cmd/engram/prune.go passes after revert"
        status: pass
    human_judgment: true
    rationale: "SC3 is an ergonomic claim about future maintenance cost, not a runtime-checkable invariant (06-VALIDATION.md's original framing, still valid even though that file is otherwise stale); the transcript is the evidence, but a human should read it rather than trust an automated pass/fail."
  - id: D5
    description: "TestOperatorDocsAreHandDeclared asserts threat T-06-01 (no operator document embeds a store package result type) across all 15 reports at once"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorDocsAreHandDeclared (exactly 15 subtest PASS lines)"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-17
status: complete
---

# Phase 6 Plan 7: Retire the Parity Gate, Land the Enumeration Gate, Prove SC3 Summary

**Retired `TestOperatorOutputParity`'s hand-built row table, carried its one good both-directions property onto a merged 15-command fixture gate, and proved SC3's "one field, both lanes, no second call site" claim with a live transcript rather than prose.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-08-17T14:55:00Z (approx.)
- **Completed:** 2026-08-17T15:13:05Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- Deleted `operatorParityRow`, `operatorParityRows` (all 15 hand-built rows), `jsonScalarValues`, `containsString`, and `TestOperatorOutputParity` itself — the obsolete json/text parity gate 06-CONTEXT.md D-09 retires. `TestOperatorOutputFormat`, `TestRenderOperatorTextAndJSON`, `TestOperatorOutputEncoding`, `TestOperatorOutputEmpty`, and `TestOperatorOutputStream` all still pass unchanged.
- Landed `operatorViewFixtures()`, merging the five sibling plans' group fixture functions (`pruneViewFixtures`, `flatViewFixtures`, `migrateViewFixtures`, `archivePurgeViewFixtures`, `spineViewFixtures`) into one map with loud panic-on-duplicate-key detection, and `TestOperatorViewFixturesCoverEveryOperatorCommand` — the retired parity gate's one genuinely good property (both-directions set equality against `operatorCommands()`), now derived from the live cobra tree rather than any hand-written list.
- Added `setDiff()` and its own committed non-vacuity proof `TestSetDiffDetectsDivergence`, plus `TestOperatorViewIdentityAcrossEveryOperatorCommand` running the shared identity gate once over the complete merged set (15 subtests, one per operator command).
- Proved the enumeration gate can actually go RED in both directions by live mutation (see transcript below), then reverted clean.
- Added `TestOperatorDocsAreHandDeclared`, the tier-wide reflection-based assertion of threat T-06-01 (every operator document is a struct hand-declared in `cmd/engram`, never an embedded `internal/store` type) across all 15 commands and every fixture variant — replacing the per-file doc-comment convention with a runtime check.
- Demonstrated ROADMAP Success Criterion 3 live: added a throwaway `Probe string` field to `pruneOutputDoc`, set it in `pruneReportDoc` only (exactly two edited lines), rendered it through both the json and text lanes, captured the transcript, then reverted completely.

## Task Commits

Each task was committed atomically:

1. **Task 1: Retire `TestOperatorOutputParity` and its hand-built row table** — `5d4ce183` (test)
2. **Task 2: Land the both-directions enumeration gate over the merged fixture set** — `7f1ed2f1` (test)
3. **Task 3: Demonstrate Success Criterion 3 live and verify the disclosure invariant across all 15 reports** — `dd336aa0` (test)

**Plan metadata:** (this commit, made after this SUMMARY)

## Files Created/Modified

- `cmd/engram/operator_output_test.go` — deleted the parity gate and its helpers; added `operatorViewFixtures`, `setDiff`, `TestOperatorViewFixturesCoverEveryOperatorCommand`, `TestSetDiffDetectsDivergence`, `TestOperatorViewIdentityAcrossEveryOperatorCommand`, and `TestOperatorDocsAreHandDeclared`
- `cmd/engram/migrate_family.go` — removed a now-dead `//nolint:unparam` on `migrateSummary` (see Deviations)
- `cmd/engram/operator_view_archive_purge_test.go`, `operator_view_flat_test.go`, `operator_view_migrate_test.go`, `operator_view_scan_test.go` — reworded stale prose comments that referenced the now-deleted `operatorParityRows`/`TestOperatorOutputParity`/`facts` identifiers, so they document provenance without naming retired code

## The Two Facts About the Retired Test (durable record `b3wd4wwwda`)

Recorded in a comment in `operator_output_test.go` where the test used to be, and here for redundancy:

1. Its `facts` strings were hand-listed per row — precisely the "test over hand-built rows" ROADMAP Success Criterion 1 rejects.
2. It was one-directional: it asserted every declared text fact appeared in the json document, but never that the json document failed to widen past the text.

Its one genuinely good property — gating its row set against `operatorCommands()` in both directions — is now `TestOperatorViewFixturesCoverEveryOperatorCommand`'s job.

## Final 15-Command Coverage List

`operatorCommands()` and `operatorViewFixtures()` both report exactly 15 entries (confirmed live via a throwaway probe, then reverted):

`reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`, `migrate`, `migrate status`, `migrate revert`, `migrate-set-owner`, `migrate-remap-owner`, `spine-review scan`, `spine-review verify`, `spine-review consolidate`, `spine-review archive`, `spine-review restore`, `spine-review purge`.

## Three Additive JSON Keys the Phase Introduced (per prior plans' SUMMARYs)

Recorded here for completeness per this plan's `<output>` instruction — these landed in earlier plans of this phase, not this one:

- `current_version` (`migrate status`) — the schema-version bucket the JSON lane previously omitted.
- `rerun` (`spine-review purge` preview) — the re-run hint the applied-mode sentence never stated.
- `scope` (`spine-review scan`) — 06-CONTEXT.md's flagged gap: the sentence already rendered the scan target, but `spineScanReportDoc` had no corresponding key until conversion under D-01 surfaced it.

## The Enumeration Gate's RED-in-both-directions Proof (live mutation transcript)

Per the plan's known-trap ("a gate that cannot go RED is worse than no gate"), `TestOperatorViewFixturesCoverEveryOperatorCommand` was mutated live in both directions and observed to fail, then reverted clean.

**Direction 1 — extra fixture key not naming a real operator command.** Injected `merged["totally-fake-command"] = []any{...}` into `operatorViewFixtures()`:

```
=== RUN   TestOperatorViewFixturesCoverEveryOperatorCommand
    operator_output_test.go:229: operatorViewFixtures() has an entry keyed "totally-fake-command", which is not in operatorCommands()
--- FAIL: TestOperatorViewFixturesCoverEveryOperatorCommand (0.00s)
FAIL
```

**Direction 2 — an operator command's fixture removed.** Removed `pruneViewFixtures()` from the merge's group list:

```
=== RUN   TestOperatorViewFixturesCoverEveryOperatorCommand
    operator_output_test.go:224: operator command "prune-expired" has no entry in operatorViewFixtures()
--- FAIL: TestOperatorViewFixturesCoverEveryOperatorCommand (0.00s)
FAIL
```

Both mutations were then reverted; `git diff --exit-code cmd/engram/operator_output_test.go` was clean immediately before re-committing the intended (non-mutated) Task 2 changes, and the full test suite passed again (`TestOperatorViewFixturesCoverEveryOperatorCommand`, `TestSetDiffDetectsDivergence`, `TestOperatorViewIdentityAcrossEveryOperatorCommand` all PASS).

## SC3 Probe Transcript (ROADMAP.md Phase 6 Success Criterion 3)

Per durable record `1xe3ze1v9s` ("prose-only non-vacuity claims retain no artifact"), the probe was run live and its output captured verbatim rather than merely claimed.

No live Qdrant/store was reachable in this environment, so — per the plan's explicit fallback — `pruneReportDoc(...)` was rendered through `json.Encoder` and `renderOperatorView` directly in a scratch test, rather than via `go run ./cmd/engram prune-expired --output json|text`.

**Edit** (exactly two lines, `git diff --stat cmd/engram/prune.go` showed `1 file changed, 2 insertions(+), 1 deletion(-)`):

```diff
 type pruneOutputDoc struct {
 	Preview    bool      `json:"preview"`
 	Eligible   uint64    `json:"eligible"`
 	Deleted    uint64    `json:"deleted"`
 	Before     time.Time `json:"before"`
 	BestEffort bool      `json:"best_effort"`
+	Probe      string    `json:"probe"` // THROWAWAY: 06-07-PLAN.md Task 3 SC3 probe, reverted before commit.
 }
 ...
 func pruneReportDoc(deleted uint64, before time.Time) pruneOutputDoc {
-	return pruneOutputDoc{Preview: false, Deleted: deleted, Before: before, BestEffort: true}
+	return pruneOutputDoc{Preview: false, Deleted: deleted, Before: before, BestEffort: true, Probe: "sc3-probe-value"}
 }
```

**Captured output** (`go test ./cmd/engram/ -run TestZZZSC3Probe -v`):

```
=== --output json ===
{"preview":false,"eligible":0,"deleted":31,"before":"2031-06-15T12:00:00Z","best_effort":true,"probe":"sc3-probe-value"}
=== --output text ===
pruned ~31 expired record(s) (not_after < 2031-06-15T12:00:00Z; best-effort count)

  Preview       false
  Eligible      0
  Deleted       31
  Before        2031-06-15T12:00:00Z
  Best effort   true
  Probe         sc3-probe-value
--- PASS: TestZZZSC3Probe (0.00s)
```

The `probe` key appears in both lanes with no second call site edited (only `pruneOutputDoc`'s struct and `pruneReportDoc`'s constructor were touched — `prunePreviewDoc` and every call site were untouched). The probe was then fully reverted: `git diff --exit-code cmd/engram/prune.go` exits 0, and the throwaway scratch test file was deleted before it was ever staged or committed.

## Decisions Made

- Reworded every stale cross-plan comment referencing the retired `operatorParityRows`/`TestOperatorOutputParity`/`facts` identifiers so the plan's literal `rg -c ...` acceptance criteria are actually satisfied by the whole `cmd/engram/` tree, not just the file this task's `<files>` scope named. Treated as Rule 1 (bug: dangling documentation reference to deleted code) rather than scope creep, since the plan's own acceptance criteria required it.
- Removed the now-dead `//nolint:unparam` directive on `migrateSummary` (`migrate_family.go`): deleting the retired test's only call site to that function dropped `unparam`'s call-site evidence below its flagging threshold specifically for `migrateSummary`. `migrateReportDoc`'s own `//nolint:unparam` is unaffected — its evidence count stays above threshold via five remaining test call sites plus two production call sites.
- Reconciled against 06-06's prior narrowing of `operatorParityRows`' `facts` lists (noted in `<current_state_you_are_inheriting>`) by deleting the whole table rather than attempting to un-narrow it — the table was retired outright, so the narrowing is moot; no code was written that preserves or extends it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Reworded stale cross-plan comments referencing retired symbols**
- **Found during:** Task 1 (verifying `rg -c 'operatorParityRow' cmd/engram/` outputs 0)
- **Issue:** `cmd/engram/operator_view_scan_test.go`, `operator_view_migrate_test.go`, `operator_view_flat_test.go`, and `operator_view_archive_purge_test.go` — written by earlier sibling plans (06-03 through 06-06) — carried prose comments naming `operatorParityRows`/`operatorParityRow`/`TestOperatorOutputParity` to explain fixture-value provenance. After Task 1's deletion these became dangling references to nonexistent code, and the plan's own acceptance criteria required a whole-tree grep of 0 matches.
- **Fix:** Reworded each comment to describe the fixed-value provenance generically ("the phase's now-retired parity gate (06-CONTEXT.md D-09)") without naming the deleted identifiers.
- **Files modified:** cmd/engram/operator_view_scan_test.go, operator_view_migrate_test.go, operator_view_flat_test.go, operator_view_archive_purge_test.go
- **Verification:** `rg -c 'operatorParityRow' cmd/engram/` and `rg -c 'jsonScalarValues|containsString|TestOperatorOutputParity' cmd/engram/` both return no matches; `go build ./...` and `task` pass.
- **Committed in:** 5d4ce183 (Task 1 commit)

**2. [Rule 1 - Bug] Removed a now-dead `//nolint:unparam` directive**
- **Found during:** Task 1 (`task lint` after the deletion)
- **Issue:** `golangci-lint`'s `nolintlint` flagged `//nolint:unparam` on `migrateSummary` (migrate_family.go:144) as unused — deleting the retired test's only call site to `migrateSummary` dropped `unparam`'s call-site evidence below the threshold that made it flag the function in the first place, so the suppression directive was now suppressing nothing.
- **Fix:** Removed the directive; kept the doc-comment explanation of why `target` is deliberately general, with a note on why the nolint is gone.
- **Files modified:** cmd/engram/migrate_family.go
- **Verification:** `task lint:go` — 0 issues.
- **Committed in:** 5d4ce183 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — bug/dangling-reference and stale-lint-suppression correctness, both directly caused by Task 1's deletion, both within the acceptance-criteria's literal requirements even though the touched files exceeded the plan's declared `<files>` list for that task).
**Impact on plan:** Both fixes were necessary for the plan's own stated acceptance criteria to pass (whole-`cmd/engram/`-tree grep-for-zero) and for `task lint` to stay green. No scope creep beyond what those two constraints required.

## Issues Encountered

None. No live Qdrant/store was reachable for the SC3 probe's preferred `go run ./cmd/engram` path, so the plan's explicit scratch-test fallback (`json.Encoder` + `renderOperatorView` directly) was used instead — this is a plan-sanctioned alternative, not an issue.

## User Setup Required

None — no external service configuration required.

## Known Stubs

None.

## Deferred Follow-up (recorded, not smuggled in)

Per this plan's `<output>` instruction and 06-CONTEXT.md's `<deferred>` section: the red-evidence patch harness at `internal/store/redevidence_harness_test.go:303-316` (durable record `366pjeht8e`) currently accepts a build failure as proof of RED. This phase deliberately did NOT use it and did NOT add phase 06 to `redEvidenceDirs` — `06-VALIDATION.md`'s opposite instruction is stale and was ignored per this plan's explicit constraint. `git diff --exit-code internal/store/redevidence_harness_test.go` confirms zero changes. Hardening that harness remains open as a follow-up, tracked outside this phase.

## Next Phase Readiness

- Phase 6 is complete: all 7 plans executed, the view renderer mechanism is in place across all 15 operator commands, and the merged both-directions enumeration gate plus the tier-wide hand-declared-doc assertion replace the retired parity test's coverage.
- `go test ./...` and `task` (lint + test) are green across the whole module; `go.mod`/`go.sum` are untouched (zero new dependencies across the whole phase, per ROADMAP verification).
- Phase 7 (Console & CLI State Surfacing) can now add the six new record-state fields to operator reports, each touching exactly one struct — the ergonomic promise this plan demonstrated live (SC3).
- No blockers.

---
*Phase: 06-typed-operator-renderer*
*Completed: 2026-08-17*

## Self-Check: PASSED

- `.planning/phases/06-typed-operator-renderer/06-07-SUMMARY.md` — FOUND
- Commits `5d4ce183`, `7f1ed2f1`, `dd336aa0` — all FOUND in `git log --oneline --all --grep="06-07"`
- All plan-level `<verification>` commands re-run and passing: `go test ./cmd/engram/...` exits 0; `TestOperatorViewIdentityAcrossEveryOperatorCommand` prints exactly 15 subtest PASS lines; `rg -c 'operatorParityRow' cmd/engram/` returns no matches; `git diff --exit-code internal/store/redevidence_harness_test.go` exits 0; `git diff --exit-code go.mod go.sum` exits 0
- `task` (lint + test) green across the whole module
- REQ-operator-renderer-typed marked complete in REQUIREMENTS.md
