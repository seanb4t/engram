---
phase: 05
slug: connect-record-state-parity
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-15
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `05-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) |
| **Config file** | none — plain `go test ./...` via `Taskfile.yaml` |
| **Quick run command** | `go test ./internal/server/... -run <NewTestName> -count=1` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~60–180 seconds (`internal/store` spins a Qdrant testcontainer) |

Note: test names are `TBD at plan time` — RESEARCH.md deliberately did not invent them. The
planner assigns the real names; the per-task map below is filled by `/gsd-validate-phase`
once PLAN.md task IDs exist.

---

## Sampling Rate

- **After every task commit:** the specific new test's `-run` command (unfiltered at package
  scope — `go test ./internal/server/ -count=1` — per the Phase 4 convention that an
  unfiltered per-package run is what makes the no-forward-reference invariant hold by
  construction rather than by rule).
- **After every plan wave:** `go test ./internal/server/... ./internal/store/... -count=1`
- **Before `/gsd-verify-work`:** `task` (full lint + test) must be green.
- **Max feedback latency:** ~180 seconds.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-connect-record-state-parity | — | N/A | unit | `go test ./internal/server/ -count=1` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REQ-connect-parity-roundtrip-proof | — | N/A | unit | `go test ./internal/server/ -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

Rows are seeded from the requirement map in RESEARCH.md; `/gsd-validate-phase` replaces `TBD`
with real task IDs after planning.

---

## Wave 0 Requirements

- [ ] New test file in `internal/server` for the exhaustive parity / population /
      permanent-negative-fixture test (D-05, D-06, D-07). Does not exist yet — this phase
      creates it. Package placement is settled: `internal/server` already imports both
      `internal/store` and `gen/go/engram/v1` (`internal/server/connectapi.go:19-22`) and
      `internal/store` imports neither, so there is no cycle.
- [ ] New test file or sub-test for the boundary-second read-lane-identity assertion (D-09).
      May reuse `dialRawQdrantClient` (`internal/server/schemaversion_wire_test.go:175`) if a
      hand-shaped sub-second record must bypass `payload()`'s codec.
- [ ] No framework install needed — stdlib `testing` only, already used throughout
      `internal/server`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `ui/src/lib/gen/` TypeScript codegen stays in sync | REQ-connect-record-state-parity | The phase gate (`task`) does not build the UI, but the proto change dirties the generated TS tree | Run `task ui:build` locally after `task proto:gen`; confirm no uncommitted drift under `ui/src/lib/gen/` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] The exhaustive detector is proven bidirectional — it PASSES on a fully-mapped struct AND
      FAILS on the permanent negative fixture. A detector only ever observed passing is a
      vacuous gate, the failure mode this repo has hit repeatedly.
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
