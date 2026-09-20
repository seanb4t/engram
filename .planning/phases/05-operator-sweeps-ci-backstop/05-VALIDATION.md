---
phase: "05"
slug: "operator-sweeps-ci-backstop"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-20"
validated: "2026-09-20"
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

Seeded without a RESEARCH.md (research was skipped — the mechanism was already built and proven by
Phases 3–4). The seed deliberately left every new test name as `TO-RESOLVE` rather than guessing
one: a `-run` pattern matching nothing exits 0 with `no tests to run` and reports a permanent false
green (`gfh6q1ack4`, `bsbsvn4hbc`; Phase 4 hit exactly this, where two of five seeded names never
existed). **Every name below was resolved against `go test -list` on 2026-09-20 and the match count
verified**, so no row can false-green.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib); real Qdrant via `internal/store/storetest` |
| **Config file** | none — `Taskfile.yaml` `test:*` tasks are the entry points |
| **Quick run command** | `go test -short ./internal/store/...` |
| **Full suite command** | `task` (lint + whole-repo test, includes the real-Qdrant `internal/store` suite) |
| **Actual runtime** | ~60 s quick · `internal/store` 487 s at the phase gate (53 red-evidence patches dominate) |

---

## Sampling Rate

- **After every task commit:** `go test -short ./internal/store/...`
- **After every plan:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` — **against a COMMITTED tree only**; a dirty tree makes `TestRedEvidencePatchesAreLive` fail spuriously (every plan this phase hit this)
- **Phase gate:** full `task` green — verified exit 0 at the phase close
- **Timeouts:** ambient load pushed `internal/store` past `go test`'s 10-minute default; pass an extended `-timeout` on the one-off command. **Never** raise a timeout in `Taskfile.yaml` or CI to hide it (05-06's explicit prohibition).

---

## Per-Requirement Verification Map

| Requirement | Behavior proven | Type | Automated Command | Tests matched | Status |
|-------------|-----------------|------|-------------------|---------------|--------|
| REQ-sweeps-bounded (spine sweeps) | `ScanSpine`, `EnumerateCitations`, `NearDuplicates`, `derivePurgeEligible` and the revert preflight each complete over a scope whose 256-record page would exceed the receive limit | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestScanSpineBoundedOverGRPCLimit\|TestEnumerateCitationsBoundedOverGRPCLimit\|TestNearDuplicatesBoundedOverGRPCLimit\|TestPreviewPurgeBoundedOverGRPCLimit\|TestRevertPreviewBoundedOverGRPCLimit)$' -count=1 -v` | 5 | ✅ green |
| REQ-sweeps-bounded (own-loop sweeps) | `engram migrate`, `migrate revert` apply, `summarize-missing` and `reindex` complete the same way; `reindex` walks its effective source, not `s.collection` | integration (real Qdrant) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestMigrateBoundedOverGRPCLimit\|TestRevertApplyBoundedOverGRPCLimit\|TestSummarizeMissingBoundedOverGRPCLimit\|TestReindexBoundedOverGRPCLimit\|TestReindexSourceOverride)$' -count=1 -v` | 5 | ✅ green |
| REQ-recv-limit-backstop | The configured `MaxCallRecvMsgSize` reaches the dial options, and the backstop precedes caller options so a caller's own limit still wins. **Pass-through and ORDER only** — never that gRPC enforces a ceiling (D-06, rule `m45p2b4bp7`) | unit | `go test ./internal/store/ -run '^(TestQdrantRecvLimitBackstopPassThrough\|TestQdrantRecvLimitBackstopPrecedesCallerOptions)$' -count=1 -v` | 2 | ✅ green |
| REQ-bounded-read-mechanism | Every inventory site routes through a shared primitive or carries a written exemption; `unbudgetedView` has no callers; the recall-gate classifications match | integration + structural | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^(TestRecallEmissionSetIsCompleteAndClassified\|TestSchemaVersionNeverGatesRecall)$' -count=1 -v` plus check (b): `test "$(rg -o 's[.]scrollAllPoints[(].*unbudgetedView[(]' internal/store --glob '!*_test.go' \| wc -l)" -eq 0` | 2 | ✅ green |
| REQ-ci-store-green | #497 closed on #498's recorded evidence (D-07). **No new test by design** — it is a recorded reasoning step, not a behavior. The CI image pin stays gated | structural | `go test ./internal/store/ -run '^TestQdrantImageMatchesCIService$' -count=1 -v`; `gh issue view 497 --json state` reports `CLOSED` | 1 | ✅ green |
| **Semantics preserved (D-02)** | The four migrated own-loop sweeps behave identically — all 83 in-place tests ran UNEDITED and green | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` (the full suite; `migrate_test.go` 14, `revert_test.go` 9, `summarize_test.go` 8, `reindex_test.go` 17, `spine_test.go` 35) | 83 | ✅ green |

**Structural gate that guards the whole phase:**

| Gate | Command | Guards |
|------|---------|--------|
| Red evidence | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -timeout 45m -v` | all 53 registered patches (Phase 1×4, 2×8, 3×13, 4×17, 5×11) still apply and still go RED — verified 242 s, exit 0 |

**Fixture-shape caveat, recorded so no row over-claims:** only `storetest.FewLarge` (40 × 128 KiB ≈ 5.24 MiB) overflows a 256-record page. `storetest.ManySmall` (1000 × ~5.2 KiB ≈ 1.34 MiB) does **not** — it proves multi-page iteration, not overflow. Every sweep regression runs both shapes; none asserts `ManySmall` overflows.

---

## Wave 0 Requirements

All satisfied — this phase wrote 7 new test files against the existing harness:

- [x] `internal/store/spinesweeps_oversized_test.go` — the four `spine.go` sweeps
- [x] `internal/store/revertpreview_oversized_test.go` — the revert preflight
- [x] `internal/store/migratesweep_oversized_test.go` — `migrate` + `revert` apply
- [x] `internal/store/summarizesweep_oversized_test.go` — `summarize-missing`
- [x] `internal/store/reindexsweep_oversized_test.go` — `reindex`, incl. the source override
- [x] `internal/store/qdrantbackstop_test.go` — the backstop's pass-through and ordering
- [x] 11 red-evidence patches under `red-evidence/`, registered in `redEvidenceDirs`
- [x] Framework install: none required

---

## Manual-Only Verifications

*None.* Every behavior this phase changed is reachable from `internal/store`'s own suite. Note that
`Store.reindexTargetContents` is **Exempt** in the inventory: its batch is bounded by the page its
caller accumulates, not a byte-budget view. **Correction (phase-5 security audit):** an earlier
wording attributed this to "an operator sizing `--batch`" — no such flag exists on `engram reindex`,
which always runs at the fixed `reindexBatch = 256`. The real limitation is that a 256-record page of
large-but-cap-compliant records can reach ~237 MiB, past the 64 MiB backstop, with no operator lever
to reduce it. Not tested here and not fixed here — tracked as GitHub #596.

---

## Validation Audit 2026-09-20

| Metric | Count |
|--------|-------|
| Requirements audited | 4 (+ the D-02 semantics property) |
| COVERED | 4 |
| PARTIAL | 0 |
| MISSING | 0 |
| Gaps filled this audit | 0 |
| Escalated to manual-only | 0 |

**`TO-RESOLVE` resolution.** The seed carried no invented names. All fifteen tests referenced above
were resolved against `go test -list` and their match counts verified (5 / 5 / 2 / 2 / 1) before
being written into this map.
