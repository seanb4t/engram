---
phase: 11-async-on-write-summaries
reviewed: 2026-07-10T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/server/summaryqueue.go
  - internal/server/summaryqueue_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/summarize.go
  - internal/config/registry.go
  - internal/config/config.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/config/config_test.go
  - internal/telemetry/metrics.go
  - cmd/engram/serve.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: resolved
---

# Phase 11: Code Review Report

**Reviewed:** 2026-07-10T00:00:00Z
**Depth:** standard
**Files Reviewed:** 12 (docs-site/CLAUDE.md/go.mod carried no diffable logic and are excluded from the counted findings scope; they were read for context only)
**Status:** issues_found

## Summary

The async-on-write summary queue (`internal/server/summaryqueue.go`) is well-engineered:
nil-safe no-op receivers, a deterministic `Wait()` drain seam, panic-recovery balanced
against `itemDone()`, a genuinely non-blocking `tryEnqueue`, and metrics that are nil-guarded
throughout. The AND-gate in `buildSummaryQueue` (tools.go) is implemented correctly and the
existing test suite (including two dedicated panic/hung-fill/shutdown-budget race tests) is
strong.

However, the core safety invariant the design relies on — "once `httpSrv.Shutdown` returns,
no in-flight HTTP handler can still call `tryEnqueue`, so closing the channel is safe" — does
**not** hold when `httpSrv.Shutdown` returns due to its context deadline expiring rather than
completing gracefully. `net/http.Server.Shutdown` does not forcibly terminate active handler
goroutines on context timeout; it simply stops waiting for them and returns the context error.
Given the server intentionally leaves `ReadTimeout`/`WriteTimeout` unset for long-lived SSE
connections, and a `store_memory`/`schedule_memory` handler's duration is bounded only by the
embedder/Qdrant round-trip (no per-request timeout), this is a realistic production
send-on-closed-channel panic under a slow-backend + SIGTERM race (e.g. a rolling k8s
deployment hitting the 15s grace period while a write is in flight). This is the review's one
BLOCKER.

Two further concurrency/tuning defects were found that the existing tests do not exercise
because they use small, test-only duration values rather than the shipped defaults: the
worker pool's context is never derived from anything shutdown-aware, and the hardcoded
`summaryQueueMaxElapsed` retry budget (20s) is actually *shorter* than the default
`ENGRAM_SUMMARY_TIMEOUT` (30s), silently defeating the documented 3-try retry protection for
any fill call that is merely slow rather than instantly-failing.

## Critical Issues

### CR-01: Shutdown-timeout path lets an in-flight HTTP handler send on the closed enqueue channel

**File:** `cmd/engram/serve.go:209-225`, `internal/server/summaryqueue.go:230-252`

**Issue:**
`summaryQueue.Shutdown` documents (and relies on) the invariant that the caller "MUST
guarantee `httpSrv.Shutdown(ctx)` has already returned before calling this, which in turn
guarantees no in-flight HTTP handler can still call `tryEnqueue`." That invariant is false
whenever `httpSrv.Shutdown` returns because its context deadline expired rather than because
every connection went idle:

```go
// cmd/engram/serve.go
shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
shutdownErr := httpSrv.Shutdown(shutdownCtx)
// ...
drainSummaries(shutdownCtx)   // uses the SAME (possibly already-expired) ctx
return shutdownErr
```

Per the Go stdlib docs for `(*http.Server).Shutdown`: it closes listeners, then closes idle
connections, then waits for the remaining connections to become idle; "if the provided
context expires before the shutdown is complete, Shutdown returns the context's error" —
it does **not** forcibly abort still-running handlers. A `store_memory`/`schedule_memory`
handler that is mid-flight (blocked in the embedder HTTP call or the Qdrant `Upsert`, which
carry no per-request timeout here — `ReadTimeout`/`WriteTimeout` are deliberately 0 for SSE)
can therefore still be running *after* `httpSrv.Shutdown` returns with
`context.DeadlineExceeded`.

Immediately afterward, `drainSummaries(shutdownCtx)` is called with the same already-expired
context. Inside `summaryQueue.Shutdown`:

```go
func (q *summaryQueue) Shutdown(ctx context.Context) {
	if q == nil {
		return
	}
	close(q.ch)          // executes unconditionally, immediately
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():    // fires instantly since ctx is already expired
	}
}
```

`close(q.ch)` runs unconditionally and immediately. If the still-running handler from above
now reaches `d.summaryQueue.tryEnqueue(m.ID)` (tools.go:580, tools.go:619) — i.e. its
`Upsert` finally completed — it executes `q.ch <- id` on a **closed channel**, which panics
("send on closed channel") and crashes the process during what was supposed to be a graceful
shutdown. This is exactly the pitfall the code's own comments identify (RESEARCH Pitfall 1,
Codex finding referenced in the Shutdown doc comment) but the mitigation ("caller guarantees
Shutdown already returned") is insufficient because it only holds on the graceful-return path,
not the timeout path — and the timeout path is the one most likely to be hit in production
(slow backend + bounded grace period is the canonical shutdown scenario this code is trying
to survive).

**Fix:** Don't rely on `close()` as the shutdown signal for a channel with concurrent,
uncoordinated producers. Use a separate stop signal (atomic flag or dedicated `done` channel)
that `tryEnqueue` checks, and stop closing `q.ch` from producers' perspective — only the
consumer side needs `q.ch` to end:

```go
type summaryQueue struct {
	ch     chan string
	closed atomic.Bool // set by Shutdown; checked by tryEnqueue before send
	// ...
}

func (q *summaryQueue) tryEnqueue(id string) {
	if q == nil || q.closed.Load() {
		return
	}
	select {
	case q.ch <- id:
		// ...
	default:
		// ...
	}
}

func (q *summaryQueue) Shutdown(ctx context.Context) {
	if q == nil {
		return
	}
	q.closed.Store(true) // stop accepting new sends first
	close(q.ch)           // now safe: no new sender can observe closed==false and race the close
	// ... rest unchanged
}
```

Note this still has a narrow TOCTOU window (a `tryEnqueue` call could pass the `closed.Load()`
check just before `Shutdown` sets it and then block on the `select`), but combined with the
existing non-blocking `select/default` it degrades to a dropped enqueue instead of a panic —
acceptable, since a dropped id is already a documented, metric-counted, backstop-recoverable
outcome. Alternatively, guard the send itself with a `recover()` inside `tryEnqueue` and count
it as a drop, which is simpler but masks the underlying design gap.

## Warnings

### WR-01: Worker pool context is `context.Background()`, never cancelled by shutdown

**File:** `internal/server/tools.go:220`, `internal/server/summaryqueue.go:144-161`, `230-252`

**Issue:** `buildSummaryQueue` starts the worker pool with `q.Start(context.Background())`,
not a context tied to the process's shutdown signal (`sigCtx` in serve.go is never threaded
through). Inside `process`, the per-attempt timeout is derived from this same unbounded
context: `context.WithTimeout(ctx, q.attemptTimeout)`. As a result:

- `backoff.Retry(ctx, ...)` never observes shutdown-driven cancellation between retry
  attempts — only the per-attempt `attemptTimeout` and the hardcoded `summaryQueueMaxElapsed`
  bound it.
- `summaryQueue.Shutdown`'s own `ctx` parameter only bounds how long `Shutdown()` blocks its
  *caller*; it does nothing to interrupt the worker goroutines actually doing the work. A
  worker mid-retry when `Shutdown` is invoked keeps running — up to `summaryQueueMaxElapsed`
  (20s) — even though `Shutdown()` has already returned to `serve.go` and the process is
  proceeding to exit.

This contradicts the `Shutdown` doc comment's claim that it "waits for in-flight fills to
finish, bounded by ctx" — it only *waits*, bounded by ctx; it does not *cancel*. In the common
case where the process actually terminates shortly after `runServe` returns, any still-running
worker goroutine is killed mid-flight by process exit, silently abandoning a fill that could
have been a Qdrant `SetPayload` half-issued. (Low severity in practice because
`summarize-missing` is a documented backstop for exactly this case, but the code does not
behave as its own comments describe.)

**Fix:** Thread a shutdown-aware context into `Start`, e.g. pass `sigCtx` from serve.go (or
have `summaryQueue` own a `context.WithCancel` that `Shutdown` cancels immediately after
`close(q.ch)`), so in-flight retries actually stop advancing once shutdown begins rather than
running to their own independent timeout.

### WR-02: `summaryQueueMaxElapsed` (20s) is shorter than the default `attemptTimeout` (30s), silently starving the retry budget

**File:** `internal/server/summaryqueue.go:23-32`, `internal/server/tools.go:257-267`,
`internal/config/registry.go:39`

**Issue:** The constant comment claims:

```go
// summaryQueueMaxElapsed bounds total retry wall time per id, well under
// the summarizer's own per-request timeout, so a single stuck id cannot
// compound across retries into a multi-minute stall (RESEARCH Pitfall 3).
summaryQueueMaxElapsed = 20 * time.Second
```

But `attemptTimeout` in production is `summaryTimeout(cfg)`, which defaults to **30 seconds**
(`ENGRAM_SUMMARY_TIMEOUT` registry default `"30s"`, `tools.go:257-267`). So with default
configuration, `attemptTimeout (30s) > summaryQueueMaxElapsed (20s)` — the *opposite* of "well
under." Concretely: if the summarizer gateway is merely slow (e.g. consistently takes
25 seconds to respond, well short of a "hang"), the very first attempt alone exceeds the
20-second elapsed budget. `backoff.Retry` evaluates `MaxElapsedTime` after an attempt returns,
so the retry loop gives up after exactly **one** attempt instead of the intended
`summaryQueueMaxTries = 3`. Any operator who raises `ENGRAM_SUMMARY_TIMEOUT` (a documented,
supported knob, e.g. for a slower local model) makes this strictly worse — the bounded-retry
protection this queue was explicitly built to provide effectively disappears, and the design
intent recorded in the code comments is factually wrong for the shipped defaults.

Consequence is graceful (a single dropped-after-one-try fill is still reclaimed by
`summarize-missing`), so this is a WARNING not a BLOCKER, but it is a real behavioral bug
relative to the documented and tested-for design (`TestSummaryQueueRetryGivesUp` only
exercises this with a 2s `attemptTimeout`, which is why it doesn't catch the default-config
regression).

**Fix:** Either derive `summaryQueueMaxElapsed` from `attemptTimeout` at construction time
(e.g. `max(summaryQueueMaxElapsed, attemptTimeout*2)`), or lower the default
`ENGRAM_SUMMARY_TIMEOUT` for the on-write path specifically, or raise the hardcoded constant
comfortably above the configurable default (e.g. 90s) so "well under" is actually true for the
shipped configuration.

### WR-03: `tryEnqueue`'s `sync.WaitGroup.Add` is called on `inFlight` with no coordination against a concurrent `Wait()`

**File:** `internal/server/summaryqueue.go:123-139`, `254-263`

**Issue:** `sync.WaitGroup` documents that `Add` calls with a positive delta must not race
with a `Wait` call that could observe the counter reaching zero — i.e. new `Add(1)` calls are
only safe while the counter is known to be non-zero, or are required to happen-before any
`Wait()` that might return concurrently. `Wait()` here is exported and is currently called
only from tests in a safe pattern (enqueue a known batch, then `Wait()`, with no further
enqueues in flight). But `tryEnqueue` is reachable from arbitrary concurrent HTTP handlers at
any time, including the instant `inFlight`'s counter transiently hits zero between bursts of
writes. If `Wait()` is ever invoked from a future production code path (e.g. a health/readiness
probe, or a future admin endpoint) concurrently with live traffic, this is the textbook
WaitGroup misuse the stdlib docs warn about and can panic ("sync: WaitGroup misuse: Add called
concurrently with Wait") or produce a `Wait()` that returns while writes are still landing.
Not currently triggered by any exercised code path, but it's a latent trap given `Wait` is a
public method on a type whose `tryEnqueue` is also public within the package and called from
arbitrary request goroutines.

**Fix:** Document on `Wait()` that it must never be called concurrently with live
`tryEnqueue` traffic (test-only / quiescent-state contract), or replace the `inFlight`
WaitGroup with a pattern that tolerates concurrent `Add`+`Wait` (e.g. a counter + condition
variable, or simply not exposing `Wait()` outside `_test.go` files via a build-tag-guarded
test helper).

## Info

### IN-01: `storeMemory` and `scheduleMemory` duplicate the Upsert-then-tryEnqueue block verbatim

**File:** `internal/server/tools.go:562-582`, `594-621`

**Issue:** Both handlers repeat the identical `MintShortID` → `Upsert` →
"Enqueue only after a confirmed-successful Upsert..." comment → `tryEnqueue` sequence
verbatim. A future change to this sequencing (e.g. adding another post-write side effect)
requires remembering to edit both call sites in lockstep.

**Fix:** Extract a small shared helper, e.g. `d.persistAndEnqueue(ctx, m, vec) (string, string, error)`, called from both handlers.

### IN-02: `TestBuildDepsFromEnvLoadsConfigOnce` does not clear `ENGRAM_SUMMARY_MODEL`/`ENGRAM_SUMMARY_ON_WRITE` and never shuts down the returned deps

**File:** `internal/server/tools_test.go:1294-1319`

**Issue:** If a developer's shell (or CI runner) happens to have `ENGRAM_SUMMARY_MODEL` and
`ENGRAM_SUMMARY_ON_WRITE=true` set ambiently, `buildDepsFromEnv(nil)` in this test would
construct and start a real `summaryQueue` (2 worker goroutines blocked on `range ch` forever,
since nothing is ever enqueued) that is never shut down — a goroutine leak for the remainder
of the test binary's process lifetime. Every other test that builds a live queue
(`testDepsWithSummaryQueue`) registers a `t.Cleanup` that calls `Shutdown`; this one does not.

**Fix:** `t.Setenv("ENGRAM_SUMMARY_MODEL", "")` (and `ENGRAM_SUMMARY_ON_WRITE`, `""`) alongside
the other `t.Setenv` calls in this test to guarantee hermeticity, matching the pattern already
used in `config_test.go`'s `TestLoadDefaults`.

---

_Reviewed: 2026-07-10T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

## Resolution (2026-07-10)

Reviewed and dispositioned during `/gsd-execute-phase 11` (fix-now on branch):

- **CR-01 (blocker) — FIXED** in `c2075697`. Verified real: `net/http.Server.Shutdown`
  does not force-terminate handlers on ctx-deadline, so a slow store/schedule handler
  could `tryEnqueue` after `close(q.ch)`. Guarded the close with `sync.RWMutex` + `closed`
  flag (send and close now mutually exclusive; Shutdown idempotent); reserve the inFlight
  slot before the send. Added `-race` regression `TestSummaryQueueEnqueueAfterShutdownIsDroppedNotPanic`.
- **WR-02 (warning) — FIXED** in `40ac40ca`. Verified real: hardcoded `maxElapsed` (20s) < per-attempt
  `attemptTimeout` (30s) collapsed the 3-try budget. `maxElapsed` is now derived per-queue from
  `attemptTimeout` so it can never pre-empt `summaryQueueMaxTries`. Regression `TestSummaryQueueRetryBudgetAccommodatesFullTryCount`.
- **WR-01 — ACCEPTED (non-issue in practice).** Workers run under `context.Background()`, but
  `serve.go` returns (process exits) immediately after `drainSummaries`, so workers are reaped at
  exit; the per-attempt `context.WithTimeout` still bounds each fill. Not fixed to preserve the
  best-effort graceful-drain semantics (D-08).
- **WR-03 / IN-01 / IN-02 — DEFERRED** to a follow-up issue (latent test-only `Wait()` misuse hazard;
  minor storeMemory/scheduleMemory duplication; `TestBuildDepsFromEnvLoadsConfigOnce` env-hermeticity).
