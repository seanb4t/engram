# Phase 11: Async-on-Write Summaries - Research

**Researched:** 2026-07-09
**Domain:** In-process async worker pool wiring around an existing Go seam (`Store.FillSummary`); OTel Go SDK instrumentation; OSS retry/backoff selection
**Confidence:** HIGH

## Summary

This phase is **wiring, not engine**. The summarization engine (`Store.FillSummary`,
`internal/summarize.Client`, the `summarize-missing` sweep) shipped in v1 and is
untouched. Phase 11 adds exactly one new thing: an in-process, bounded, non-blocking
worker pool that enqueues freshly-written record ids (from `store_memory` /
`schedule_memory`) and drains them through the already-idempotent, already-vector-preserving
`FillSummary` seam — off the synchronous write path, gated behind a new
`ENGRAM_SUMMARY_ON_WRITE` switch, and fully OTLP-observable.

Every integration point named in discuss-phase was verified against the actual
source and is accurate with one correction: `deps` (the struct that will carry
the enqueue channel) is **not** constructed in `serve.go` — it is built inside
`server.Register()` → `buildDepsFromEnv()` (`internal/server/tools.go:149`),
which `serve.go` calls once at `cmd/engram/serve.go:145`. `Register()` currently
returns only `error` and has exactly one caller in the whole repo (`serve.go`),
so its signature can safely grow a second return value (a shutdown/drain func)
without touching any other caller. That is the one seed that needed correcting
before planning.

For the retry library (D-04/D-05), `github.com/cenkalti/backoff/v5` is the clear
recommendation: it is **already an indirect dependency** of this exact module
graph (pulled in transitively by `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`'s
internal retry logic, confirmed via `go mod why`), so promoting it to a direct
import costs **zero** net new dependency-tree growth. Its v5 API
(`backoff.Retry[T](ctx, operation, opts...)`) is context-first, generic,
exposes `WithMaxTries`, `WithMaxElapsedTime`, and a `WithNotify(func(error,
time.Duration))` per-attempt hook that bridges directly to an OTel `retried`
counter — exactly the D-05 requirement, with no hand-rolling needed.

**Primary recommendation:** bounded buffered channel (`chan string`, seed size
256) + a fixed pool of N (seed 2) worker goroutines calling
`Store.FillSummary` through `cenkalti/backoff/v5`-wrapped retry; non-blocking
`select`/`default` enqueue at the two write-handler tails; pool lifecycle
started in `buildDepsFromEnv`/`Register` and drained via a `Register()`-returned
shutdown func invoked from `serve.go`'s existing 15s shutdown window; OTel
`Int64ObservableGauge` (queue depth, async callback over `len(chan)`) +
`Float64Histogram` (fill latency) + three `Int64Counter`s (enqueued/dropped/failed,
retried folded into the failed-attempt path via the backoff `Notify` hook).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Enqueue on write | API / Backend (MCP handler) | — | `storeMemory`/`scheduleMemory` are the only producers; enqueue is a side effect of a successful `Upsert`, not a new capability of its own |
| Worker pool / retry / drain | API / Backend (process-local) | — | Purely in-process Go goroutines inside the `engram` binary; no new service boundary |
| Summary generation (unchanged) | API / Backend → external gateway | — | `internal/summarize.Client` calls the same OpenAI-compatible `/v1/chat/completions` gateway used for embeddings; out of scope this phase |
| Persistence of filled summary | Database / Storage (Qdrant) | — | `SetSummary` → Qdrant `SetPayload`, vector-preserving, unchanged from v1 |
| Backstop / reclaim | API / Backend (CLI, offline) | — | `summarize-missing` sweep, unchanged; the durable retry for every drop/give-up |
| Observability | API / Backend (OTLP export) | — | New instruments registered on the existing Phase-6 meter provider; no new export path |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/cenkalti/backoff/v5` | v5.0.3 (latest) `[VERIFIED: go module proxy — go list -m -versions]` | Bounded exponential-backoff retry with jitter, context-first, per-attempt notify hook | Already an indirect dependency of this exact repo's `go.sum` (pulled in by `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`'s internal retry — `[VERIFIED: go mod why]`); promoting to direct costs zero new transitive deps. Widely used, actively maintained, Google-style exponential-backoff algorithm port. |
| `go.opentelemetry.io/otel/metric` | v1.44.0 (already vendored `[VERIFIED: go.mod]`) | Queue-depth gauge, fill-latency histogram, enqueued/dropped/failed counters | Already the project's sole metrics SDK (Phase 6 seam); no new dependency |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `sync` (`WaitGroup`) | go1.26.3 stdlib | Worker-pool lifecycle: wait for in-flight `FillSummary` calls to finish during drain | Standard idiom; the repo has no `errgroup` precedent (checked: `golang.org/x/sync` is indirect-only, not imported anywhere in `internal/`/`cmd/`) — stick with stdlib to match existing style |
| stdlib `context` | go1.26.3 stdlib | Cancel-on-shutdown for in-flight retry loops | `backoff.Retry` is context-first; the shutdown context from `serve.go`'s existing 15s window feeds it directly |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `cenkalti/backoff/v5` | `sethvargo/go-retry` (latest v0.3.0 `[VERIFIED: go module proxy]`) | Zero-dep, clean context-first `retry.Do(ctx, backoff, fn)` API with `retry.RetryableError` marking. **No built-in per-attempt notify hook** — you'd increment the OTel `retried` counter manually inside the retry closure instead of via a callback. Slightly more code, not a blocker, but backoff/v5's free-lunch (already-vendored) tips it. |
| `cenkalti/backoff/v5` | `avast/retry-go/v4` (v4.7.0, mature) or `/v5` (v5.0.0, brand-new — only one release so far) `[VERIFIED: go module proxy]` | Has `OnRetry(func(uint, error))` hook — comparable to backoff's `WithNotify`. v4 is battle-tested; v5 is a fresh major-version rewrite (`retry.New(...).Do(...)` builder style) with only one tagged release, higher adoption risk for a phase that must stay green. Either way it's a **new** dependency-tree branch (not already vendored), unlike backoff/v5. |
| Hand-rolled ~30-line backoff | — | Explicitly de-prioritized per D-05 ("prefer OSS lib"); backoff/v5's zero-net-dependency-cost removes the main argument for hand-rolling (dependency bloat) |

**Installation:**
```bash
go get github.com/cenkalti/backoff/v5@v5.0.3
```
This changes `go.mod`'s `cenkalti/backoff/v5` line from `// indirect` to a direct
require (no new line, no new entry in `go.sum` — the module is already resolved).

**Version verification:** `go list -m -versions github.com/cenkalti/backoff/v5`
→ `v5.0.0-alpha.1 v5.0.0 v5.0.1 v5.0.2 v5.0.3` (queried against the live Go
module proxy this session; v5.0.3 is latest and is the version already pinned
in this repo's `go.sum`, so **no version bump is needed** — only promotion from
indirect to direct).

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/cenkalti/backoff/v5` | Go module proxy | Long-established project (v4 line dates to ~2014; v5 is a 2024+ major rewrite) | High — pulled transitively by OpenTelemetry-Go's own OTLP exporters, one of the most widely vendored Go retry libs | github.com/cenkalti/backoff | OK | Approved — already resolved in `go.sum` as indirect; promoting to direct is a `go.mod` edit only, no new module fetch |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

*No new module needs to be fetched from the network for this phase — `cenkalti/backoff/v5` is already present in `go.sum` at the exact version recommended.* The two other seeded candidates (`sethvargo/go-retry`, `avast/retry-go`) were evaluated (see Alternatives Considered) but not selected, so they are not installed and do not need an audit row.

## Architecture Patterns

### System Architecture Diagram

```
                     store_memory / schedule_memory (MCP tool call)
                                    │
                                    ▼
                        d.storeMemory / d.scheduleMemory
                         (internal/server/tools.go)
                                    │
                     embed(content) ──► Qdrant Upsert
                                    │
                         [Upsert returns nil]
                                    │
                    ┌───────────────┴────────────────┐
                    │ if !ON_WRITE enabled: return    │
                    │ if ON_WRITE enabled:             │
                    │   select {                       │
                    │     case queue <- id:  (enqueued)│
                    │     default: (dropped, counted)  │
                    │   }                               │
                    │   return  ◄── ALWAYS unconditional│
                    └───────────────┬────────────────┘
                                    │ (id flows async)
                                    ▼
                     bounded chan string (cap 256, seed)
                                    │
              ┌─────────────────────┼─────────────────────┐
              ▼                     ▼                     ▼
         worker 1               worker 2              worker N (seed: 2)
              │                     │                     │
              └─────────┬───────────┴─────────┬───────────┘
                         ▼                     ▼
              backoff.Retry(ctx, func() {      OTel:
                st.FillSummary(id, ...)        - enqueued++ (on send)
              }, WithMaxTries, WithNotify)      - dropped++ (on default branch)
                         │                     - retried++ (via Notify hook)
              ┌──────────┴──────────┐          - failed++ (on give-up)
              ▼                     ▼          - fill-latency histogram
        success: SetPayload    give-up after
        (vector-preserving)    bounded retries
        summary_source=auto         │
                                     ▼
                          record stays summary-less;
                          reclaimed by next
                          `summarize-missing` sweep (backstop)

  Shutdown (SIGTERM):
    signal.NotifyContext → close(enqueue side) → workers finish in-flight
    FillSummary call under the shutdown ctx (existing 15s window) →
    WaitGroup.Wait (bounded by ctx) → remainder left queued, dropped to sweep.
```

### Recommended Project Structure

No new package is required — the worker pool is small enough to live as a new
file in an existing package. Two placement options, in order of preference:

```
internal/server/
├── tools.go              # existing: deps struct, storeMemory/scheduleMemory tails
├── summaryqueue.go        # NEW: worker pool type, Enqueue, Start, Shutdown
└── summaryqueue_test.go   # NEW: deterministic drain-based tests (see Validation Architecture)
```

Rationale: `deps` already owns summarizer construction
(`summarizerFromConfig`/`StoreAndSummarizerFromEnv`, `tools.go:231-256`); keeping
the queue type in the same package avoids a new internal package for ~150 lines
of code and keeps the enqueue call a same-package method call, not a cross-package
interface. (`internal/store` was considered and rejected: the store package is
explicitly kept ignorant of the summarizer via the injected `SummarizeFunc` seam
— see `internal/store/summarize.go:21-23` — and must stay that way; the worker
pool belongs at the same layer that already constructs `summarize.Client`.)

### Pattern 1: Bounded non-blocking producer (write-path enqueue)

**What:** The write handler tries a non-blocking channel send; on a full queue
it drops and counts instead of blocking.
**When to use:** Any producer that must never be slowed down by a downstream
consumer's backpressure — here SC#2 makes this a hard invariant.
**Example:**
```go
// Source: idiomatic Go non-blocking channel send (training-knowledge pattern,
// no library-specific API — confirmed against Go language spec select semantics)
func (q *summaryQueue) tryEnqueue(id string) {
	if q == nil { // disabled (ON_WRITE=false or model unset): nil queue, no-op
		return
	}
	select {
	case q.ch <- id:
		q.enqueued.Add(context.Background(), 1)
	default:
		q.dropped.Add(context.Background(), 1)
		slog.Warn("summary queue full; dropped, will be reclaimed by summarize-missing", "id", id)
	}
}
```
A `nil`-safe `*summaryQueue` (rather than an interface with a no-op
implementation) keeps `deps` simple: when `ENGRAM_SUMMARY_ON_WRITE=false` (the
default) or `ENGRAM_SUMMARY_MODEL==""`, `deps.summaryQueue` is `nil` and every
call site is a guarded no-op — no extra branching needed at the two call sites
beyond one nil check inside `tryEnqueue` itself.

### Pattern 2: Fixed worker pool draining a bounded channel

**What:** N long-lived goroutines `range` over the channel, each doing bounded
retry via `backoff.Retry`.
**When to use:** Bounded concurrency against a rate-limited/brownout-prone
external gateway (the summary chat-completions endpoint).
**Example:**
```go
// Source: cenkalti/backoff/v5 API confirmed via context7 (/cenkalti/backoff)
// and pkg.go.dev for v5.0.3 specifically (v7's docs differ slightly — verify
// against v5, not the newest major, since v5.0.3 is what's already vendored).
func (q *summaryQueue) worker(ctx context.Context) {
	defer q.wg.Done()
	for id := range q.ch {
		start := time.Now()
		_, err := backoff.Retry(ctx, func() (struct{}, error) {
			m, ferr := q.store.Get(ctx, id) // or pass the Memory through the channel directly (see Open Questions)
			if ferr != nil {
				return struct{}{}, backoff.Permanent(ferr) // record vanished/unauthorized: don't retry
			}
			_, ferr = q.store.FillSummary(ctx, m, q.summarize, q.model, q.maxChars)
			return struct{}{}, ferr // transient (gateway 529/timeout): retryable
		},
			backoff.WithMaxTries(3), // seed; bounded so a 529 brownout drains fast
			backoff.WithMaxElapsedTime(20*time.Second), // stay under the embedder's 30s hardcoded timeout budget
			backoff.WithNotify(func(err error, d time.Duration) {
				q.retried.Add(ctx, 1)
				slog.Debug("summary fill retrying", "id", id, "err", err, "next_in", d)
			}),
		)
		q.fillLatency.Record(ctx, time.Since(start).Seconds())
		if err != nil {
			q.failed.Add(ctx, 1)
			continue // give up; summarize-missing sweep reclaims it
		}
	}
}
```

### Pattern 3: Graceful, bounded, best-effort drain

**What:** On shutdown, stop accepting new work, let in-flight work finish under
a deadline, and abandon anything still queued.
**When to use:** Any background worker pool that must not block process exit
(D-08) but whose work is safely re-triable by a backstop (the sweep).
**Example:**
```go
// Source: idiomatic Go shutdown pattern (close-channel + WaitGroup + deadline
// context) — matches this repo's existing 15s shutdown-window idiom in
// cmd/engram/serve.go:207-209 (training-knowledge pattern, confirmed against
// stdlib context/sync semantics, not a library-specific API)
func (q *summaryQueue) Shutdown(ctx context.Context) {
	close(q.ch) // stop new enqueues from... wait: see note below
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done(): // bounded by the caller's 15s shutdown context
	}
}
```
**Important correction to the D-08 seed:** closing the channel that
`storeMemory`/`scheduleMemory` still send to is a **panic risk** ("send on
closed channel") if a request is mid-handler during shutdown. The safe pattern
is: (1) flip an atomic/context-based "accepting" flag that `tryEnqueue` checks
*before* closing the channel, OR (2) never close the producer-side channel at
all — instead cancel a shutdown context that `tryEnqueue` checks first (`select
{ case <-shutdownCtx.Done(): drop; default: select{send/drop} }`), and only
`close()` the channel after confirming no HTTP handler can still be in flight
(the HTTP server's own `Shutdown(ctx)` already guarantees this once it
returns — so the ordering must be: **`httpSrv.Shutdown(ctx)` completes first,
then `close(q.ch)`, then `wg.Wait()`**). This ordering detail is a planner-level
decision; flag it explicitly in the plan (see Common Pitfalls).

### Anti-Patterns to Avoid

- **Blocking enqueue (`ch <- id` without `select`/`default`):** silently
  reintroduces the exact hazard SC#2 forbids — a full queue would stall
  `store_memory` indefinitely. Always non-blocking.
- **Unbounded retry / infinite backoff loop:** under a sustained 529 brownout,
  every worker stalls in backoff, the queue backs up, and drops start
  cascading — exactly the failure mode D-04 calls out. Always cap
  `WithMaxTries` and `WithMaxElapsedTime`.
- **Re-embedding on summary fill:** `FillSummary`/`SetSummary` already uses
  `SetPayload` (never `Upsert` with a new vector) — a worker-pool
  implementation must call `FillSummary`, never hand-roll a path that
  re-embeds content.
- **Enqueuing from `storeDiscovery`/`storeRule`:** explicitly out of scope
  (D-06) — discoveries and rules own their summaries; do not wire enqueue
  calls there even though `FillSummary`'s idempotency would make it harmless.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exponential backoff with jitter + bounded attempts + cancellation | A custom `time.Sleep` retry loop | `cenkalti/backoff/v5` | Already vendored (zero net dep cost); correct jitter math, context-first cancellation, and a notify hook are easy to get subtly wrong by hand (e.g. thundering-herd jitter, off-by-one attempt counts) |
| Async gauge sampling | A goroutine that polls `len(ch)` on a ticker and pushes to a counter | `metric.WithInt64Callback` on an `Int64ObservableGauge` | The OTel SDK already provides pull-based (collection-cycle-driven) async instruments precisely for "read a live value on demand" — no extra goroutine, no drift between sample and export |
| Idempotent, vector-preserving summary fill | A new "fill" function in the worker package | `Store.FillSummary` (existing, `internal/store/summarize.go:75`) | This is the entire point of the phase — reuse the seam, don't reimplement eligibility/idempotency/vector-preservation logic that already has test coverage |

**Key insight:** every piece of "hard" logic in this phase (summarization,
idempotency, vector-preservation, sweep backstop) already exists and is
tested. The only genuinely new logic is queue/pool/retry/observability
plumbing, and even the trickiest part of that (backoff) has a free, already-vendored
library. Resist the urge to add abstraction beyond what's needed to wire these
existing pieces together.

## Common Pitfalls

### Pitfall 1: Shutdown ordering panics on "send on closed channel"

**What goes wrong:** Closing the enqueue channel while an HTTP handler is still
mid-flight (post-`Upsert`, about to call `tryEnqueue`) causes a panic.
**Why it happens:** `signal.NotifyContext` fires as soon as SIGTERM arrives,
but in-flight HTTP requests are still running until `httpSrv.Shutdown(ctx)`
completes (which itself waits for active connections to finish, up to the same
context's deadline).
**How to avoid:** Order shutdown strictly as: (1) `httpSrv.Shutdown(shutdownCtx)`
first (this is `serve.go`'s existing behavior at `serve.go:209`), (2) *then*
close the summary-queue enqueue channel and drain workers within whatever
budget remains of the 15s window. Do not parallelize these two shutdown steps.
**Warning signs:** flaky `panic: send on closed channel` under load-testing a
SIGTERM during active traffic; a `-race` test that stores a record and cancels
context concurrently.

### Pitfall 2: `backoff.Retry`'s generic signature needs an explicit type param when the closure returns nothing useful

**What goes wrong:** `backoff.Retry[T any](ctx, operation Operation[T], ...)`
requires `operation` to return `(T, error)`; a naive `func() error { ... }`
does not satisfy `Operation[T]` and fails to compile.
**Why it happens:** v5's generic API is a deliberate change from v4's
non-generic `RetryNotify(operation func() error, ...)`.
**How to avoid:** Use `backoff.Retry(ctx, func() (struct{}, error) { ...; return struct{}{}, err }, opts...)`
(as in Pattern 2 above), or wrap with a small non-generic helper if the
repeated `struct{}{}` return feels noisy across call sites.
**Warning signs:** compile error "cannot use func literal (type func() error)
as type backoff.Operation[T]".

### Pitfall 3: Retry budget exceeding the embedder's hardcoded 30s timeout

**What goes wrong:** If `WithMaxElapsedTime` (or attempts × per-attempt
timeout) exceeds ~30s, a single stuck worker can hold up pool throughput for
uncomfortably long during a brownout, and — per the repo's known embedder
papercut — the summarizer client itself already carries a 30s HTTP timeout
(`summaryTimeout`, default 30s, `tools.go:194-207`) per attempt. Multiple
retries at 30s each compounds badly.
**Why it happens:** The summarizer's own per-request timeout and the retry
wrapper's elapsed-time budget are two independent knobs that must be reasoned
about together.
**How to avoid:** Keep `WithMaxElapsedTime` well under the summarizer's
per-request timeout × max-tries product — e.g. 3 tries at a short per-attempt
budget, or explicitly document that a single stuck HTTP call under the
existing 30s summarizer timeout already consumes most of the retry budget
in one attempt. Seed: `WithMaxTries(3)` with the existing 30s
`summarizerFromConfig` timeout per call is likely sufficient without adding a
second elapsed-time ceiling that's hard to reason about — this is a
**planner-level tuning decision**, not resolved here.
**Warning signs:** a single 529-brownout period visibly stalls all N workers
for multiples of 30s, causing queue depth to spike and drops to cascade.

### Pitfall 4: Passing only the record id through the channel forces an extra Qdrant `Get` per worker iteration

**What goes wrong:** `FillSummary` takes a `store.Memory`, not just an id.
If the channel only carries `id string` (as SC#1 literally specifies:
"the record id is enqueued"), each worker must re-fetch the full record
before calling `FillSummary` — an extra round-trip per item, and a
window where the record could have been deleted/updated between enqueue and
fetch (`shouldSummarize` will correctly skip an already-summarized record, so
this is safe, just an extra call).
**Why it happens:** SC#1's exact wording ("the record id is enqueued") is a
hard requirement, but the cheapest implementation (pass the full `Memory`
through the channel) would violate it.
**How to avoid:** Accept the extra `Get`-then-`FillSummary` round trip — it is
cheap relative to the summarization LLM call, and it's the only way to honor
SC#1 literally while staying correct if the record changed between enqueue and
processing (re-fetching is what makes the async worker safe against a
concurrent `update_memory` race). Document this call sequence explicitly in
the plan.
**Warning signs:** a plan that puts a full `store.Memory` on the channel
without addressing SC#1's literal wording — this should be caught in plan
review.

### Pitfall 5: `Register()`'s signature change ripples to nothing — but verify before assuming

**What goes wrong:** Assuming `server.Register()` has multiple callers (tests,
alternate entrypoints) and therefore avoiding the signature change, leading to
an awkward package-level global or a second exported function just to expose
shutdown.
**Why it happens:** Without grepping, it's easy to over-conservatively avoid
touching a public API.
**How to avoid:** Verified this session: `grep -rln "server\.Register("` across
the whole repo returns exactly one file, `cmd/engram/serve.go`. Changing
`Register`'s signature to `func Register(...) (shutdown func(context.Context), err error)`
(or similar) is safe and is the cleanest way to satisfy D-08's drain
requirement without new global state.
**Warning signs:** N/A — this is a "don't over-engineer around a
non-constraint" pitfall, confirmed clear.

## Code Examples

### Non-blocking enqueue call site (storeMemory tail)

```go
// Source: internal/server/tools.go:502-516 (verified current source, this session)
func (d *deps) storeMemory(ctx context.Context, a storeArgs) (string, string, error) {
	subj, err := subjectFromContext(ctx)
	if err != nil {
		return "", "", err
	}
	m := a.toMemory(subj.Owner(), actorFromContext(ctx), d.clock())
	vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
	if err != nil {
		return "", "", err // embed first: on error we never touch the store
	}
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID) // NEW — nil-safe no-op when disabled; never blocks, never errors
	return m.ID, m.ShortID, nil
}
```
The existing single-`return d.st.Upsert(...)` line must be split into an
explicit `if err := ...` so the enqueue only fires on a confirmed-successful
Upsert — this is a small but real behavior-preserving refactor of the current
one-liner at `tools.go:515`.

### OTel queue-depth gauge (async callback)

```go
// Source: go.opentelemetry.io/otel/metric API confirmed via context7
// (/open-telemetry/opentelemetry-go), pattern matches this repo's existing
// telemetry.NewToolMetrics (internal/telemetry/metrics.go:25-31)
depthGauge, _ := m.Int64ObservableGauge("engram.summary_queue.depth",
	metric.WithUnit("{item}"),
	metric.WithDescription("current summary-on-write queue depth"),
	metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(int64(len(q.ch)))
		return nil
	}),
)
```

### Fill-latency histogram + counters (mirrors `telemetry.ToolMetrics`)

```go
// Source: pattern mirrors internal/telemetry/metrics.go:25-31 (this repo,
// verified this session) — new instruments should live alongside ToolMetrics/
// layerMetrics in internal/telemetry, following the exact same construction
// idiom (instrument-creation errors ignored; nil instrument from the no-op
// provider still records safely).
type SummaryQueueMetrics struct {
	depth    metric.Int64ObservableGauge
	fillDur  metric.Float64Histogram
	enqueued metric.Int64Counter
	dropped  metric.Int64Counter
	failed   metric.Int64Counter
	retried  metric.Int64Counter
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Offline-only `summarize-missing` sweep is the sole path to a filled summary | Async-on-write enqueue fills most records within seconds of write; the sweep becomes a pure backstop for drops/failures | This phase | New records surface with real summaries almost immediately instead of waiting for an operator-run sweep |
| `cenkalti/backoff` v4 non-generic `RetryNotify(func() error, ...)` | v5 generic `Retry[T](ctx, Operation[T], ...)` | v5.0.0 (per module proxy version history) | Any new code should target v5's context-first API, not v4's (v4 is only present here as *another* library's transitive dep, not for direct use) |

**Deprecated/outdated:**
- None directly relevant — this is new wiring, not a migration.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Seed queue depth 256 / worker count 2 are reasonable starting defaults for a single-process MCP server's write volume | Standard Stack / Worker & queue sizing (D-07) | Low — both are operator-configurable via new env vars per D-07; if wrong, an operator can retune without a code change |
| A2 | `WithMaxTries(3)` + the existing 30s per-request summarizer timeout is an adequate retry budget for the 529-brownout case without adding a separate `WithMaxElapsedTime` ceiling | Common Pitfalls #3 | Medium — if too generous, a brownout could stall a worker for minutes; the planner should treat exact attempt count / elapsed-time ceiling as a tunable to verify against real brownout behavior (repo memory: qwen3-embedding-8b 529s) |
| A3 | Passing only the record id through the channel (per SC#1's literal wording) and re-fetching via `Get` inside the worker is preferred over passing the full `Memory` | Common Pitfalls #4 | Low — safe either way; re-fetching is strictly more correct against a concurrent update, at the cost of one extra Qdrant round-trip per item |
| A4 | The new worker-pool code should live in `internal/server/summaryqueue.go` rather than a new `internal/summaryqueue` package | Recommended Project Structure | Low — a structural/style call; either works, no functional risk |

## Open Questions (RESOLVED during planning)

> Both questions below were closed by the finalized plans (2026-07-09). Retained for provenance.

1. **Exact `WithMaxTries` / backoff interval defaults for D-04's bounded retry**
   - What we know: must be short/bounded so a sustained brownout drains-to-failure
     fast (D-04); backoff/v5's `ExponentialBackOff` defaults are `InitialInterval=500ms`,
     `Multiplier=1.5`, `MaxInterval=60s`, `RandomizationFactor=0.5` — the
     `MaxInterval` default is almost certainly too generous for this use case.
   - What's unclear: the precise numbers (3 tries? 2? what interval ceiling?)
     that keep total worst-case per-item time comfortably below the pool
     stalling under load.
   - Recommendation: planner sets an explicit low `MaxInterval` (e.g. 2-5s) and
     low `WithMaxTries` (e.g. 3), rather than relying on library defaults tuned
     for longer-running network operations; verify with a forced-failure test
     (see Validation Architecture) that N failing items don't stall the pool
     past a few seconds each.
   - **RESOLVED (Plan 11-02):** the planner locked an explicit low `WithMaxTries`
     + low `MaxInterval` (not library defaults), verified by
     `TestSummaryQueueRetryGivesUp` bounding worst-case wall time (T-11-03).

2. **Where do the new OTel instruments live — a new `SummaryQueueMetrics` type in `internal/telemetry`, or fields on the queue struct in `internal/server`?**
   - What we know: `telemetry.NewToolMetrics` / `telemetry.InitLayerMetrics`
     establish the existing pattern of instrument ownership in the `telemetry`
     package, constructed once in `serve.go` from the shared meter.
   - What's unclear: whether the queue struct in `internal/server` should hold
     instrument handles directly (simpler, fewer files) or go through a
     `telemetry.SummaryQueueMetrics` type for consistency with `ToolMetrics`.
   - Recommendation: follow the existing `ToolMetrics` pattern for consistency
     (construct in `serve.go`, pass into `Register`) — this is a style
     consistency call for the planner, not a functional risk either way.
   - **RESOLVED (Plan 11-01):** instruments live in a `telemetry.SummaryQueueMetrics`
     type constructed from the shared meter, mirroring `ToolMetrics` (D-09).

## Environment Availability

Skipped — this phase has no new external service/tool dependency beyond what's
already required for the shipped v1 summarizer (the OpenAI-compatible gateway,
already probed and gated by `ENGRAM_SUMMARY_MODEL`/`Validate()`). No new
runtime, CLI, or service is introduced; `cenkalti/backoff/v5` is a pure Go
library dependency already resolved in `go.sum`.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib testing), `task test:go` → `go test ./...` |
| Config file | none — Taskfile.yaml targets (`test`, `test:go`, `test:short`, `eval:summary`) |
| Quick run command | `go test ./internal/server/... -run TestSummaryQueue -v` |
| Full suite command | `task test` (Go + Python hook tests) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-async-summaries (SC#1) | Successful `Upsert` enqueues id; worker drains via `FillSummary` | unit/integration | `go test ./internal/server/... -run TestStoreMemoryEnqueuesOnSuccess -v` | ❌ Wave 0 |
| REQ-async-summaries (SC#2) | Write path returns unconditionally even when summarizer is forced to fail/hang; full queue drops without blocking | unit | `go test ./internal/server/... -run TestSummaryQueueNeverBlocksWrite -v` | ❌ Wave 0 |
| REQ-async-summaries (SC#2) | Drop-and-count fires under queue saturation, `dropped` counter increments | unit | `go test ./internal/server/... -run TestSummaryQueueDropsWhenFull -v` | ❌ Wave 0 |
| REQ-async-summaries (SC#1) | `FillSummary` idempotency/vector-preservation still holds after async fill (no re-embed) | integration (Qdrant testcontainer, existing pattern) | `go test ./internal/store/... -run TestFillSummary -v` (existing coverage — confirm it still passes; likely no new test needed here since `FillSummary` itself is unchanged) | ✅ (existing) |
| REQ-async-summaries (D-08) | Shutdown drains best-effort within budget; leaves remainder for the sweep; never hangs shutdown | unit | `go test ./internal/server/... -run TestSummaryQueueShutdownDrainsWithinBudget -v` | ❌ Wave 0 |
| REQ-async-summaries (D-06) | `store_discovery`/`store_rule` never enqueue | unit | `go test ./internal/server/... -run TestDiscoveryAndRuleNeverEnqueue -v` | ❌ Wave 0 |
| REQ-async-summaries (D-01) | Worker only runs when both `ENGRAM_SUMMARY_ON_WRITE=true` AND `ENGRAM_SUMMARY_MODEL` set | unit (config) | `go test ./internal/config/... -run TestSummaryOnWrite -v` | ❌ Wave 0 |
| REQ-async-summaries (D-04/D-05) | Bounded retry gives up after N tries and counts `failed`, never blocks pool indefinitely | unit | `go test ./internal/server/... -run TestSummaryQueueRetryGivesUp -v` | ❌ Wave 0 |
| task eval:summary (D-02) | Fidelity of the cheap model's summaries | manual, NOT CI | `ENGRAM_SUMMARY_EVAL=1 go test ./internal/summarize/ -run TestSummaryFidelity -v` | ✅ (existing, unchanged — manual operator gate per D-02, not a new automated test) |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/... ./internal/config/... -run TestSummaryQueue|TestStoreMemoryEnqueues|TestDiscoveryAndRuleNeverEnqueue|TestSummaryOnWrite -v`
- **Per wave merge:** `task test:go` (full `go test ./...`)
- **Phase gate:** `task` (lint + test) green before `/gsd-verify-work`. `task eval:summary` is run manually by the operator per D-02 — it is documented but not part of the automated gate.

### Wave 0 Gaps

- [ ] `internal/server/summaryqueue.go` — the worker pool type itself (production code, not a test, but the deterministic test seam depends on its shape)
- [ ] `internal/server/summaryqueue_test.go` — needs a **test seam to await queue drain deterministically**: an exported (or test-only, same-package) `q.testDrain(t)` / `q.Wait()` helper that blocks until the channel is empty AND all in-flight `FillSummary` calls have returned, so tests never rely on `time.Sleep` polling. Recommend a `sync.WaitGroup`-based `Idle()` method the test can call after enqueueing a known set of ids, OR inject a completion-signal channel the test reads from (mirrors how `d.now` is injected for deterministic clock control in existing tests — `tools.go:41-44`).
- [ ] A fake/injectable `SummarizeFunc` that can be told to hang (for the "write path returns even when summarizer hangs" test) or fail N times then succeed (for the retry-gives-up test) — this can reuse the exact pattern already used in `internal/store/summarize_test.go` (closures implementing `store.SummarizeFunc`), just wired one layer up in `internal/server`.
- [ ] `testDeps(t)` (existing helper, `internal/server/connectapi_test.go` and friends) needs a variant or optional field to inject a `*summaryQueue` with test-controlled worker count/queue size/summarizer, without breaking every existing `testDeps(t)` call site (default: `nil` queue, i.e. disabled — matches production default `ENGRAM_SUMMARY_ON_WRITE=false`).
- [ ] Framework install: none — `go test` and the existing Qdrant-testcontainer harness (`internal/store/store_test.go`) already cover everything needed; no new test framework required.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Unaffected — enqueue happens strictly after the existing authenticated `Upsert` succeeds; no new auth surface |
| V3 Session Management | no | N/A — in-process background workers, no session concept |
| V4 Access Control | no | The worker re-fetches records via the store's existing `Get`, which is not owner-scoped internally at the store layer (store-layer fetch by id is used the same way the existing `summarize-missing` sweep already does it — see `internal/store/summarize.go:137`, `fromPayload` with no owner filter in the scroll). No new access-control surface is introduced beyond what `SummarizeMissing` already does today. |
| V5 Input Validation | yes | New config vars (`ENGRAM_SUMMARY_ON_WRITE`, `ENGRAM_SUMMARY_WORKERS`, `ENGRAM_SUMMARY_QUEUE_SIZE`) validated in `Config.Validate()` following the existing `if c.Summarize.Model != ""` gated-validation pattern (`internal/config/validate.go:84-104`) — positive-integer checks for workers/queue size |
| V6 Cryptography | no | No new cryptographic material; reuses the existing OpenAI-compatible gateway auth (`ENGRAM_OPENAI_API_KEY`) unchanged |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unbounded queue growth under sustained write load (memory exhaustion / DoS) | Denial of Service | Bounded channel (D-03) with non-blocking drop-and-count — already the design; the queue cannot grow past its configured capacity |
| Worker pool stalls holding goroutines/connections open indefinitely during a gateway outage | Denial of Service | Bounded retry (D-04) + per-request summarizer timeout (existing, 30s default) + `WithMaxElapsedTime`/`WithMaxTries` ceiling |
| Prompt injection via record content flowing into the summarizer's chat-completion call | Tampering (of model output) | Already mitigated in v1 — `internal/summarize/summarize.go`'s `newFenceToken`/tokenized-fence containment control (k1oe.1) wraps record content as opaque data; unchanged by this phase since the async worker calls the exact same `Summarize`/`FillSummary` path |
| A dropped/failed record silently never gets a summary (availability of a *feature*, not a security issue per se, but a correctness-adjacent risk) | — | The `summarize-missing` sweep is the explicit backstop (D-03/D-04); OTLP `dropped`/`failed` counters make this observable, not silent |

## Sources

### Primary (HIGH confidence)
- `/cenkalti/backoff` (context7) — v7 usage-pattern docs for `Retry`, `WithNotify`, `WithMaxTries`, `ExponentialBackOff`/jitter; cross-checked against pkg.go.dev for the exact v5.0.3 API (v5 and v7 share the same `Retry[T](ctx, Operation[T], opts...)` shape; v5.0.3 confirmed via WebFetch of pkg.go.dev)
- `/sethvargo/go-retry` (context7) — `NewExponential`/`NewFibonacci`, `WithJitter`, `WithMaxRetries`, `retry.Do`/`RetryableError` API
- `/avast/retry-go` (context7) — `retry.OnRetry`, `retry.Context`, `FullJitterBackoffDelay`, `retry.New`/`.Do()` (v5 builder API) vs classic functional-options API
- `/open-telemetry/opentelemetry-go` (context7) — `Int64ObservableGauge` + `WithInt64Callback`, `Float64ObservableGauge`, async-instrument callback registration semantics
- Go module proxy (`go list -m -versions`) — authoritative version lists for `cenkalti/backoff/v5`, `sethvargo/go-retry`, `avast/retry-go/v4`, `avast/retry-go/v5`
- `go mod why github.com/cenkalti/backoff/v5` — confirms the existing indirect-dependency path via `otel/exporters/otlp/otlplog/otlploggrpc`
- Direct codebase reads this session: `internal/store/summarize.go`, `internal/summarize/summarize.go`, `internal/server/tools.go` (deps struct, storeMemory/scheduleMemory, Register), `cmd/engram/serve.go`, `cmd/engram/summarize.go`, `internal/config/{config,registry,validate}.go`, `internal/telemetry/metrics.go`, `internal/store/summarize_test.go`, `internal/store/store_test.go` — all line numbers cited above were verified against current source, not assumed from CONTEXT.md seeds

### Secondary (MEDIUM confidence)
- None — all load-bearing claims this phase were verified against either official docs (context7) or this repo's own source/module graph.

### Tertiary (LOW confidence)
- Seed defaults for worker count (2) / queue size (256) / retry attempts (3) — carried from discuss-phase seeds and reasoned about qualitatively, not empirically load-tested; flagged in the Assumptions Log

## Metadata

**Confidence breakdown:**
- Standard stack (retry library choice): HIGH — verified via module proxy + context7 + `go mod why`, with a concrete zero-cost dependency argument
- Architecture (wiring points): HIGH — every file:line claim re-verified against current source this session; one CONTEXT.md seed corrected (deps construction site)
- Pitfalls: HIGH — shutdown-ordering and generic-signature pitfalls are derived directly from reading the actual `serve.go` shutdown sequence and the verified v5 API signature, not speculation
- Validation architecture: MEDIUM — the test seam design (drain helper) is a reasonable extrapolation from this repo's existing deterministic-clock injection pattern (`d.now`), but the exact helper shape is a planner-level design decision, not verified against an existing precedent in this repo (no prior async-worker test in this codebase to point to)

**Research date:** 2026-07-09
**Valid until:** 30 days (stable Go stdlib + long-lived OTel/backoff APIs; re-verify `cenkalti/backoff` version if this research is reused after a `go.mod` dependency bump)
