---
phase: 15
slug: additive-proto-stub-write-handlers
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-11
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none — `go test ./...` via `Taskfile.yaml` `test:go` target |
| **Quick run command** | `go test ./internal/server/... -run 'TestEngramServiceDescriptor|TestWriteRPCNegativeMatrix' -v` |
| **Full suite command** | `task test` (= `go test ./...` + python skill-hook tests) |
| **Estimated runtime** | ~60 seconds (full suite; Qdrant testcontainer-backed store tests dominate) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/... -run 'TestEngramServiceDescriptor|TestWriteRPCNegativeMatrix' -v`
- **After every plan wave:** Run `task test` plus `go tool buf lint` and gen-drift check (`go tool buf generate && git diff --exit-code -- gen/`)
- **Before `/gsd-verify-work`:** Full suite green + CI `buf` job checks green (lint, breaking, gen-drift, new idempotency ban)
- **Max feedback latency:** ~60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| *(pending — filled once PLAN.md task IDs exist)* | | | REQ-connect-write-rpcs | | | | | | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/` descriptor test (new file or appended to `connectapi_test.go`) — SC2 idempotency invariant + SC4 read-lane req/resp type pinning via descriptor walk
- [ ] `internal/server/` negative-matrix test — SC3 full negative-path matrix (Unimplemented / Unauthenticated / GET-405 / InvalidArgument) across all six write RPCs; reuse the `httptest.NewServer` + real-interceptor-chain pattern from `connectapi_cookie_test.go`
- [ ] `internal/server/connectvalidate.go` + `connectvalidate_test.go` — hand-rolled protovalidate interceptor unit tests (valid passes, invalid → `CodeInvalidArgument`, non-proto defensively passes)

*(No test framework install needed — `go test` is already fully configured.)*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
