---
phase: 01-gate-ci-integrity
verified: 2026-08-13T18:30:00Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 01: Gate & CI Integrity Verification Report

**Phase Goal:** The build can actually go red for schema/migration work — key-link pattern gates
are provably matchable again, and the Qdrant testcontainer no longer masks real failures with
unrelated infra flakiness.
**Verified:** 2026-08-13
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A key-link `pattern:` field containing `\\` escaping compiles into an actually-matchable RegExp, and a guard test proves a reintroduced corrupted-pattern instance fails the build (fail-first, not silently passing) | ✓ VERIFIED | `internal/keylinks` package built and tested; `TestNoEscapedPatternsRepoWide` scans every `-PLAN.md` under `.planning` (repo-wide, archived milestones included). Live re-proof performed during this verification: reintroduced `pattern: "withConnectLane\\(ctx"` into an archived v0.12.x plan → `go test ./internal/keylinks/... -run TestNoEscapedPatternsRepoWide` FAILED naming the exact file:line, shape, raw pattern, and corrected form; reverted, `git diff --stat` confirmed zero residual diff, re-run PASSED. `rg -n 'pattern:.*[\\]' .planning --glob '*-PLAN.md'` finds nothing outside 01-02-PLAN.md's own prose (self-documented, not a real key-link). |
| 2 | Every v0.13.x Phase 1–2 key-link is re-resolved against the tool: each is either genuinely pinned or explicitly recorded as unpinned — a past "key-links passed" claim is never accepted as evidence on its own | ✓ VERIFIED | `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md` records all 30 v0.13.x Phase 1–2 key-links with exactly one verdict each (26 pinned, 3 pinned-via-target, 1 unpinned, 0 unreadable, 0 invalid). Re-ran `go test ./internal/keylinks/... -run TestReassessV013Phase12 -v` during this verification — output matches the committed table row-for-row, including the one `unpinned` entry (`02-interface-discoverability/02-04-PLAN.md:48`, `surfaces[.]ClassForTool`, routed through a wrapper) with its non-generic, git-log-backed reason. `TestReassessmentTableIsComplete` PASSED (mechanical completeness: verdict count == parsed-link count, every verdict in the closed set). Per D-14 (REQ is a record requirement, not a repair one) the unpinned gate was correctly left unrepaired — confirmed no diff to `internal/server/tools.go`/`toolannotations.go`/`toolclass.go`. |
| 3 | A full `go test ./...` run no longer fails from `internal/store`'s Qdrant testcontainer dying mid-run; when the container does die, its exit reason is captured in the failure output so a recurrence is diagnosable from evidence | ✓ VERIFIED | `.github/workflows/ci.yaml`'s `test` job declares one `services: qdrant:` container (health-gated via a bash `/dev/tcp` probe — curl-less, confirmed against the actual image), asserts exactly one running container, and asserts per-package shared-address participation (3 PASS + 1 SKIP, `internal/retrievaleval` correctly excluded — its `TestMain` never dials Qdrant in CI). An `if: failure()` step captures container state/exit-code/logs/OOM evidence. Confirmed against a real, successful GitHub Actions run (31718449162, job 94509047766, head `73ea27c9`, matching current HEAD) — read as log text, not badge: `running Qdrant containers...: 1`, `shared-address PASS=3 SKIP=1`, all 16 packages `ok`, gofmt step clean. Locally reproduced the full `go test ./... -count=1` against one shared Qdrant during this verification: all packages passed, exit 0. |

**Score:** 3/3 truths verified (0 present, behavior-unverified)

### Plan-Level Must-Haves (all 6 plans)

| Plan | Must-have truths | Status |
|------|-------------------|--------|
| 01-01 | `internal/keylinks` core: escaping/named-group/compile-error/unsatisfiable detection, fixture pair, `ScanPlans` | ✓ VERIFIED — `go test ./internal/keylinks/... -v` all PASS, no SKIP |
| 01-02 | Repo-wide normalization (39 offenders, 20 files) + two recurring gates with D-04 asymmetric scope | ✓ VERIFIED — `TestNoEscapedPatternsRepoWide`, `TestActiveMilestoneKeyLinksSatisfiable`, `TestGateScopesAreDistinct` all PASS; zero backslash patterns remain repo-wide |
| 01-03 | One-time v0.13.x Phase 1–2 reassessment, record not repair | ✓ VERIFIED — 30/30 verdicts recorded, completeness test PASS, D-01 checkpoint resolved (spine-track) |
| 01-04 | Shared CI Qdrant, collision resolved, per-package address proof, one-container assertion, diagnostics | ✓ VERIFIED — CI wiring confirmed both locally and against real run 31718449162 |
| 01-05 | Per-package collection-name namespacing enforced by runtime seam (`newTestStore`) | ✓ VERIFIED — all four packages' seams present; `internal/store` fully swept (spine_test.go, reindex_test.go); idempotency proof (two consecutive runs) reproduced locally |
| 01-06 | Source-level AST conformance gate + prefix-disjointness proof + real-CI-run checkpoint | ✓ VERIFIED — `TestEveryStoreConstructionRoutesThroughSeam` and `TestCollectionPrefixesAreDisjoint` both PASS; checkpoint resolved against run 31718449162 |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/keylinks/keylinks.go` | escaping/subset/satisfiability checker + `ScanPlans` | ✓ VERIFIED | Present, stdlib-only (`go list -deps` confirms), all functions from PLAN frontmatter present |
| `internal/keylinks/keylinks_test.go` | fixture-pair fail-first proof | ✓ VERIFIED | `TestFixturePairEscaping`, `TestFixturePairSubsetAndSatisfiability` (11 subtests), all PASS |
| `internal/keylinks/testdata/{good,bad}_key_links.md` | committed fixture pair | ✓ VERIFIED | Both present, drive both green and red directions |
| `internal/keylinks/gate_test.go` | two recurring repo gates | ✓ VERIFIED | `TestNoEscapedPatternsRepoWide`, `TestActiveMilestoneKeyLinksSatisfiable`, `TestGateScopesAreDistinct` all present and PASS |
| `internal/keylinks/sweep_test.go` | one-time reassessment | ✓ VERIFIED | `TestReassessV013Phase12`, `TestReassessmentTableIsComplete` present and PASS |
| `.planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md` | verdict table | ✓ VERIFIED | 30 rows, all 5 verdict classes represented in rollup, D-14/D-01 statements present |
| `.github/workflows/ci.yaml` | shared Qdrant service + gates | ✓ VERIFIED | `services.qdrant`, `ENGRAM_QDRANT_TEST_ADDR`, one-container assertion, shared-participation assertion, `if: failure()` diagnostics all present |
| `internal/store/collectionprefix_conformance_test.go` | AST-level bypass gate + disjointness | ✓ VERIFIED | `TestEveryStoreConstructionRoutesThroughSeam`, `TestCollectionPrefixesAreDisjoint`, committed fixture pair all present and PASS |

### Key Link Verification

All plan-declared key_links (`ScanPlans(...)` calls, `ModeSatisfiability` usage, `testCollection(...)`
routing, `newTestStore(...)` seam usage) verified present and wired by direct source reads and test
execution above — no separate table needed beyond the plan-level rows; every plan's `<verify>` block
was independently re-run during this verification (not merely re-read from the SUMMARY).

### Behavioral Spot-Checks / Fail-First Re-Proofs (performed independently during this verification)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Escaping gate goes RED on a reintroduced defect | reintroduce `\\(` into an archived v0.12.x plan, run `TestNoEscapedPatternsRepoWide` | FAILED naming file:line:shape:fix; reverted cleanly | ✓ PASS |
| Reassessment sweep reproduces the committed table | `go test ./internal/keylinks/... -run TestReassessV013Phase12 -v` | Output matches `01-KEYLINK-REASSESSMENT.md` row-for-row (30 links, same rollup) | ✓ PASS |
| Collection-prefix conformance gates | `go test ./internal/store/... -run 'TestEveryStoreConstructionRoutesThroughSeam|TestCollectionPrefixesAreDisjoint' -v` | Both PASS, real-package scan visits 64 files, 12 ordered pairs compared | ✓ PASS |
| Full suite against one shared Qdrant | `ENGRAM_QDRANT_TEST_ADDR=... ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1` | All 16 testable packages `ok`, exit 0 | ✓ PASS |
| `task lint` / `task license:check` | both | Clean (0 issues; 300 valid/0 invalid, 1088 ignored) | ✓ PASS |
| Real CI run mechanism claims | `gh run view 31718449162 --log --job 94509047766` | Container count=1, shared-address PASS=3/SKIP=1, all packages `ok`, gofmt clean | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|-------------|-----------------|--------|----------|
| REQ-keylink-pattern-matchable | 01-01, 01-02 | ✓ SATISFIED | `internal/keylinks` package + repo-wide normalization + recurring gate, re-proven red/green live |
| REQ-keylink-past-gates-reassessed | 01-03 | ✓ SATISFIED | 30/30 v0.13.x Phase 1–2 key-links carry exactly one recorded verdict; record, not repair (D-14), honored |
| REQ-ci-qdrant-container-stability | 01-04, 01-05, 01-06 | ✓ SATISFIED | Shared Qdrant, namespacing, conformance gate, and a real green CI run all confirmed |

No orphaned requirements: `.planning/REQUIREMENTS.md`'s Phase 1 row maps exactly these three IDs,
and all three appear in at least one plan's `requirements:` frontmatter field.

(Note: `.planning/REQUIREMENTS.md`'s checkbox column still shows `[ ]`/"Pending" for the first two
IDs and `[x]`/"Complete" for the third — this file is updated by the orchestrator at phase close,
not by plan execution, and is expected to be stale until then. Not treated as a gap.)

### Anti-Patterns Found

None. Scanned every file touched by this phase's six plans (`internal/keylinks/*.go`,
`internal/store/collectionprefix_conformance_test.go`, `.github/workflows/ci.yaml`,
`internal/store/{store,spine,reindex}_test.go`, `internal/server/tools_test.go`,
`internal/e2e/{harness,spine_review}_test.go`, `internal/retrievaleval/retrieval_eval_test.go`) for
`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers and empty-implementation stubs — zero hits.

### Known Deviations (verified accurate, not re-flagged as new findings)

All six deviations named in the verification brief were independently confirmed present and
accurately recorded in their respective SUMMARY.md / deferred-items.md:

1. `.licenserc.yaml` gained a `paths-ignore` entry for `internal/keylinks/testdata/*.md` (01-01, `.licenserc.yaml:74-80`) — confirmed present, matches the SUMMARY's Rule-3 auto-fix account.
2. `.rumdl.toml` gained an `internal/keylinks/testdata` exclude (`.rumdl.toml:30`) — confirmed present; `deferred-items.md` records the orchestrator-level resolution at commit `51d0269e`.
3. `internal/e2e/spine_review_test.go`'s `newSpineReviewStore` routed through the prefix seam by 01-05 outside its declared `files_modified` — confirmed in 01-05-SUMMARY.md's Decisions/Deviations sections, and the file appears in git history for plan 01-05's commits.
4. The `go/ast` conformance scan excludes nil-client constructions (12 pre-existing hermetic sites) — confirmed in 01-06-SUMMARY.md's key-decisions, and the exclusion rationale (`isNilIdent`) is documented in-file per the SUMMARY's own account.
5. CI's shared-Qdrant participation is 3 packages, not 4 (`internal/retrievaleval` correctly SKIPs) — confirmed both in the CI step's own comment block and in the live CI log read during this verification (`shared-address PASS=3 SKIP=1`).
6. Plan 01-04's original `curl`-based health-cmd was replaced by a bash `/dev/tcp` probe in commit `b4b5b578` — confirmed present in `.github/workflows/ci.yaml`'s current `options:` block and its surrounding comment, and the commit exists in `git log`.

### Additional Observation (not a gap)

`TestSharedQdrantAddressHonored`'s address-equality half (`testQdrantAddr != addr`) is structurally
weak: `TestMain` assigns `testQdrantAddr = os.Getenv("ENGRAM_QDRANT_TEST_ADDR")` directly in the
shared-address branch, so the equality check compares a value against itself and can only fail if
some other test mutates the package-level `testQdrantAddr` variable and fails to restore it before
this test runs. The load-bearing half — `testQdrantContainerBooted == false` — is genuinely
discriminating (it is what the fail-first red-proof in 01-04-SUMMARY.md actually exercises) and is
correctly identified as load-bearing in the code's own doc comment ("Address equality alone is not
enough ... so the load-bearing assertion is testQdrantContainerBooted == false"). This is a disclosed
design choice, not a hidden vacuous gate, and does not affect the phase's pass/fail determination.

## Gaps Summary

None. All three ROADMAP success criteria are independently re-verified against the live codebase
(not merely re-read from SUMMARY.md claims), including live fail-first re-proofs of both key-link
gates, a live re-run of the v0.13.x reassessment sweep matching the committed record row-for-row, a
live re-run of the collection-prefix conformance gates, a full local `go test ./...` run against one
shared Qdrant, and independent confirmation of the real GitHub Actions run's mechanism claims by
reading the job log text directly via `gh run view --log`.

---

*Verified: 2026-08-13*
*Verifier: Claude (gsd-verifier)*
