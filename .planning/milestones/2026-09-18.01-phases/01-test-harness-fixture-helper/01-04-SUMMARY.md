---
phase: 01-test-harness-fixture-helper
plan: 04
subsystem: testing
tags: [qdrant, grpc, storetest, testcontainers, ast-gate]

requires:
  - phase: 01-01
    provides: "internal/store/storetest package: RecvLimit, QdrantImage, Run/terminate lifecycle, RequireQdrant/SkipOrFailNoQdrant, Addr/EnvAddr/ContainerBooted, AssertSharedAddressHonored, Dial, SeedOversized"
  - phase: 01-02
    provides: "internal/store's dialTestClient builds through NewQdrantClient"
provides:
  - "internal/store's TestMain lives in an external main_test.go (package store_test), delegating to storetest.Run and installing storetest.SkipOrFailNoQdrant via store.SetNoQdrantHandler"
  - "The migrated #583 regression test (TestListScopesFullPayloadsOverGRPCLimit) over both storetest fixture shapes, at the named storetest.RecvLimit"
  - "TestManySmallShapeFitsOneListPage pinning storetest.ManySmallRecords to store.MaxListLimit"
affects: ["01-05"]

actuals:
  tokens: 6938
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "External-package TestMain as the D-07 import-cycle breaker: internal/store's TestMain moved to package store_test (main_test.go), delegating entirely to storetest.Run; the skip-or-fail decision crosses the store/store_test boundary through an export_test.go hook (SetNoQdrantHandler), never a shared package-level var"
    - "Named-limit self-proof carried through migration: the migrated #583 test still names storetest.RecvLimit explicitly at both Dial and SeedOversized call sites, so a passing result proves the bounded-read mechanism, not a client-side accident (rule m45p2b4bp7)"

key-files:
  created:
    - internal/store/main_test.go
    - internal/store/export_test.go
    - internal/store/listscopes_oversized_test.go
  modified:
    - internal/store/store_test.go
    - internal/store/instrument_test.go

key-decisions:
  - "noQdrantHandler is a var func(testing.TB) declared in store_test.go (package store, _test.go-only), set exactly once by main_test.go's TestMain via the new export_test.go SetNoQdrantHandler — the only channel across the D-07 import-cycle boundary, since a package-level var in one _test.go file is invisible from the other package's files in the same directory."
  - "dialTestClient now reads os.Getenv(\"ENGRAM_QDRANT_TEST_ADDR\") directly instead of the deleted testQdrantAddr package var, and calls noQdrantHandler(t) unconditionally when the address is empty — matching storetest.Dial's own address-then-handler shape."
  - "TestDialTestClientFailsWhenRequiredAndUnavailable's re-exec'd child now carries ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:1 in its environment (added after os.Environ() so it wins) so the child's own TestMain always takes storetest.Run's envAddr fast path and never attempts to boot a container; the in-test helper branch clears the var to \"\" before calling dialTestClient, so the placeholder address is set but never dialed."
  - "export_test.go's NewTestStore/PrefixedTestCollection/MaxListLimit exports were added in Task 2 (not Task 1) since Task 1 had no external caller for them yet — SetNoQdrantHandler was Task 1's only addition to that file."
  - "listscopes_oversized_test.go's Spec literal is written on one line (Limit: storetest.RecvLimit, Shape: shape, Vector: ...) rather than gofmt's default field-aligned multi-line form, matching the plan's exact literal text and the acceptance criterion's single-space regex; gofmt does not reformat a struct literal that already fits one line."
  - "qdrantTOCTOUVerifiedVersion's doc comment and all four TOCTOU failure messages were reworded to name storetest.QdrantImage instead of the deleted qdrantImageTag constant, keeping the two constants' D-10 separation legible in prose as well as in code."

requirements-completed: [REQ-oversized-fixture-helper, REQ-test-client-parity]

coverage:
  - id: D1
    description: "internal/store's TestMain lives in an external package (main_test.go, package store_test); it delegates container lifecycle to storetest.Run and installs storetest.SkipOrFailNoQdrant via store.SetNoQdrantHandler before any test runs. The duplicated requireQdrant, terminateQdrant, TestRequireQdrant, image-tag constant, and address/booted vars are gone."
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "rg -n 'func (requireQdrant|terminateQdrant|TestRequireQdrant)[(]' internal/store/*_test.go -> 0 matches"
        status: pass
      - kind: unit
        ref: "rg -n -w 'testQdrant(Addr|ContainerBooted)|qdrantImageTag' internal/store/*_test.go -> 0 matches"
        status: pass
      - kind: integration
        ref: "internal/store — ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -v (top-level failures = exactly TestRedEvidencePatchesAreLive)"
        status: pass
    human_judgment: false
  - id: D2
    description: "dialTestClient reads the Qdrant address from ENGRAM_QDRANT_TEST_ADDR and takes its skip-or-fail decision from the installed noQdrantHandler hook; the fail-closed contract (ENGRAM_REQUIRE_QDRANT set + no Qdrant -> FAIL, not skip) is preserved verbatim through the relocated handler"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: integration
        ref: "internal/store#TestDialTestClientFailsWhenRequiredAndUnavailable"
        status: pass
      - kind: integration
        ref: "internal/store#TestDialTestClientSkipsWhenNotRequired"
        status: pass
    human_judgment: false
  - id: D3
    description: "TestSharedQdrantAddressHonored is defined exactly once in internal/store (main_test.go), and CI's pinned shared-address invariant across internal/store, internal/server, internal/e2e, internal/retrievaleval reads exactly 3 PASS + 1 SKIP"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "rg -n '^func (TestMain|TestSharedQdrantAddressHonored)[(]' internal/store/*_test.go -> exactly two lines, both in main_test.go"
        status: pass
      - kind: integration
        ref: "ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:6334 ENGRAM_REQUIRE_QDRANT=1 go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -> PASS=3 SKIP=1"
        status: pass
    human_judgment: false
  - id: D4
    description: "TestListScopesFullPayloadsOverGRPCLimit migrated onto storetest.Dial + storetest.SeedOversized over both fixture shapes (few-large, many-small), each naming storetest.RecvLimit explicitly to both the dial and the seeder; every seeded record is counted by ListScopes"
    requirement: "REQ-oversized-fixture-helper"
    verification:
      - kind: integration
        ref: "internal/store#TestListScopesFullPayloadsOverGRPCLimit/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestListScopesFullPayloadsOverGRPCLimit/many-small"
        status: pass
    human_judgment: false
  - id: D5
    description: "The migrated regression test is observed RED against a reverted ListScopes payload selector (both shapes), proving detection power was preserved through the migration — not the pass depending on a client-side accident"
    requirement: "REQ-oversized-fixture-helper"
    verification:
      - kind: manual_procedural
        ref: "Live RED observation this session (recorded below); reverted via git apply -R, git status --porcelain confirmed clean"
        status: pass
    human_judgment: false
  - id: D6
    description: "storetest.ManySmallRecords equals internal/store's maxListLimit (exposed as store.MaxListLimit), so a many-small fixture always fits in one List page"
    requirement: "REQ-oversized-fixture-helper"
    verification:
      - kind: unit
        ref: "internal/store#TestManySmallShapeFitsOneListPage"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-09-19
status: complete
---

# Phase 1 Plan 4: internal/store's Harness Converges on storetest; the #583 Regression Migrates Onto Both Fixture Shapes Summary

**internal/store's `TestMain` now lives in an external `main_test.go` delegating to `storetest.Run`; the last duplicated harness (parser, terminator, image tag, `TestSharedQdrantAddressHonored`) is gone, and the migrated `TestListScopesFullPayloadsOverGRPCLimit` runs over both fixture shapes at the named 4 MiB limit — observed RED against a reverted payload selector before landing GREEN.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-09-19
- **Tasks:** 2 completed
- **Files:** 5 changed (3 created, 2 modified)

## Accomplishments

- `internal/store/main_test.go` (new, `package store_test`) hosts the single `TestMain` for this directory's test binary: `store.SetNoQdrantHandler(storetest.SkipOrFailNoQdrant)` then `os.Exit(storetest.Run(m))`. `TestSharedQdrantAddressHonored` moved here too (RESEARCH.md Pitfall 2 — it needs container-booted state only the `TestMain`-holding package can see) and now delegates to `storetest.AssertSharedAddressHonored(t)`.
- `internal/store/export_test.go` grew from Task 1's `SetNoQdrantHandler(f func(testing.TB))` — the D-07 cross-package channel handing storetest's skip-or-fail decision down to in-package tests — to also expose (Task 2) `NewTestStore`, `PrefixedTestCollection`, and `const MaxListLimit = maxListLimit`, so the external `store_test` package builds through the same prefix-enforcing seam and page-size constant every in-package test uses.
- `dialTestClient` (in-package) now reads `ENGRAM_QDRANT_TEST_ADDR` directly and calls the installed `noQdrantHandler` hook instead of its own inline `requireQdrant`/skip logic; the package's own `requireQdrant`, `TestMain`, `terminateQdrant`, `TestRequireQdrant`, in-package `TestSharedQdrantAddressHonored`, and the `qdrantImageTag`/`testQdrantAddr`/`testQdrantContainerBooted` declarations are all deleted — `qdrantTOCTOUVerifiedVersion` stays as the separate constant D-10 requires, with its doc comment and all four TOCTOU failure messages reworded to name `storetest.QdrantImage`.
- `internal/store/listscopes_oversized_test.go` (new, `package store_test`) migrates `TestListScopesFullPayloadsOverGRPCLimit` onto `storetest.Dial(t, storetest.RecvLimit)` + `storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})`, run once per shape (`few-large`, `many-small`) via `t.Run(shape.String(), ...)`; every seeded record is asserted counted by `ListScopes`. `TestManySmallShapeFitsOneListPage` pins `storetest.ManySmallRecords == store.MaxListLimit`.
- **Live RED observation** (required by Task 2, recorded here for plan 01-05 to package as a red-evidence patch): reverting `ListScopes`' `WithPayload` selector from `qdrant.NewWithPayloadInclude("scope")` to `qdrant.NewWithPayload(true)` and re-running the migrated test produced:
  ```
  --- FAIL: TestListScopesFullPayloadsOverGRPCLimit (2.79s)
      --- FAIL: TestListScopesFullPayloadsOverGRPCLimit/few-large (0.57s)
      --- FAIL: TestListScopesFullPayloadsOverGRPCLimit/many-small (2.22s)
  listscopes_oversized_test.go:52: ListScopes: Scroll() failed: store_oversized_listscopes_...: rpc error: code = ResourceExhausted desc = grpc: received message after decompression larger than max 4194304 (full-payload scroll exceeded the 4194304-byte named receive limit)
  ```
  Both subtests failed for both fixture shapes, proving detection power carried through the migration. `store.go` was restored via `git diff -- internal/store/store.go > patch && git apply -R patch`; `git status --porcelain -- internal/store/store.go` printed nothing before the GREEN commit.

## Task Commits

Each task was committed atomically:

1. **Task 1: internal/store's harness runs on storetest end to end: external TestMain, in-package handler hook, env-var address, single shared-address test** - `fe6d1eee` (test)
2. **Task 2: Migrate the #583 regression test onto storetest over both shapes, observed RED against the pre-fix ListScopes** - `3b458e18` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/store/main_test.go` - New external `TestMain`/`TestSharedQdrantAddressHonored`, package `store_test`.
- `internal/store/export_test.go` - `SetNoQdrantHandler` (Task 1); `NewTestStore`, `PrefixedTestCollection`, `MaxListLimit` (Task 2).
- `internal/store/listscopes_oversized_test.go` - The migrated `TestListScopesFullPayloadsOverGRPCLimit` (both shapes) and `TestManySmallShapeFitsOneListPage`.
- `internal/store/store_test.go` - `dialTestClient` reads the env var + handler hook; deleted the duplicated harness (parser, `TestMain`, terminator, `TestRequireQdrant`, in-package shared-address test, image-tag/address/booted declarations) and the in-package `TestListScopesFullPayloadsOverGRPCLimit`; reworded TOCTOU doc comments/messages.
- `internal/store/instrument_test.go` - One comment reworded (no longer names the deleted `testQdrantAddr` var).

## Decisions Made

- `noQdrantHandler` lives in `store_test.go` (package `store`, `_test.go`-only) rather than `export_test.go`, since it is package-private state, not an exported seam; only its setter (`SetNoQdrantHandler`) needed to be exported.
- `TestDialTestClientFailsWhenRequiredAndUnavailable`'s re-exec'd child carries a placeholder `ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:1` so its `TestMain` always takes `storetest.Run`'s fast path and never boots a container; the helper branch clears the var before dialing, so the placeholder is set but never actually dialed — matching the plan's explicit design.
- `listscopes_oversized_test.go`'s `Spec` literal is written on one line rather than gofmt's default multi-line field-aligned form, matching the plan's exact literal text and the acceptance criterion's single-space `Limit: storetest.RecvLimit` regex.
- `export_test.go`'s three Task 2 exports (`NewTestStore`, `PrefixedTestCollection`, `MaxListLimit`) were added on top of Task 1's `SetNoQdrantHandler` in the same file rather than a second file, since D-07's rationale (exposing internals to `package store_test`) is identical for all four.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] gofmt's field-aligned multi-line struct literal broke the plan's single-space acceptance-criterion regex**
- **Found during:** Task 2 acceptance-criteria verification
- **Issue:** Writing the `storetest.Spec{...}` literal across multiple lines let gofmt insert alignment padding (`Limit:  storetest.RecvLimit,` — two spaces), so `rg -o 'Limit: storetest[.]RecvLimit'` (one space) matched 0 times instead of the required 1.
- **Fix:** Rewrote the literal on one line, exactly as the plan's `<action>` specifies (`storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}}`); gofmt does not reformat a struct literal that already fits one line, so it stays stable.
- **Files modified:** internal/store/listscopes_oversized_test.go
- **Verification:** `rg -o 'Limit: storetest[.]RecvLimit' internal/store/listscopes_oversized_test.go` now prints `1`; `go build`/`go vet`/the full acceptance-criteria re-run all pass.
- **Committed in:** 3b458e18 (part of Task 2 commit — caught before committing)

---

**Total deviations:** 1 auto-fixed (1 bug). **Impact:** Cosmetic formatting fix only; no behavior change. No scope creep.

## Issues Encountered

**Acceptance-criterion grep count mismatch (informational, not a deviation):** Task 1's acceptance criterion `rg -n 'qdrant/qdrant[:]v' internal/store/*_test.go | wc -l` expects `0` but the repo has printed `1` since before this plan started — a pre-existing, out-of-scope prose comment in `internal/store/migrate_faultinject_test.go:10` ("The pinned server (qdrant/qdrant:v1.19.1) chunks a multi-ID payload write..."), unrelated to this plan's `files_modified` list and unrelated to the 4 duplicated `TestMain`/image-tag *declarations* D-10 targets. Verified via `git show HEAD:internal/store/migrate_faultinject_test.go` that the line predates this plan's first commit. Left unmodified per the scope-boundary rule (only auto-fix issues directly caused by the current task's changes); not tracked as a deviation since nothing this plan touched caused it.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four duplicated `TestMain`s, all three duplicated `ENGRAM_REQUIRE_QDRANT` parsers, and all four image-tag copies now collapse into `storetest` (D-10) — internal/store was the last package carrying its own copy.
- The RED observation above (both `--- FAIL:` lines, full `ResourceExhausted` message) is ready for plan 01-05 to package as a registered red-evidence patch in `redEvidenceDirs`, alongside the D-11 gate's own red-evidence patch.
- `internal/store` stays green except the single expected `TestRedEvidencePatchesAreLive` red, which plan 01-05's Task 3 closes.
- `go test -short ./internal/store/...` passes (23.3s); `task lint` passes repo-wide; `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0.
- No blockers or concerns for the next plan (01-05: red-evidence registration).

---
*Phase: 01-test-harness-fixture-helper*
*Completed: 2026-09-19*

## Self-Check: PASSED

All 5 created/modified files verified present on disk. Both task commits (`fe6d1eee`, `3b458e18`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing except the one pre-existing, out-of-scope grep hit documented above under "Issues Encountered"; `go vet ./internal/store/` and `golangci-lint run ./internal/store/...` both clean; `task license:check` clean; the full plan-level `<verification>` block (RED/GREEN suite, shared-address 3 PASS + 1 SKIP, `-short` mode, `task lint`, go.mod/go.sum drift) all pass.
