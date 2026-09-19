---
phase: 02-error-classification-resourceexhausted-mapping
plan: 02
subsystem: api
tags: [connect, mcp, grpc, error-classification, resourceexhausted, mcp-middleware]

requires:
  - phase: 02-error-classification-resourceexhausted-mapping (plan 01)
    provides: "store.ErrResponseTooLarge sentinel and store.ResponseTooLargeError, classified once in NewQdrantClient's base dial options"
provides:
  - "Connect connectError arm: errors.Is(err, store.ErrResponseTooLarge) -> connect.CodeResourceExhausted carrying the shared field=response hint=too_large envelope, never CodeInternal"
  - "MCP addToolMiddleware(s, record): the single tools/call middleware registration (instrumentTools outermost, mapResponseTooLarge innermost) — the same envelope for every tool"
  - "argerror.go's renderHintEnvelope(fields, hint, detail) — the ONE renderer both lanes and argError.Error() itself now call"
  - "HintTooLarge (11th HintCode, \"too_large\")"
affects: ["02-03", "02-04"]

actuals:
  tokens: 9473
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "One shared wire-envelope renderer (renderHintEnvelope) called from both the Connect error arm and the MCP receiving middleware, and by argError.Error() itself, so no lane ever hand-builds a second field=/hint= string."
    - "MCP error mapping as a second receiving middleware, edited into the existing single AddReceivingMiddleware call (instrumentTools outermost, mapResponseTooLarge innermost) rather than a second registration call — go-sdk v1.8.0 applies the variadic list backwards."
    - "go/parser source-gate test (mirroring conditionalsweep_test.go's house style) pinning a middleware registration's call graph structurally, not just its behavior."

key-files:
  created:
    - internal/server/responsetoolarge.go
    - internal/server/responsetoolarge_test.go
  modified:
    - internal/server/argerror.go
    - internal/server/connecterror.go
    - internal/server/connecterror_test.go
    - internal/server/instrument.go
    - internal/server/tools.go

key-decisions:
  - "argError.Error()'s string-building was extracted into renderHintEnvelope(fields, hint, detail) rather than left inline — both the new Connect arm and the new MCP mapper call it directly, and the sentinel is never constructed as (or wrapped by) an *argError, so it never enters that type's Unwrap()/ConnectCode() machinery (which would misclassify it as CodeInvalidArgument)."
  - "The Connect arm sits after the existing context.DeadlineExceeded case, still strictly after the errors.As(err, &ae) case the doc comment requires stay first — verified by TestConnectErrorStaleSummaryDistinctFromMalformed-style distinctness assertions, not just a 'not CodeInternal' check (durable record 667p88n2be)."
  - "addToolMiddleware(s, record) is the ONE registration site for the tool-call middleware stack, called by both Register and every test — a go/parser source gate (TestRegisterInstallsToolMiddleware) pins Register's single call to it and its own single AddReceivingMiddleware call listing instrumentTools then mapResponseTooLarge, in that order, so the ordering can't silently drift."
  - "responsetoolarge.go was authored in full (including mapResponseTooLarge, needed by Task 2) during Task 1's commit rather than split exactly at the plan's per-task file boundary — Task 1's own RED/GREEN cycle still exercised only the Connect arm (genuine RED observed with the arm absent), and Task 2's RED/GREEN cycle still exercised genuine RED by temporarily unregistering the mapper from addToolMiddleware, so both tasks' TDD evidence is real; only the file-authorship boundary shifted, not the behavior under test."

requirements-completed: [REQ-exhausted-connect, REQ-exhausted-mcp]

coverage:
  - id: D1
    description: "A real 4 MiB-named overflow through Connect's ListMemories, reached over real HTTP via the production mountConnect interceptor chain, returns resource_exhausted with the shared envelope and no leaked byte count or upstream grpc/Qdrant text"
    requirement: "REQ-exhausted-connect"
    verification:
      - kind: integration
        ref: "internal/server#TestConnectListMemoriesResponseTooLarge"
        status: pass
    human_judgment: false
  - id: D2
    description: "The Connect wire message equals the envelope byte-for-byte after the HTTP round trip and parses under the field=/hint= grammar; connectError(ctx, nil) stays nil and an empty *store.ResponseTooLargeError maps to the identical envelope"
    requirement: "REQ-exhausted-connect"
    verification:
      - kind: unit
        ref: "internal/server#TestConnectError/response_too_large_scrubbed"
        status: pass
      - kind: unit
        ref: "internal/server#TestConnectError/response_too_large_empty_payload"
        status: pass
      - kind: unit
        ref: "internal/server#TestConnectError/nil_is_nil"
        status: pass
    human_judgment: false
  - id: D3
    description: "CodeResourceExhausted is distinct from CodeInvalidArgument, CodeOutOfRange, CodeFailedPrecondition and CodeInternal; a server-sent ResourceExhausted the classifier did not relabel still maps to CodeInternal with the generic message"
    requirement: "REQ-exhausted-connect"
    verification:
      - kind: unit
        ref: "internal/server#TestConnectError/response_too_large_distinct_from_arg_classes"
        status: pass
      - kind: unit
        ref: "internal/server#TestConnectError/server_resource_exhausted_not_relabeled"
        status: pass
    human_judgment: false
  - id: D4
    description: "A real 4 MiB-named overflow through the real list_memory MCP tool, over an in-memory transport against a server built with addToolMiddleware and registerTools, returns IsError=true with exactly one text content equal to the shared envelope"
    requirement: "REQ-exhausted-mcp"
    verification:
      - kind: integration
        ref: "internal/server#TestMCPListMemoryResponseTooLarge"
        status: pass
    human_judgment: false
  - id: D5
    description: "The raw error (method + upstream grpc text) is logged server-side exactly once, at ERROR level, and never on the wire, on both lanes"
    requirement: "REQ-exhausted-connect"
    verification:
      - kind: integration
        ref: "internal/server#TestConnectListMemoriesResponseTooLarge (captureSlog assertions)"
        status: pass
      - kind: integration
        ref: "internal/server#TestMCPListMemoryResponseTooLarge (captureSlog assertions)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every other MCP tool error (argError, store.ErrNotFound, an unclassified ResourceExhausted), every non-tools/call method, every non-error result, and a Go-level error from next pass through mapResponseTooLarge completely unchanged"
    requirement: "REQ-exhausted-mcp"
    verification:
      - kind: unit
        ref: "internal/server#TestMapResponseTooLargePassesOtherResultsThrough"
        status: pass
    human_judgment: false
  - id: D7
    description: "addToolMiddleware registers instrumentTools outermost and mapResponseTooLarge innermost in ONE AddReceivingMiddleware call; Register calls addToolMiddleware and nothing else adds receiving middleware; instrumentTools still records outcome=error for the mapped list_memory result"
    requirement: "REQ-exhausted-mcp"
    verification:
      - kind: unit
        ref: "internal/server#TestRegisterInstallsToolMiddleware"
        status: pass
      - kind: integration
        ref: "internal/server#TestMCPListMemoryResponseTooLarge (toolCallRecorder assertion)"
        status: pass
    human_judgment: false
  - id: D8
    description: "renderHintEnvelope is the ONE renderer: argError.Error() and responseTooLargeEnvelope() both call it, no *argError is ever built for the sentinel, and every existing argError grammar test stays green byte-for-byte; HintTooLarge is the 11th HintCode constant"
    verification:
      - kind: unit
        ref: "internal/server#TestArgErrorGrammar"
        status: pass
      - kind: unit
        ref: "internal/server#TestResponseTooLargeEnvelopeShape"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-19
status: complete
plan_head_before: 69110cd0c803492733916315ad5e0ae6b0019adc
commits: 3
---

# Phase 2 Plan 2: Map ErrResponseTooLarge Across the Connect and MCP Lanes Summary

**Both server lanes now scrub a real receive-limit overflow to the single `field=response hint=too_large: ...` envelope — Connect returns `resource_exhausted` instead of `internal`, MCP's `list_memory` (and every other tool) returns `IsError` with the same text — via one shared renderer and one new MCP receiving middleware, with the raw error logged server-side exactly once per lane.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-09-19
- **Completed:** 2026-09-19
- **Tasks:** 3 completed
- **Files modified:** 7 (2 created, 5 modified)

## Accomplishments

- `internal/server/argerror.go` — extracted `renderHintEnvelope(fields, hint, detail)` as the ONE field/hint grammar renderer; `argError.Error()` now delegates to it; added the 11th `HintCode`, `HintTooLarge = "too_large"`.
- `internal/server/responsetoolarge.go` (new) — `responseTooLargeDetail`, `responseTooLargeEnvelope()` (the shared envelope both lanes render), and `mapResponseTooLarge() mcp.Middleware` (the MCP-side mapper).
- `internal/server/connecterror.go` — new `connectError` arm: `errors.Is(err, store.ErrResponseTooLarge)` → `connect.CodeResourceExhausted` carrying the scrubbed envelope, logging the raw error first; the `errors.As(err, &ae)` case stays first.
- `internal/server/instrument.go` — `addToolMiddleware(s, record)`, the single registration of the tool-call middleware stack (`instrumentTools` outermost, `mapResponseTooLarge` innermost); `internal/server/tools.go`'s `Register` now calls it (one line changed, no tool closure touched).
- `internal/server/responsetoolarge_test.go` (new) — `TestConnectListMemoriesResponseTooLarge` and `TestMCPListMemoryResponseTooLarge` (real 4 MiB-named overflows, real HTTP / in-memory transport, both RED-then-GREEN), `TestRegisterInstallsToolMiddleware` (go/parser source gate), `TestMapResponseTooLargePassesOtherResultsThrough` (7 pass-through subtests), `TestResponseTooLargeEnvelopeShape`, and the `captureSlog` test helper.
- `internal/server/connecterror_test.go` — five new `TestConnectError` rows/subtests: scrubbing, empty-payload identity, non-relabeled server-side exhaustion, and explicit code-distinctness (durable record `667p88n2be`).

## Task Commits

Each task was committed atomically:

1. **Task 1: Connect end to end** — `9e32210e` (feat)
2. **Task 2: MCP end to end** — `03ef104a` (feat)
3. **Task 3: The mappers' edges** — `fa6bb610` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/server/argerror.go` — `HintTooLarge` constant, `renderHintEnvelope` extraction.
- `internal/server/responsetoolarge.go` — the shared envelope + the MCP mapper.
- `internal/server/connecterror.go` — the new Connect arm.
- `internal/server/instrument.go` — `addToolMiddleware`.
- `internal/server/tools.go` — `Register` calls `addToolMiddleware`.
- `internal/server/responsetoolarge_test.go` — every new test for both lanes plus the source gate.
- `internal/server/connecterror_test.go` — the five new `TestConnectError` rows/subtests.

## Decisions Made

See `key-decisions` in frontmatter — the renderer extraction, the Connect arm's ordering relative to the `*argError` case, `addToolMiddleware` as the one registration seam pinned by a source gate, and the deliberate Task-1/Task-2 file-authorship overlap in `responsetoolarge.go` (documented as a deviation below, not a defect).

## Deviations from Plan

### Auto-fixed Issues

**1. [Organizational — file-authorship boundary, not a bug] `mapResponseTooLarge` authored in Task 1's commit of `responsetoolarge.go` instead of split into Task 2**
- **Found during:** Task 1 (writing `responsetoolarge.go` in one pass)
- **Issue:** The plan's per-task file breakdown places `mapResponseTooLarge` in Task 2's action item, but the whole file was written in a single pass during Task 1 since it was more natural to write the file once.
- **Fix:** No functional change needed — Task 1's commit still only *exercised* the Connect arm (its RED/GREEN cycle genuinely failed/passed on the Connect assertion alone), and Task 2's RED/GREEN cycle independently proved the MCP wiring by temporarily removing `mapResponseTooLarge` from `addToolMiddleware`'s registration (not from the file) and observing the real raw-text leak, then restoring it. Both tasks' TDD evidence is genuine; only which commit's diff *added the function's text* shifted.
- **Files modified:** `internal/server/responsetoolarge.go` (all in Task 1's commit `9e32210e`)
- **Verification:** Task 1's and Task 2's RED transcripts below are both genuine, independent observations.
- **Committed in:** `9e32210e` (Task 1 commit)

---

**Total deviations:** 1 organizational (0 auto-fixed bugs/missing-functionality/blockers). **Impact:** None on behavior or test genuineness — every RED/GREEN cycle described in the plan was independently, genuinely observed.

## RED Evidence (per task)

**Task 1** — with the envelope helpers in place but the Connect arm absent (temporarily removed, then restored):

```
=== RUN   TestConnectListMemoriesResponseTooLarge
    responsetoolarge_test.go:137: storetest: seeded few-large fixture: 40 records, 131072 bytes/record, 5242880 total bytes, limit 4194304 bytes, in 50.3145ms
    responsetoolarge_test.go:170: ListMemories: code = internal, want CodeResourceExhausted
--- FAIL: TestConnectListMemoriesResponseTooLarge (0.32s)
```

**Task 2** — with `addToolMiddleware` registering only `instrumentTools` (mapper temporarily unregistered, then restored):

```
=== RUN   TestMCPListMemoryResponseTooLarge
    responsetoolarge_test.go:256: storetest: seeded few-large fixture: 40 records, 131072 bytes/record, 5242880 total bytes, limit 4194304 bytes, in 65.159958ms
    responsetoolarge_test.go:307: CallTool: Text = "Scroll() failed: server_mem_eval_test: qdrant response exceeded the client's receive limit: /qdrant.Points/Scroll: grpc: received message after decompression larger than max 4194304", want "field=response hint=too_large: the result is too large to return in one response; retry with a smaller limit or k, or omit full"
--- FAIL: TestMCPListMemoryResponseTooLarge (0.39s)
```

**Task 3** — pure test-additions over already-correct Task 1/2 production code (per the plan's own framing: "change production code only if a row exposes a defect"); all rows passed on first run — no defect found, so no RED cycle applies.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02-03 (CLI exit code + `docs-site/reference/errors.md`) can now map `connect.CodeResourceExhausted` to `exitTooLarge` and document the 11th hint code — both server-side mappings this plan built are in place and green.
- Plan 02-04 (this phase's own red-evidence registration) can register RED patches against `TestConnectListMemoriesResponseTooLarge` (Connect arm absent) and `TestMCPListMemoryResponseTooLarge` (mapper unregistered) — both patches would reproduce the exact RED transcripts captured above.
- `internal/server` and `internal/store` stay green under `ENGRAM_REQUIRE_QDRANT=1`; `TestRedEvidencePatchesAreLive` (Phase 1's own registrations) still passes unaffected; `go test ./internal/keylinks/ -count=1` passes.
- `task lint` and `task license:check` are clean repo-wide; `go.mod`/`go.sum` byte-unchanged — zero new Go dependencies.
- No blockers or concerns for plan 02-03.

---
*Phase: 02-error-classification-resourceexhausted-mapping*
*Completed: 2026-09-19*

## Self-Check: PASSED

All 7 files (2 created, 5 modified) verified present on disk; all three task commits (`9e32210e`, `03ef104a`, `fa6bb610`) verified present in `git log --oneline --all`. Every acceptance criterion for all three tasks re-run and confirmed passing (`HintTooLarge`/`HintCode` counts, `renderHintEnvelope` call count, the Connect arm's exact expression and case ordering, the `addToolMiddleware`/`Register` wiring, `git diff --numstat` on `tools.go` = `1 1`, the `argErrf(classOutOfRange, HintTooLong` literal, 7 pass-through subtests). `go vet ./internal/server/...` and `golangci-lint run ./internal/server/...` both clean. The full plan-level `<verification>` block passes: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/store/... -count=1` ok; `task lint` all green; `git diff --exit-code HEAD -- go.mod go.sum` exits 0; `go test ./internal/keylinks/ -count=1` ok; `TestRedEvidencePatchesAreLive` still passes.
