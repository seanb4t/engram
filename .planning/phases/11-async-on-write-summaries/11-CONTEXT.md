# Phase 11: Async-on-Write Summaries - Context

**Gathered:** 2026-07-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire an **in-process worker pool** that fills summaries for summary-less records
*after* a successful `store_memory` / `schedule_memory` upsert — off the
synchronous write path, bounded, observable, and behind an explicit operator
opt-in. This phase is **wiring, not engine**: the reusable seam already shipped
in v1 (design doc `2026-06-25-auto-summary-curated-memories-design.md`, §"Core
fill operation"):

- `Store.FillSummary(ctx, m, summarize, model, maxChars)` — idempotent
  (no-op when `Summary != ""` or content ≤ cap), vector-preserving (writes via
  `SetPayload`, never re-embeds). `internal/store/summarize.go:75`.
- `internal/summarize.Client.Summarize` — OpenAI-compatible `/v1/chat/completions`
  call on the same gateway as embeddings. `internal/summarize/summarize.go:131`.
- `ENGRAM_SUMMARY_MODEL` / `_MAX_CHARS` / `_MAX_TOKENS` / `_TIMEOUT` config
  (`internal/config/registry.go:36-39`); `summarize-missing` CLI backstop.

**In scope:** the enqueue hook at the write handlers, the worker pool, its
lifecycle wiring in `serve.go`, the bounded queue + overflow policy, bounded
retry, OTLP observability, new config knobs, config `Validate()` rules, and
docs/CLAUDE.md updates for the new behavior + vars.

**Out of scope:** any change to how summaries are *generated* (the summarizer
client, prompt, fidelity) — that is v1 and unchanged; usage signals (Phase 12);
CI-gating the fidelity eval.

</domain>

<decisions>
## Implementation Decisions

### Enablement gate
- **D-01:** Add a **new `ENGRAM_SUMMARY_ON_WRITE` bool (default `false`)** config
  knob (koanf registry + `Validate()`), distinct from `ENGRAM_SUMMARY_MODEL`.
  The model var continues to enable the summarizer + `summarize-missing` CLI;
  the async worker only runs when **both** `ENGRAM_SUMMARY_MODEL` is set **and**
  `ENGRAM_SUMMARY_ON_WRITE=true`. Rationale: SC#3 gates *broad* enablement behind
  the fidelity eval, and engram's ethos is explicit/operator-invoked — so
  enabling the CLI backstop must not silently auto-summarize every write. Two
  deliberate operator steps: (1) set the model + run `task eval:summary` and
  judge fidelity, (2) flip `ON_WRITE`.
- **D-02:** The `task eval:summary` fidelity check is a **manual per-deployment
  operator judgment**, *not* a CI hard gate. (Resolves the design doc's §Open
  questions "CI gate or manual operator step" toward manual — avoids coupling
  merge/CI to a live cheap-model gateway that suffers brownouts.)

### Overflow / backpressure (queue full)
- **D-03:** The queue is **bounded** (SC#3). Enqueue is **non-blocking
  drop-and-count**: a `select { case q <- id: default: /* drop */ }`. On drop,
  increment an OTLP `dropped` counter (+ debug/warn log); the record stays
  summary-less until the next `summarize-missing` sweep reclaims it. The write
  path **returns unconditionally** — never blocks, never fails on a full queue
  (SC#2, hard). The existing idempotent CLI sweep is precisely what makes
  dropping safe.

### Failure & retry (per-record gateway failure — the 529 brownout case)
- **D-04:** On a transient `FillSummary` failure, the worker does **bounded
  in-worker retry with backoff**, then gives up → counts a `failed` metric →
  the record is reclaimed by the next `summarize-missing` sweep (the durable
  retry). Retry must be **short/bounded** so that under a *sustained* brownout
  the pool drains-to-failure fast rather than every worker stalling in backoff
  (a stalled pool just feeds D-03's drop path anyway).
- **D-05:** **Prefer an OSS retry/backoff library over hand-rolling.** Its
  per-attempt hook MUST be bridged to OTel (retry counter). Candidate libs to
  compare (seed ideas — **do NOT constrain the researcher**):
  `github.com/sethvargo/go-retry` (zero-dep, context-first),
  `github.com/cenkalti/backoff/v5` (has `Notify` hook),
  `github.com/avast/retry-go` (has `OnRetry` hook). Exact lib, attempt count,
  and backoff schedule are **research/planner decisions**.

### Enqueue scope
- **D-06:** Enqueue from **`store_memory` and `schedule_memory`** (both are plain
  curated memories through the same `Upsert` path with no guaranteed summary).
  **Exclude** `store_discovery` (discoveries carry client-authored summaries by
  design) and `store_rule` (rules REQUIRE a single-line client summary as their
  index). `FillSummary` idempotency is a safety net, not the reason to enqueue
  kinds that own their summaries.

### Worker & queue sizing
- **D-07:** Worker-pool size and queue depth are **operator-configurable via the
  koanf registry**: add `ENGRAM_SUMMARY_WORKERS` and `ENGRAM_SUMMARY_QUEUE_SIZE`
  with sensible defaults + `Validate()` rules (positive integers). Consistent
  with the existing `ENGRAM_SUMMARY_*` family and engram's "everything in the
  registry" convention. **Seed defaults** (research finalizes): ~2 workers, queue
  depth ~256 (matches the `SummarizeMissing` scroll batch). Defaults are seeds,
  not locked.

### Shutdown
- **D-08:** On SIGTERM, the pool does a **best-effort drain within the existing
  15s shutdown window**: stop accepting new ids (close the enqueue side), let
  workers finish their in-flight call under the shutdown context, drop whatever
  is still queued to the next `summarize-missing` sweep. Never hang shutdown.
  Safe because the seam is idempotent and the sweep is the backstop.

### Observability (required by SC#3; surface details are planner's)
- **D-09:** OTLP is **mandatory**: a **queue-depth gauge** and a **fill-latency
  histogram** are required by SC#3. Plus counters for enqueued / dropped (D-03) /
  failed / retried (D-04/D-05). Reuse the Phase-6 OTLP seam + existing
  `telemetry.RecordStoreOp` pattern; exact metric names/attributes are the
  planner's call.

### Claude's Discretion (carry to research/planner — not re-asking the user)
- `deps` (`internal/server/tools.go:33`) currently has no summarizer/queue field
  — it gains one (a bounded enqueue channel + the summarizer/model/maxChars it
  needs). The worker pool is **constructed and its lifecycle owned in
  `cmd/engram/serve.go`** (build summarizer from config → start pool → inject the
  enqueue handle into `deps` → drain in the shutdown branch near serve.go:195).
- The enqueue call sites are the two handler tails at
  `internal/server/tools.go:515` (`storeMemory`) and `:548` (`scheduleMemory`),
  fired **only after `Upsert` returns nil** and **only when async-on-write is
  enabled** (D-01). Enqueue takes the **record id** (per SC#1); the worker
  re-fetches / operates via the store, or the id + minimal memory is passed —
  planner's call, but keep the write path's post-return work to a bare channel
  send.
- Reuse `StoreAndSummarizerFromEnv` / `summarizerFromConfig`
  (`internal/server/tools.go:244`) construction rather than a parallel builder.
- New config vars land in `internal/config/registry.go` as `SummarizeConfig`
  fields (koanf-tagged) with `Validate()` rules; docs/CLAUDE.md
  memory-contract + a docs-site ops note describe the new knobs + the "no
  summary yet" degradation behavior.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Design source of truth (#320)
- `docs/superpowers/specs/2026-06-25-auto-summary-curated-memories-design.md` —
  the auto-summary design; §"Core fill operation (the reusable seam)" defines
  `FillSummary` properties, §5 "Auto fallback" documents the async-on-write
  **future seam** this phase now builds, §"Error handling / Summarizer failures"
  states the never-on-write-path invariant, §"Validation" describes the fidelity
  eval. GitHub issue **#320** is the tracked work item.
- `docs/superpowers/plans/2026-06-25-auto-summary-curated-memories.md` —
  companion implementation plan for the v1 seam.

### The seam this phase wires (read before planning)
- `internal/store/summarize.go` — `FillSummary` (`:75`), `SetSummary` (`:49`,
  vector-preserving `SetPayload`), `SummarizeMissing` sweep (`:97`, the
  backstop), `shouldSummarize` eligibility (`:43`).
- `internal/summarize/summarize.go` — `Client.Summarize` (`:131`), `New` (`:75`);
  `internal/summarize/fidelity_test.go` — `task eval:summary` harness (the D-02
  gate).
- `internal/server/tools.go` — `deps` struct (`:33`), `storeMemory` enqueue tail
  (`:515`), `scheduleMemory` tail (`:548`), `StoreAndSummarizerFromEnv` (`:244`).
- `cmd/engram/serve.go` — server construction + graceful-shutdown drain branch
  (`~:195`) where the worker lifecycle hangs; `cmd/engram/summarize.go` — how the
  CLI wires store + summarizer today.
- `internal/config/registry.go` (`:36-39`) — existing `ENGRAM_SUMMARY_*` family
  the new `ON_WRITE` / `WORKERS` / `QUEUE_SIZE` vars extend;
  `internal/config/validate_test.go` — the `summarizeEnabled()` validate pattern.

### Roadmap / requirements
- `.planning/ROADMAP.md` §"Phase 11: Async-on-Write Summaries" — goal + 3 success
  criteria.
- `.planning/REQUIREMENTS.md` — **REQ-async-summaries** (line ~100).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Store.FillSummary` / `Store.SetSummary` — the entire per-record engine; the
  worker is a thin caller. Idempotent + vector-preserving; no re-embed.
- `internal/summarize.Client` + `summarizerFromConfig` /
  `StoreAndSummarizerFromEnv` — summarizer construction from config, already used
  by the CLI. Reuse, don't reimplement.
- `Store.SummarizeMissing` — the offline sweep that is the durable backstop /
  retry for every drop (D-03) and give-up (D-04).
- Phase-6 OTLP seam + `telemetry.RecordStoreOp` — the pattern for D-09 metrics.

### Established Patterns
- **Presence-enables config** (`oidc.issuer`, `summarize.model`): D-01 adds an
  explicit *second* switch precisely to break that coupling for the write path.
- **Embed-first, then store** (`tools.go:508-515`): the summarizer stays strictly
  *after* a successful `Upsert`, mirroring the "on error we never touch the
  store" discipline — here "on summarizer trouble we never touch the response".
- **Injected function seam** (`SummarizeFunc`, `EmbedFunc`): the store never
  imports the summarizer package; the worker respects the same boundary.
- **Graceful shutdown via `signal.NotifyContext` + 15s `Shutdown` ctx**
  (`serve.go`): D-08 drain rides this existing window.

### Integration Points
- Write-path enqueue: `storeMemory`/`scheduleMemory` handler tails (post-`Upsert`).
- Worker lifecycle: `serve.go` (construct → start → inject into `deps` → drain).
- Config: `internal/config/registry.go` + `Validate()`.
- Observability: OTLP metrics/spans (Phase-6 seam).

</code_context>

<specifics>
## Specific Ideas

- Known operational reality driving D-04/D-05: qwen3-embedding/chat gateways
  throw **529 provider-overload brownouts** and engram's gateway client carries a
  **hardcoded 30s timeout** (see repo memory: embedder gateway papercuts). A
  worker that retries naively or blocks inherits these failure modes — hence
  bounded/short retry + non-blocking overflow + sweep backstop.
- User preference (verbatim intent): prefer an OSS retry/backoff lib over
  hand-rolling, wire its OTel metrics, and **do not over-constrain the
  researcher** on lib/attempt/backoff specifics — seed ideas only.

</specifics>

<deferred>
## Deferred Ideas

- **CI-gating the fidelity eval** (`task eval:summary` as a build gate) —
  explicitly rejected for this phase (D-02); revisit only if manual operator
  judgment proves insufficient. Not a new phase, just not now.
- **Re-summarization / summary refresh policy** beyond the existing stale-on-edit
  rule — out of scope (design doc Non-goals); belongs to a future curation phase.
- **Usage-signal-driven or lazy-at-first-recall summarization** — Phase 12 /
  design-doc Non-goals; must never couple to ranking.

None of the above expand this phase — discussion stayed within the async-on-write
wiring scope.

</deferred>

---

*Phase: 11-async-on-write-summaries*
*Context gathered: 2026-07-09*
