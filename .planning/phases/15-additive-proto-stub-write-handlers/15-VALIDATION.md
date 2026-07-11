---
phase: 15
slug: additive-proto-stub-write-handlers
status: planned
nyquist_compliant: true
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
| 15-01-01 | 01 | 1 | REQ-connect-write-rpcs | T-15-SC | BSR protovalidate dep pinned via committed `buf.lock` | CLI assert | `test -f buf.lock && grep -q 'protovalidate' buf.lock && grep -q 'buf.build/bufbuild/protovalidate' buf.yaml` | ✅ `buf.yaml` (edit); `buf.lock` new | ⬜ pending |
| 15-01-02 | 01 | 1 | REQ-connect-write-rpcs | T-15-01 | Additive-only proto; no `NO_SIDE_EFFECTS` on any write RPC | lint | `go tool buf lint` | ✅ `engram.proto` (edit) | ⬜ pending |
| 15-01-03 | 01 | 1 | REQ-connect-write-rpcs | T-15-SC | Codegen idempotent; `gen/` matches proto exactly | build + drift | `go tool buf generate && go build ./... && git add gen/ && go tool buf generate && git diff --exit-code -- gen/` | ✅ `gen/` (regen) | ⬜ pending |
| 15-02-01 | 02 | 1 | REQ-connect-write-rpcs | T-15-01 | Build fails if `idempotency_level = NO_SIDE_EFFECTS` appears in `proto/` | grep gate | `task proto:lint` | ✅ `Taskfile.yaml` (edit) | ⬜ pending |
| 15-02-02 | 02 | 1 | REQ-connect-write-rpcs | T-15-01 | CI mirrors the ban inline (no `task` binary on runners) | grep assert | `grep -q 'idempotency ban' .github/workflows/ci.yaml && ! grep -q 'setup-task' .github/workflows/ci.yaml` | ✅ `ci.yaml` (edit) | ⬜ pending |
| 15-03-01 | 03 | 2 | REQ-connect-write-rpcs | T-15-03 | Malformed payloads rejected `CodeInvalidArgument` before handler code | unit (TDD) | `go test ./internal/server/... -run TestConnectValidateInterceptor -v` | ❌ W0 (new files) | ⬜ pending |
| 15-03-02 | 03 | 2 | REQ-connect-write-rpcs | T-15-02 | auth (401) strictly before validate (400) in `mountConnect` chain | unit | `go build ./internal/server/... && go test ./internal/server/... -run 'TestConnect' -v` | ✅ `connectapi.go` (edit) | ⬜ pending |
| 15-04-01 | 04 | 3 | REQ-connect-write-rpcs | T-15-01 | 11 RPCs all `IDEMPOTENCY_UNKNOWN`; read-lane req/resp types pinned | descriptor unit | `go test ./internal/server/... -run TestEngramServiceDescriptor -v` | ❌ W0 (new file) | ⬜ pending |
| 15-04-02 | 04 | 3 | REQ-connect-write-rpcs | T-15-01 / T-15-02 | Exact codes per cell: Unimplemented / Unauthenticated / GET-405 / InvalidArgument ×6 RPCs | unit (httptest matrix) | `go test ./internal/server/... -run TestWriteRPCNegativeMatrix -v` | ❌ W0 (new file) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/connectdescriptor_test.go` — SC2 idempotency invariant + SC4 read-lane req/resp type pinning via descriptor walk *(delivered by task 15-04-01)*
- [ ] `internal/server/connectapi_negative_test.go` — SC3 full negative-path matrix (Unimplemented / Unauthenticated / GET-405 / InvalidArgument) across all six write RPCs; reuses the `httptest.NewServer` + real-interceptor-chain pattern from `connectapi_cookie_test.go` *(delivered by task 15-04-02)*
- [ ] `internal/server/connectvalidate.go` + `connectvalidate_test.go` — hand-rolled protovalidate interceptor unit tests (valid passes, invalid → `CodeInvalidArgument`, non-proto defensively passes) *(delivered by task 15-03-01)*

*(No test framework install needed — `go test` is already fully configured.)*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (every task has one)
- [x] Wave 0 covers all MISSING references (15-03-01, 15-04-01, 15-04-02 create them)
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** planned 2026-07-11 (task IDs mapped to 15-01/15-02/15-03/15-04)
