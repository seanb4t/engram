---
phase: 1
slug: gate-ci-integrity
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-13
validated: 2026-08-16
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) |
| **Config file** | none — `Taskfile.yaml` wraps lint + test as `task` |
| **Quick run command** | `go test ./internal/keylinks/... -count=1` |
| **Full suite command** | `task` (lint + `go test ./...`) |
| **Estimated runtime** | ~2s quick (keylinks 0.16s + store conformance 1.29s); full suite requires a Qdrant container |

Qdrant-backed packages (`internal/store`, `internal/server`, `internal/e2e`,
`internal/retrievaleval`) are driven against one shared instance via
`ENGRAM_QDRANT_TEST_ADDR`, with `ENGRAM_REQUIRE_QDRANT=1` forcing a hard failure
instead of a silent skip. This is itself a phase-1 deliverable, not pre-existing.

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/keylinks/... -count=1`
- **After every plan wave:** Run `task`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 2 seconds (quick), ~5 minutes (full, container boot included)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | REQ-keylink-pattern-matchable | — | N/A | unit | `go test ./internal/keylinks/... -run TestFixturePairEscaping -v` | ✅ | ✅ green |
| 1-01-02 | 01 | 1 | REQ-keylink-pattern-matchable | — | N/A | unit | `go test ./internal/keylinks/... -run 'TestFixturePair' -v` | ✅ | ✅ green |
| 1-01-03 | 01 | 1 | REQ-keylink-pattern-matchable | — | N/A | unit | `go test ./internal/keylinks/... -v` | ✅ | ✅ green |
| 1-02-01 | 02 | 2 | REQ-keylink-pattern-matchable | — | N/A | integration | `go test ./internal/keylinks/... -v && ! rg -q 'pattern:.*[\\]' .planning --glob '*-PLAN.md'` | ✅ | ✅ green |
| 1-02-02 | 02 | 2 | REQ-keylink-pattern-matchable | — | N/A | integration | `go test ./internal/keylinks/... -v` | ✅ | ✅ green |
| 1-03-01 | 03 | 3 | REQ-keylink-past-gates-reassessed | — | N/A | unit | `go test ./internal/keylinks/... -run 'TestReassess' -v` | ✅ | ✅ green |
| 1-03-02 | 03 | 3 | REQ-keylink-past-gates-reassessed | — | N/A | artifact | `test -f .planning/phases/01-gate-ci-integrity/01-KEYLINK-REASSESSMENT.md && head -1 … \| rg -q '^---$' && task license:check` | ✅ | ✅ green |
| 1-03-03 | 03 | 3 | REQ-keylink-past-gates-reassessed | — | N/A | checkpoint:decision | — (D-01 upstream-reporting decision) | n/a | manual-only |
| 1-04-01 | 04 | 1 | REQ-ci-qdrant-container-stability | — | N/A | integration | Qdrant rehearsal → `go test ./internal/store/... ./internal/server/... -count=1` | ✅ | ✅ green |
| 1-04-02 | 04 | 1 | REQ-ci-qdrant-container-stability | — | N/A | lint | `actionlint .github/workflows/ci.yaml && yamlfmt -lint .github/workflows/ci.yaml` | ✅ | ✅ green |
| 1-04-03 | 04 | 1 | REQ-ci-qdrant-container-stability | — | N/A | integration | Qdrant rehearsal → `-run TestSharedQdrantAddressHonored` across 4 packages | ✅ | ✅ green |
| 1-05-01 | 05 | 2 | REQ-ci-qdrant-container-stability | — | N/A | integration | Qdrant rehearsal → `go test ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1` | ✅ | ✅ green |
| 1-05-02 | 05 | 2 | REQ-ci-qdrant-container-stability | — | N/A | integration | Qdrant rehearsal → `go test ./internal/store/... -count=1` run twice (namespace idempotency) | ✅ | ✅ green |
| 1-06-01 | 06 | 3 | REQ-ci-qdrant-container-stability | — | N/A | unit | `go test ./internal/store/... -run TestEveryStoreConstructionRoutesThroughSeam -v` | ✅ | ✅ green |
| 1-06-02 | 06 | 3 | REQ-ci-qdrant-container-stability | — | N/A | unit | `go test ./internal/store/... -run 'TestCollectionPrefixesAreDisjoint\|TestEveryStoreConstructionRoutesThroughSeam' -v` | ✅ | ✅ green |
| 1-06-03 | 06 | 3 | REQ-ci-qdrant-container-stability | — | N/A | checkpoint:human-verify | — (observe a real GitHub Actions run) | n/a | manual-only |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Tests backing this map** (all re-executed green during this audit, `-count=1`,
observing individual `--- PASS` lines rather than a package-level `ok`):

| Test | File | Requirement |
|------|------|-------------|
| `TestFixturePairEscaping` | `internal/keylinks/keylinks_test.go:40` | REQ-keylink-pattern-matchable |
| `TestFixturePairSubsetAndSatisfiability` | `internal/keylinks/keylinks_test.go:93` | REQ-keylink-pattern-matchable |
| `TestScanPlansDeterministic` | `internal/keylinks/keylinks_test.go:246` | REQ-keylink-pattern-matchable |
| `TestNoEscapedPatternsRepoWide` | `internal/keylinks/gate_test.go:93` | REQ-keylink-pattern-matchable |
| `TestActiveMilestoneKeyLinksSatisfiable` | `internal/keylinks/gate_test.go:128` | REQ-keylink-pattern-matchable |
| `TestGateScopesAreDistinct` | `internal/keylinks/gate_test.go:139` | REQ-keylink-pattern-matchable |
| `TestZeroApplicabilityGuardFires` | `internal/keylinks/gate_test.go:160` | REQ-keylink-pattern-matchable |
| `TestReassessV013Phase12` | `internal/keylinks/sweep_test.go:246` | REQ-keylink-past-gates-reassessed |
| `TestReassessmentTableIsComplete` | `internal/keylinks/sweep_test.go:269` | REQ-keylink-past-gates-reassessed |
| `TestSharedQdrantAddressHonored` (×4 pkgs) | `store_test.go:241`, `tools_test.go:267`, `harness_test.go:192`, `retrieval_eval_test.go:404` | REQ-ci-qdrant-container-stability |
| `TestRequireQdrant` | `internal/store/store_test.go:200`, `internal/server/tools_test.go:286` | REQ-ci-qdrant-container-stability |
| `TestEveryStoreConstructionRoutesThroughSeam` | `internal/store/collectionprefix_conformance_test.go:244` | REQ-ci-qdrant-container-stability |
| `TestCollectionPrefixesAreDisjoint` | `internal/store/collectionprefix_conformance_test.go:395` | REQ-ci-qdrant-container-stability |

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No framework install, no
stub files, no shared fixtures were needed: this phase's own deliverable *is*
test infrastructure (`internal/keylinks` is a new package created by plan 01-01,
and the collection-prefix seam is enforced by a source-level AST scan added by
plan 01-06), so every requirement had a home from the first wave.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| D-01 upstream-reporting decision (report the key-link gate defect upstream vs. track on the spine) | REQ-keylink-past-gates-reassessed | A judgement call about disposition, not a program behavior — there is nothing for an assertion to observe. Resolved at plan time as spine-track. | Read `01-03-PLAN.md`'s checkpoint block and `01-KEYLINK-REASSESSMENT.md`'s D-01 entry; confirm the recorded disposition still reflects intent. |
| The three CI mechanism claims hold on a real GitHub Actions run (one container, `shared-address PASS=3 SKIP=1`, all packages `ok`) | REQ-ci-qdrant-container-stability | Requires observing a real hosted CI run; a local rehearsal proves the assertions but not that the hosted job wiring reaches them. | `gh run view <id> --log` and read the job log as text, not a badge. Confirmed against run `31718449162`, job `94509047766`, head `73ea27c9`. |
| Container exit reason is captured on failure (`if: failure()` step at `ci.yaml:143`) | REQ-ci-qdrant-container-stability | Exercising it requires deliberately killing the Qdrant service mid-run in hosted CI; the step is reachable only on a real failure path. | Inspect `ci.yaml:143` for state/exit-code/logs/OOM capture; on any future red CI run, confirm the diagnostic block emitted. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none — no Wave 0 needed)
- [x] No watch-mode flags
- [x] Feedback latency < 2s (quick command)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-16

---

## Validation Audit 2026-08-16

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Reconciled from a `status: draft` seed that `plan-phase` wrote and
`validate-phase` had never processed — the body was still the unfilled template
(`{pytest 7.x / jest 29.x / ...}` placeholders), so `nyquist_compliant: false`
was a template default, never a measured verdict. All 16 tasks across the 6
plans were mapped to their `<automated>` blocks; 14 are automated and 2 are
blocking checkpoints that are inherently manual (a disposition decision and a
hosted-CI observation).

No test was generated, because none was missing. Coverage was established by
execution, not by reading plan text: the full `internal/keylinks` suite (8 test
functions plus subtests) ran green; both `internal/store` conformance gates ran
green; and the four-package shared-address sweep was rehearsed against a real
pinned `qdrant/qdrant:v1.18.2` container, reproducing exactly the `PASS=3
SKIP=1` split `ci.yaml:111` asserts. `actionlint` and `yamlfmt -lint` on the
workflow both exited clean.

One disclosed weakness is carried forward rather than counted as a gap:
`TestSharedQdrantAddressHonored`'s address-equality half compares a value
against itself, so only its `testQdrantContainerBooted == false` half is
discriminating. This is stated in the code's own doc comment and in
`01-VERIFICATION.md`, and the load-bearing half is what plan 01-04's fail-first
red proof actually exercised.
