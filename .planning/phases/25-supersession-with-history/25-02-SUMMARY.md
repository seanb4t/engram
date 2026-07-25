---
phase: 25-supersession-with-history
plan: 02
subsystem: mcp-server
tags: [go, mcp, tools, authz, supersession, connect]

# Dependency graph
requires:
  - phase: 25-supersession-with-history
    plan: "01"
    provides: "Store.Supersede(ctx, newMem, vec, target, subj) owner-gated back-stamp primitive, store.ErrAlreadySuperseded sentinel, Memory.Supersedes/SupersededBy link fields"
provides:
  - "supersede_memory MCP tool: caller-facing single verb to correct a memory they own"
  - "supersedeArgs (embeds storeArgs) + deps.supersedeMemory handler"
  - "connectError sentinel-switch exhaustiveness for store.ErrAlreadySuperseded"
affects: [connect-supersede-rpc-if-ever-added]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dedicated-verb MCP tool embeds storeArgs (Go field promotion) to inherit the full store_memory field set without a hand-rolled parallel field list (mirrors scheduleArgs)"
    - "memStore narrow interface must be extended alongside any new *store.Store method a deps.* handler calls; spyStore fake must gain a matching scripted implementation in the same task"

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/connecterror.go
    - internal/server/connecterror_test.go

key-decisions:
  - "supersedeMemory does NOT call checkIdempotentReplay despite storeArgs.IdempotencyKey riding along via embedding — plan's numbered handler steps omit it; RESEARCH Pitfall 2 accepted this as an open, non-blocking gap for this phase"
  - "connectError maps store.ErrAlreadySuperseded to CodeFailedPrecondition, pre-positioning only (no Connect RPC exposes supersede_memory this phase) — same discipline as ErrIdempotencyConflict"
  - "toRecallView left untouched — Supersedes/SupersededBy do not surface in the compact recall view this phase (matches NotBefore/NotAfter precedent)"

patterns-established:
  - "Any new *store.Store method that a deps.* MCP handler calls through d.st must be added to the narrow memStore interface (store_iface.go) AND given a scripted implementation on spyStore (fakestore_test.go) in the same task — both are required for go vet/go build to pass, not just go build"

requirements-completed: [REQ-supersession-links]

coverage:
  - id: D1
    description: "supersede_memory MCP tool registered; supersedeArgs embeds storeArgs (D-03)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSupersedeMemory"
        status: pass
    human_judgment: false
  - id: D2
    description: "Handler stores a new correcting record, back-stamps the target's superseded_by, target content/vector untouched, target absent from search_memory/list_memory, still fetchable via get_memory"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSupersedeMemory"
        status: pass
    human_judgment: false
  - id: D3
    description: "SC3: a non-owner caller cannot supersede a target they don't own; error re-wraps store.ErrNotFound with the caller's ORIGINAL a.Supersedes input, never the resolved UUID (404-indistinguishability)"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSupersedeMemory (cross-owner subtest)"
        status: pass
    human_judgment: false
  - id: D4
    description: "connectError sentinel switch is exhaustive: store.ErrAlreadySuperseded maps to CodeFailedPrecondition and is enumerated in the doc comment"
    requirement: "REQ-supersession-links"
    verification:
      - kind: unit
        ref: "internal/server/connecterror_test.go#TestConnectError/already_superseded"
        status: pass
    human_judgment: false

# Metrics
duration: 3min
completed: 2026-07-19
status: complete
---

# Phase 25 Plan 02: supersede_memory MCP Tool Summary

**supersede_memory is now a registered MCP verb — supersedeArgs embeds storeArgs, deps.supersedeMemory resolves the target and delegates the create+back-stamp entirely to Store.Supersede's owner write gate, and connectError's sentinel switch is exhaustive for store.ErrAlreadySuperseded.**

## Performance

- **Duration:** ~3 min (task-commit span; excludes context-loading)
- **Started:** 2026-07-19T13:45:41-04:00 (first task commit)
- **Completed:** 2026-07-19T13:48:09-04:00 (last task commit)
- **Tasks:** 2 (Task 1 tdd="true", RED then GREEN; Task 2 single commit)
- **Files modified:** 6 (`internal/server/tools.go`, `internal/server/tools_test.go`, `internal/server/store_iface.go`, `internal/server/fakestore_test.go`, `internal/server/connecterror.go`, `internal/server/connecterror_test.go`)

## Accomplishments
- `supersedeArgs` embeds `storeArgs` (mirrors `scheduleArgs`'s embedding shape, D-03/RESEARCH A4) so `supersede_memory` inherits `content`/`scope`/`source`/`category`/`tags`/`repo`/`workspace`/`worktree_path`/`base_dir`/`summary`/`idempotency_key` and adds one field: `Supersedes` (id, full UUID or short_id, of the record this new one corrects).
- `(*deps).supersedeMemory(ctx, c, a)`: resolves the target via `ResolvePointID`, builds the correcting `Memory` via the promoted `a.toMemory(...)`, embeds the content, mints a short id, and delegates entirely to `Store.Supersede` (Plan 01) for the owner-gated create+back-stamp. Never calls `GetReadable` — the write gate lives solely in `Store.Supersede`'s `getWritable`/`ActionWrite` path (SC3, T-25-07).
- 404-indistinguishability: on `store.ErrNotFound` the handler re-wraps with the caller's **original** `a.Supersedes` input, never the resolved target UUID — matches `setVisibility`/`storeDiscovery` (T-25-08).
- The new correcting record is enqueued via `d.summaryQueue.tryEnqueue(m.ID)` — store_memory-shaped, participates in async summary-on-write (RESEARCH Open Question 1, resolved: yes).
- `supersede_memory` registered with `mcp.AddTool`, mirroring `set_visibility`'s registration shape; returns `{"id": ..., "short_id": ...}` like `store_memory`.
- `connectError`'s sentinel switch gained `store.ErrAlreadySuperseded -> connect.CodeFailedPrecondition`, pre-positioning only (no Connect RPC exposes `supersede_memory` this phase) — same discipline as the existing `ErrIdempotencyConflict` case (Phase 24); the function's doc-comment enumeration was updated to match.
- `TestSupersedeMemory` (handler-level, real Qdrant): seeds a target via `storeMemory`, supersedes it, and asserts the new record's id/short_id are non-empty, the target's `SupersededBy` equals the new id with content untouched, the new record's `Supersedes` equals the target, the target is absent from both `list_memory` and `search_memory`, and a second (different-owner) caller's supersede attempt is rejected with `store.ErrNotFound` echoing the caller's original short_id input — never the resolved UUID.
- `already_superseded` row added to `TestConnectError`, mirroring the `idempotency_conflict` row.

## Task Commits

1. **Task 1: supersede_memory tool — args, handler, registration** (tdd="true", strict RED → GREEN)
   - `51dd80ea` (test) — `TestSupersedeMemory`, confirmed failing to compile (`supersedeArgs`/`d.supersedeMemory` undefined) after temporarily stashing the implementation to prove RED
   - `83741771` (feat) — `supersedeArgs`, `deps.supersedeMemory`, `supersede_memory` registration, plus the required `memStore` interface extension (`store_iface.go`) and matching `spyStore.Supersede` fake (`fakestore_test.go`); `TestSupersedeMemory` green
   - `3d4f4a2c` (fix) — golangci-lint (staticcheck QF1008) flagged `a.storeArgs.toMemory(...)` as unnecessary since `storeArgs.toMemory` is already promoted onto `supersedeArgs`; simplified to `a.toMemory(...)`, matching `scheduleMemory`'s existing call shape
2. **Task 2: connectError exhaustiveness for ErrAlreadySuperseded**
   - `30817057` (feat) — sentinel-switch case + doc-comment enumeration update + `already_superseded` `TestConnectError` row

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/server/tools.go` — `supersedeArgs` struct, `(*deps).supersedeMemory` handler, `supersede_memory` tool registration
- `internal/server/tools_test.go` — `TestSupersedeMemory`
- `internal/server/store_iface.go` — `Supersede(...)` added to the `memStore` narrow interface (required: `d.st.Supersede` would not compile against the interface-typed `deps.st` without it)
- `internal/server/fakestore_test.go` — `spyStore.Supersede` scripted fake, mirroring `Store.Supersede`'s owner-gate + already-superseded + create-then-back-stamp shape (required for `go vet`'s compile-time `var _ memStore = (*spyStore)(nil)` assertion)
- `internal/server/connecterror.go` — `store.ErrAlreadySuperseded -> CodeFailedPrecondition` case + doc-comment enumeration
- `internal/server/connecterror_test.go` — `already_superseded` row in `TestConnectError`

## Decisions Made
- `supersedeMemory` does **not** call `checkIdempotentReplay`, even though `storeArgs.IdempotencyKey` rides along via embedding. The plan's numbered handler steps (1–7) deliberately omit it, and RESEARCH Pitfall 2 flags this as an accepted, non-blocking gap ("does not fully solve the two-step problem... a caller can retry with a new key"). Implemented exactly as the plan specified — no deviation.
- Extended the narrow `memStore` interface (`store_iface.go`) and added a matching `spyStore.Supersede` fake (`fakestore_test.go`) as a Rule 3 (blocking-issue) auto-fix: the plan's `<files>` list for Task 1 named only `tools.go`/`tools_test.go`, but `d.st.Supersede(...)` cannot compile against the interface-typed `deps.st` without the interface extension, and `go vet`'s `var _ memStore = (*spyStore)(nil)` compile-time assertion cannot pass without the fake. Both are direct, minimal, same-shape extensions of the exact pattern every other `Store` method already follows in these two files — not an architectural change.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `memStore` interface and `spyStore` fake required for `d.st.Supersede` to compile**
- **Found during:** Task 1, first `go build`/`go vet` after adding the handler
- **Issue:** The plan's Task 1 `<files>` list only named `internal/server/tools.go` and `internal/server/tools_test.go`. `deps.st` is typed as the narrow `memStore` interface (`store_iface.go`), not the concrete `*store.Store` — calling `d.st.Supersede(...)` fails to compile until `Supersede` is added to that interface, and `go vet`'s compile-time interface-satisfaction assertion on `spyStore` (`fakestore_test.go`) fails until `spyStore` gets a matching method.
- **Fix:** Added `Supersede(ctx, newMem, vec, target, subj) error` to `memStore` (placed alphabetically between `SetVisibility` and `Update`, matching the interface's existing ordering), and `spyStore.Supersede` — a scripted fake mirroring `Store.Supersede`'s owner-gate (`ErrNotFound` on non-owner/absent target), single-hop rejection (`ErrAlreadySuperseded` on an already-superseded target), and create-new-record-then-back-stamp-target ordering.
- **Files modified:** `internal/server/store_iface.go`, `internal/server/fakestore_test.go`
- **Commit:** `83741771`

**2. [Rule 1 - Bug/lint] Simplified promoted-field access flagged by staticcheck**
- **Found during:** Task 1, `task lint:go` after the GREEN commit
- **Issue:** `a.storeArgs.toMemory(...)` — golangci-lint's staticcheck (QF1008) flagged the explicit embedded-field selector as unnecessary, since `toMemory` is already promoted onto `supersedeArgs` via Go's embedding rules.
- **Fix:** Changed to `a.toMemory(...)`, matching `scheduleMemory`'s existing call shape at `tools.go:825` byte-for-byte in spirit.
- **Files modified:** `internal/server/tools.go`
- **Commit:** `3d4f4a2c`

None of the plan's locked decisions (D-01 through D-09) or success criteria (SC1–SC4) required any adjustment — both deviations are additive, same-shape extensions of existing patterns, not architectural changes.

## Issues Encountered

- `task lint:yaml` fails on `Taskfile.yaml` itself (yamlfmt lint complaint), reconfirmed via `git stash` (isolating this plan's changes) to fail identically on a clean tree — pre-existing and unrelated, first documented in `25-01-SUMMARY.md`. `task lint:go` (the Go-scoped lint these changes are actually subject to) is clean (`0 issues.`). `task license:check` is clean (`985 files, valid: 207, invalid: 0`). `task test:go` (`go test ./...`) is green across every package, including `internal/store`/`internal/server` with `-race`.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness
- `supersede_memory` is a fully wired, tested MCP tool: `supersedeArgs`/`deps.supersedeMemory`/registration in `tools.go`, `Supersede` in the `memStore` interface with a matching `spyStore` fake, and `connectError`'s sentinel switch stays exhaustive for `store.ErrAlreadySuperseded`.
- All four of Phase 25's success criteria (SC1–SC4) are now verified end-to-end through the MCP tool surface, on top of Plan 01's store-layer primitive.
- No blockers. `go build ./...` and `go vet ./...` clean across the whole repo.

---
*Phase: 25-supersession-with-history*
*Completed: 2026-07-19*

## Self-Check: PASSED

All claimed files (`internal/server/tools.go`, `internal/server/tools_test.go`, `internal/server/store_iface.go`, `internal/server/fakestore_test.go`, `internal/server/connecterror.go`, `internal/server/connecterror_test.go`, this SUMMARY.md) and all four task commit hashes (`51dd80ea`, `83741771`, `3d4f4a2c`, `30817057`) verified present.
