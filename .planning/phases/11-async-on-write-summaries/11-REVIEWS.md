---
phase: 11
reviewers: [codex]
reviewed_at: 2026-07-10T03:59:36Z
plans_reviewed: [11-01-PLAN.md, 11-02-PLAN.md, 11-03-PLAN.md]
model: codex-cli 0.143.0 (default model)
overall_risk: MEDIUM
---

# Cross-AI Plan Review — Phase 11

## Codex Review

## Summary

The three-wave plan is broadly sound and matches the current repo shape: config is centralized in `internal/config`, deps are built in `buildDepsFromEnv`, `store_memory` / `schedule_memory` are the right post-`Upsert` hooks, `Register` has a single serve-time caller, and `backoff/v5` is already present as an indirect dependency. I would not call this ready as-is, though: there is one compile/design ambiguity around `SummaryQueueMetrics` + queue-depth callback ownership, and one operational risk where “short bounded retry” is weaker than the plan implies because the summarizer request can still consume the existing 30s timeout before backoff limits are evaluated.

## Strengths

- The wave ordering is mostly clean: config/telemetry/dependency foundation first, queue core second, server wiring/docs last. Current anchors support that split: summary config lives at `internal/config/registry.go:36`, `SummarizeConfig` at `internal/config/config.go:82`, and telemetry follows constructor/helper patterns at `internal/telemetry/metrics.go:14`.
- The selected enqueue points are correct. `storeMemory` and `scheduleMemory` currently return `d.st.Upsert(...)` directly at `internal/server/tools.go:515` and `internal/server/tools.go:548`, so splitting those tails to enqueue only after a nil `Upsert` is the right mechanism.
- D-06 is well-scoped. Discovery and rule writes have their own summary contract and are separate code paths at `internal/server/tools.go:551` and `internal/server/rules.go:92`; keeping enqueue out of those handlers is correct.
- The dependency claim is verified. `github.com/cenkalti/backoff/v5 v5.0.3` is already indirect in `go.mod:60`, checksummed in `go.sum:59`, and `go mod why` resolves it through the OTLP log exporter retry package.
- Shutdown ordering is correctly identified. Current serve shutdown is a single `httpSrv.Shutdown(shutdownCtx)` at `cmd/engram/serve.go:205`; the plan’s “HTTP shutdown first, queue close/drain second” avoids the send-on-closed-channel race.

## Concerns

- **HIGH: Metrics/queue construction is internally inconsistent across 11-01 and 11-03.** Plan 11-01 says `telemetry.NewSummaryQueueMetrics` takes a depth callback, but Plan 11-03 says `serve.go` constructs `sqm` before `Register`, then `buildDepsFromEnv` wires the gauge against `q.depth()` later. Current code constructs `tm` in `serve.go` before registration at `cmd/engram/serve.go:93`, and `Register` currently calls zero-arg `buildDepsFromEnv()` at `internal/server/tools.go:914`. As written, the plan does not define a compile-clear ownership model for the queue-depth observable gauge.
- **HIGH: Retry bounds do not bound a hung summarizer attempt.** The existing summarizer timeout defaults to 30s at `internal/server/tools.go:194`. `backoff.Retry` evaluates elapsed time only after `operation()` returns, then before sleeping; the relevant checks are after the operation call in the v5 source. A low `WithMaxElapsedTime` therefore will not interrupt a single stuck `/v1/chat/completions` attempt. This weakens the D-04 “drains-to-failure fast” claim under brownout/hang conditions.
- **MEDIUM: The new async egress path lacks an explicit audit/logging requirement.** `FillSummary` itself summarizes and writes payload without egress audit logging at `internal/store/summarize.go:75`. The existing sweep logs per-record egress outcome, owner, scope, content length, and model around `FillSummary` at `internal/store/summarize.go:150`. The worker creates a second path to the summarizer gateway, so the plan should require equivalent content-free audit logging or move that audit into a shared helper.
- **MEDIUM: Worker panic behavior is not specified.** The queue plan uses worker goroutines and a deterministic in-flight `Wait` seam, but it does not require `defer` cleanup around per-item accounting or panic recovery. A panic from a store/client edge could kill a worker and potentially leave the test drain waitgroup unbalanced. This is especially relevant because the plan’s own tests rely on deterministic `Wait()` rather than sleeping.
- **LOW: Config tests need direct-struct defaults updated.** Production config defaults come from the registry, but `validConfig()` in `internal/config/validate_test.go:13` builds a `Config` manually and currently leaves `Summarize` empty. If `OnWrite`, `Workers`, and `QueueSize` are validated unconditionally, tests must set their default strings there too, not only rely on `config.Load`.

## Suggestions

- Pick one telemetry construction model before execution:
  - `Register` receives the meter, builds the queue, then constructs `SummaryQueueMetrics(meter, q.depth)`, or
  - `NewSummaryQueueMetrics` does not own the depth gauge; expose a separate `RegisterSummaryQueueDepth(meter, depth func() int64)`, or
  - use a nil-safe callback closure whose queue pointer is assigned during registration.

  The current hybrid should be tightened in 11-01 and 11-03 acceptance criteria.
- Add a queue-level per-attempt timeout or explicitly document that `ENGRAM_SUMMARY_TIMEOUT` is the hard operational bound. For the phase goal, I’d prefer `context.WithTimeout` around each `fill` attempt using a small internal constant, or a derived cap from `summaryTimeout`, then test a hung fill path.
- Require audit logging in the worker path with the same no-content posture as `SummarizeMissing`: id, scope, visibility, owner, content length, model, outcome, and error string only.
- Make the queue implementation require `defer itemDone()` around every accepted id, plus worker-level `recover` that increments `failed` and keeps the pool alive or deliberately terminates with balanced accounting.
- Add acceptance checks for `backoff.WithBackOff(...)` with an explicit `ExponentialBackOff{MaxInterval: ...}`. Checking only `WithMaxTries` and `WithNotify` would still allow v5’s 60s max interval / 15m elapsed defaults.

## Risk Assessment

**Overall risk: MEDIUM.** The plan is well aimed and should achieve the phase goal after tightening the telemetry ownership and retry timeout semantics. The write-path safety invariant is well protected by the planned non-blocking send, and the source confirms the wiring points are real. The main residual risks are operational rather than architectural: queue observability could be awkward or fail to compile without a clearer metrics design, and sustained gateway hangs can still stall workers longer than the plan’s “bounded retry” language suggests.

---

## Consensus Summary

Single-reviewer pass (Codex, source-grounded against the live working tree). Codex **confirmed** the plan's five load-bearing claims against real source — the `backoff/v5` indirect-dependency claim (`go.mod:60`), the enqueue points (`tools.go:515`/`:548`), the single `Register` caller, the shutdown anchor (`serve.go:205`), and the D-06 exclusion of discovery/rule — so the structural spine of the plan is validated, not just restated. It did **not** find any defect in the SC#2 write-path safety invariant (the non-blocking send is well protected).

### Agreed Strengths

- Wave ordering (config/telemetry foundation → queue core → server wiring) matches the real file coupling.
- Enqueue mechanism (split the `Upsert` tail, enqueue only on nil error) and D-06 scope (discovery/rule excluded) are correct against source.
- The `backoff/v5` zero-cost promotion and the shutdown-ordering constraint are both verified true.

### Agreed Concerns (priority for `--reviews` replan)

1. **[HIGH] Queue-depth gauge ownership is ambiguous/inconsistent between 11-01 and 11-03** — no compile-clear model for who constructs `SummaryQueueMetrics` vs. who wires the `q.depth()` observable callback. This is the same instrument-ownership question RESEARCH.md marked "resolved"; Codex shows the resolution is under-specified across the two plans. Pick one construction model and pin it in both plans' acceptance criteria.
2. **[HIGH] Bounded retry does not bound a *hung* attempt** — `backoff.Retry` only checks elapsed time *after* `operation()` returns, so a single stuck `/v1/chat/completions` call still burns the full 30s summarizer timeout (`tools.go:194`) before any backoff limit applies. Under the known qwen3 529-brownout this weakens the D-04 "drains-to-failure fast" claim. Wrap each `fill` attempt in a `context.WithTimeout` (small internal cap or derived from `summaryTimeout`) and add a hung-fill test.
3. **[MEDIUM] No egress audit logging on the new worker path** — the existing sweep logs content-free egress metadata around `FillSummary` (`summarize.go:150`); the async worker is a second gateway path and should log the same (id/scope/owner/visibility/content-length/model/outcome — no content), ideally via a shared helper.
4. **[MEDIUM] Worker panic recovery unspecified** — a panic could leave the drain `WaitGroup` unbalanced and break the deterministic `Wait()` seam the tests depend on. Require `defer itemDone()` + worker-level `recover` that increments `failed`.
5. **[LOW] `validConfig()` direct-struct test defaults** — if the new `Summarize` fields are validated unconditionally, `validate_test.go:13` must set their default strings.

### Divergent Views

None — single reviewer.
