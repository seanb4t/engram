---
phase: 2
slug: record-schema-versioning-foundation
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-13
validated: 2026-08-16
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) |
| **Config file** | none — `Taskfile.yaml` wraps lint + test as `task` |
| **Quick run command** | `go test ./internal/migrate/... -count=1` |
| **Full suite command** | `task` (lint + `go test ./...`) |
| **Estimated runtime** | ~0.1s quick (`internal/migrate`); ~21s `internal/store` incl. the red-evidence harness; ~1.5s `internal/server` |

Qdrant-backed packages (`internal/store`, `internal/server`) run against a
container provisioned by testcontainers, or against `ENGRAM_QDRANT_TEST_ADDR`
when set. **Every command in the map below must be run with
`ENGRAM_REQUIRE_QDRANT=1`** — without it a missing Qdrant makes the integration
rows skip silently and report `ok`, which is indistinguishable from passing.
That fail-closed switch is a phase-1 deliverable this phase depends on.

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... -short -count=1`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/migrate/... ./internal/store/... ./internal/server/... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~2 seconds (`-short`, harness skipped), ~25 seconds (full three-package sweep incl. container boot)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | REQ-schema-version-stamped | — | N/A | integration | `go test ./internal/store/... -run '^TestSchemaVersionEndToEnd$' -count=1 -v` | ✅ | ✅ green |
| 2-01-02 | 01 | 1 | REQ-schema-version-stamped | — | N/A | unit | `go test ./internal/migrate/... -run '^TestCurrentVersionValue$' -count=1 -v && go test ./internal/store/... -run '^TestPayloadRoundTripsSchemaVersion$' -count=1 -v` | ✅ | ✅ green |
| 2-01-03 | 01 | 1 | REQ-schema-version-wire-visible | — | N/A | integration | `go test ./internal/store/... -run '^TestEnsureCollectionIndexesSchemaVersion$' -count=1 -v && go test ./internal/store/... -run '^TestUpdateRefreshesSchemaVersionUnderLock$' -count=1 -v && go test ./internal/store/... -run '^TestReindexRoundtrip$' -count=1 -v` | ✅ | ✅ green |
| 2-02-01 | 02 | 2 | REQ-schema-version-stamped | — | N/A | unit | `go test ./internal/store/... -run '^TestEveryPointWriteRoutesThroughPayload$' -count=1 -v` | ✅ | ✅ green |
| 2-02-02 | 02 | 2 | REQ-schema-version-stamped | — | N/A | unit | `go test ./internal/store/... -run '^TestPartialWritePathsAreClassifiedNonStamping$' -count=1 -v && go test ./internal/store/... -run '^TestQdrantClientIsHeldOnlyByStorePackage$' -count=1 -v` | ✅ | ✅ green |
| 2-02-03 | 02 | 2 | REQ-schema-version-stamped | — | N/A | integration | `go test ./internal/store/... -run '^TestEveryFullWriteMethodStampsSchemaVersion$' -count=1 -v` | ✅ | ✅ green |
| 2-03-01 | 03 | 3 | REQ-schema-version-never-gates-recall | — | N/A | unit | `go test ./internal/store/... -run '^TestFilterWalkerSeesEveryPosition$' -count=1 -v` | ✅ | ✅ green |
| 2-03-02 | 03 | 3 | REQ-schema-version-never-gates-recall | — | N/A | integration | `go test ./internal/store/... -run '^TestSchemaVersionNeverGatesRecall$' -count=1 -v && go test ./internal/store/... -run '^TestRecallEmissionSetIsCompleteAndClassified$' -count=1 -v` | ✅ | ✅ green |
| 2-03-03 | 03 | 3 | REQ-schema-version-never-gates-recall | — | N/A | integration | `go test ./internal/store/... -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | ✅ | ✅ green |
| 2-04-01 | 04 | 2 | REQ-schema-version-forward-compatible | — | N/A | integration | `go test ./internal/store/... -run '^TestSchemaVersionForwardBackwardCompat$/newer' -count=1 -v` | ✅ | ✅ green |
| 2-04-02 | 04 | 2 | REQ-schema-version-forward-compatible | — | N/A | integration | `go test ./internal/store/... -run '^TestSchemaVersionForwardBackwardCompat$' -count=1 -v` | ✅ | ✅ green |
| 2-05-01 | 05 | 2 | REQ-schema-version-wire-visible | — | N/A | unit | `go test ./internal/server/... -run '^TestSchemaVersionOnRecallWire$' -count=1 -v` | ✅ | ✅ green |
| 2-05-02 | 05 | 2 | REQ-schema-version-wire-visible | — | N/A | integration | `go test ./internal/server/... -run '^TestSchemaVersionOnGetMemoryWire$' -count=1 -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Reading these commands correctly.** `go test -run` exits **0 when the pattern
matches nothing** — a renamed or deleted test reports `ok` and the row reads
green while proving nothing. Every command above is therefore anchored (`^…$`)
and carries `-v`; confirm the run by observing an explicit `--- PASS: <Test>`
line, never a bare package-level `ok`. Prefix each with
`ENGRAM_REQUIRE_QDRANT=1` so a missing Qdrant fails instead of skipping.

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. `go test` and the shared
Qdrant testcontainer harness were both in place before this phase; the only new
test-infrastructure this phase adds is its own gate files, listed in the map
above.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

Two coverage deliverables — `02-02` D5 and `02-03` D4 — originally shipped as
`human_judgment: true` with `verification: []`, on the rationale that
reproducing a prove-RED cycle means running `git apply` / `go test` /
`git apply -R` by hand. That rationale no longer holds: task **2-03-03**'s
`TestRedEvidencePatchesAreLive` performs the whole cycle for every patch in
`red-evidence/` on each run.

The honest scope note: the harness proves each patch **still** applies cleanly,
**still** drives its target test RED, and **still** reverts to an identical
tree. It does not re-attest the original historical observation — that stays in
the plan SUMMARYs. Going forward the harness is the stronger property, because
it re-proves the claim on every CI run instead of once at authoring time.

**Two properties of the harness the reader should know up front.**

1. *It is a Go test that reads `.planning/**`.* That follows existing house
   precedent (`internal/keylinks/gate_test.go`, `internal/keylinks/sweep_test.go`,
   `internal/server/verbtabledocs_test.go` all do the same), but it does couple
   the Go suite to a planning artifact. If that coupling is unwanted, deleting
   `internal/store/redevidence_harness_test.go` reverts the decision completely
   and returns `02-02` D5 / `02-03` D4 to manual-only — nothing else depends on it.
2. *It hardcodes this phase's directory* (`redEvidenceDir`). Archiving the
   milestone with `/gsd-cleanup` moves `red-evidence/` out from under that path,
   at which point the zero-applicability guard fires and the test goes RED — by
   design, since a vanished evidence directory must not read as clean. Archiving
   this milestone therefore means deleting or repointing this test in the same
   change.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies — 13/13
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references — none were MISSING
- [x] No watch-mode flags
- [x] Feedback latency < 25s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-16

---

## Validation Audit 2026-08-16

| Metric | Count |
|--------|-------|
| Gaps found | 2 |
| Resolved | 2 |
| Escalated | 0 |

This file was an unfilled template before this audit (`status: draft`, every row
a placeholder). It was reconstructed from the five plan SUMMARYs' `coverage:`
blocks, and every claim was re-executed at HEAD rather than carried over.

**Verified, not assumed.** All 16 test functions named across the coverage
ledger were confirmed to resolve to real source, and all were run under
`ENGRAM_REQUIRE_QDRANT=1` on 2026-08-16 — 16/16 green.

**G-1 — `red-evidence/02-review-wr01-monotonic-max.patch` was corrupt.** Its
hunk header declared 7 context lines over a 6-line body; `git apply` rejected it
with `corrupt patch at …:11`. The phase's own `02-VERIFICATION.md` sampled 2 of
9 patches and both samples passed, so the defect survived verification. The
patch was regenerated via `git diff -U3` against current `store.go` and proven
to apply clean → drive `TestPayloadRoundTripsSchemaVersion` RED (subtests `zero`
and `below`) → revert clean.

**G-2 — the prove-RED cycle had no automated gate.** Now
`internal/store/redevidence_harness_test.go`. It globs the patch directory
(never a hand-listed set), `t.Fatal`s if the glob matches zero patches, asserts
patch↔target-test mapping by set equality in **both** directions, refuses to run
if the files a patch touches are already dirty, and restores the tree through
both `defer` and `t.Cleanup` so a panic mid-subtest cannot leave `store.go`
mutated. Gated behind `testing.Short()`. Observed: 9/9 subtests confirm RED,
~20s, tree clean after.

**The two gaps are linked.** Restoring the original corrupt patch was confirmed
to drive the new harness RED at its `git apply --check` guard — so G-2's gate
subsumes G-1's defect, and this class of rot cannot recur silently.

### Findings carried out of this audit

1. **The phase's one deferred item is now closed.**
   `02-VERIFICATION.md` still records truth 5's `older-explicit` compatibility
   row as dormant and deferred to Phase 4, because `migrate.CurrentVersion` was
   0 at authoring time. Phase 4's `8fb9d6d9` raised it to 1, and because
   `schemaversion_compat_test.go` derives its expected row count from the
   constant at runtime rather than calling `t.Skip`, the row **self-activated
   with no code change**: `executed compatibility rows: [absent older-explicit
   newer] (expected 3, derived from migrate.CurrentVersion=1)`. Read
   `02-VERIFICATION.md`'s deferred table as stale-in-the-good-direction, not as
   outstanding work.

2. **`02-VERIFICATION.md`'s `REQUIREMENTS.md` discrepancy is already resolved.**
   That report (2026-08-13) recorded `REQ-schema-version-forward-compatible` as
   an unchecked `[ ]` with "Pending" in the traceability table and recommended a
   follow-up commit. That commit happened: `119749ba` ("docs(phase-02): complete
   phase execution") checked the box and set the row to "Complete". Verified in
   the current file — nothing outstanding here. Read the discrepancy note in
   `02-VERIFICATION.md` as closed.
