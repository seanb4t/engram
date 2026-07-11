---
phase: 12-per-memory-usage-signals
reviewed: 2026-07-10T21:07:21Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/engram/serve.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/server/connectapi.go
  - internal/server/summary.go
  - internal/server/summary_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/server/usagequeue.go
  - internal/server/usagequeue_test.go
  - internal/store/instrument_test.go
  - internal/store/rerank_test.go
  - internal/store/store.go
  - internal/store/usage_test.go
  - internal/telemetry/metrics.go
  - proto/engram/v1/engram.proto
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-07-10T21:07:21Z
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

Phase 12 adds per-record `access_count` / `last_accessed_at` curation metadata with an async best-effort get-path incrementer (`usageQueue`) and a synchronous update-path bump. The core design invariants hold up under scrutiny:

- **Counting boundary is respected.** `IncrementAccess` is called only from the `usageQueue` fill (get path), and `st.Update` (the update-path bump) is called only from the `updateMemory` handler. Neither is reachable from search/list/list_scheduled, and neither runs inside `store.Get`/`GetReadable` (verified by call-site grep). Both get handlers (`tools.go:959`, `connectapi.go:204`) enqueue strictly after a successful, ownership-gated fetch, call-and-ignore.
- **Ranking isolation holds.** `access_count` is never read by `rerank.go`/`SearchReranked` or any recall-gate filter, and `rerank_test.go` guards this with a negative-space test.
- **Concurrency kernel is sound.** The `usageQueue` shutdown-safety kernel (RWMutex `closed` guard + `inFlight` reserve-before-send + `recover()` in `process` + idempotent `Shutdown`) is correct: send and close are mutually exclusive, the WaitGroup can't go negative, a panicking fill can't wedge the pool, and Shutdown is bounded by ctx. Well covered by `usagequeue_test.go` (including `-race` tests).
- **Reindex preserves the new fields** — `Reindex` writes the raw source `p.Payload` verbatim (`store.go:2142`), so `access_count`/`last_accessed_at` round-trip; the migrate/backfill partial `SetPayload` paths never touch them.
- **RMW last-writer-wins** on the counter is the accepted D-05 tradeoff and is not flagged.

Two contract/behavior defects survived review, plus one minor dead-code note. Neither warning is a security or data-loss issue; the accepted `status` is `issues_found`.

## Warnings

### WR-01: `ENGRAM_USAGE_SIGNALS=false` does not stop the update-path counter write (doc/behavior mismatch)

**File:** `internal/store/store.go:1356-1360`, `internal/server/tools.go:936`, `internal/config/config.go:135`
**Issue:** `UsageConfig`'s doc comment states the flag *"gates the payload access_count write on **get/update** (D-09)."* In practice the flag only gates the get path: `buildUsageQueue` (tools.go:245) returns a nil `usageQueue` when signals are off, disabling the async get incrementer. The **update** path bump lives unconditionally inside `store.Update`:

```go
cur.AccessCount++
cur.LastAccessedAt = s.now()
```

`updateMemory` (tools.go:936) calls `d.st.Update(...)` with no reference to `d.usageQueue` or `cfg.Usage.Signals`. So with `ENGRAM_USAGE_SIGNALS=false`, `get_memory` correctly stops incrementing, but `update_memory` still mutates `access_count`/`last_accessed_at` on every edit. An operator who sets the flag to `false` expecting zero usage-metadata churn (as the comment promises) still gets a partial, update-only signal — a surprising and undocumented behavior.

**Fix:** Pick one and make code and doc agree. Either gate the update bump behind the same switch (thread the enable decision into the handler, e.g. only bump when `d.usageQueue != nil`):

```go
// tools.go updateMemory — pass an "enabled" signal to Update, or:
bumpUsage := d.usageQueue != nil
return d.st.Update(ctx, cur, a.Content, a.Shared, a.Tags, sumArg, vec, bumpUsage)
// store.Update: if bumpUsage { cur.AccessCount++; cur.LastAccessedAt = s.now() }
```

…or, if the design genuinely intends the update bump to be always-on (as the phase's "free bump in store.Update" framing suggests), correct the `UsageConfig` doc comment to say the flag gates only the **get** path, so operators are not misled.

### WR-02: Never-accessed records surface a bogus `0001-01-01T00:00:00Z` `last_accessed_at` instead of being omitted

**File:** `internal/server/summary.go:59`, `internal/store/store.go:126`, `internal/server/connectapi.go:44`
**Issue:** `last_accessed_at` is a value-typed `time.Time`, and the code tries to omit it when zero via a struct tag:

```go
// summary.go recallView
LastAccessedAt time.Time `json:"last_accessed_at,omitempty"`
// store.go Memory
LastAccessedAt time.Time `json:"last_accessed_at,omitempty"`
```

`encoding/json`'s `omitempty` is a **no-op on struct types** like `time.Time` (a zero `time.Time` is not considered "empty"), so both the compact MCP recall view (`toRecallView`) and the full `store.Memory` recall shape emit `"last_accessed_at":"0001-01-01T00:00:00Z"` for every record that has never been accessed. The `,omitempty` tag signals the author intended omission, but it never fires. On the Connect wire the same happens explicitly: `memoryToProto` calls `timestamppb.New(m.LastAccessedAt)` unconditionally (connectapi.go:44), producing a year-1 `Timestamp` (`seconds:-62135596800`) rather than a nil/unset field. Clients cannot distinguish "never accessed" from a genuine timestamp except by separately checking `access_count == 0`, and any UI that sorts or renders "last accessed" will show year-1 dates.

Note this is a *new* inconsistency: the codebase's other optional timestamps (`NotBefore`/`NotAfter`) are `*time.Time` precisely so `omitempty` works; `CreatedAt` is always populated so it never manifests. `LastAccessedAt` is the first genuinely-optional value-typed timestamp and inherited a tag that can't do its job.

**Fix:** Make `LastAccessedAt` a `*time.Time` on `recallView` (and, if the recall JSON contract matters, on `store.Memory`) so `omitempty` actually omits it when unset; on the Connect side, emit nil for the zero case:

```go
// connectapi.go memoryToProto
var lastAccessed *timestamppb.Timestamp
if !m.LastAccessedAt.IsZero() {
    lastAccessed = timestamppb.New(m.LastAccessedAt)
}
// ... LastAccessedAt: lastAccessed,
```

## Info

### IN-01: `usageQueue.depth()` is unreferenced production code this phase

**File:** `internal/server/usagequeue.go:204-209`
**Issue:** `depth()` mirrors `summaryQueue.depth`, but unlike the summary queue (`buildSummaryQueue` calls `telemetry.RegisterSummaryQueueDepth`), `buildUsageQueue` never registers a usage-queue-depth gauge, and `UsageQueueMetrics` has no depth instrument. `depth()` is therefore exercised only by `usagequeue_test.go`, with no production caller. The doc comment acknowledges this ("for a future D-09-style gauge"), so it is intentional forward-scaffolding rather than a defect.
**Fix:** Either wire the gauge now for parity with the summary queue (add `RegisterUsageQueueDepth` and call it in `buildUsageQueue` after `q.Start`), or leave a tracking issue so the dead method doesn't linger unexplained.

---

_Reviewed: 2026-07-10T21:07:21Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
