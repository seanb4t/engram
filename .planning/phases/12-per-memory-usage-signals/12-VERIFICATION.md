---
phase: 12-per-memory-usage-signals
verified: 2026-07-10T17:55:55Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
resolution: >
  The single gap below (task lint:go failing on 2 revive findings introduced by
  12-04) was fixed inline by the orchestrator in commit badd4a5d
  ("fix(12): resolve revive lint regressions in store.go recall-id helper"):
  the recallIDCap/recallIDs block was reordered above Store.Search's doc comment
  (reattaching it) and the shadowing `max` parameter was renamed to `limit`.
  Re-verified: `task lint:go` → 0 issues; `go build ./...` + `go test ./... -count=1`
  green. Status flipped gaps_found → passed. Functional goal was already 3/3.
gaps:
  - truth: "Terminal `task` (lint + test) is green — explicit deliverable of 12-06-PLAN.md (\"Terminal `task` (lint + test) green\", must_have artifact list, Task 3 completion checklist)."
    status: resolved
    resolved_by: "badd4a5d — renamed recallIDs `max`→`limit`, reattached Store.Search doc comment; task lint:go now 0 issues"
    reason: >
      `task lint:go` (and therefore `task` / `task lint`) currently fails with 2
      golangci-lint `revive` findings in internal/store/store.go:
      `redefines-builtin-id` (recallIDs' `max` parameter shadows the builtin) and
      `exported` (Store.Search now lacks an attached doc comment). Both were
      introduced by THIS phase's own commit df8c2077 ("feat(12-04): attach
      bounded recall-id attributes to store recall spans") — verified via
      `git show df8c2077 -- internal/store/store.go`: the diff inserted the new
      `recallIDCap` const + `recallIDs` func between Search's pre-existing doc
      comment and the `Search` function declaration itself, detaching the
      comment from the method it documented. 12-06-SUMMARY.md and
      deferred-items.md both characterize these as "pre-existing golangci-lint
      findings ... not by 12-06" — that framing is misleading: they are not
      pre-existing to Phase 12 at all (git history shows zero occurrences
      before df8c2077); they are an unresolved regression introduced earlier in
      this same phase (12-04) and left unfixed through 12-06's terminal gate
      check, contradicting the SUMMARY's "No blockers for closing out Phase 12"
      and CLAUDE.md's "task lint ... must be clean" convention.
    artifacts:
      - path: "internal/store/store.go"
        issue: "Line 575 recallIDs(out []Memory, max int) shadows builtin max; Line 587 Store.Search doc comment is detached (now attached to the recallIDCap const inserted above it), making Search lint-flagged as undocumented."
    missing:
      - "Rename recallIDs' max parameter (e.g. to `cap` or `limit`) to stop shadowing the builtin."
      - "Restore a doc comment directly above `func (s *Store) Search(...)` (the existing comment block is now orphaned onto `const recallIDCap`), or reorder recallIDCap/recallIDs above the Search comment so the comment stays attached to Search."
      - "Re-run `task lint:go` (and ideally full `task`, modulo the separately-tracked pre-existing .planning/ markdown debt) to confirm green."
---

# Phase 12: Per-Memory Usage Signals Verification Report

**Phase Goal:** Track strong per-record usage to inform curation, as operational metadata that never silently changes what recall returns.
**Verified:** 2026-07-10T17:55:55Z
**Status:** passed (initial verify found 1 process gap; resolved inline — see Resolution)
**Re-verification:** Gap resolved by commit `badd4a5d`; `task lint:go` re-run → 0 issues

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Counters increment **only** on `get_memory` fetch-by-id and `update_memory` — never on search/list result-set membership | ✓ VERIFIED | `d.usageQueue.tryEnqueue` is called from exactly two sites: `deps.getMemory` (`internal/server/tools.go:959`, on the `err==nil` branch after `GetReadable`) and Connect `GetMemory` (`internal/server/connectapi.go:204`, same success-only pattern). `store.IncrementAccess` is called nowhere except inside `usagequeue.go`'s injected `fill`. `store.Update` bumps `cur.AccessCount++`/`LastAccessedAt` synchronously before `Upsert` (`internal/store/store.go:1359-1360`). Grepped `searchMemory`/`listMemory`/`listScheduled`/`SearchMemories`/`ListMemories` — zero `tryEnqueue`/`IncrementAccess` references. Negative-space tests `TestGetMemoryEnqueuesUsageSignalOnSuccessOnly` (asserts denied get enqueues nothing) and `TestSearchListMemoryDoNotEnqueueUsageSignal` (asserts zero enqueues across search/list/list_scheduled even when the same record was just get-fetched) both read and executed — genuinely assert the invariant, not name-only stubs. `go test -run` of both: PASS. |
| 2 | Hybrid storage: recall ids ride OTLP spans → ClickStack for analytics (zero storage change), payload `access_count` maintained on get/update for MCP-visible curation tools | ✓ VERIFIED | Payload half: `access_count`/`last_accessed_at` round-trip via `payload()`/`fromPayload()` (`store.go:306-308,393-398`); `IncrementAccess` (`store.go:1430-1455`) does a `SetPayload` RMW (no re-embed) fired only from `usagequeue.go`'s `fill`; `store.Update`'s free bump. Analytics half: `store.Search`/`store.List`/`store.Get` spans all carry `engram.recall.ids` (capped at `recallIDCap=50`) + `engram.recall.count` on the success path (`store.go:601-605,813-817,1147-1150`) — NOT in `instrument.go`/`instrumentTools` (grepped, zero hits — matches D-06's store-layer-not-MCP-only placement requirement). Span-recorder tests `TestStoreSearchEmitsRecallIDs`, `TestStoreListEmitsRecallIDs`, `TestStoreGetEmitsRecallIDs`, `TestStoreListRecallIDsCappedAtLimit` all executed and PASS. |
| 3 | Usage counters are server-set and **never** silently affect ranking; usage-weighted recall remains an explicit out-of-scope future decision | ✓ VERIFIED | `internal/store/rerank.go` grepped for `access_count`/`AccessCount` — zero hits (reranker never reads the field). `TestRerankHitsIgnoresAccessCount` (`rerank_test.go:105`) is a genuine negative-space guard: two input sets identical in Content/Tags/Score/ID but wildly divergent, non-monotonic AccessCount values (0 vs 500K/1M/1) — asserts output ORDER is unchanged; not a name-only stub. Executed: PASS. Server-set-only: `storeArgs`/`updateArgs` (tools.go:383-394,470-476) carry no client-writable `access_count`/`last_accessed_at` field — grepped and read in full. |

**Score:** 3/3 truths verified. All three ROADMAP Success Criteria hold against the real codebase (not SUMMARY claims), backed by executed, substantive negative-space tests — not presence-only checks.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` — `Memory.AccessCount`/`LastAccessedAt` + `IncrementAccess` | Payload fields + partial-write primitive | ✓ VERIFIED | Fields at lines 118-126; `IncrementAccess` at 1421-1455; `Update`'s free bump at 1356-1360. |
| `internal/server/usagequeue.go` | Bounded async worker pool, CR-01 shutdown kernel, no retry | ✓ VERIFIED | Full file read. RWMutex `closed` guard + `inFlight` reserve-before-send at `tryEnqueue` (mirrors CR-01 exactly); `process` does a single `fill` attempt, no backoff/retry loop; `Shutdown` is idempotent and ctx-bounded. |
| `internal/config/registry.go` — `usage.signals` / `ENGRAM_USAGE_SIGNALS` | koanf field, default "true" | ✓ VERIFIED | `registry.go:58`: `{Key: "usage.signals", Env: "ENGRAM_USAGE_SIGNALS", Default: "true"}`; `config.go:135-141` `UsageConfig.Signals string`; `validate.go:114` parseability check. |
| `proto/engram/v1/engram.proto` fields 19/20 + committed `gen/` | Additive Memory fields | ✓ VERIFIED | `proto/engram/v1/engram.proto:33,35`: `access_count=19`, `last_accessed_at=20`. `gen/go/engram/v1/engram.pb.go` and `gen/ts/engram/v1/engram_pb.ts` both carry the generated fields. `git diff --exit-code -- gen/` exits 0 (clean, committed). |
| `internal/server/summary.go` — `recallView.AccessCount`/`LastAccessedAt` + `toRecallView` | D-07 read-only exposure | ✓ VERIFIED | `summary.go:58` field; `toRecallView` populates it (line ~102); `TestToRecallViewSurfacesUsageSignals` executed, PASS. |
| `internal/server/connectapi.go` — `memoryToProto` | D-07 Connect exposure | ✓ VERIFIED | `connectapi.go:43-44`: `AccessCount: m.AccessCount, LastAccessedAt: timestamppb.New(m.LastAccessedAt)`. |
| `internal/telemetry/metrics.go` — `UsageQueueMetrics` | enqueued/dropped/failed, no retry counter | ✓ VERIFIED | `metrics.go:189-223`; struct has exactly `enqueued`/`dropped`/`failed` fields — no retry/retried counter (matches 12-02's prohibition). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `buildDepsFromEnv` | `buildUsageQueue` | `deps.usageQueue` field assignment | ✓ WIRED | `tools.go:187`. |
| `deps.getMemory` / Connect `GetMemory` | `usageQueue.tryEnqueue` | success-only, call-and-ignore | ✓ WIRED | `tools.go:956-960`, `connectapi.go:193-204`. Confirmed no return-value check on `tryEnqueue` (it returns nothing) and no branching on the outcome. |
| `usageQueue.fill` | `store.IncrementAccess` | injected closure | ✓ WIRED | `tools.go:252-253`. |
| `serve.go` | `Register` shutdown drain | `NewUsageQueueMetrics` construction + composed shutdown | ✓ WIRED | `cmd/engram/serve.go:104,156`; `tools.go:1140-1143` composes `d.summaryQueue.Shutdown` then `d.usageQueue.Shutdown` into the single returned closure. |
| `store.Search`/`List`/`Get` | OTLP span attributes | `recallIDs` helper | ✓ WIRED | `store.go:601-605,813-817,1147-1150`; NOT present in `instrument.go` (correct per D-06's store-layer placement requirement). |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-usage-signals | 12-01..12-06 (all 6 plans declare it) | Strong per-record usage signals; get/update-only counting; hybrid storage; never affects ranking | ✓ SATISFIED | All three ROADMAP Success Criteria verified above. REQUIREMENTS.md line 104 checked `[x]`, no orphaned requirement IDs found for Phase 12 (line 157 is the only mapping, matches the declared plan requirement). |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/store/store.go` | 575 | `revive: redefines-builtin-id` — `recallIDs(out []Memory, max int)` shadows the builtin `max` | ✅ RESOLVED | Fixed in `badd4a5d`: parameter renamed `max`→`limit`. `task lint:go` now 0 issues. |
| `internal/store/store.go` | 587 | `revive: exported` — `Store.Search` doc comment orphaned onto `recallIDCap` const inserted above it by the same 12-04 commit | ✅ RESOLVED | Fixed in `badd4a5d`: recallIDCap/recallIDs reordered above the Search doc comment, reattaching it. Was correctly identified as a 12-04 regression (not pre-existing). |

No `TBD`/`FIXME`/`XXX` debt markers found in any phase-12-touched file. No stub/placeholder/empty-implementation patterns found in `store.go`, `usagequeue.go`, `tools.go`, `connectapi.go`, `summary.go`, `rerank.go`.

### Build & Test Verification (run directly, not from SUMMARY claims)

- `go build ./...` — clean, no output, exit 0.
- `go test ./... -count=1` — all 11 packages `ok` (including `internal/server` 14.0s, `internal/store` 16.9s with live Qdrant testcontainers).
- Named spot-checks (single-test, not full-suite reruns): `TestGetMemoryEnqueuesUsageSignalOnSuccessOnly`, `TestSearchListMemoryDoNotEnqueueUsageSignal`, `TestUpdateMemoryIncrementsAccessCountOnceNoAsyncEnqueue`, `TestBuildUsageQueueConfigGate`, `TestToRecallViewSurfacesUsageSignals`, `TestStoreSearchEmitsRecallIDs`, `TestStoreListEmitsRecallIDs`, `TestStoreGetEmitsRecallIDs`, `TestStoreListRecallIDsCappedAtLimit`, `TestRerankHitsIgnoresAccessCount` — all PASS.
- `task lint:go` — initial verify: **FAILED** with 2 `revive` issues in `internal/store/store.go` (see Anti-Patterns above). **After fix `badd4a5d`: PASSES — 0 issues** (re-run by orchestrator).
- `git diff --exit-code -- gen/` — clean (proto codegen committed, no drift).
- `git status --short` — clean working tree (only unrelated untracked `.planning/config.json`).

### Human Verification Required

None. All three Success Criteria and the D-07/D-09/D-10 supporting decisions were verifiable via code inspection, executed tests, and direct build/test runs.

### Gaps Summary

The phase's **functional goal is fully achieved**: all three ROADMAP Success Criteria (get/update-only counting, hybrid OTLP+payload storage, ranking isolation) are solidly verified against real code and executed negative-space tests, not SUMMARY narrative. `go build`/`go test` are clean.

The one gap is process, not function: 12-06-PLAN.md explicitly lists "Terminal `task` (lint + test) green" as a required deliverable, and the 12-06-SUMMARY.md/deferred-items.md claim this gate is met ("No blockers for closing out Phase 12"), attributing 2 `golangci-lint` failures in `internal/store/store.go` to a "pre-existing" state. `git show` of commit df8c2077 (12-04) proves both issues were introduced by this phase itself (the `recallIDCap`/`recallIDs` insertion detached `Search`'s doc comment and introduced a builtin-shadowing parameter name) and were never fixed before the phase was marked complete. `task lint:go` (and thus `task lint`/`task`) fails today as a direct, reproducible consequence.

This was a small, mechanical fix (rename a parameter, reattach a doc comment) — not a design or correctness problem.

**Resolution (orchestrator, commit `badd4a5d`):** the `recallIDCap`/`recallIDs` block was reordered above `Store.Search`'s doc comment (reattaching the comment to the method) and the shadowing `max` parameter was renamed `limit`. `task lint:go` re-run → **0 issues**; `go build ./...` and `go test ./... -count=1` remain green. The misleading "pre-existing" characterization in 12-06-SUMMARY.md / deferred-items.md is noted here for the record; the finding was correctly a 12-04 regression, now fixed. Phase status: **passed**.

---

_Verified: 2026-07-10T17:55:55Z (gap resolved same day via `badd4a5d`)_
_Verifier: Claude (gsd-verifier); resolution applied by orchestrator_
