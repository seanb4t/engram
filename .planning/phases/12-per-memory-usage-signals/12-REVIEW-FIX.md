---
phase: 12-per-memory-usage-signals
fixed_at: 2026-07-10T00:00:00Z
review_path: .planning/phases/12-per-memory-usage-signals/12-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 12: Code Review Fix Report

**Fixed at:** 2026-07-10
**Source review:** .planning/phases/12-per-memory-usage-signals/12-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, WR-02 — Critical + Warning scope)
- Fixed: 2
- Skipped: 0

IN-01 (Info) was out of scope and not addressed.

## Fixed Issues

### WR-01: `ENGRAM_USAGE_SIGNALS=false` does not stop the update-path counter write (doc/behavior mismatch)

**Files modified:** `internal/config/config.go`
**Commit:** 88758b4e
**Applied fix:** Doc-comment-only fix, zero behavior change (per CONTEXT D-09 intent). The `Signals` flag was designed to gate only the get-path async payload write to eliminate read-path write amplification — not to disable all counting. The `store.Update` bump rides the existing Upsert (D-04, no extra round-trip), so it is intentionally not gated. Corrected the `UsageConfig` doc comment to state accurately that the flag gates the get-path async `access_count`/`last_accessed_at` write, and that the `update_memory` bump piggybacks the existing Upsert unconditionally by design (no read-path amplification). Runtime behavior unchanged.

### WR-02: Never-accessed records surface a bogus `0001-01-01T00:00:00Z` `last_accessed_at` instead of being omitted

**Files modified:** `internal/store/store.go`, `internal/server/summary.go`, `internal/server/connectapi.go`, `internal/store/usage_test.go`, `internal/server/summary_test.go`
**Commit:** a1764c0e
**Applied fix:** Pointer-ized `LastAccessedAt` to mirror the existing `NotBefore`/`NotAfter` pattern (nil = never accessed) so `json:",omitempty"` actually fires:
- `store.Memory.LastAccessedAt` → `*time.Time`; `payload()` writes the key only when non-nil; `fromPayload()` sets a non-nil pointer only when the key is present; `store.Update`'s bump now assigns `&now`.
- `recallView.LastAccessedAt` → `*time.Time`; `toRecallView` copies the pointer (nil stays nil).
- `memoryToProto` (connectapi.go) emits the proto `LastAccessedAt` only when the source pointer is non-nil, else leaves it unset (no year-1 timestamp).
- `IncrementAccess` was unchanged — it already writes the `last_accessed_at` payload string directly via `SetPayload` rather than through the struct field.
- Tests updated to the pointer shape; added assertions that a never-accessed record omits `last_accessed_at` (recallView nil case + legacy-record nil case). `access_count` (uint64) left as-is — `omitempty` works on scalars.

## Verification

- `go build ./...` — clean
- `go test ./... -count=1` — all packages pass (incl. Qdrant testcontainers: `internal/server` 10.1s, `internal/store` 15.8s)
- `task lint:go` — 0 issues

---

_Fixed: 2026-07-10_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
