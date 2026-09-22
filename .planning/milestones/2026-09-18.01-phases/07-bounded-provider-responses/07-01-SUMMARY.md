---
phase: 07-bounded-provider-responses
plan: 01
subsystem: infra
tags: [http, embed, drain, timeout, provider-bounds]

# Dependency graph
requires: []
provides:
  - "internal/httpdrain.Drain — the shared, byte-and-time bounded response-body drain (D-01, D-02)"
  - "internal/testhttp.TrickleHandler — a shared, bounded-by-construction slow-response test handler"
  - "internal/embed.Client.WithDrainBytes / WithDrainTimeout / WithMaxTimeout options"
  - "internal/embed's non-2xx and success drain sites routed through httpdrain.Drain"
  - "internal/embed's WithTimeout(d<=0) resolves to a configurable ceiling instead of unbounded"
affects: [07-02, 07-03, 07-04, 07-05]

# Actuals (#2632)
actuals:
  tokens: 7731
  tasks: 3
  commits: 3
plan_head_before: 3adfad8bf73e5ad31bb6c1342af5ca8d00b5e5a3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared internal/ non-_test.go helper package imported by production code (internal/httpdrain), same shape as internal/testhttp"
    - "Timer-closes-the-body bound (time.AfterFunc + defer t.Stop(), never a goroutine) for a response-body read that must never stall a caller"
    - "Defaults-in-struct-literal-before-options-loop, to let an explicit Option(0) be honored instead of swallowed by a post-loop fallback — deliberate divergence from the sibling WithMaxResponseBytes convention, documented at every site it diverges from"

key-files:
  created:
    - internal/httpdrain/httpdrain.go
    - internal/httpdrain/httpdrain_test.go
    - internal/testhttp/trickle.go
  modified:
    - internal/embed/embed.go
    - internal/embed/embed_test.go
    - internal/testhttp/reuse.go

key-decisions:
  - "Task 1's byte-axis regression (TestEmbedDrainBoundedByBytes) required discovering and working around net/http's OWN post-close safety-net drain (maxPostCloseReadBytes, 256 KiB, transport.go) — a body under that size with a known Content-Length gets silently fully drained by net/http itself regardless of this package's own drain bound, which would have made the 'over the bound: not reused' assertion pass or fail for the wrong reason. Fixed by serving a body >256 KiB with an explicit Content-Length header so the assertion actually exercises httpdrain's bound."
  - "Left .planning/phases/07-bounded-provider-responses/07-02-PLAN.md's escaped-quote example untouched despite it failing task's internal/keylinks.TestNoEscapedPatternsRepoWide gate — pre-existing since commit 2a15f189, before this plan began, and outside this plan's file scope. Documented in deferred-items.md per this plan's own executor notes."

requirements-completed: []  # REQ-provider-drain-bounded and REQ-provider-error-body-closed are shared with plans 07-02..07-05 (the requirements.ready-ids shared-ID gate reports 0/2 ready) — not marked complete until every declaring plan has a SUMMARY.

coverage:
  - id: D1
    description: "A new internal/httpdrain package owns the one bounded-drain helper both provider clients will call; the helper bounds both bytes (io.LimitReader) and time (time.AfterFunc closing the body), with no goroutine outliving the call"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/httpdrain/httpdrain_test.go#TestDrainStopsAtByteBound"
        status: pass
      - kind: unit
        ref: "internal/httpdrain/httpdrain_test.go#TestDrainClosesBodyAtTimeBound"
        status: pass
      - kind: unit
        ref: "internal/httpdrain/httpdrain_test.go#TestDrainZeroSkipsEntirely"
        status: pass
    human_judgment: false
  - id: D2
    description: "Both embed drain sites (non-2xx and success) go through the shared helper; no unbounded io.Copy(io.Discard, ...) remains in internal/embed/embed.go"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "rg -o -e 'httpdrain[.]Drain[(]resp[.]Body' internal/embed/embed.go | wc -l == 2"
        status: pass
      - kind: unit
        ref: "rg -o -e 'io[.]Discard' internal/embed/embed.go | wc -l == 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "WithDrainBytes(0) and WithDrainTimeout(0) are honored as zero, not swallowed by a post-loop default (D-05, D-06)"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedDrainOptionsHonorZero"
        status: pass
    human_judgment: false
  - id: D4
    description: "A body over the drain byte bound leaves the connection unreusable; a body inside it is reused (control) — proven against net/http's own post-close safety-net drain"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedDrainBoundedByBytes"
        status: pass
    human_judgment: false
  - id: D5
    description: "A slow-trickle body under WithTimeout(0) (no request deadline at all) returns in a small fraction of the trickle's own bounded cost, on the success path"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout"
        status: pass
    human_judgment: false
  - id: D6
    description: "A non-positive WithTimeout resolves to a configurable ceiling (default 10m) rather than to unbounded; a positive value is honored uncapped up to the ceiling; the clamp is applied in New after all options, preserving option order"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedTimeoutCeiling"
        status: pass
    human_judgment: false
  - id: D7
    description: "An oversized non-2xx provider error body is surfaced TRUNCATED at maxErrorBodyBytes, closing the embed half of REQ-provider-error-body-closed"
    requirement: "REQ-provider-error-body-closed"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedNon2xxErrorBodyTruncated"
        status: pass
    human_judgment: false

duration: 29min
completed: 2026-09-21
status: complete
---

# Phase 07 Plan 01: Bounded Embed Drain, Byte and Time Axes Summary

**A shared `internal/httpdrain.Drain` helper — timer-closes-the-body, no goroutine, honored-zero defaults — now bounds both embed post-response drain sites and closes the last `WithTimeout(0)` escape hatch on the embed lane.**

## Performance

- **Duration:** 29 min
- **Started:** 2026-09-21T15:20:49Z
- **Completed:** 2026-09-21T15:49:21Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- New `internal/httpdrain` package: one exported `Drain(body, maxBytes, maxTime)` function, bounded on both axes, zero goroutines, mechanism pinned to verified Go 1.27.1 `net/http` source (D-01).
- Both `internal/embed` drain sites (non-2xx error path and success path) now call `httpdrain.Drain` instead of the prior unbounded `io.Copy(io.Discard, resp.Body)`.
- `WithDrainBytes`/`WithDrainTimeout` honor an explicit `0` — defaults live in `New`'s struct literal, before the options loop, a deliberate and documented divergence from `WithMaxResponseBytes`'s post-loop fallback.
- New shared `internal/testhttp.TrickleHandler`: bounded-by-construction slow-response handler (chunk count/pause are caller parameters), also honoring request-context cancellation so a client that abandons the body releases it immediately.
- `WithTimeout(d <= 0)` no longer means "no timeout" — it resolves to a configurable ceiling (`WithMaxTimeout`, default 10m), clamped in `New` after all options run so option ordering is preserved. A positive `d` is still honored uncapped.
- The already-shipped bounded error read is now pinned as *truncating*, not merely present, closing the embed half of REQ-provider-error-body-closed (D-10).

## Task Commits

Each task was committed atomically:

1. **Task 1: The bounded drain, end to end** - `d83892c3` (feat)
2. **Task 2: The time axis — bounded trickle handler + zero-timeout regression** - `fd36aa6e` (test)
3. **Task 3: The request-timeout ceiling and the missing truncation assertion** - `423b34b7` (feat)

_No TDD gate applies to this plan (`type: execute`); each commit is a single implementation+test unit, not a RED/GREEN/REFACTOR split._

## Files Created/Modified
- `internal/httpdrain/httpdrain.go` - the shared bounded drain (byte axis via `io.LimitReader`, time axis via `time.AfterFunc` closing the body)
- `internal/httpdrain/httpdrain_test.go` - unit proof of both bounds plus the zero-skips-entirely case, driven by a hand-written fake `io.ReadCloser`
- `internal/testhttp/trickle.go` - the shared bounded slow-trickle handler; `internal/testhttp/reuse.go`'s package doc widened to describe it
- `internal/embed/embed.go` - `drainBytes`/`drainTimeout`/`maxTimeout` fields, `WithDrainBytes`/`WithDrainTimeout`/`WithMaxTimeout` options, both drain sites routed through `httpdrain.Drain`, the post-options timeout clamp
- `internal/embed/embed_test.go` - five new tests: `TestEmbedDrainBoundedByBytes`, `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout`, `TestEmbedDrainOptionsHonorZero`, `TestEmbedTimeoutCeiling`, `TestEmbedNon2xxErrorBodyTruncated`

## Recorded test evidence (acceptance criteria)

`go test -list '.*'`:
- `internal/httpdrain`: `TestDrainStopsAtByteBound`, `TestDrainClosesBodyAtTimeBound`, `TestDrainZeroSkipsEntirely` (3, matches plan)
- `internal/embed`: 18 tests total — the 13 pre-existing plus the 5 new ones listed above (matches plan's "five new embed test names plus the thirteen that existed before")

Observed `Reused()`/`Total()` from `TestEmbedDrainBoundedByBytes`:
- Over the bound: `Reused=0 Total=2` (neither call reused a connection)
- Inside the bound (control): `Reused=1 Total=2` (the second call reused the first's connection)

Observed elapsed time from `TestEmbedDrainBoundedByTimeUnderZeroRequestTimeout`: `~101ms`, against the trickle's own bounded budget of `~3s` (30 chunks × 100ms pause).

Task 3's diff-only-additions check: `git diff 423b34b7^..423b34b7 -- internal/embed/embed_test.go` contains zero removed lines — the task only appended tests, as required.

Which of the four pre-existing `WithTimeout`/non-2xx tests needed changing under the new ceiling behavior: **none** — `TestEmbedWithTimeoutCancelsSlowRequest`, `TestWithTimeoutComposesWithHTTPTransport`, `TestEmbedNon2xxIncludesStatusAndBody`, and `TestEmbedNon2xxDrainsForReuse` all still pass unmodified.

## Decisions Made
- Task 1's byte-axis regression required accounting for net/http's own post-close safety-net drain (`maxPostCloseReadBytes`, 256 KiB, `net/http/transport.go`): on `Close`, the Transport itself tries to finish draining up to 256 KiB within 50ms whenever the declared `Content-Length` is within that bound — regardless of what this package's own drain did. Discovered via an instrumented repro (`go test -run TestDrainReproDebug`) after the first version of the "over the bound" assertion unexpectedly observed a reused connection. Fixed by serving a body over 256 KiB with an explicit `Content-Length` header so the assertion is actually proving `httpdrain`'s bound, not net/http's own safety net. Recorded here since it is a non-obvious fact about the network layer this test depends on, not a plan deviation.
- No architectural deviations. All three tasks executed as planned; the plan's `<interfaces>` API shape (`Drain(body io.ReadCloser, maxBytes int64, maxTime time.Duration)`) was implemented verbatim.

## Deviations from Plan

### Auto-fixed Issues

None — no Rule 1/2/3 auto-fixes were needed. The net/http safety-net discovery above required adjusting the *test's own* body-size/Content-Length setup to make the assertion valid, not a fix to production code, a bug, or a missing feature.

**Total deviations:** 0 auto-fixed.
**Impact on plan:** None. Plan executed as written; one test-construction detail (see Decisions Made) was needed to make an assertion trustworthy against net/http's own behavior.

## Issues Encountered

- **`task` (the full repository gate) fails** on `internal/keylinks.TestNoEscapedPatternsRepoWide` against `.planning/phases/07-bounded-provider-responses/07-02-PLAN.md:53` (an escaped `koanf:\"drain_bytes\"` example that should read `koanf:"drain_bytes"`). This line was introduced in commit `2a15f189` ("docs(07): create phase plan — bounded provider responses"), before this plan (07-01) began execution, and lives entirely inside a sibling plan file this plan does not touch. Per this plan's own executor notes ("If `task` shows a failure this plan did not cause, STOP and report it rather than editing unrelated code"), left unfixed and logged to `.planning/phases/07-bounded-provider-responses/deferred-items.md` (status: open) rather than edited here. All package-scoped verify commands for this plan (`go test ./internal/httpdrain/ ./internal/testhttp/ ./internal/embed/ -count=1 -v`, `task lint:go`, `task fmt`, `task license:check`) pass cleanly; only the bare `task` invocation's repo-wide doc-pattern gate is affected, and only because of the unrelated sibling file.
- `task fmt` (gofmt) repeatedly reformatted three files this plan never touched (`internal/server/outofrange_test.go`, `internal/store/reindexsweep_oversized_test.go`, `internal/store/storetest/seed_test.go`) — pre-existing gofmt drift in the repo, unrelated to this plan. Reverted with `git checkout --` before each commit so no unrelated changes were staged (project rule 1: explicit pathspec on every commit).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/httpdrain.Drain` and `internal/testhttp.TrickleHandler` are now available for plan 07-03 to apply the identical pattern to `internal/summarize`.
- Plan 07-02's config registry lane (the six `ENGRAM_*_DRAIN_*`/`ENGRAM_*_MAX_TIMEOUT` knobs) is unblocked to proceed in parallel, as the phase's wave design anticipated.
- Blocker: `.planning/phases/07-bounded-provider-responses/07-02-PLAN.md:53`'s escaped `koanf:` example should be un-escaped before or during that plan's own execution, or `task`'s doc-pattern gate stays red for the whole repo.

---
*Phase: 07-bounded-provider-responses*
*Completed: 2026-09-21*

## Self-Check: PASSED

- FOUND: `.planning/phases/07-bounded-provider-responses/07-01-SUMMARY.md`
- FOUND: `internal/httpdrain/httpdrain.go`
- FOUND: `internal/testhttp/trickle.go`
- FOUND commit `d83892c3` in `git log --oneline --all`
- FOUND commit `fd36aa6e` in `git log --oneline --all`
- FOUND commit `423b34b7` in `git log --oneline --all`
