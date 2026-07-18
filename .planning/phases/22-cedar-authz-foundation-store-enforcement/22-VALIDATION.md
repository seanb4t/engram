---
phase: 22
slug: cedar-authz-foundation-store-enforcement
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-17
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib; store suite uses a Qdrant testcontainer) |
| **Config file** | none — `Taskfile.yaml` drives `task test` |
| **Quick run command** | `go test ./internal/authz/ ./internal/store/` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~35 seconds (authz <1s; store ~31s incl. testcontainer) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/authz/ ./internal/store/`
- **After every plan wave:** Run `task` (lint + test)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | REQ-cedar-pdp-foundation | T-22-01/02/03/SC | 4-policy embedded corpus parses; scoped empty-owner forbid | unit | `go test ./internal/authz/ -run TestPolicyCorpus` (OwnRecordAllow, SharedReadOnly, CrossOwnerWriteDeny, EmptyOwnerDenyAll, AnonOwnBucketReachable, ForbidOverridesPermit) | ✅ | ✅ green |
| 22-01-02 | 01 | 1 | REQ-cedar-pdp-foundation | — | DecideBucket/DecideRecord wiring; entity construction | unit | `go test ./internal/authz/ -run "TestPDP_DecideBucketWiring\|TestPDP_DecideRecordWiring"` | ✅ | ✅ green |
| 22-01-03 | 01 | 1 | REQ-cedar-pdp-foundation | — | PDP immutable + concurrency-safe | unit (`-race`) | `go test ./internal/authz/ -race -run TestPDP_ConcurrentDecideRace` | ✅ | ✅ green |
| 22-02-01 | 02 | 2 | REQ-cedar-store-enforcement | T-22-05 | PDP injection; WithAuthz option; MustDefault default | unit | `go test ./internal/store/ -run TestWithAuthzOption` | ✅ | ✅ green |
| 22-02-02 | 02 | 2 | REQ-cedar-store-enforcement | T-22-05/06 | Bulk builders PDP-backed, same filter shapes, outer-Must | integration (Qdrant) | `go test ./internal/store/ -run "TestBulkFilterOwnAndSharedAdjacency\|TestBulkFilterZeroBucketFailsClosed\|TestBulkFilterOrderIndependent"` | ✅ | ✅ green |
| 22-02-03 | 02 | 2 | REQ-cedar-store-enforcement | T-22-06 | O(buckets) never O(records) on recall | integration (Qdrant) | `go test ./internal/store/ -run TestSearchAuthzCallCount` | ✅ | ✅ green |
| 22-03-01 | 03 | 3 | REQ-cedar-store-enforcement | T-22-08/10 | Id-addressed gates via DecideRecord; Deny ≡ missing id; absent short-circuit | integration (Qdrant) | `go test ./internal/store/ -run "TestGetReadableDenyMapsToNotFound\|TestGetWritableAndOwnedOrAbsentDenyMapsToNotFound\|TestIdAddressedAbsentShortCircuit"` | ✅ | ✅ green |
| 22-03-02 | 03 | 3 | REQ-cedar-store-enforcement | T-22-01/07 | Behavior preservation: pre-existing isolation/sharing suite unchanged | integration (Qdrant) | `go test ./internal/store/ -run "TestSearchListOwnerIsolation\|TestGetReadableOwnerGate\|TestDeleteOwnerGate\|TestUpdateOwnerGateAndSharedFlag\|TestSetVisibilityOwnerGate\|TestOwnedOrAbsent\|TestAnonBucketReadIsolation\|TestAnonBucketWriteSemantics\|TestListScheduledOwnerIsolation"` | ✅ | ✅ green |
| 22-03-03 | 03 | 3 | REQ-cedar-store-enforcement | T-22-05 | DeleteAll PDP-backed (post-review fix); denied bucket deletes nothing | integration (Qdrant) | `go test ./internal/store/ -run "TestDeleteAllOwnerScoped\|TestDeleteAllDeniedBucketDeletesNothing"` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covered all phase requirements — `internal/store/store_test.go`
already hosted the isolation/sharing suite (the behavior-preservation oracle); the new
`internal/authz` package added its own policy-corpus test files as ordinary `go test` files.
No Wave 0 stubs were needed.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Audit 2026-07-17

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Both requirements fully COVERED. Live verification at audit time: `go test ./internal/authz/ -count=1` → ok (0.086s, 9 tests); `go test ./internal/store/ -count=1` (targeted authz/isolation runs) → ok (30.6s, Qdrant testcontainer). The two test gaps identified during the deep code review (write-gate deny mapping, DeleteAll denied-bucket branch) were already closed by the review `--fix` loop (`1da70fc2`, `d3f6c740`) before this audit.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none existed)
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-17
