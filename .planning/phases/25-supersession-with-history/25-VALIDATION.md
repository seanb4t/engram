---
phase: 25
slug: supersession-with-history
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-19
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> **NYQUIST-COMPLIANT** — every requirement/success-criterion has automated verification;
> coverage expanded by the deep code review (CR-01..CR-04, WR-01..WR-04, IN-01..IN-03).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`, `-race`) |
| **Config file** | none — Go modules; store/server integration tests use testcontainers (Docker) or `ENGRAM_QDRANT_TEST_ADDR` |
| **Quick run command** | `go test ./internal/store/... ./internal/server/...` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~30–90 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/server/...`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite must be green (incl. `-race` vs real Qdrant)
- **Max feedback latency:** ~90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|--------|
| 25-01-T1 | 25-01 | 1 | REQ-supersession-links | superseded record soft-hidden from Search AND List, fetchable via Get; `*string` link codec round-trips | unit | `go test ./internal/store/... -run 'TestSupersedeRecallGate\|TestSupersedeStamp\|TestSupersedeVectorPreserved' -count=1` | ✅ green |
| 25-01-T2 | 25-01 | 1 | REQ-supersession-links | owner-only supersede (ActionWrite); already-superseded rejected; back-stamp preserves content+vector; TOCTOU fail-closed; forward chains | unit | `go test ./internal/store/... -run 'TestSupersede(OwnerGate\|AlreadySuperseded\|ForwardChain\|TOCTOU\|Stamp)' -count=1` | ✅ green |
| 25-02-T1 | 25-02 | 2 | REQ-supersession-links | `supersede_memory` routes through owner write gate; target-not-found re-wrapped 404-indistinguishable; rule/discovery targets handled | unit | `go test ./internal/server/... -run 'TestSupersedeMemory' -count=1` | ✅ green |
| 25-02-T2 | 25-02 | 2 | REQ-supersession-links | `ErrAlreadySuperseded → CodeFailedPrecondition` (sentinel switch exhaustive) | unit | `go test ./internal/server/... -run TestConnectError -count=1` | ✅ green |

### Review-added coverage (deep code review, all green under `-race`)

| Finding | Test(s) | Package |
|---------|---------|---------|
| CR-01 concurrent-supersede race | `TestSupersedeConcurrent` | store |
| CR-04 Update-erases-back-stamp race | `TestSupersedeVsUpdateConcurrent` | store |
| CR-01/CR-04 lock primitive | `TestInProcessTargetLocker{CanceledContextRejected,SameKeySerializes,DifferentKeysDoNotBlock,ConcurrentDistinctKeys}` | store |
| CR-02 rule-immutability | `TestSupersedeMemoryRejectsRule` | server |
| CR-03 cost-amplification (gate before embed) | `TestSupersedeMemoryEmbedNotCalledForNonOwner` | server |
| WR-01 SearchDiscovery soft-hide | `TestSearchDiscoverySupersededHidden` | store |
| WR-02 ListScheduled soft-hide | `TestListScheduledSupersededHidden` | store |
| WR-03 idempotency schema-excluded + ignored | `TestSupersedeMemorySchemaExcludesIdempotencyKey`, `TestSupersedeMemoryIgnoresIdempotencyKey` | server |
| WR-04 idempotency decode-but-ignore | `TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey` | server |
| IN-01 vector preservation | `TestSupersedeVectorPreserved` | store |
| IN-02 discovery-target handler | `TestSupersedeMemoryDiscoveryTarget` | server |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/store/store_test.go` — supersede owner-gate + TOCTOU + already-superseded + recall-gate (both Search & List) + Get-still-fetchable
- [x] `internal/store/locker_test.go` — TargetLocker unit tests (CR-01/IN-03)
- [x] `internal/server/tools_test.go` — handler owner-gate, rule/discovery targets, cost path, idempotency-ignored
- [x] Existing `go test -race` + real-Qdrant (testcontainers) harness covers integration — no new framework install

*Existing infrastructure covered the framework; all Wave-0 stubs landed RED-first (tdd) then green.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| — | REQ-supersession-links | All phase behaviors are automated via `go test` against real Qdrant | — |

*All phase behaviors have automated verification.*

---

## Validation Audit 2026-07-19

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

REQ-supersession-links and all 4 Success Criteria are COVERED by automated `-race` tests; the deep code
review expanded coverage to 24 supersession/locker test functions. No MISSING or PARTIAL requirements —
`nyquist_compliant: true`.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-07-19
