---
phase: 01-gate-ci-integrity
plan: 06
subsystem: testing
tags: [go-testing, go-ast, qdrant, ci, conformance-gate]

# Dependency graph
requires:
  - phase: 01-gate-ci-integrity
    provides: "01-04's shared-Qdrant CI seam and 01-05's newTestStore prefix-enforcing runtime seam in all four Qdrant-backed test packages"
provides:
  - "TestEveryStoreConstructionRoutesThroughSeam — a stdlib-only go/ast source-level scan proving no live Store construction across the four Qdrant-backed packages' test sources bypasses that package's newTestStore seam by passing a raw collection-name literal, proven both directions by a committed fixture pair"
  - "TestCollectionPrefixesAreDisjoint — reads all four packages' testCollectionPrefix constants out of their own test sources and asserts pairwise disjointness (equality AND leading-substring), failing (not skipping) when a package declares no prefix"
  - "The four resolved, pairwise-disjoint collection-namespace prefixes as a checkable fact: store_, server_, e2e_, retrievaleval_"
affects: [internal/store, ci]

# Actuals (#2632)
actuals:
  tokens: 4890
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "go/ast source-level conformance scan (parser.ParseFile over bytes, not go/types) as the complement to a runtime-enforcing seam: the AST scan forces every construction through the seam in the first place; the runtime seam (already landed in 01-05) checks the value the seam receives. Neither alone is the whole story."
    - "Scope a source-level bypass gate to constructions that can actually reach the shared resource: a client argument that is the literal nil identifier is excluded from the scan, since a nil-client Store construction never dials Qdrant and therefore can never collide on the shared CI instance — this is what let the real-package subtest scan 64 test files and land at zero findings without touching any of the pre-existing hermetic (nil-client) unit tests."
    - "Reading a sibling package's unexported test-only const via go/parser (never import) to let one package's test see four packages' otherwise-unreachable declarations without creating an import edge or cycle."

key-files:
  created:
    - internal/store/collectionprefix_conformance_test.go
    - internal/store/testdata/collectionprefix/good_pkg_test.go.txt
    - internal/store/testdata/collectionprefix/bad_pkg_test.go.txt
  modified: []

key-decisions:
  - "The conformance scan excludes constructions whose client argument is the literal `nil` identifier. Every pre-existing raw-collection-literal construction across all four Qdrant-backed packages' test sources (internal/store's decisionlog_test.go, the WithClock/WithAuthz pair and two production-defaults calls in store_test.go, bench_test.go, rerank_test.go, reindex_test.go's TestReindexRejectsInvalidArgs, and the cross-package store_test package's spine_forgery_test.go — 12 call sites total, none touched by 01-04/01-05's sweeps) is a hermetic unit test that never dials Qdrant (several with comments saying so directly, e.g. rerank_test.go: \"a nil client is safe\"). Without this exclusion the real-package subtest would fail immediately on 12 pre-existing, out-of-scope constructions this plan's files_modified list does not name. Documented in-file as the load-bearing design rationale for isNilIdent."
  - "The unqualified-New match (internal/store's own bare `New(...)` calls) is gated behind an explicit allowUnqualified flag, true only for internal/store itself, rather than matching any bare `New(` call in any scanned package. Go permits at most one top-level func New per package, so this is precise for internal/store; grep confirmed no other Qdrant-backed package defines or calls an unqualified New, so the flag adds defense-in-depth against a hypothetical future collision rather than fixing an observed one."
  - "Both tasks' scanning helpers (qdrantBackedPackages, scanConstructions/scanPackageDir, extractTestCollectionPrefix) live in one file and Task 2 reuses Task 1's infrastructure directly, per the plan's own read_first guidance — no parallel copy of the AST-walking logic."

patterns-established:
  - "Fail-first proof recorded verbatim for BOTH gates in both directions (4 total red-proofs: bad-fixture-neutralized, real-construction-injected, leading-substring-collision, missing-constant), each performed as a temporary edit + test run + verbatim capture + immediate revert + git diff --stat confirmation of zero residual diff."

requirements-completed: []

coverage:
  - id: D1
    description: "TestEveryStoreConstructionRoutesThroughSeam: source-level gate proving every live store construction across the four Qdrant-backed packages routes through the newTestStore seam, proven both directions by a committed fixture pair plus a live red-proof against a real construction site"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "go test ./internal/store/... -run TestEveryStoreConstructionRoutesThroughSeam -v — PASS (good fixture, bad fixture, zero-applicability guard, real packages subtests)"
        status: pass
      - kind: unit
        ref: "go vet ./internal/store/... && gofmt -l . && task lint — all clean"
        status: pass
    human_judgment: false
  - id: D2
    description: "TestCollectionPrefixesAreDisjoint: the four packages' collection-name prefixes (store_, server_, e2e_, retrievaleval_) are pairwise disjoint (equality and leading-substring), proven fail-first in both red directions"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "go test ./internal/store/... -run TestCollectionPrefixesAreDisjoint -v — PASS, 12 ordered pairs compared across 4 packages"
        status: pass
      - kind: integration
        ref: "ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1 — all packages ok, shared address"
        status: pass
    human_judgment: false
  - id: D3
    description: "A real GitHub Actions test job log confirms the one-container assertion count, the resolved shared address, and per-package TestSharedQdrantAddressHonored results with no SKIP in the store/server integration suites"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: manual
        ref: "GitHub Actions run 31718449162, test job 94509047766, head 73ea27c9 — claim 1: 'running Qdrant containers (ancestor qdrant/qdrant:v1.18.2): 1'; claim 2: 'ENGRAM_QDRANT_TEST_ADDR resolved to: localhost:6334' (matches the 6334:6334 service mapping); claim 3: 'shared-address PASS=3 SKIP=1 (expected PASS=3 SKIP=1)' with three verbatim '--- PASS: TestSharedQdrantAddressHonored' lines. Zero SKIP lines in the main 'go test ./...' step. gofmt step emitted no 'gofmt needed on:' line."
        status: pass
    human_judgment: true
    rationale: |
      Resolved by the orchestrator against a real run, read as log TEXT rather than as a badge, per D-20.

      Claim 3 was RESTATED, not merely satisfied. As written it demanded four PASS lines; that was
      unobtainable for two independent reasons, both discovered by attempting it:
      (1) CI runs `go test ./...` without -v, so per-test lines are never emitted at all — and a
      package whose every test SKIPs still prints `ok`, so package-level `ok` cannot distinguish a
      running suite from a skipped one (threat T-01-23 exactly);
      (2) `internal/retrievaleval`'s TestMain returns before assigning testQdrantAddr unless
      ENGRAM_RETRIEVAL_EVAL=1, so in CI that package never dials Qdrant and does not participate in
      the shared instance at all. Four PASS was therefore never achievable without running the full
      retrieval eval in CI.

      The honest arithmetic is THREE packages sharing one Qdrant (store, server, e2e), not four.
      A dedicated CI step now asserts PASS=3 and SKIP=1 as pinned counts, so a package dropping out
      (3->2) or retrievaleval quietly joining (1->0) each trip the gate.

      Also fixed en route: the services: health-cmd used `curl`, which qdrant/qdrant:v1.18.2 does not
      ship (verified by running `command -v` inside the image; wget/nc/python3 also absent, bash
      present). The first real run died at "Initialize containers" with the container itself healthy
      (status=running exitCode=0 oomKilled=false) — a failure mode that mimics the #497 cascade this
      phase exists to fix. Replaced with a bash /dev/tcp probe against 6334, proven exit 0 against a
      serving qdrant and exit 1 against both a dead port and this same image run with
      `--entrypoint sleep`. This is precisely the defect D-20 predicted local green would miss.

duration: ~15min (Tasks 1-2) + orchestrator-resolved Task 3 checkpoint
completed: 2026-08-13
status: complete
---

# Phase 01 Plan 06: Collection-Prefix Conformance Gate Summary

**A stdlib go/ast source-level scan (`TestEveryStoreConstructionRoutesThroughSeam`) proves no live Store construction across the four Qdrant-backed packages bypasses its package's `newTestStore` runtime seam, and a companion test (`TestCollectionPrefixesAreDisjoint`) proves the four packages' `store_`/`server_`/`e2e_`/`retrievaleval_` prefixes are pairwise disjoint — both gates proven fail-first in every direction the plan's acceptance criteria name. Task 3, the real-CI-run checkpoint, is RESOLVED against GitHub Actions run 31718449162 — and resolving it exposed two defects no local gate could see: a `curl`-based service health-cmd against an image that ships no curl, and a claim 3 whose evidence the CI job did not emit at all.**

## Performance

- **Duration:** ~15 min (Tasks 1-2; execution stopped at Task 3's blocking checkpoint)
- **Tasks:** 3 of 3 completed (Task 3's `checkpoint:human-verify gate="blocking"` was resolved by the orchestrator against a real CI run after Sean authorized the push + draft PR)
- **Files modified:** 3 (all newly created)

## Accomplishments

- `internal/store/collectionprefix_conformance_test.go` (Task 1): `TestEveryStoreConstructionRoutesThroughSeam` parses every `_test.go` file in the four Qdrant-backed packages via `go/parser`/`go/ast` and reports every call to the store constructor whose collection argument is a string literal (and whose client argument is not `nil`) as a `t.Error`, collecting findings into a slice rather than failing on the first. A zero-applicability guard subtest asserts a nonexistent package directory fails loudly rather than reporting clean.
- Committed fixture pair `internal/store/testdata/collectionprefix/{good,bad}_pkg_test.go.txt` (`.go.txt` so neither `gofmt`, `go vet`, nor the linter sweeps them, while `go/parser` still reads them): the good fixture's only construction routes through a local seam with an identifier collection argument (zero findings); the bad fixture constructs directly over the raw literal `"hardcoded-bad-pkg-collection"` (at least one finding naming it).
- `TestCollectionPrefixesAreDisjoint` (Task 2, same file): extracts each package's `testCollectionPrefix` constant from its own test source (no import — server/e2e/retrievaleval already import store, so store importing back would cycle) and asserts, over every one of 12 ordered pairs across the four packages, that no two prefixes are equal and neither is a leading substring of the other. A package whose constant cannot be found is a `t.Errorf`, and the test additionally `t.Fatalf`'s if fewer than four prefixes resolve — a missing constant cannot silently degrade into "three comparisons and pass."
- Four fail-first red-proofs performed live and reverted (see Verbatim Evidence): the bad fixture's offending line removed, a raw literal injected into a real `internal/server/tools_test.go` construction, `internal/e2e`'s prefix temporarily collapsed to a leading substring of two other packages' prefixes, and `internal/retrievaleval`'s constant temporarily renamed away.
- Full-suite rehearsal (`ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1`) passes end to end against a real shared Qdrant instance, after each red-proof was reverted and confirmed clean via `git diff --stat`.

## Task Commits

1. **Task 1: End-to-end "a raw collection literal in a test file fails the build" — fixture proven, then applied (tracer)** - `8350296f` (test)
2. **Task 2: Assert the four prefixes are pairwise disjoint** - `18bc7cc1` (test)

**Task 3: Confirm the three mechanism claims in a real GitHub Actions run** — NOT executed. This is a `checkpoint:human-verify gate="blocking"` task requiring the human to push the branch, open/update the PR, and read a real GitHub Actions run's `test` job log. This executor's instructions explicitly prohibit pushing, opening a PR, or otherwise triggering CI (an outward-facing action reserved for the human), and this branch has never been pushed — no such run exists to read.

## Files Created/Modified

- `internal/store/collectionprefix_conformance_test.go` - `TestEveryStoreConstructionRoutesThroughSeam` + `TestCollectionPrefixesAreDisjoint`, plus shared AST-scanning helpers (`qdrantBackedPackages`, `scanConstructions`, `scanPackageDir`, `extractTestCollectionPrefix`)
- `internal/store/testdata/collectionprefix/good_pkg_test.go.txt` - known-good fixture: sole construction routes through a local seam via an identifier
- `internal/store/testdata/collectionprefix/bad_pkg_test.go.txt` - known-bad fixture: direct construction over a raw literal, non-nil client

## Decisions Made

- **The scan excludes nil-client constructions.** Every pre-existing raw-collection-literal construction across the four packages' test sources (12 call sites: `internal/store`'s `decisionlog_test.go` ×4, `store_test.go`'s `WithClock`/`WithAuthz` pair ×4, `bench_test.go`, `rerank_test.go`, `reindex_test.go`'s `TestReindexRejectsInvalidArgs`, and the cross-package `store_test` package's `spine_forgery_test.go`) passes `nil` as the client — these are hermetic unit tests that never dial Qdrant (several say so directly in their own comments, e.g. `rerank_test.go`: "a nil client is safe"). A nil-client `Store` can never collide with another package's collection name on the shared CI instance, so enforcing the seam on it would force out-of-scope edits to tests this plan's `files_modified` list does not name, for zero collision-safety gain. Without this exclusion, the "real packages" subtest would have failed immediately on 12 findings that are not a genuine CI-collision risk. This is a necessary interpretive decision filling a gap the plan's own text did not resolve explicitly, made to satisfy the plan's own acceptance criterion that "the four real packages ... yield zero findings" without touching files outside this plan's declared scope.
- **The unqualified-`New` match is gated behind `allowUnqualified`, true only for `internal/store`.** Go permits one top-level `New` per package; grep confirmed no other Qdrant-backed package defines or calls a bare `New(`, so this is defense-in-depth against a hypothetical future collision (e.g. another package later defining its own unqualified `New`) rather than a fix for an observed false positive.
- **Split the initial single-`Write` draft into two atomic commits.** The file was first drafted whole (both tasks' tests) in one pass; before committing, it was split so Task 1's commit contains only `TestEveryStoreConstructionRoutesThroughSeam` + fixtures, and Task 2's commit adds `TestCollectionPrefixesAreDisjoint` on top — matching the plan's per-task commit requirement. Used `git reset --soft HEAD~1` (not `--hard`) to undo the merged first commit without losing any working-tree content, per this repo's destructive-git prohibitions.

## Deviations from Plan

None beyond the design decisions documented above, which fill genuine gaps the plan's own text left open (Rule 1/interpretive-necessity class — no rule strictly categorizes "which construction sites the gate should ignore," but the alternative was either touching a dozen out-of-scope files or leaving the real-package acceptance criterion unsatisfiable as literally written).

## Verbatim Evidence

**Task 1 — good/bad fixture pair, green (`go test ./internal/store/... -run TestEveryStoreConstructionRoutesThroughSeam -v`):**
```
=== RUN   TestEveryStoreConstructionRoutesThroughSeam
=== RUN   TestEveryStoreConstructionRoutesThroughSeam/good_fixture_yields_zero_findings
=== RUN   TestEveryStoreConstructionRoutesThroughSeam/bad_fixture_yields_at_least_one_finding_naming_the_offending_literal
    collectionprefix_conformance_test.go:233: bad fixture finding: bad_pkg_test.go.txt:26: raw collection literal "hardcoded-bad-pkg-collection" bypasses this package's newTestStore seam — route it through testCollection()
=== RUN   TestEveryStoreConstructionRoutesThroughSeam/zero-applicability_guard:_nonexistent_package_directory_fails_loudly
=== RUN   TestEveryStoreConstructionRoutesThroughSeam/real_packages
    collectionprefix_conformance_test.go:265: scanned 64 _test.go files across 4 packages
--- PASS: TestEveryStoreConstructionRoutesThroughSeam (0.02s)
    --- PASS: .../good_fixture_yields_zero_findings (0.00s)
    --- PASS: .../bad_fixture_yields_at_least_one_finding_naming_the_offending_literal (0.00s)
    --- PASS: .../zero-applicability_guard:_nonexistent_package_directory_fails_loudly (0.00s)
    --- PASS: .../real_packages (0.02s)
PASS
```

**Task 1 red-proof #1 — bad fixture's literal replaced by an identifier (`name := "..."; store.New(c, name)`), FAIL:**
```
=== RUN   TestEveryStoreConstructionRoutesThroughSeam/bad_fixture_yields_at_least_one_finding_naming_the_offending_literal
    collectionprefix_conformance_test.go:232: bad fixture: want at least one finding naming the offending literal, got none
--- FAIL: TestEveryStoreConstructionRoutesThroughSeam (0.00s)
    --- FAIL: .../bad_fixture_yields_at_least_one_finding_naming_the_offending_literal (0.00s)
FAIL
```
(reverted immediately; `git diff --stat` on the fixture confirmed clean afterward)

**Task 1 red-proof #2 — raw literal injected into a real construction site (`internal/server/tools_test.go`), FAIL naming file+line+literal:**
```
=== RUN   TestEveryStoreConstructionRoutesThroughSeam/real_packages
    collectionprefix_conformance_test.go:265: scanned 64 _test.go files across 4 packages
    collectionprefix_conformance_test.go:267: ../server/tools_test.go:381: raw collection literal "raw-literal-probe-bypass" bypasses this package's newTestStore seam — route it through testCollection()
--- FAIL: TestEveryStoreConstructionRoutesThroughSeam (0.03s)
    --- FAIL: .../real_packages (0.03s)
FAIL
```
(reverted immediately; `git diff --stat internal/server/tools_test.go` confirmed clean afterward)

**Task 2 — green, all four prefixes resolved (`go test ./internal/store/... -run TestCollectionPrefixesAreDisjoint -v`):**
```
=== RUN   TestCollectionPrefixesAreDisjoint
    collectionprefix_conformance_test.go:364: resolved prefix: internal/e2e = "e2e_"
    collectionprefix_conformance_test.go:364: resolved prefix: internal/retrievaleval = "retrievaleval_"
    collectionprefix_conformance_test.go:364: resolved prefix: internal/server = "server_"
    collectionprefix_conformance_test.go:364: resolved prefix: internal/store = "store_"
    collectionprefix_conformance_test.go:400: compared 12 ordered pairs across 4 packages
--- PASS: TestCollectionPrefixesAreDisjoint (0.02s)
PASS
```

**Task 2 red-proof #1 — leading-substring collision (`internal/e2e`'s prefix temporarily set to `"s"`, a leading substring of both `server_` and `store_`), FAIL naming both packages and both values in each colliding pair:**
```
=== RUN   TestCollectionPrefixesAreDisjoint
    collectionprefix_conformance_test.go:364: resolved prefix: internal/e2e = "s"
    collectionprefix_conformance_test.go:364: resolved prefix: internal/retrievaleval = "retrievaleval_"
    collectionprefix_conformance_test.go:364: resolved prefix: internal/server = "server_"
    collectionprefix_conformance_test.go:364: resolved prefix: internal/store = "store_"
    collectionprefix_conformance_test.go:395: internal/e2e's prefix "s" is a leading substring of internal/server's prefix "server_": a collection name in internal/server beginning with "erver_" could collide with one in internal/e2e
    collectionprefix_conformance_test.go:395: internal/e2e's prefix "s" is a leading substring of internal/store's prefix "store_": a collection name in internal/store beginning with "tore_" could collide with one in internal/e2e
    collectionprefix_conformance_test.go:400: compared 12 ordered pairs across 4 packages
--- FAIL: TestCollectionPrefixesAreDisjoint (0.03s)
FAIL
```
(reverted immediately; `git diff --stat internal/e2e/harness_test.go` confirmed clean afterward)

**Task 2 red-proof #2 — missing constant (`internal/retrievaleval`'s `testCollectionPrefix` temporarily renamed to `testCollectionPrefixRenamed`), FAIL naming the package, NOT a silent 3-comparison pass:**
```
=== RUN   TestCollectionPrefixesAreDisjoint
    collectionprefix_conformance_test.go:353: internal/retrievaleval (../retrievaleval): no testCollectionPrefix constant found
    collectionprefix_conformance_test.go:359: resolved 3 of 4 package prefixes; see errors above
--- FAIL: TestCollectionPrefixesAreDisjoint (0.02s)
FAIL
```
(reverted immediately; `git diff --stat internal/retrievaleval/retrieval_eval_test.go` confirmed clean afterward)

**Full-suite rehearsal against a real, shared, freshly-started Qdrant container:**
```
$ docker run -d -p 6333:6333 -p 6334:6334 qdrant/qdrant:v1.18.2
$ curl -s http://localhost:6333/readyz
all shards are ready
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1
ok  	github.com/seanb4t/engram/cmd/engram	2.231s
ok  	github.com/seanb4t/engram/internal/auth	0.451s
ok  	github.com/seanb4t/engram/internal/authz	0.079s
ok  	github.com/seanb4t/engram/internal/config	0.088s
ok  	github.com/seanb4t/engram/internal/e2e	4.374s
ok  	github.com/seanb4t/engram/internal/embed	0.179s
ok  	github.com/seanb4t/engram/internal/keylinks	0.142s
ok  	github.com/seanb4t/engram/internal/openaiurl	0.099s
ok  	github.com/seanb4t/engram/internal/retrievaleval	0.344s
ok  	github.com/seanb4t/engram/internal/server	15.274s
ok  	github.com/seanb4t/engram/internal/shortid	0.070s
ok  	github.com/seanb4t/engram/internal/store	25.898s
ok  	github.com/seanb4t/engram/internal/summarize	0.131s
ok  	github.com/seanb4t/engram/internal/surfaces	0.633s
ok  	github.com/seanb4t/engram/internal/telemetry	0.284s
ok  	github.com/seanb4t/engram/internal/webauth	0.802s
```
(container stopped and removed afterward)

**Overall verification block:**
- `gofmt -l .` — prints nothing (the `.go.txt` fixtures are not swept)
- `go vet ./...` — clean
- `task lint` — 0 issues (one stray golangci-lint cache warning referencing an unrelated sibling worktree's now-deleted path, not this plan's changes)
- `task license:check` — 299 valid, 0 invalid
- `git diff --stat go.mod go.sum` — empty (no dependency changes; this plan is stdlib-only)

## Issues Encountered

None beyond the initial single-`Write` draft needing a `git reset --soft` split into two atomic task commits (documented under Decisions Made).

## User Setup Required

**Task 3's checkpoint requires human action this executor is prohibited from taking.** To close this plan:

1. Push the current branch and open/update its pull request (`main` is protected — never push directly to it).
2. Find the run: `gh run list --branch $(git branch --show-current) --workflow ci --limit 5 --json databaseId,status,conclusion,headSha`.
3. Read the `test` job's log: `gh run view <databaseId> --log --job <test job id>`.
4. Confirm, by reading the log (not the pass/fail badge):
   - The one-container assertion step ran and reported a count of exactly 1.
   - That step's echoed shared address matches the `services:` port mapping.
   - Four `--- PASS: TestSharedQdrantAddressHonored` lines appear (one per Qdrant-backed package), no `--- SKIP` for that test.
5. Search the log for `--- SKIP` in the store and server packages — a skipped integration suite means the fail-closed gate was bypassed.
6. Confirm the separate `gofmt` CI step passed (`task`'s default target does not run it).
7. If the job failed, sort failures by timestamp, read the earliest one first, and read the `if: failure()` diagnostics output.

Full detail in Task 3 of `.planning/phases/01-gate-ci-integrity/01-06-PLAN.md`.

## Next Phase Readiness

- All three of D-20's checkable claims now have a mechanical gate: one container + shared address (plan 01-04), disjoint prefixes (this plan). The fourth — confirming the mechanism in a real CI run — is the one remaining open item for the whole phase.
- Both conformance gates (this plan's AST scan, plan 01-05's runtime seam) fail loudly rather than silently when their input disappears, proven by dedicated subtests/red-proofs rather than manual trial alone.
- **Task 3's checkpoint is RESOLVED** (run 31718449162, job 94509047766, head 73ea27c9). Sean authorized the push and a draft PR (#498); the orchestrator then read the three claims out of the job log as text. Two defects surfaced and were fixed in the process — the curl-less health-cmd (`b4b5b578`) and claim 3's unobtainable evidence (`73ea27c9`) — and claim 3 was restated from "four PASS" to the accurate "3 PASS + 1 SKIP", since `internal/retrievaleval` does not participate in the shared instance in CI at all. `status: complete`; no downstream plan is blocked.

---
*Phase: 01-gate-ci-integrity*
*Completed: 2026-08-13 (partial — Tasks 1-2 only; Task 3 blocked at checkpoint)*

## Self-Check: PASSED

All created files confirmed present on disk (`internal/store/collectionprefix_conformance_test.go`, `internal/store/testdata/collectionprefix/{good,bad}_pkg_test.go.txt`, this SUMMARY). Both task commits (`8350296f`, `18bc7cc1`) confirmed present in `git log --oneline --all`.
