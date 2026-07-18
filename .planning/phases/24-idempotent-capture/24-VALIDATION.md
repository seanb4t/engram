---
phase: 24
slug: idempotent-capture
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-18
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
| 24-01-01 | 01 | 1 | REQ-idempotent-capture | T-24-02 | Payload-only fingerprint round-trips wire-invisibly; distinct ErrIdempotencyConflict sentinel (not ErrNotFound) | unit | `go test -run 'TestPayloadRoundTripsIdempotencyFingerprint' ./internal/store/...` | ❌ W0 | ⬜ pending |
| 24-01-02 | 01 | 1 | REQ-idempotent-capture | T-24-01 / T-24-03 | Deterministic + injective + owner-scoped ID (T-24-01); determinism underpins race-safe Upsert (T-24-03); fingerprint tag-order-stable + key-independent | unit | `go test -run 'TestIdempotencyPointID\|TestContentFingerprint' ./internal/server/...` | ❌ W0 | ⬜ pending |
| 24-02-01 | 02 | 2 | REQ-idempotent-capture (SC5) | — | No-key path unchanged: two keyless repeats mint two distinct random ids | integration | `go test -run 'TestStoreMemoryNoKeyAlwaysFresh' ./internal/server/...` | ❌ W0 | ⬜ pending |
| 24-02-02 | 02 | 2 | REQ-idempotent-capture (SC1, SC2) | T-24-02 | SC1 replay→original (zero side-effect); SC2 mismatch→ErrIdempotencyConflict before embed, no silent overwrite | integration | `go test -run 'TestStoreMemoryIdempotentReplayReturnsOriginal\|TestStoreMemoryIdempotentReplayRejectsMismatch' ./internal/server/...` | ❌ W0 | ⬜ pending |
| 24-02-03 | 02 | 2 | REQ-idempotent-capture (SC3, SC4) | T-24-01 / T-24-03 | SC3 two-owner matrix → two independent records (T-24-01); SC4 concurrent identical → exactly one point, no-duplicate invariant only (T-24-03, D-12) | integration + `-race` | `go test -race -run 'TestStoreMemoryIdempotentKeyScopedPerOwner\|TestStoreMemoryIdempotentConcurrentIdenticalOnePoint\|TestScheduleMemoryIdempotentIgnoresWindowChange' ./internal/server/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All SC tests are net-new but co-located with the behavior they verify (each task writes its own
tests, TDD-style where noted). No separate Wave 0 plan is needed — the existing harness covers
every fixture requirement:

- [ ] `internal/store/store_test.go` — `TestPayloadRoundTripsIdempotencyFingerprint` (mirror of `TestPayloadRoundTripsEmbedderIdentity`, pure, no Qdrant)
- [ ] `internal/server/idempotency_test.go` — pure-fn unit tests for `idempotencyPointID` / `contentFingerprint` (new file, no Qdrant)
- [ ] `internal/server/tools_test.go` — SC1–SC5 + schedule-window tests via existing `testDepsWithStore` / `requireQdrant` / `failOrSkipNoQdrant` harness; SC4 uses the `TestStoreMemoryReturnsWhenSummarizerHangs` goroutine fan-out pattern under `go test -race`
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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
