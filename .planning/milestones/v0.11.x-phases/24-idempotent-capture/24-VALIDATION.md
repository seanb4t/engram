---
phase: 24
slug: idempotent-capture
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-18
validated: 2026-07-26
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) |
| **Config file** | none — `Taskfile.yaml` (`task test`) |
| **Quick run command** | `go test ./internal/server/... ./internal/store/...` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~60–120 seconds (store tests exercise a real Qdrant per `internal/store` bench/integration convention) |

---

## Sampling Rate

- **After every task commit:** Run `{quick run command}`
- **After every plan wave:** Run `{full suite command}`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

> Seeded scaffold — the planner populates one row per task, mapping each phase task to a
> success criterion (SC1–SC5) and threat ref. Reference SC coverage:
> SC1 replay→same record · SC2 mismatch→distinct error · SC3 two-owners-same-key matrix ·
> SC4 concurrent-identical→one point (`-race`) · SC5 no-key→fresh-every-time.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 24-01-01 | 01 | 1 | REQ-idempotent-capture | T-24-02 | Payload-only fingerprint round-trips wire-invisibly; distinct ErrIdempotencyConflict sentinel (not ErrNotFound) | unit | `go test -run 'TestPayloadRoundTripsIdempotencyFingerprint' ./internal/store/...` | ✅ `internal/store/store_test.go` | ✅ green |
| 24-01-02 | 01 | 1 | REQ-idempotent-capture | T-24-01 / T-24-03 | Deterministic + injective + owner-scoped ID (T-24-01); determinism underpins race-safe Upsert (T-24-03); fingerprint tag-order-stable + key-independent | unit | `go test -run 'TestIdempotencyPointID\|TestContentFingerprint' ./internal/server/...` | ✅ `internal/server/idempotency_test.go` | ✅ green |
| 24-02-01 | 02 | 2 | REQ-idempotent-capture (SC5) | — | No-key path unchanged: two keyless repeats mint two distinct random ids | integration | `go test -run 'TestStoreMemoryNoKeyAlwaysFresh' ./internal/server/...` | ✅ `internal/server/tools_test.go` | ✅ green |
| 24-02-02 | 02 | 2 | REQ-idempotent-capture (SC1, SC2) | T-24-02 | SC1 replay→original (zero side-effect); SC2 mismatch→ErrIdempotencyConflict before embed, no silent overwrite | integration | `go test -run 'TestStoreMemoryIdempotentReplayReturnsOriginal\|TestStoreMemoryIdempotentReplayRejectsMismatch' ./internal/server/...` | ✅ `internal/server/tools_test.go` | ✅ green |
| 24-02-03 | 02 | 2 | REQ-idempotent-capture (SC3, SC4) | T-24-01 / T-24-03 | SC3 two-owner matrix → two independent records (T-24-01); SC4 concurrent identical → exactly one point, no-duplicate invariant only (T-24-03, D-12) | integration + `-race` | `go test -race -run 'TestStoreMemoryIdempotentKeyScopedPerOwner\|TestStoreMemoryIdempotentConcurrentIdenticalOnePoint\|TestScheduleMemoryIdempotentIgnoresWindowChange' ./internal/server/...` | ✅ `internal/server/tools_test.go` | ✅ green |

> **Test-name resolution note (2026-07-26 audit):** rows 24-01-02's scaffold named
> `TestIdempotencyPointID` / `TestContentFingerprint`, which do not exist as literal function
> names. They are *prefixes* — `go test -run` takes a regex, so the command as written already
> selects all 8 shipped tests: `TestIdempotencyPointID{Deterministic,BoundaryShiftInjective,
> OwnerScoped,KeySensitive,ValidUUID}` and `TestContentFingerprint{TagOrderStable,
> FieldSensitivity,TagsBoundaryShiftInjective}`. No gap; no command change needed.

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All SC tests are net-new but co-located with the behavior they verify (each task writes its own
tests, TDD-style where noted). No separate Wave 0 plan is needed — the existing harness covers
every fixture requirement:

- [x] `internal/store/store_test.go` — `TestPayloadRoundTripsIdempotencyFingerprint` (mirror of `TestPayloadRoundTripsEmbedderIdentity`, pure, no Qdrant)
- [x] `internal/server/idempotency_test.go` — pure-fn unit tests for `idempotencyPointID` / `contentFingerprint` (new file, no Qdrant) — shipped with 8 test functions
- [x] `internal/server/tools_test.go` — SC1–SC5 + schedule-window tests via existing `testDepsWithStore` / `requireQdrant` / `failOrSkipNoQdrant` harness; SC4 uses the `TestStoreMemoryReturnsWhenSummarizerHangs` goroutine fan-out pattern under `go test -race`
- Shared fixtures: none new — existing harness sufficient.

*Note:* SC4 is the first `-race` invocation in this repo — flag as new CI-invocation surface (not an
existing convention). Race detector needs `CGO_ENABLED=1` for the test binary only (does not affect
the `CGO_ENABLED=0` distroless release build).

---

## Manual-Only Verifications

All phase behaviors have automated verification (SC1–SC5 + the schedule-window decision are all
`go test` assertions; SC4 runs under `-race`). No manual-only verifications.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s (measured: ~4.5s across both tiers)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-07-26

---

## Validation Audit 2026-07-26

Retroactive reconciliation — this file was seeded by plan-phase on 2026-07-18 and never promoted
by validate-phase, so it sat at `status: draft` with every row `⬜ pending` while the phase itself
shipped and merged (PR #404). The milestone audit classified it NOT-VALIDATED rather than PARTIAL
for exactly this reason (#2117): `nyquist_compliant: false` was the untouched seed value, not a
finding.

| Metric | Count |
|--------|-------|
| Task rows audited | 5 |
| Gaps found | 0 |
| Resolved | 0 (none needed) |
| Escalated | 0 |
| Tests generated | 0 — every named test already shipped with the phase |

**Evidence.** All 9 test functions named by the scaffold resolve to shipped tests (2 as regex
prefixes covering 8 functions — see the resolution note above). Both tiers were executed directly
at audit time, not inferred from the SUMMARY:

- Unit tier — `go test -count=1 -run 'TestIdempotencyPointID|TestContentFingerprint'
  ./internal/server/...` → **ok** (1.658s); `go test -count=1 -run
  'TestPayloadRoundTripsIdempotencyFingerprint' ./internal/store/...` → **ok** (1.237s).
- Integration tier — all six SC1–SC5 + schedule-window tests run under `-race` with
  `ENGRAM_REQUIRE_QDRANT=1 CGO_ENABLED=1` → **6/6 PASS** (2.786s), each confirmed by an explicit
  `=== RUN` / `--- PASS` pair.

The fail-closed `ENGRAM_REQUIRE_QDRANT=1` gate matters here: without it a bare `go test` reports
`ok` while every Qdrant-gated test silently SKIPS, which would have made an unexercised suite look
green. Setting it forces a hard failure if Qdrant is unreachable, so the passes above prove the
tests actually ran against a live instance.

**Verdict:** Phase 24 is Nyquist-compliant. No test generation was required; the gap was
bookkeeping, not coverage.
