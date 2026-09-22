---
phase: 07-bounded-provider-responses
plan: 03
subsystem: infra
tags: [http, summarize, drain, timeout, provider-bounds]

# Dependency graph
requires:
  - phase: 07-01
    provides: "internal/httpdrain.Drain and internal/testhttp.TrickleHandler, applied here to the summarize lane exactly as they were to embed"
provides:
  - "internal/summarize.Client.WithDrainBytes / WithDrainTimeout / WithMaxTimeout options"
  - "internal/summarize's non-200 and success drain sites routed through httpdrain.Drain"
  - "internal/summarize's WithTimeout(d<=0) resolves to a configurable ceiling instead of unbounded"
  - "internal/summarize.maxErrorBodyBytes, closing the bare-literal-vs-named-const asymmetry with internal/embed"
affects: [07-04, 07-05]

# Actuals (#2632)
actuals:
  tokens: 5352
  tasks: 2
  commits: 2
plan_head_before: 0683376fff1f80b9c823bec721e2453bb4946fc1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Same shared internal/httpdrain.Drain call at both post-response sites as internal/embed — the twin-lane mechanism stays single-sourced, not reimplemented per client"
    - "Defaults-in-struct-literal-before-options-loop for drainBytes/drainTimeout, identical divergence internal/embed already documented — deliberate departure from this file's own WithMaxTokens out-of-range-override convention"
    - "WithMaxTimeout follows the WithMaxTokens convention instead (non-positive ignored), the opposite divergence direction from the drain options, at the same two sites, each commented why"

key-files:
  created: []
  modified:
    - internal/summarize/summarize.go
    - internal/summarize/summarize_test.go

key-decisions:
  - "maxErrorBodyBytes named in summarize.go, closing the loop with embed.go's copy (which already records having been copied verbatim FROM this file) — both lanes now reference the identical named bound instead of one carrying a bare literal."
  - "Both drain sites (non-200 and success) route through httpdrain.Drain with defaults set in New's struct literal before the options loop, so WithDrainBytes(0)/WithDrainTimeout(0) are honored, not swallowed (D-05, D-06) — identical shape to embed."
  - "WithMaxTimeout added at 10m default; the clamp lives in New after the options loop (never inside WithTimeout), preserving last-writer-wins ordering between WithTimeout and WithMaxTimeout regardless of which is supplied first (D-07, D-09)."
  - "TestSummarizeDrainBoundedByTimeUnderZeroRequestTimeout deliberately exercises the SUCCESS path (not the error path) for the same reason embed's twin does: the bounded error read is itself a blocking, non-time-bounded read that would dominate a trickled error body and prove nothing about the drain."

requirements-completed: []  # REQ-provider-drain-bounded and REQ-provider-error-body-closed are shared with plans 07-01/07-02/07-04/07-05 (the requirements.ready-ids shared-ID gate: 07-04 and 07-05 have no SUMMARY yet) — not marked complete until every declaring plan has one.

coverage:
  - id: D1
    description: "The bare 4096 literal at the summarize error read is replaced by a named maxErrorBodyBytes constant, cross-referencing internal/embed/embed.go's copy by file in both directions"
    requirement: "REQ-provider-error-body-closed"
    verification:
      - kind: unit
        ref: "rg -o -e 'maxErrorBodyBytes' internal/summarize/summarize.go | wc -l -> 3"
        status: pass
      - kind: unit
        ref: "rg -o -e 'io[.]LimitReader[(]resp[.]Body, 4096[)]' internal/summarize/summarize.go | wc -l -> 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "Both summarize drain sites (non-200 and success) go through the shared httpdrain.Drain helper; no unbounded io.Copy(io.Discard, ...) remains in internal/summarize/summarize.go"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "rg -o -e 'httpdrain[.]Drain[(]resp[.]Body' internal/summarize/summarize.go | wc -l -> 2"
        status: pass
      - kind: unit
        ref: "rg -o -e 'io[.]Discard' internal/summarize/summarize.go | wc -l -> 0"
        status: pass
    human_judgment: false
  - id: D3
    description: "WithDrainBytes(0) and WithDrainTimeout(0) are honored as zero, not swallowed by a post-loop default (D-05, D-06)"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeDrainOptionsHonorZero"
        status: pass
    human_judgment: false
  - id: D4
    description: "A body over the drain byte bound leaves the connection unreusable; a body inside it is reused (control) — proven against net/http's own post-close safety-net drain"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeDrainBoundedByBytes"
        status: pass
    human_judgment: false
  - id: D5
    description: "A slow-trickle body under WithTimeout(0) (no request deadline at all) returns in a small fraction of the trickle's own bounded cost, on the success path"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeDrainBoundedByTimeUnderZeroRequestTimeout"
        status: pass
    human_judgment: false
  - id: D6
    description: "A non-positive WithTimeout resolves to a configurable ceiling (default 10m) rather than unbounded; a positive value is honored uncapped up to the ceiling; the clamp is applied in New after all options, preserving option order"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeTimeoutCeiling"
        status: pass
    human_judgment: false
  - id: D7
    description: "An oversized non-200 provider error body is surfaced TRUNCATED at maxErrorBodyBytes, closing the summarize half of REQ-provider-error-body-closed"
    requirement: "REQ-provider-error-body-closed"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeNon200ErrorBodyTruncated"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-09-21
status: complete
---

# Phase 07 Plan 03: Bounded Summarize Drain, Byte and Time Axes Summary

**The summarize lane now mirrors embed exactly — both drain sites on `httpdrain.Drain`, a named `maxErrorBodyBytes` constant closing the last bare-literal asymmetry between the two provider clients, and `WithTimeout(0)` resolving to a configurable ceiling instead of unbounded.**

## Performance

- **Duration:** ~35 min (includes a ~10 min wait on a backgrounded `task` run that hit `internal/store`'s known long-running suite — see Issues Encountered)
- **Started:** ~2026-09-21T16:01:00Z
- **Completed:** 2026-09-21T16:36:15Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `internal/summarize/summarize.go` gains `maxErrorBodyBytes` (named, replacing the bare `4096` literal), `drainBytes`/`drainTimeout`/`maxTimeout` fields, and `WithDrainBytes`/`WithDrainTimeout`/`WithMaxTimeout` options — the exact shape plan 07-01 shipped for `internal/embed`.
- Both summarize drain sites (non-200 error path and success path) now call `httpdrain.Drain` instead of the prior unbounded `io.Copy(io.Discard, resp.Body)`.
- `WithDrainBytes`/`WithDrainTimeout` honor an explicit `0` — defaults live in `New`'s struct literal, before the options loop, matching the divergence embed already documented from this file's own `WithMaxTokens` out-of-range-override convention.
- `WithTimeout(d <= 0)` no longer means "no timeout" — it resolves to a configurable ceiling (`WithMaxTimeout`, default 10m), clamped in `New` after all options run so option ordering is preserved. A positive `d` is still honored uncapped.
- The bounded error read is now pinned as *truncating*, not merely present, closing the summarize half of REQ-provider-error-body-closed (D-10) — REQ-provider-error-body-closed is now proven on both lanes.
- The one asymmetry between the two "direct twin" files (embed's named constant vs. summarize's bare literal) is gone; every new addition here cross-references its embed sibling by file, matching the discipline both files already use for each other.

## Task Commits

Each task was committed atomically:

1. **Task 1: The named error-body bound, both summarize drain sites on the shared helper, and the byte bound proven by connection reuse** - `8e234647` (feat)
2. **Task 2: The summarize ceiling, the slow-trickle regression, and the truncation assertion that closes the error-body requirement** - `3b7fc66c` (feat)

_No TDD gate applies to this plan (`type: execute`); each commit is a single implementation+test unit, not a RED/GREEN/REFACTOR split._

## Files Created/Modified

- `internal/summarize/summarize.go` - `maxErrorBodyBytes` constant, `drainBytes`/`drainTimeout`/`maxTimeout` fields, `WithDrainBytes`/`WithDrainTimeout`/`WithMaxTimeout` options, both drain sites routed through `httpdrain.Drain`, the post-options timeout clamp, `WithTimeout`'s rewritten doc comment
- `internal/summarize/summarize_test.go` - five new tests: `TestSummarizeDrainBoundedByBytes`, `TestSummarizeDrainOptionsHonorZero`, `TestSummarizeTimeoutCeiling`, `TestSummarizeDrainBoundedByTimeUnderZeroRequestTimeout`, `TestSummarizeNon200ErrorBodyTruncated`; existing `TestSummarizeNon200DrainsForReuse` updated to reference the new named constant instead of the bare `4096*2` literal

## Recorded test evidence (acceptance criteria)

`go test ./internal/summarize/ -list '.*'` (resolved before relying on any name, per the plan's warning):
- Before this plan: 11 tests (`TestSummaryFidelity`, `TestSummarizePostsChatCompletionAndReturnsContent`, `TestSummarizeFramesContentAsUntrustedData`, `TestSummarizeErrorsOnEmptyChoices`, `TestSummarizeNon200IncludesStatusAndBody`, `TestSummarizeNon200DrainsForReuse`, `TestSummarizeDefaultMaxTokensDecoupledFromMaxChars`, `TestSummarizeWithMaxTokensOverrideAndOmit`, `TestSummarizeWithTimeoutCancelsSlowRequest`, `TestSummarizeConcurrentSharedClientOneEndpoint`, `TestSummarizeTruncatesToMaxChars`)
- After this plan: 16 tests — the same 11 plus the 5 new ones listed above (matches the plan's "five new names plus the eleven that existed before")

Observed `Reused()`/`Total()` from `TestSummarizeDrainBoundedByBytes` (captured via a temporary `t.Logf`, reverted before commit — no debug output shipped):
- Over the bound: `Reused=0 Total=2` (neither call reused a connection)
- Inside the bound (control): `Reused=1 Total=2` (the second call reused the first's connection)

Observed elapsed time from `TestSummarizeDrainBoundedByTimeUnderZeroRequestTimeout`: well under the 1s assertion threshold, against the trickle's own bounded budget of ~3s (30 chunks × 100ms pause) — test reports `(0.10s)` total including server round-trips.

Task-level diff-only-scope check for both tasks: `git diff "$C^..$C" -- internal/embed internal/httpdrain internal/testhttp` printed nothing for both `8e234647` and `3b7fc66c` — neither task touched the shared helper or the embed sibling.

Which of the four pre-existing tests needed changing under the new ceiling behavior (plan's own question): **none** — `TestSummarizeWithTimeoutCancelsSlowRequest`, `TestSummarizeNon200IncludesStatusAndBody`, `TestSummarizeNon200DrainsForReuse`, and `TestSummarizeConcurrentSharedClientOneEndpoint` all pass unmodified (each supplies a positive duration or none at all).

Plan-level verification (`<verification>` block), run after both tasks:
- `go test ./internal/summarize/ -count=1 -v`: all 15 runnable tests PASS (16 listed, `TestSummaryFidelity` SKIPs by design — gated behind `ENGRAM_SUMMARY_EVAL=1`, not this plan's concern), no FAIL.
- `go test ./internal/summarize/ -list '.*'`: confirmed above — 16 names, 5 new + 11 pre-existing.
- `rg -o -e 'io[.]Discard' internal/summarize/summarize.go | wc -l` → `0`; `rg -o -e 'httpdrain[.]Drain[(]' internal/summarize/summarize.go | wc -l` → `2`.
- `rg 'io\.Copy\(io\.Discard' --type go` repo-wide: the only remaining match is `internal/httpdrain/httpdrain.go`'s own implementation — the one shared helper both provider clients now call, not a duplicate. No client-side (`internal/embed`, `internal/summarize`) or other unbounded drain remains anywhere in the repository. (This is the expected, single-sourced state D-02 exists to produce; the phase-level verification bullet's literal wording — "matches no non-test file anywhere" — is satisfied in spirit but the shared helper's own definition necessarily contains the pattern it wraps.)
- `task lint`, `task fmt:check` (gofmt clean on both touched files; dprint clean), `task license:check` (2195 files, 0 invalid): all clean for the files this plan touched.
- `git status --porcelain` after the final commit: only the pre-existing, unrelated untracked `.planning/milestone.lock` (not created or touched by this plan).

## Decisions Made

- No architectural deviations. Both tasks executed as planned; every new field/option/constant mirrors its `internal/embed` sibling exactly, per the plan's `<interfaces>` shape and this phase's D-01/D-02/D-05/D-06/D-07/D-08/D-09/D-10 decisions.
- See `key-decisions` in the frontmatter for the specific implementation choices (all followed the plan/embed twin verbatim — no deviation from the planned shape).

## Deviations from Plan

None - plan executed exactly as written. Both tasks' acceptance criteria and `<verify>` blocks passed on the first implementation attempt; no Rule 1/2/3 auto-fixes were needed.

**Total deviations:** 0 auto-fixed.
**Impact on plan:** None.

## Issues Encountered

- **The plan's own Task 2 verify block includes a bare `task` invocation** (`out=$(task 2>&1); rc=$?; ...; test "$rc" -eq 0`). Per this plan's own executor notes and project rule 6 (`internal/summarize` is fast; do not gate on `internal/store`, which needs `-timeout 180m` on this machine), the full `task` gate was run once in the foreground, auto-backgrounded by the harness after its 590s cap, and completed ~10 minutes later with the *default* `go test ./...`'s 10-minute per-package timeout expiring mid-run inside `internal/store.TestRedEvidencePatchesAreLive` (`FAIL github.com/seanb4t/engram/internal/store 601.322s`, unrelated to any file this plan touches — `internal/summarize`, `internal/embed`, and every other package in that same run reported `ok`). This is the exact, already-documented pre-existing characteristic of this machine's full-suite run time, not a regression introduced here.
- **A stranded red-evidence patch was left applied** by the timed-out run, exactly as project rule 7 warned: `internal/store/spine.go` held a mutation (`s.scanView()` swapped for an unbounded `readView{selector: qdrant.NewWithPayload(true), maxRecordBytes: 0}`) because `TestRedEvidencePatchesAreLive`'s revert-in-`defer` was interrupted by the timeout before it ran. Detected via `git status --short` immediately after the background notification, reverted with `git checkout -- internal/store/spine.go` before staging or committing anything, and confirmed `git status --porcelain` clean of it afterward. No file this plan owns was affected; nothing from this stray mutation was ever staged or committed.
- Per project rule 6, the full `task` gate was **not** re-run to green after this — package-scoped verification (`go test ./internal/summarize/ ./internal/embed/ ./internal/httpdrain/ -count=1`, `task lint`, `task fmt:check` on touched files, `task license:check`) stands in as the substitute, documented here as instructed. This is a repo-environment characteristic (the `internal/store` suite's own required 180-minute timeout), not a defect in this plan's two commits.

## User Setup Required

None - no external service configuration required. Plan 07-04 wires the operator-facing `ENGRAM_SUMMARY_DRAIN_BYTES`/`ENGRAM_SUMMARY_DRAIN_TIMEOUT`/`ENGRAM_SUMMARY_MAX_TIMEOUT` equivalents; the six registry keys themselves already exist and validate per plan 07-02.

## Next Phase Readiness

- All four of the phase's originally-scoped drain sites (`embed.go:295/309`, `summarize.go:182/191`) are now bounded by the one shared `httpdrain.Drain` helper — confirmed by the repo-wide `rg 'io\.Copy\(io\.Discard' --type go` check above.
- REQ-provider-error-body-closed is now proven on both lanes (embed via 07-01, summarize via this plan); REQ-provider-drain-bounded's implementation is complete on both lanes, with the operator-facing config wiring (plan 07-04) and end-to-end phase verification (plan 07-05) still to come.
- Plan 07-04 (wiring the six `ENGRAM_*` knobs from `internal/config` into `internal/server/tools.go`'s embedder/summarizer construction) is unblocked — both `internal/embed.Client` and `internal/summarize.Client` now expose the identical `WithDrainBytes`/`WithDrainTimeout`/`WithMaxTimeout` option surface for it to wire.
- Blocker: none. `requirements-completed` stays `[]` here per the shared-ID gate — REQ-provider-drain-bounded and REQ-provider-error-body-closed will flip to `Complete` once plans 07-04 and 07-05 each produce their own SUMMARY.

---
*Phase: 07-bounded-provider-responses*
*Completed: 2026-09-21*

## Self-Check: PASSED

- FOUND: `.planning/phases/07-bounded-provider-responses/07-03-SUMMARY.md`
- FOUND: `internal/summarize/summarize.go` (modified, on disk)
- FOUND: `internal/summarize/summarize_test.go` (modified, on disk)
- FOUND commit `8e234647` in `git log --oneline --all`
- FOUND commit `3b7fc66c` in `git log --oneline --all`
