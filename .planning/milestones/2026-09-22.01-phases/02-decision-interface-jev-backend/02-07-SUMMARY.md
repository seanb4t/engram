---
phase: 02-decision-interface-jev-backend
plan: 07
subsystem: decide
tags: [decide, jev, error-classification, retry, otel, telemetry, httpdrain]

requires:
  - phase: 02-03
    provides: "internal/decide contract, errors.go D-12/D-09 sentinels, decide.Error/decide.Status"
  - phase: 02-04
    provides: "jev.Decide's validate-before-network path, DecideMany bounded worker pool"
provides:
  - "classifyStatus/classifyTransport/isRetryable: HTTP-status-only D-12 classification for both OpenRouter (numeric code) and LiteLLM (string code) error dialects, with structural (never substring-matched) max_tokens_exceeded detection"
  - "jev.Client.attempt: one bounded HTTP exchange, extracted from Decide, retried exactly once on 429/5xx or a non-timeout transport failure, inside one timeout budget"
  - "WithMaxResponseBytes + a named ErrDecisionResponseTooLarge for a success body over the bound"
  - "decide span always carries decide.Status(err) as its status word; usage attributes omitted (not zeroed) when the response has none; failure span status description is the class word, never err.Error()"
affects: [phase-03-decision-consumers, phase-04-decision-consumers, decide-jev-maintenance]

# Actuals (#2632)
actuals:
  tokens: 13654
  tasks: 3
  commits: 3

tech-stack:
  added: [math/rand/v2]
  patterns:
    - "Status-only HTTP error classification shared across both provider error dialects (code field decoded but never inspected)"
    - "Single jittered retry gated by isRetryable(err) AND remaining context.Context deadline, never a second per-attempt timeout"
    - "Pointer wire-struct field (wireResponse.Usage *wireUsage) to distinguish 'usage omitted' from 'usage present but zero' through JSON decode"

key-files:
  created:
    - internal/decide/jev/classify.go
    - internal/decide/jev/classify_test.go
    - internal/decide/jev/fixtures_test.go
    - internal/decide/jev/jev_test.go
    - internal/decide/jev/telemetry_test.go
  modified:
    - internal/decide/jev/jev.go

key-decisions:
  - "D-06 branch confirmed reject-hand-write (decided in 02-SDK-EVALUATION.md before this plan ran): classification and bounds live in jev.go's own net/http path, not a wrapped SDK RoundTripper. No new go.mod dependency added."
  - "wireResponse.Usage changed from a value field to a pointer field (Rule 1 bug, found by the no-usage RED subtest): decoding into a value type meant decide.Response.Usage was never nil even when the wire body omitted \"usage\" entirely, contradicting decide.Response's own doc comment (\"Usage is nil when the provider omits it\") and violating E10's span-attribute-omission requirement at the source."
  - "The connection-close claim in Task 2's behavior note (\"the handler observes that the connection closed\") was implemented as a request-completes-normally assertion (write count == 2, retried once) rather than a literal TCP-close observation: Go's default http.Client keep-alive returns the connection to the idle pool rather than closing the socket, so a literal r.Context().Done() wait deadlocks under default transport settings. Tracked as a documented interpretation, not a gap in coverage — the underlying claims (bounded read, drained rest, retried once) are all proven by request/write counts."

requirements-completed: [DEC-04, DEC-06]

coverage:
  - id: D1
    description: "Every non-200 response (both error dialects, non-JSON bodies, transport failures, deadlines, cancellation) classifies into the D-12 vocabulary by HTTP status alone, with structural (never substring-matched) max_tokens_exceeded detection"
    requirement: DEC-04
    verification:
      - kind: unit
        ref: "internal/decide/jev/classify_test.go#TestJevErrorClassification"
        status: pass
    human_judgment: false
  - id: D2
    description: "Decide retries exactly once, only on 429/5xx or a non-timeout transport failure, only when the remaining timeout budget covers the jittered delay; success/error bodies are bounded and drained"
    requirement: DEC-04
    verification:
      - kind: unit
        ref: "internal/decide/jev/jev_test.go#TestJevRetryAndBounds"
        status: pass
      - kind: integration
        ref: "go test ./internal/decide/... ./internal/server/ -count=1 -race"
        status: pass
    human_judgment: false
  - id: D3
    description: "The decide span always carries the shared decide.Status(err) status word, failure spans stay fully attributed with the class word (not err.Error()) as the span status description, usage attributes are omitted (not zeroed) when absent, cost is exact, and no engram-authored telemetry or error text ever carries decision state, instructions, criteria or the API key"
    requirement: DEC-06
    verification:
      - kind: unit
        ref: "internal/decide/jev/telemetry_test.go#TestJevDecideEmitsSpan"
        status: pass
      - kind: unit
        ref: "internal/decide/jev/telemetry_test.go#TestJevTelemetryCarriesNoContent"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 07: Jev Backend Failure Path & Telemetry Summary

**Status-only HTTP classification (both OpenRouter/LiteLLM error dialects), a single budget-bounded jittered retry, a named response-too-large error, and a fully D-13-attributed decide span with zero content leakage — proven by a fixed real bug where usage attributes were never omitted because the wire struct decoded usage as a value, not a pointer.**

## Performance

- **Duration:** 55 min
- **Started:** 2026-09-23T15:15:00Z (approx.)
- **Completed:** 2026-09-23T16:08:17Z
- **Tasks:** 3
- **Files modified:** 6 (5 created, 1 modified)

## Accomplishments

- `classify.go`: `classifyStatus` maps every non-200 status to a `*decide.Error` by HTTP status alone — 401/402/403 → Auth, 400 → BadRequest or ContextTooLarge (only on structural `max_tokens_exceeded` detection, checked first on the whole body then on the first `{`-onward slice of `error.message`), 413 → ContextTooLarge, 429 → RateLimited, 5xx → Unavailable, any other 4xx → BadRequest. `classifyTransport` maps deadline/timeout/cancellation/other transport failures; `isRetryable` names the two retryable classes.
- `jev.go`'s `Decide` now calls a new `attempt` method exactly once, then retries exactly once when `isRetryable(err)` and the remaining `context.Context` deadline exceeds the jittered `retryDelay()` — never a third attempt, never past the budget. A success body over `WithMaxResponseBytes` (default 1 MiB) returns a named `ErrDecisionResponseTooLarge` after `httpdrain.Drain`, and the old plain `"decisions: status %d: ..."` error string is gone entirely.
- The `decide` span's status attribute is always `decide.Status(err)`; on failure the span status description is that same class word (never `err.Error()`, which may carry bounded provider detail text); usage attributes (`input_tokens`, `output_tokens`, `cost_usd`) are set only when the response actually reports usage.

## Task Commits

Each task was committed atomically:

1. **Task 1: Classify every failure by HTTP status alone** - `8150b7e` (feat)
2. **Task 2: Bounded calls with exactly one jittered retry** - `94e2cc7` (feat)
3. **Task 3: Status-attributed decide span with no content in telemetry** - `768659f` (feat)

_All three tasks were TDD (`tdd="true"`): each commit is RED+GREEN combined per this plan's file-level commit scope (test file(s) + implementation in one commit, as directed by the plan's own per-task commit instructions)._

## Files Created/Modified

- `internal/decide/jev/classify.go` - `classifyStatus`, `contextTooLarge`, `classifyTransport`, `isRetryable`; `errorEnvelope`/`detailEnvelope` wire shapes shared by both error dialects
- `internal/decide/jev/classify_test.go` - `TestJevErrorClassification`: 19 table-driven status/dialect cases + a 20KiB-body-bound case + a `transport` subtree (4 cases) + a `retryable` subtree (27 total `--- PASS` lines including nesting)
- `internal/decide/jev/fixtures_test.go` - Verbatim spike-extracted fixtures (`fixtureOpenRouter401/400Choices/400BadType/400MaxTokens`, `fixtureChatPath400`, `fixtureLiteLLM401`) plus synthetic fixtures for statuses the spikes never exercised live (`fixtureLiteLLM403`, numeric 402/404/413/429/500/502/503/524/529, `fixtureHTML502`, `fixtureNoulOK`, `fixtureNoulNoUsage`)
- `internal/decide/jev/jev.go` - Added `attempt`, `WithMaxResponseBytes`, `retryDelay` field + default (100ms + up to 300ms jitter via `math/rand/v2`), retry loop in `Decide`; changed `wireResponse.Usage` from `wireUsage` to `*wireUsage`; rewrote the telemetry defer block for D-13 (see Deviations)
- `internal/decide/jev/jev_test.go` - `TestJevRetryAndBounds`: 13 subtests over retry counts (503/429 retry-then-succeed and retry-then-fail), non-retryable 400/401, a hijacked-connection retry, a short-budget timeout, response-too-large, a budget-exhausted no-retry case, a 20KiB bounded error body, empty-API-key auth, and cost-decode precision
- `internal/decide/jev/telemetry_test.go` - `TestJevDecideEmitsSpan` (success/http failure/validation failure/no usage, 4 subtests) and `TestJevTelemetryCarriesNoContent` (sentinel scan across span attributes, span events, slog output and both calls' `err.Error()`); a package-level shared `SpanRecorder` installed exactly once via `sync.Once` (see Deviations)

## Decisions Made

- D-06 branch reconfirmed as `reject-hand-write` (recorded in `02-SDK-EVALUATION.md` before this plan started) — classification and bounds implemented directly in `jev.go`'s `net/http` path. No `github.com/OpenRouterTeam/go-sdk` or `github.com/spyzhov/ajson` added to `go.mod` (context_note honored, `git diff go.mod go.sum` is empty for this plan).
- `wireResponse.Usage` changed from a value (`wireUsage`) to a pointer (`*wireUsage`) field — see Deviations below.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `wireResponse.Usage` decoded as a value, so `decide.Response.Usage` was never nil even when the provider omitted "usage" entirely**
- **Found during:** Task 3, writing the `TestJevDecideEmitsSpan/no_usage` RED subtest against `fixtureNoulNoUsage`
- **Issue:** `wireResponse.Usage` was typed `wireUsage` (a value), so `json.Unmarshal` against a body with no `"usage"` key left it at its zero value (`InputTokens: 0, OutputTokens: 0, Cost: nil`) rather than absent. `attempt` then unconditionally wrapped it into a non-nil `&decide.Usage{...}`, so `resp.Usage != nil` always — contradicting `decide.Response.Usage`'s own doc comment ("Usage is nil when the provider omits it") and causing the decide span to set `input_tokens`/`output_tokens` attributes to `0` instead of omitting them (E10 requires omission, never zeroing).
- **Fix:** Changed `wireResponse.Usage` to `*wireUsage`; `attempt` now builds `decide.Response.Usage` only `if wr.Usage != nil`. The telemetry defer block already gated on `resp.Usage != nil`, so no further change was needed there once the wire type was fixed.
- **Files modified:** `internal/decide/jev/jev.go`
- **Verification:** `TestJevDecideEmitsSpan/no_usage` failed RED (input_tokens/output_tokens attributes present when they should be absent), then passed GREEN after the fix; full `go test ./internal/decide/... ./internal/server/ -count=1 -race` stayed green.
- **Committed in:** `768659f` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug).
**Impact on plan:** The fix is exactly the kind of correctness gap TDD's RED phase is designed to surface — a genuinely missing behavior found before merge, not a scope change. No other deviations.

### Documented Implementation Notes (not counted as deviations — no code behavior change)

- **Task 2's "handler observes that the connection closed" behavior note:** implemented as a request-completes-normally assertion (the handler's `w.Write` for a 20KiB body returns without error on both the first and the retried request — `wrote == 2`) rather than a literal `r.Context().Done()` wait. Go's default `http.Client` transport returns a fully-read connection to its idle pool rather than closing the underlying socket, so a literal wait on the server's request context deadlocks under default keep-alive settings. The underlying claims this behavior note exists to prove — the read is bounded at `maxErrorBodyBytes`, the rest is drained via `httpdrain.Drain`, and the retry happens exactly once — are all proven by the existing request/write-count assertions in `TestJevRetryAndBounds/large_error_body_bounded_and_drained`.
- **OTel global TracerProvider pinning (discovered while writing Task 3's tests):** `go.opentelemetry.io/otel`'s global package permanently binds a `Tracer` obtained via `otel.Tracer(name)` (this package's package-level `tracer` var, evaluated once at package init) to whichever `TracerProvider` is installed FIRST via `otel.SetTracerProvider` within a test binary's process lifetime — a later `SetTracerProvider` call does not rebind an already-delegated `Tracer`. The original per-subtest `SetTracerProvider`+`t.Cleanup`-restore pattern (mirroring `internal/embed/embed_test.go`'s single-span-test usage) silently produced empty span lists for every subtest after the first, and — more importantly — made `TestJevTelemetryCarriesNoContent`'s sentinel scan iterate zero spans and report a **false pass**. Fixed by installing one process-wide `tracetest.SpanRecorder` via `sync.Once` and having each subtest slice `sr.Ended()` by an index mark instead of swapping providers. This is an implementation detail of the test file, not a production code change.

## Issues Encountered

- **Plan's Task 3 `<verify>` automated command has a regex anchoring bug that can never pass as written.** The command's second `test` clause is `rg -o -e '^--- PASS: Test(JevDecideEmitsSpan|JevTelemetryCarriesNoContent)$' | wc -l) -eq 2` — the trailing `$` anchors immediately after the test name, but `go test -v`'s actual top-level output is `--- PASS: TestJevDecideEmitsSpan (0.00s)` (a trailing ` (Ns)` duration always follows). The anchored pattern therefore matches 0 lines regardless of test outcome, making `VERIFY_RC` always `1`. Verified the underlying intent instead with the same pattern minus the `$` anchor (` ` instead), which returns exit `0`: both top-level tests pass, and exactly 4 `TestJevDecideEmitsSpan/*` subtests pass. `go test ./internal/decide/jev/ -run '^(TestJevDecideEmitsSpan|TestJevTelemetryCarriesNoContent)$' -count=1 -race -v` is unconditionally green; `go vet ./internal/decide/...` and `golangci-lint run ./internal/decide/...` are both clean. This is a plan-authoring defect in the literal verify string, not a code defect — no PLAN.md edit was made (out of scope for an executor); flagging here for the phase verifier / a future plan-authoring lint pass.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/decide/jev` is now feature-complete for this phase's Jev-backend hardening scope: DEC-01 (config/wiring, plan 02-05), DEC-02/DEC-03 (contract, plan 02-03), DEC-04 (this plan's classification/bounds/retry), DEC-06 (this plan's telemetry) all landed.
- Plan 02-08 (per the phase's own artifact table: `task eval:decisions` and `TestJevLive`) is next — it can build directly on `classifyStatus`/`classifyTransport`/`isRetryable` and the D-13 span shape without further backend changes.
- No blockers. The one open item is the plan-authoring verify-regex defect noted above (informational only — does not block phase completion since the underlying behavior is proven green).

---
*Phase: 02-decision-interface-jev-backend*
*Completed: 2026-09-23*

## Self-Check: PASSED

All key-files (5 created, 1 modified) verified present on disk with `[ -f ]`. All three task commit hashes (`8150b7e`, `94e2cc7`, `768659f`) verified present via `git log --oneline --all`. `go test ./internal/decide/... ./internal/server/ -count=1 -race` and `golangci-lint run ./internal/decide/...` both re-run clean immediately before this SUMMARY was written.
