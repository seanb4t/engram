---
phase: 05-operator-correctness
verified: 2026-09-24T17:30:05Z
status: passed
score: 5/5 must-haves verified
covered_files:
  - .planning/phases/05-operator-correctness/05-01-PLAN.md
  - .planning/phases/05-operator-correctness/05-01-SUMMARY.md
  - .planning/phases/05-operator-correctness/05-02-PLAN.md
  - .planning/phases/05-operator-correctness/05-02-SUMMARY.md
  - .planning/phases/05-operator-correctness/05-03-PLAN.md
  - .planning/phases/05-operator-correctness/05-03-SUMMARY.md
  - .planning/phases/05-operator-correctness/05-04-PLAN.md
  - .planning/phases/05-operator-correctness/05-04-SUMMARY.md
  - .planning/phases/05-operator-correctness/05-05-PLAN.md
  - .planning/phases/05-operator-correctness/05-05-SUMMARY.md
  - cmd/engram/exitcode_baseline_test.go
  - cmd/engram/golden_test.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operator_view.go
  - cmd/engram/operator_view_test.go
  - docs-site/src/content/docs/guides/cli.md
  - internal/keylinks/keylinks.go
  - internal/keylinks/keylinks_test.go
  - internal/store/migrate_converge_test.go
covered_digest: "v1:sha256:03b745df889fd039c092247879a092d10089a24689bccd447efe4008b12d06ac"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 5: Operator Correctness Verification Report

**Phase Goal:** A handful of small, independent operator-surface bugs are fixed and covered by tests.
**Verified:** 2026-09-24T17:30:05Z
**Status:** passed
**Re-verification:** No. This is the initial verification. It checks the code as it stands after the code-review fixes (ccb1441e..c36aa815).

## Goal Achievement

Each success criterion was checked two ways. The new test passes on HEAD. The same test fails on the pre-phase tree (`git archive f69b9bd6`, extracted to a scratch directory), which shows it is a real regression test and does not pass vacuously.

### Observable Truths

| # | Truth (ROADMAP SC) | Status | Evidence |
|---|--------------------|--------|----------|
| 1 | The exit-code baseline test passes with `ENGRAM_REINDEX_TARGET` / `ENGRAM_MIGRATE_OWNER` set in the environment | ✓ VERIFIED | `TestExitCodeBaseline` passes on HEAD with `ENGRAM_REINDEX_TARGET`, `ENGRAM_MIGRATE_OWNER`, `ENGRAM_RUNTIME` and `ENGRAM_HEADERS` all set to non-empty values. It also passes under `-shuffle=on`, and with those variables unset. On the pre-phase tree it FAILS: `reindex/missing-target` and `migrate-set-owner/missing-owner` return 5, want 2. The fix is `neutralizeEnvDerivedFlagDefaults` (golden_test.go). It blanks the DefValue and the bound variable, and resets slice flags through `SliceValue.Replace`. It is called by `TestExitCodeBaseline` and by `withGoldenDeterminism`. The `envDerivedFlagDefaults` map covers all 4 init-time `os.Getenv` defaults (reindex.go:152, migrate.go:246, setup.go:342/366). No production file changed (D-03). |
| 2 | `viewFields`' bare nested-object branch is covered by a test, or removed if genuinely unreachable | ✓ VERIFIED | `TestViewFieldsBareNestedObject`, `TestViewFieldsEmptyNestedObjectRendersNoRows` and `TestViewFieldsBlankArrayElementKeepsItsRow` pass. The coverage profile shows non-zero counts on operator_view.go blocks 114.4–115.18, 118.4, 119.5 and 121.5 (the `case '{':` branch). The only zero-count block is the `viewRow` error return. The array-of-arrays sibling branch is also covered. The empty-object case now renders zero rows, and a blank array element keeps one non-blank row (review fix ccb1441e). |
| 3 | `ParsePlanKeyLinks` emits no empty key-link for a fieldless list item, matching its doc comment | ✓ VERIFIED | keylinks.go:165 filters `isFieldlessKeyLink` items out of `parsePlanKeyLinkItems`. `ScanPlansWithStats` (:564) still reads the raw items, so the malformed check is kept. `TestParsePlanKeyLinksSkipsFieldlessItems` passes on HEAD. On the pre-fix code it FAILS: it gets 5 links where 3 are expected, and 2 links where 0 are expected. `go test ./internal/keylinks/` is green, including `TestActiveMilestoneKeyLinksSatisfiable` and `TestMalformedKeyLinkEntry`. |
| 4 | A test covers a record inserted mid-sweep whose id sorts below the migrate cursor | ✓ VERIFIED | `TestMigrateBelowCursorInsertConverges` ran against a live Qdrant testcontainer (Docker 29.4.0), not skipped. It passed 4 times: once alone, 3 times with `-count=3`, and it also passed in a full `internal/store` run. The test records the triggering scroll Offset and asserts that both inserted ids sort strictly below it. It asserts that the already-current upsert is never written by the sweep. It asserts that the key-absent laggard is migrated after every seeded record, with `Passes >= 2`. It asserts convergence: Backlog 0 and an empty fresh backlog. The fixture preconditions (`fires == 1` and a non-empty cursor) are checked in the parent test (review fix 323691ac). `TestMigrateConvergesWithoutLock` still passes. `internal/store/migrate.go` is unchanged (D-05). |
| 5 | `docs-site` `guides/cli.md` lists `migrate`, `migrate status`, and `migrate revert` among the operator commands | ✓ VERIFIED | cli.md:139–144 names `` `migrate` (plus its `status` and `revert` subcommands; see the [Migrate guide](/guides/migrate/)) `` and also `setup`. The link target `guides/migrate.md` exists. The docs gate `TestCLIGuideOperatorCommandsListsEveryOperatorCommand` takes the required set from the live `operatorCommands()` tree and passes. On the pre-edit cli.md it FAILS, naming exactly `[migrate migrate revert migrate status setup]`. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

The plan-level truths (05-01..05-05 `must_haves.truths`) are covered by the checks above. The shuffle edge probe, the setup `--runtime`/`--header` adjacency, the `via:`-only non-fieldless item, the all-fieldless block, and `internal/surfaces` staying green were each run and passed.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/golden_test.go` | shared `neutralizeEnvDerivedFlagDefaults` helper | ✓ VERIFIED | Used by `TestExitCodeBaseline` and by `withGoldenDeterminism` |
| `cmd/engram/exitcode_baseline_test.go` | baseline rows isolated from ambient env | ✓ VERIFIED | Calls the helper on every row |
| `cmd/engram/operator_view.go` | bare-object and blank-element handling | ✓ VERIFIED | Substantive and covered by tests |
| `cmd/engram/operator_view_test.go` | bare nested-object tests | ✓ VERIFIED | 3 new tests, all passing |
| `internal/keylinks/keylinks.go` | fieldless filter plus raw item walk | ✓ VERIFIED | Scanner wired to the raw walk |
| `internal/keylinks/keylinks_test.go` | fieldless regression test | ✓ VERIFIED | RED before the fix, GREEN after |
| `internal/store/migrate_converge_test.go` | below-cursor convergence test | ✓ VERIFIED | Live Qdrant, not skipped |
| `docs-site/src/content/docs/guides/cli.md` | operator list names migrate, migrate status, migrate revert | ✓ VERIFIED | Enforced by a gate |
| `cmd/engram/operator_output_test.go` | docs gate derived from `operatorCommands()` | ✓ VERIFIED | RED before the fix, GREEN after |

`gsd-tools verify.artifacts`: 9/9 passed across the five plans.

### Key Link Verification

`gsd-tools verify.key-links`: 10/10 verified across the five plans.

| From | To | Via | Status |
|------|----|-----|--------|
| `TestExitCodeBaseline` | `neutralizeEnvDerivedFlagDefaults` | direct call per row | WIRED |
| `withGoldenDeterminism` | `neutralizeEnvDerivedFlagDefaults` | direct call | WIRED |
| `ScanPlansWithStats` | `parsePlanKeyLinkItems` | direct call (keylinks.go:564) | WIRED |
| `ParsePlanKeyLinks` | `isFieldlessKeyLink` | filter loop | WIRED |
| `midSweepInterceptor` | `midSweepHook.onScroll(req)` | records the Offset cursor | WIRED |
| docs gate | `operatorCommands()` / `commandKey` | live cobra tree | WIRED |

### Data-Flow Trace (Level 4)

Not applicable. The phase renders no dynamic UI data. The operator-view changes are pure functions and are exercised directly by the tests.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Baseline with env leak vars | `env -u ENGRAM_RETRIEVAL_EVAL ENGRAM_REINDEX_TARGET=leak_target ENGRAM_MIGRATE_OWNER=leak-owner ENGRAM_RUNTIME=codex ENGRAM_HEADERS=X-Leak=1 go test ./cmd/engram/ -run 'TestExitCodeBaseline$' -count=1` | ok | ✓ PASS |
| Same, on the pre-phase tree | same command in the `f69b9bd6` scratch extract | 2 rows FAIL (5 vs 2) | ✓ RED confirmed |
| Full cmd/engram with env set, shuffled | `... go test ./cmd/engram/ -shuffle=on -count=1` | ok | ✓ PASS |
| Full cmd/engram without env | `env -u ENGRAM_RETRIEVAL_EVAL go test ./cmd/engram/ -count=1` | ok | ✓ PASS |
| keylinks gate | `env -u ENGRAM_RETRIEVAL_EVAL go test ./internal/keylinks/ -count=1` | ok (12 tests) | ✓ PASS |
| Below-cursor migrate | `go test ./internal/store/ -run TestMigrateBelowCursorInsertConverges -count=3` | ok | ✓ PASS |
| Full internal/store | `env -u ENGRAM_RETRIEVAL_EVAL go test ./internal/store/ -count=1` | ok (117.6s) | ✓ PASS |
| surfaces | `go test ./internal/surfaces/ -count=1` | ok | ✓ PASS |
| Lint | `golangci-lint run ./cmd/engram/... ./internal/keylinks/... ./internal/store/...` | 0 issues | ✓ PASS |
| License | `task license:check` | invalid: 0 | ✓ PASS |

### Probe Execution

None declared. The phase has no `probe-*.sh` scripts and its criteria do not mention probes.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| OPS-01 (#476) | 05-01 | Exit-code baseline passes with the env vars set | ✓ SATISFIED | Truth 1; commit 99719b2f `Closes #476` |
| OPS-02 (#504) | 05-02 | `viewFields` bare nested-object branch tested | ✓ SATISFIED | Truth 2; commits a1a4a40f and 3ab6c86f `Closes #504` |
| OPS-03 (#502) | 05-03 | `ParsePlanKeyLinks` emits no empty key-link | ✓ SATISFIED | Truth 3; commit ddbae718 `Closes #502` |
| OPS-04 (#501) | 05-04 | Below-cursor mid-sweep insert test | ✓ SATISFIED | Truth 4; commit 39ef398f `Closes #501` |
| OPS-05 (#503) | 05-05 | cli.md lists migrate, migrate status, migrate revert | ✓ SATISFIED | Truth 5; commit 102c87f0 `Closes #503` |

No requirements are orphaned. REQUIREMENTS.md maps only OPS-01..05 to Phase 5, and every one is claimed by a plan. D-02 is met: each issue number appears in a `Closes #NNN` trailer in `git log f69b9bd6..HEAD`.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (added lines, all 9 files) | none | TBD/FIXME/XXX/TODO/HACK/t.Skip | none | No debt markers were added |
| `cmd/engram/operator_view_test.go` | 441 | `go vet` structtag: repeated json tag "dup" | ℹ️ Info | Pre-existing (62c39c22, 2026-08-22). It is an intentional duplicate-key fixture, is not from this phase, and golangci-lint is clean |

All 7 findings in 05-REVIEW.md (2 warnings, 5 info) are marked Fixed, in commits ccb1441e, e0ef82fb, e04a7359, 6be892a8 and 323691ac. The current code was re-verified after those commits.

### Human Verification Required

None. Every success criterion is checked by an automated test that was run in this verification.

### Gaps Summary

No gaps were found. All five operator fixes are in the codebase and covered by tests that pass on HEAD. Each test fails when run against the pre-phase tree.

---

_Verified: 2026-09-24T17:30:05Z_
_Verifier: Claude (gsd-verifier)_
