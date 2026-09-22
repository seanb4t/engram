---
phase: 01-test-harness-fixture-helper
plan: 05
subsystem: testing
tags: [qdrant, grpc, ast-gate, ci, red-evidence]

requires:
  - phase: 01-01
    provides: "store.NewQdrantClient(host, port, opts...); internal/store/storetest package; qdrantClientHolderAllowlist with the D-13 generalized never-writes check; redevidence_harness_test.go's redEvidenceDirs (empty)"
  - phase: 01-04
    provides: "The migrated #583 regression test (TestListScopesFullPayloadsOverGRPCLimit) over both storetest fixture shapes; the observed RED against a reverted ListScopes payload selector"
provides:
  - "internal/store/qdrant_client_convergence_test.go: D-11's repo-wide AST gate — every Qdrant client construction, _test.go files included, must resolve to store.NewQdrantClient's own body"
  - "internal/store/storetest/cipin_test.go: TestQdrantImageMatchesCIService gate-enforcing CI's services.qdrant image against storetest.QdrantImage (D-10)"
  - "Four registered red-evidence patches in redEvidenceDirs, closing TestRedEvidencePatchesAreLive"
affects: []

actuals:
  tokens: 7780
  tasks: 3
  commits: 3
plan_head_before: da0b7bb8f94ec3d63a7210f3bbc698fe614166dd

tech-stack:
  added: []
  patterns:
    - "Narrower, purpose-built AST walker rather than reusing an existing gate's walker when the predicate genuinely differs (call-expression-only vs type-reference-and-call, test-files-included vs excluded)"
    - "Set-equality (enclosingFunc, callee) pair assertions for bypass-shape fixtures, never a count-only or contains check"
    - "CI pin enforced by a Go test reading the workflow YAML as text, not by a maintained comment"
    - "One-mutation-per-patch red-evidence authored by hand-verifying git apply --check / apply / go test -run '^Target$' / apply -R before registering"

key-files:
  created:
    - internal/store/qdrant_client_convergence_test.go
    - internal/store/testdata/qdrantclient/good_store.go.txt
    - internal/store/testdata/qdrantclient/bad_test.go.txt
    - internal/store/testdata/qdrantclient/bad_aliased_test.go.txt
    - internal/store/testdata/qdrantclient/bad_store.go.txt
    - internal/store/storetest/cipin_test.go
    - .planning/phases/01-test-harness-fixture-helper/red-evidence/01-01-storetest-raw-client-write.patch
    - .planning/phases/01-test-harness-fixture-helper/red-evidence/01-04-listscopes-full-payload-selector.patch
    - .planning/phases/01-test-harness-fixture-helper/red-evidence/01-05-bare-qdrant-newclient-in-test.patch
    - .planning/phases/01-test-harness-fixture-helper/red-evidence/01-05-ci-qdrant-image-drift.patch
  modified:
    - .github/workflows/ci.yaml
    - internal/store/redevidence_harness_test.go

key-decisions:
  - "D-11's gate is a NEW, narrower AST walker (scanQdrantClientConstructions/scanRepoForClientConstructions) rather than a reuse of fileRefsQdrantClient/scanRepoForQdrantClientRefs — those conflate type references with calls and exclude _test.go files; D-11 needs the opposite on both axes (RESEARCH.md Pitfall 5)."
  - "isSanctionedConstruction keys on (display path == internal/store/store.go) AND (enclosing FuncDecl name == NewQdrantClient) together, never file identity alone — proven by the bad_store.go.txt fixture's second, illegitimate function in the same file."
  - "A dot-import of the qdrant package is itself recorded as a violation (enclosingFunc <import>, callee dot-import) rather than silently skipped, since it would otherwise make unqualified constructor calls invisible to the scanner."
  - "TestQdrantImageMatchesCIService lives in package storetest (not internal/store) since it asserts a property of storetest.QdrantImage against CI text, with its own small moduleRoot helper duplicated from internal/store's findModuleRoot rather than exported cross-package for one caller."
  - "Each red-evidence patch's mutation was chosen to be the smallest single-statement change that trips exactly its target test, verified by hand (git apply --check/apply/go test -run/apply -R) before registration — not merely assumed from the plan's prose."
  - "01-05-bare-qdrant-newclient-in-test.patch targets internal/e2e/spine_review_test.go's spineReviewQdrantClient rather than server or retrievaleval — the plan's example fixture for D-11's cross-package bypass shape, distinct from the storetest.go in-package write-injection Task 1 used for its own RED proof (not registered)."

requirements-completed: [REQ-test-client-parity, REQ-oversized-fixture-helper]

coverage:
  - id: D1
    description: "D-11's AST gate scans every .go file in the module including _test.go files, permitting a Qdrant client construction only inside store.NewQdrantClient's own receiver-less body"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient/good_fixture_yields_one_sanctioned_site"
        status: pass
      - kind: unit
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient/bad_test-helper_fixture"
        status: pass
      - kind: unit
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient/bad_aliased_fixture"
        status: pass
      - kind: unit
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient/bad_second_construction_in_store.go"
        status: pass
      - kind: integration
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient/real_module"
        status: pass
    human_judgment: false
  - id: D2
    description: "The gate's fixture subtests catch every bypass shape by set equality (direct call, sibling constructor, aliased import, dot-import, second construction in the sanctioned file) — never a count-only or contains check"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store#TestQdrantClientConstructedOnlyByNewQdrantClient (all four bad-fixture subtests, assertPairSetEqual)"
        status: pass
    human_judgment: false
  - id: D3
    description: "TestQdrantImageMatchesCIService fails when any qdrant/qdrant:vX.Y.Z reference in ci.yaml differs from storetest.QdrantImage or the services.qdrant image line is missing"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store/storetest#TestQdrantImageMatchesCIService"
        status: pass
      - kind: manual_procedural
        ref: "Observed RED against a drifted services.qdrant image line (v1.19.2), reverted cleanly — recorded below"
        status: pass
    human_judgment: false
  - id: D4
    description: "CI comments that pointed at store_test.go's image constant, the retrievaleval package variable, and server's parser now name storetest; ci.yaml stays yamlfmt/actionlint clean with its pinned 3 PASS + 1 SKIP step unchanged"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "rg -n 'store_test[.]go|testQdrantAddr|tools_test[.]go requireQdrant' .github/workflows/ci.yaml -> 0 matches"
        status: pass
      - kind: integration
        ref: "yamlfmt -lint .github/workflows/ci.yaml; actionlint .github/workflows/ci.yaml"
        status: pass
    human_judgment: false
  - id: D5
    description: "redEvidenceDirs maps this phase's red-evidence directory to four patches, each hand-verified RED against its named test, turning TestRedEvidencePatchesAreLive green"
    requirement: "REQ-oversized-fixture-helper"
    verification:
      - kind: integration
        ref: "internal/store#TestRedEvidencePatchesAreLive"
        status: pass
    human_judgment: false
  - id: D6
    description: "task (lint + test) exits 0 for the whole repository, and CI's exact shared-address command reports 3 PASS + 1 SKIP"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: integration
        ref: "task (full repo lint + test suite)"
        status: pass
      - kind: integration
        ref: "ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:6334 ENGRAM_REQUIRE_QDRANT=1 go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -> PASS=3 SKIP=1"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 5: D-11 Convergence Gate, CI Image Pin Gate, and Registered Red-Evidence Summary

**D-11's repo-wide AST gate (test files included) proves every Qdrant client construction resolves to `store.NewQdrantClient`; CI's shared image is now gate-enforced against `storetest.QdrantImage`; and four hand-verified red-evidence patches close `TestRedEvidencePatchesAreLive`, turning `task` fully green.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-09-18
- **Tasks:** 3 completed
- **Files:** 12 changed (10 created, 2 modified)

## Accomplishments

- `internal/store/qdrant_client_convergence_test.go` implements D-11: a NEW, narrower AST walker (`scanQdrantClientConstructions`, `scanRepoForClientConstructions`, `isSanctionedConstruction`) that scans every `.go` file in the module — `_test.go` included — and permits exactly one construction site: the receiver-less `NewQdrantClient` function in `internal/store/store.go`. Four fixtures under `testdata/qdrantclient/` prove the direct, sibling-constructor, aliased/dot-import, and second-construction-in-store.go bypass shapes by set equality. `TestQdrantClientConstructedOnlyByNewQdrantClient` passes all five subtests, logging non-zero non-test AND test file counts (139/215 in the real module) proving `_test.go` files were actually in scope.
- `internal/store/storetest/cipin_test.go`'s `TestQdrantImageMatchesCIService` (package `storetest`) turns D-10's CI image pin from a comment into a gate: it walks up to `go.mod`, reads `ci.yaml`, and fails if any `qdrant/qdrant:vX.Y.Z` reference differs from `storetest.QdrantImage` or the `image:` line is missing. Three stale CI comments (pointing at `store_test.go`'s deleted constant, `testQdrantAddr`, and server's deleted `requireQdrant`/`TestMain`) now name `storetest` instead; no functional CI change — `yamlfmt`/`actionlint` stay clean.
- Four red-evidence patches authored under `.planning/phases/01-test-harness-fixture-helper/red-evidence/`, each the smallest single-statement mutation that trips exactly its target test, hand-verified (`git apply --check`/`apply`/`go test -run '^Target$'`/`apply -R`) before registration in `redEvidenceDirs`:
  - `01-01-storetest-raw-client-write.patch` → `TestQdrantClientIsHeldOnlyByStorePackage` (D-13): injects `c.Upsert(...)` right after `Dial`'s client binding in `storetest.go`.
  - `01-04-listscopes-full-payload-selector.patch` → `TestListScopesFullPayloadsOverGRPCLimit` (#583): reverts `ListScopes`' scope-only `WithPayload` selector to `qdrant.NewWithPayload(true)`.
  - `01-05-bare-qdrant-newclient-in-test.patch` → `TestQdrantClientConstructedOnlyByNewQdrantClient` (D-11): injects a bare `qdrant.NewClient(&qdrant.Config{Host: "127.0.0.1", Port: 1})` call into `internal/e2e/spine_review_test.go`'s `spineReviewQdrantClient`.
  - `01-05-ci-qdrant-image-drift.patch` → `TestQdrantImageMatchesCIService` (D-10): changes only the `services.qdrant` `image:` value to `qdrant/qdrant:v1.19.2`.
  `TestRedEvidencePatchesAreLive` now passes with all four `confirmed RED:` log lines (RESEARCH.md Pitfall 1 closed).
- `task` (lint + test) exits 0 for the whole repository. CI's exact shared-address command (`TestSharedQdrantAddressHonored` across `internal/store`, `internal/server`, `internal/e2e`, `internal/retrievaleval`) reports `PASS=3 SKIP=1`. `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0.

## Task Commits

Each task was committed atomically:

1. **Task 1: D-11 convergence gate end to end: repo-wide scan including test files, proven on fixtures, green on the converged tree** - `b7e490cc` (test)
2. **Task 2: CI's pinned Qdrant image is gate-enforced against storetest.QdrantImage, and CI comments name storetest** - `d80a5b1e` (ci)
3. **Task 3 (LAST task of the phase): Author and register this phase's red-evidence patches so TestRedEvidencePatchesAreLive goes green** - `575756f8` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/store/qdrant_client_convergence_test.go` - D-11's AST gate: `qdrantImportPath`, `qdrantClientConstructors`, `clientConstruction`, `isSanctionedConstruction`, `scanQdrantClientConstructions`, `scanRepoForClientConstructions`, `TestQdrantClientConstructedOnlyByNewQdrantClient`.
- `internal/store/testdata/qdrantclient/{good_store.go.txt, bad_test.go.txt, bad_aliased_test.go.txt, bad_store.go.txt}` - the four bypass-shape fixtures.
- `internal/store/storetest/cipin_test.go` - `moduleRoot`, `qdrantImageRefPattern`, `TestQdrantImageMatchesCIService`.
- `.github/workflows/ci.yaml` - three comments reworded to name `storetest` instead of the deleted in-package harness; no functional change.
- `internal/store/redevidence_harness_test.go` - `redEvidenceDirs` populated with the phase's one directory entry mapping four patches to their target tests.
- `.planning/phases/01-test-harness-fixture-helper/red-evidence/*.patch` - the four red-evidence patches.

## Decisions Made

- D-11's gate deliberately does NOT reuse `fileRefsQdrantClient`/`scanRepoForQdrantClientRefs`: those conflate type references with calls and exclude `_test.go` files — the opposite of what D-11 needs on both axes.
- `isSanctionedConstruction` keys on (display path, enclosing function name) together, never file identity alone, proven by the `bad_store.go.txt` fixture's second illegitimate function.
- A dot-import of the qdrant package is recorded as its own violation (`<import>`/`dot-import`) rather than silently ignored, since it would otherwise make unqualified constructor calls invisible to the scanner.
- `TestQdrantImageMatchesCIService` lives in package `storetest` with its own small `moduleRoot` helper, duplicated from `internal/store`'s `findModuleRoot` rather than exported cross-package for a single caller.
- Each red-evidence patch's mutation was independently hand-verified (not merely assumed from the plan's prose) before registration.
- The D-11 red-evidence patch targets `internal/e2e/spine_review_test.go` (the plan's named example) rather than the `storetest.go` write-injection Task 1 used for its own (unregistered) RED proof — two distinct RED observations for two distinct gates, only one of which the plan asks to register per gate.

## Deviations from Plan

None - plan executed exactly as written. Every RED observation required by the plan (Task 1's injected-bypass proof, Task 2's drifted-image proof, Task 3's four hand-verified patches) is documented above as normal flow, not a deviation.

**Total deviations:** 0. **Impact:** None.

## Issues Encountered

None. Docker was reachable throughout (`docker info` exit 0), satisfying Task 3's precondition; no container was left running after any of the manual RED-proof cycles or the `task test` run — `storetest.Run`'s teardown terminated its ephemeral container in every case, confirmed in each test's log output.

## RED Observations (recorded for the plan's acceptance criteria)

**Task 1** — injected `_, _ = qdrant.NewClient(&qdrant.Config{Host: "127.0.0.1", Port: 1})` into `internal/server/tools_test.go`'s `testDeps` helper (a `_test.go` file outside `internal/store` that already imports the qdrant package):
```
qdrant_client_convergence_test.go:377: Qdrant client construction outside store.NewQdrantClient: internal/server/tools_test.go:199: NewClient in testDeps
--- FAIL: TestQdrantClientConstructedOnlyByNewQdrantClient
```
Restored via `git diff`-captured scratchpad patch + `git apply -R`; `git status --porcelain -- internal/server/tools_test.go` printed nothing afterward.

**Task 2** — changed `.github/workflows/ci.yaml`'s `services.qdrant` `image:` to `qdrant/qdrant:v1.19.2`:
```
cipin_test.go:63: .../ci.yaml: found image reference "qdrant/qdrant:v1.19.2", want "qdrant/qdrant:v1.19.1" (byte-identical to storetest.QdrantImage)
--- FAIL: TestQdrantImageMatchesCIService
```
Restored via `git diff`-captured scratchpad patch + `git apply -R`; clean afterward.

**Task 3** — each registered patch, hand-verified before registration:

| Patch | Target | Observed `--- FAIL:` |
|---|---|---|
| `01-01-storetest-raw-client-write.patch` | `TestQdrantClientIsHeldOnlyByStorePackage` | `--- FAIL: TestQdrantClientIsHeldOnlyByStorePackage (0.04s)` |
| `01-04-listscopes-full-payload-selector.patch` | `TestListScopesFullPayloadsOverGRPCLimit` | `--- FAIL: TestListScopesFullPayloadsOverGRPCLimit (1.62s)` |
| `01-05-bare-qdrant-newclient-in-test.patch` | `TestQdrantClientConstructedOnlyByNewQdrantClient` | `--- FAIL: TestQdrantClientConstructedOnlyByNewQdrantClient (0.08s)` |
| `01-05-ci-qdrant-image-drift.patch` | `TestQdrantImageMatchesCIService` | `--- FAIL: TestQdrantImageMatchesCIService (0.00s)` |

`TestRedEvidencePatchesAreLive` then confirmed all four via its own `confirmed RED:` log lines (see Accomplishments), and left `store.go`, `storetest.go`, `spine_review_test.go`, and `ci.yaml` clean afterward (`git status --porcelain` empty for all four).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 (Test Harness & Fixture Helper) is complete: the shared client constructor, `storetest` harness, oversized-fixture seeder, D-11 convergence gate, D-10 CI pin gate, and registered red-evidence are all in place and green.
- `task` (lint + test) is green repo-wide; `TestRedEvidencePatchesAreLive` and `TestQdrantClientConstructedOnlyByNewQdrantClient` both pass; CI's shared-address invariant holds at 3 PASS + 1 SKIP.
- `go test ./internal/keylinks/ -count=1` passes (checked below, per this plan's phase-expectations gate).
- No blockers or concerns for the next phase.

---
*Phase: 01-test-harness-fixture-helper*
*Completed: 2026-09-18*

## Self-Check: PASSED

All 10 created files and both modified files verified present on disk. All 3 task commits (`b7e490cc`, `d80a5b1e`, `575756f8`) verified present in `git log --oneline --all`. Every acceptance criterion for all three tasks re-run and confirmed passing: the D-11 gate's five subtests pass with non-zero test/non-test scan counts; `TestQdrantImageMatchesCIService` passes and only comment lines changed in `ci.yaml` (0 non-comment diff lines); `TestRedEvidencePatchesAreLive` passes with four `confirmed RED:` lines; `task` exits 0; `task license:check` exits 0; `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0; the shared-address invariant reads `PASS=3 SKIP=1`; no red-evidence patch carries an SPDX header; `git status --porcelain` after the final commit showed no modified tracked source file.
