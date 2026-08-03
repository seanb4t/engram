---
phase: 04-diagnosability
plan: 03
subsystem: api
tags: [go, http, embeddings, chat-completions, error-handling, connection-reuse, httptrace]

requires:
  - phase: 04-diagnosability
    provides: "D-13/D-14/D-15/D-16 decisions from 04-CONTEXT.md governing the provider error-body and drain fix"
provides:
  - "internal/testhttp.ReuseTracker — the phase's only new test infrastructure, an httptrace-based connection-reuse observer shared by internal/embed and internal/summarize"
  - "internal/embed.Client: bounded, verbatim non-2xx error-body surfacing (status + provider text), a drain on both the error and success paths, and a WithMaxResponseBytes option bounding the success decode"
  - "internal/summarize.Client: a drain on both the error and success paths (the surfacing half was already correct and untouched)"
affects: [04-06]

actuals:
  tokens: 3801
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "ReuseTracker.Context(ctx) wraps a context with httptrace.WithClientTrace so a GotConn hook counts fresh vs. reused connections — only observes requests built with http.NewRequestWithContext(ctx, ...) on the wrapped context, per the type's doc comment"
    - "Bounded-read-then-drain: io.ReadAll(io.LimitReader(...)) or json.NewDecoder(io.LimitReader(...)) followed unconditionally by io.Copy(io.Discard, resp.Body) before the deferred Close, so a non-2xx or oversized response never leaks the connection"

key-files:
  created:
    - internal/testhttp/reuse.go
  modified:
    - internal/embed/embed.go
    - internal/embed/embed_test.go
    - internal/summarize/summarize.go
    - internal/summarize/summarize_test.go

key-decisions:
  - "D-13: maxErrorBodyBytes = 4096 in internal/embed is a literal copy of the constant already used at summarize.go:181, with a comment naming the source, rather than a new number."
  - "D-14 resolved asymmetrically as directed: surfacing (status + body) is embeddings-only since the chat lane already had it; the drain is added to BOTH lanes since neither had it."
  - "D-15: the provider error body is surfaced verbatim within the 4096-byte bound, not scrubbed. The code comment above the read documents the honest finding (see 'D-15 Finding' below) rather than the reassuring-but-false premise CONTEXT.md's D-15 text originally stated."
  - "D-16: the embeddings success decode is bounded via WithMaxResponseBytes(int64), following the existing Option pattern, with a 1 MiB defaultMaxResponseBytes fallback matching the sibling's constant. The dimension-derived wiring from ENGRAM_EMBED_DIM is explicitly plan 04-06's job (embedderFromConfig in tools.go), not this plan's — this plan only builds and defaults the option."

patterns-established:
  - "internal/testhttp is a non-_test.go internal package (importable across package test files) exposing only counters/accessors and importing no test framework, so it can be shared by internal/embed and internal/summarize without leaking test-only code into a production import graph."

requirements-completed: [REQ-embed-provider-error-body]

coverage:
  - id: D1
    description: "A non-2xx embeddings response returns an error carrying the status code and a bounded, verbatim prefix of the provider's error body"
    requirement: "REQ-embed-provider-error-body"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedNon2xxIncludesStatusAndBody"
        status: pass
    human_judgment: false
  - id: D2
    description: "After a non-2xx on the embeddings lane, the connection is drained and reusable by a second request"
    requirement: "REQ-embed-provider-error-body"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedNon2xxDrainsForReuse"
        status: pass
    human_judgment: false
  - id: D3
    description: "After a non-200 on the chat/summarize lane, the connection is drained and reusable by a second request (the shared half of the audit clause)"
    requirement: "REQ-embed-provider-error-body"
    verification:
      - kind: unit
        ref: "internal/summarize/summarize_test.go#TestSummarizeNon200DrainsForReuse"
        status: pass
    human_judgment: false
  - id: D4
    description: "The embeddings success-path decode is bounded (via WithMaxResponseBytes) rather than unbounded"
    requirement: "REQ-embed-provider-error-body"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedSuccessDecodeBounded"
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 03: Embeddings Provider Error Body, Both-Lane Drain, and Bounded Decode Summary

**A 502 from the embedder now names both the status and the provider's own diagnostic text (bounded, verbatim), and both provider lanes drain their response bodies so the connection that carried the error survives it — proven by an httptrace-based `ReuseTracker` observed failing without the drain and passing with it.**

## Performance

- **Duration:** ~6 min (task-commit span 14:14:13-14:17:21 local; reading/verification bracketing that)
- **Started:** 2026-08-01T14:12:00-04:00 (approx, first read)
- **Completed:** 2026-08-01T14:17:21-04:00
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- `internal/testhttp/reuse.go` — new `ReuseTracker` type: `Context(ctx)` wraps a context with `httptrace.WithClientTrace`, a `GotConn` hook counts fresh vs. reused connections, `Reused()`/`Total()` accessors. Imports no test framework; confirmed by gate `rg -c '"testing"'` = 0. This is the phase's only new test infrastructure, shared by both provider packages' test files (a `_test.go` helper cannot cross package boundaries in Go).
- `internal/embed/embed.go`'s `embed` method: the non-2xx branch now reads a bounded (`maxErrorBodyBytes = 4096`, copied from `summarize.go:181` per D-13), trimmed prefix of the provider body into the returned error alongside the status code, then drains the remainder (`io.Copy(io.Discard, resp.Body)`) before returning. The success branch decodes through `io.LimitReader(resp.Body, c.maxResponseBytes)` and also drains afterward. `WithMaxResponseBytes(int64) Option` added following the existing option shape (non-positive `n` is a no-op, matching `WithMaxTokens`'s guard style in the sibling package); `New` falls back to `defaultMaxResponseBytes` (1 MiB) when unset.
- `internal/summarize/summarize.go`'s `Summarize` method: added exactly the drain, nothing else — after the existing bounded error read and after the existing bounded success decode. The 4096/1 MiB constants, error text, and every other line are byte-identical to before this plan.
- Four new tests: `TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`, `TestEmbedSuccessDecodeBounded` (`internal/embed/embed_test.go`), `TestSummarizeNon200DrainsForReuse` (`internal/summarize/summarize_test.go`).

## D-15 Finding (recorded per plan instruction, not assumed away)

`04-CONTEXT.md`'s D-15 text states the provider error body is "the provider's own text, not caller data." Reading `embed.go:223-242` (the request body build) shows this is **false as an absolute claim**: `m["input"] = text` at line 232 puts caller content — the memory being stored or the query being searched — directly into the outbound request body. A provider that reflects its request back inside an error body (a real pattern for malformed-request 4xxs) would therefore echo that caller content back in the response this plan now surfaces.

The corrected analysis, written into the code comment above the error-body read in `embed.go`:
- The reflection is **same-actor**, not cross-actor: the content returns to the exact caller who supplied it, on the same synchronous call path. This is not a disclosure to a different party.
- The residual exposure is that `connectError`'s `default` arm (`internal/server/connecterror.go`) logs unmatched errors server-side, so a reflecting provider could put one caller's content into an **operator log** — bounded at `maxErrorBodyBytes` (4096 bytes) by the same read that surfaces the diagnostic.
- This is registered in the plan's threat model as `T-04-05` (Information Disclosure, medium severity, **accept**) — accepted, not mitigated, because D-15 is explicit that the body is surfaced verbatim and scrubbing would destroy the diagnostic this requirement exists to deliver.

No scrubbing was added. The comment documents the honest tradeoff rather than an unexamined "it's safe" assumption.

## RED -> GREEN Transcripts (drain-reuse assertions)

Both reuse tests were validated as capable of failing, per the plan's explicit instruction that a reuse assertion never observed failing is indistinguishable from one that cannot fail. For each, the drain line was temporarily commented out (backed up first, restored via `diff` confirming a byte-identical file afterward — verified with `diff` returning no output), the test rerun, and the failure recorded below.

### `TestEmbedNon2xxDrainsForReuse`

RED (drain commented out in `embed.go`'s non-2xx branch):
```
=== RUN   TestEmbedNon2xxDrainsForReuse
    embed_test.go:483: want at least one reused connection, got Reused()=0 Total()=2
--- FAIL: TestEmbedNon2xxDrainsForReuse (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/internal/embed	0.158s
```

GREEN (drain restored):
```
=== RUN   TestEmbedNon2xxDrainsForReuse
--- PASS: TestEmbedNon2xxDrainsForReuse (0.00s)
PASS
ok  	github.com/seanb4t/engram/internal/embed	0.154s
```

### `TestSummarizeNon200DrainsForReuse`

RED (drain commented out in `summarize.go`'s non-200 branch):
```
=== RUN   TestSummarizeNon200DrainsForReuse
    summarize_test.go:185: want at least one reused connection, got Reused()=0 Total()=2
--- FAIL: TestSummarizeNon200DrainsForReuse (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/internal/summarize	0.178s
```

GREEN (drain restored):
```
=== RUN   TestSummarizeNon200DrainsForReuse
--- PASS: TestSummarizeNon200DrainsForReuse (0.00s)
PASS
ok  	github.com/seanb4t/engram/internal/summarize	0.120s
```

Both fake error bodies are deliberately sized at `2 * maxErrorBodyBytes` (8192 bytes) — strictly larger than the 4096-byte bound — so the unread remainder that the drain must consume is real, not hypothetical. Both tests issue two sequential calls through the same `*Client` instance (and therefore the same underlying `*http.Client`/transport connection pool).

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): surface and drain the embeddings provider error body** — `ef80ee96` (fix)
2. **Task 2: drain the chat-completions response body for connection reuse** — `0695a7a8` (fix)
3. **Task 3: pin provider error-body surfacing, both drains, and the success bound** — `c65984f8` (test)

No separate plan-metadata commit beyond this SUMMARY's own state-update commit (below) — the plan carried no checkpoint to resolve.

## Files Created/Modified

- `internal/testhttp/reuse.go` — new: `ReuseTracker` (`Context`, `Reused`, `Total`)
- `internal/embed/embed.go` — `maxErrorBodyBytes`, `defaultMaxResponseBytes` consts; `maxResponseBytes` field; `WithMaxResponseBytes` option; bounded-read-and-drain on both branches of `embed()`; D-15 finding recorded in a comment
- `internal/embed/embed_test.go` — `TestEmbedNon2xxIncludesStatusAndBody`, `TestEmbedNon2xxDrainsForReuse`, `TestEmbedSuccessDecodeBounded`
- `internal/summarize/summarize.go` — two `io.Copy(io.Discard, resp.Body)` drains added; nothing else changed
- `internal/summarize/summarize_test.go` — `TestSummarizeNon200DrainsForReuse`

## Decisions Made

All decisions were pre-set by `04-CONTEXT.md` (D-13 through D-16); no new architectural decisions were made during execution beyond the D-15 finding documented above, which the plan explicitly required rather than left as an open question.

## Deviations from Plan

None — plan executed exactly as written, including the corrected D-15 analysis the plan itself flagged as a planner-level falsification needing to be honored rather than propagated.

## Issues Encountered

**Shared, non-isolated working tree with a concurrently-executing wave-1 sibling plan (04-02).** This plan runs in the same git working directory as another agent executing plan 04-02 (Cedar decision diagnostics, touching `internal/authz/authz.go` and `internal/store/store.go`). Two concrete effects, both handled without touching the other plan's work:

1. `go vet ./...` and `task` (full lint+test) both failed throughout this session — but only on `internal/store`/`internal/authz`, due to the other plan's in-flight, not-yet-committed signature changes (e.g. `s.decideBucket`/`s.ownerScopeFilter`/`s.ownerOrSharedCondition` gaining a `context.Context` first parameter mid-edit). Verified out of scope per the SCOPE BOUNDARY rule by running `go vet` and `golangci-lint run` scoped to this plan's three packages only (`internal/embed`, `internal/summarize`, `internal/testhttp`), both clean. Did not touch, fix, or wait on the other plan's files.
2. **Self-caught commit-boundary error, corrected before this SUMMARY:** Task 3's `git commit -m "..."` (no pathspec) initially swept up the other agent's already-*staged* `internal/authz/authz.go` and `internal/store/store.go` into this plan's commit, because `git commit` with no pathspec commits the whole index, not just newly `git add`-ed files. Caught immediately by inspecting `git show --stat HEAD` after the commit (expected 2 files, saw 4). Fixed via `git reset --soft HEAD~1` (returns the index to its exact pre-commit state, losing nothing) followed by `git commit ... -- internal/embed/embed_test.go internal/summarize/summarize_test.go` (explicit pathspec, so the index-wide foreign staged files are excluded). Confirmed via `git show --stat HEAD` afterward (2 files, +131/-0, matching the pre-commit `git diff --stat`) and confirmed the other agent's staged files were restored untouched in the index. No destructive git command was used — `reset --soft` moves HEAD only, the working tree and index were unchanged throughout.

Both effects are recorded here per the plan's "surface a false plan premise loudly" spirit — this one is an execution-environment hazard, not a plan-content falsification, but it's the kind of thing a later reader debugging a weird git-log entry would want documented.

## Next Phase Readiness

- The `WithMaxResponseBytes` option and `defaultMaxResponseBytes` constant are ready for plan 04-06 to wire from `ENGRAM_EMBED_DIM` via `embedderFromConfig` (`internal/server/tools.go:346-367`), per the plan's stated boundary — that wiring line is explicitly NOT part of this plan.
- `internal/testhttp.ReuseTracker` is available for any future provider-lane work needing the same connection-reuse assertion shape.
- No blockers. `go.mod`/`go.sum` show zero diff (net/http/httptrace is stdlib) — verified via `git diff --exit-code -- go.mod go.sum`.
- This plan's own three packages are lint-clean and pass `go test ... -shuffle=on`; the repo-wide `task` gate will need a re-run once plan 04-02's concurrent `internal/store`/`internal/authz` changes land, since that failure is unrelated to and predates this plan's changes.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*

## Self-Check: PASSED

All created/modified files confirmed present on disk (`internal/testhttp/reuse.go`,
`internal/embed/embed.go`, `internal/embed/embed_test.go`, `internal/summarize/summarize.go`,
`internal/summarize/summarize_test.go`, this SUMMARY.md). All three task commits (`ef80ee96`,
`0695a7a8`, `c65984f8`) confirmed present in `git log --oneline --all`.
