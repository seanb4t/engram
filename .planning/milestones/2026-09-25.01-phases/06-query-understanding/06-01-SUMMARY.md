---
phase: 06-query-understanding
plan: 01
subsystem: search
tags: [connect-rpc, jev, typed-decisions, query-understanding, proto, internal-decide]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend
    provides: internal/decide (Decider, Noul, Choice, Request/Response, D-12 error classes), internal/decide/jev backend
provides:
  - UnderstandQuery Connect RPC (Connect-only, D-02) with SuggestionSource enum and FilterSuggestion/UnderstandQueryRequest/UnderstandQueryResponse proto messages
  - internal/understand — the third internal/decide consumer (category-question slice: NewRequest, FromResponse, Suggest)
  - internal/server/understand.go — the shared core (deps.understandQuery) and understandResultToProto wire shaping
  - Dedicated no-retry Jev client for query understanding (understandDeciderFromConfig/understandDecider), never shared with consolidate or search rerank
  - Config resolution: search.understanding/_timeout/_audit (D-01/D-01a/D-16), understandingEnabled/understandingTimeout resolvers
  - CSRF read classification for UnderstandQuery; registry rows and docs-gate narrowing
affects: [06-02-suggestions, 06-03-console-row, 06-04-config-disclosure, 06-05-audit-telemetry, 06-06-vendoring]

# Actuals (#2632)
actuals:
  tokens: 42720
  tasks: 3
  commits: 2
  plan_head_before: 10de14ea77259bd2878dc14ac9b49ed7d138e001
  plan_head_after: b85bdd7546a01fe2b673b75d370026d2ed9e41e9

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/understand mirrors internal/relevance's NewRequest/FromResponse/Suggest shape — the third bounded internal/decide consumer"
    - "Per-consumer dedicated no-retry Jev client (consolidate, search rerank, understanding — never shared)"
    - "Understanding-off short-circuits before validation and before any store call (D-04)"

key-files:
  created:
    - internal/understand/understand.go
    - internal/understand/understand_test.go
    - internal/server/understand.go
    - internal/server/understand_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/search_docs_test.go
    - internal/server/decider.go
    - internal/server/decider_test.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/connectcsrf_test.go

key-decisions:
  - "Task 1 decision gate resolved before dispatch: option-a (user-selected 2026-09-28, no item-level changes). Full contract recorded verbatim below."

patterns-established:
  - "Pattern: a new internal/decide consumer package (internal/understand) builds exactly one budgeted Request, maps answers via FromResponse, and never returns an error from its Suggest entrypoint — a decision failure is always 'no decision', never a caller-visible error."
  - "Pattern: the off/disabled short-circuit for an optional decision-backed feature runs as the FIRST statement in the shared core method, before any validation or store call."

requirements-completed: [NLQ-01, NLQ-02]

coverage:
  - id: D1
    description: "UnderstandQuery RPC exists, Connect-only, wired end to end for category suggestions against a real Jev client"
    requirement: "NLQ-02"
    verification:
      - kind: integration
        ref: "internal/server/understand_test.go#TestUnderstandQueryTracer"
        status: pass
    human_judgment: false
  - id: D2
    description: "Understanding-off short-circuit (D-04): zero decision calls, zero store calls, {enabled:false}"
    requirement: "NLQ-01"
    verification:
      - kind: integration
        ref: "internal/server/understand_test.go#TestUnderstandQueryTracer/off"
        status: pass
      - kind: unit
        ref: "internal/server/understand_test.go#TestUnderstandQueryOffNeverDecides"
        status: pass
    human_judgment: false
  - id: D3
    description: "Category question set (D-06) and the 0.9 decision threshold (D-05) mapping"
    requirement: "NLQ-02"
    verification:
      - kind: unit
        ref: "internal/understand/understand_test.go#TestNewRequestCategories"
        status: pass
      - kind: unit
        ref: "internal/understand/understand_test.go#TestFromResponseCategoryThreshold"
        status: pass
      - kind: unit
        ref: "internal/understand/understand_test.go#TestSuggestCategoryPaths"
        status: pass
    human_judgment: false
  - id: D4
    description: "Understanding resolution (D-01/D-01a) via a dedicated no-retry Jev client, independent of consolidate/search-rerank clients"
    requirement: "NLQ-01"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestUnderstandingEnabledResolver"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestUnderstandingTimeoutResolver"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestUnderstandDeciderGate"
        status: pass
    human_judgment: false
  - id: D5
    description: "Registry rows plus docs-gate narrowing (D-01) and CSRF read classification (D-02)"
    verification:
      - kind: unit
        ref: "internal/config/search_docs_test.go#TestSearchVarsDocumented"
        status: pass
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestCSRFWriteProcedureAllowlist"
        status: pass
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestReadRPCsCSRFExempt"
        status: pass
    human_judgment: false
  - id: D6
    description: "Generated Go/TS trees committed and buf-clean; no proto-comment conformance regression"
    verification:
      - kind: other
        ref: "task proto:lint && task proto:gen (no drift) && go tool buf breaking --against '.git#branch=main'"
        status: pass
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-09-28
status: complete
---

# Phase 6 Plan 1: Query Understanding Wire Contract Summary

**UnderstandQuery Connect RPC delivering DECIDED category-chip suggestions through a dedicated no-retry Jev client, with a byte-clean off/on gate and zero MCP/CLI surface (D-02).**

## Performance

- **Duration:** 35 min (estimate — start not explicitly timestamped)
- **Started:** 2026-09-28T13:37:00Z (approx.)
- **Completed:** 2026-09-28T13:58:04Z
- **Tasks:** 3 (1 decision, 1 tracer, 1 expansion)
- **Files modified:** 17

## Accomplishments

- Shipped the `UnderstandQuery` RPC (Connect-only, D-02), the `SuggestionSource` enum, and the `TimeWindowSuggestion`/`FilterSuggestion`/`UnderstandQueryRequest`/`UnderstandQueryResponse` proto messages appended after `ListTags`, buf-lint-clean and non-breaking against `main`.
- Built `internal/understand` — the third `internal/decide` consumer: one Noul question per unanswered console category, a 0.9 probability threshold for a DECIDED suggestion, and a `Suggest` entrypoint that never returns an error (a decision failure degrades to "no decision").
- Wired the full server-side path: `understandingEnabled`/`understandingTimeout` resolvers, a dedicated no-retry Jev client (`understandDeciderFromConfig`/`understandDecider`) never shared with the consolidate or search-rerank clients, the shared `deps.understandQuery` core, and the `UnderstandQuery` Connect handler — proven end to end against a real Jev client and an httptest decisions server (on/off/unauthenticated/oversized subtests).
- Registered the three `ENGRAM_SEARCH_UNDERSTANDING*` config keys, narrowed the reranking docs-gate to exclude them, and classified `UnderstandQuery` as a CSRF-exempt read Procedure — all pinned by behaviour tests.

## Task Commits

Each task was committed atomically:

1. **Task 1: DECISION — the UnderstandQuery wire contract (one-way)** — resolved before dispatch (see Decisions Made below); no code commit of its own.
2. **Task 2: UnderstandQuery end to end for category suggestions** — `d0e60e95` (feat)
3. **Task 3: Registry rows, docs-gate narrowing, CSRF classification, unit tests** — `b85bdd75` (test)

**Plan metadata:** committed alongside this SUMMARY (docs commit follows).

## Files Created/Modified

- `proto/engram/v1/engram.proto` — SuggestionSource enum, TimeWindowSuggestion/FilterSuggestion/UnderstandQueryRequest/UnderstandQueryResponse messages, UnderstandQuery RPC
- `gen/go/engram/v1/engram.pb.go`, `gen/go/engram/v1/engramv1connect/engram.connect.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` — regenerated, committed, buf-clean
- `internal/config/config.go` — `SearchConfig.Understanding`/`UnderstandingTimeout`/`UnderstandingAudit`
- `internal/config/registry.go` — the three `search.understanding*` registry rows
- `internal/config/search_docs_test.go` — `searchRegistryEnvNames` excludes `ENGRAM_SEARCH_UNDERSTANDING*`
- `internal/server/decider.go` — `understandingEnabled`, `understandingTimeout`, `understandDeciderFromConfig`, `understandDecider`
- `internal/server/decider_test.go` — `TestUnderstandingEnabledResolver`, `TestUnderstandingTimeoutResolver`, `TestUnderstandDeciderGate`
- `internal/server/tools.go` — `deps.understandDec` field, wired in `buildDepsFromEnv`
- `internal/understand/understand.go` — the category-suggestion core (new package)
- `internal/understand/understand_test.go` — `TestNewRequestCategories`, `TestFromResponseCategoryThreshold`, `TestSuggestCategoryPaths`
- `internal/server/understand.go` — `understandArgs`/`understandResult`, `deps.understandQuery`, `understandResultToProto` (new file)
- `internal/server/understand_test.go` — `TestUnderstandQueryTracer`, `TestUnderstandQueryOffNeverDecides` (new file)
- `internal/server/connectapi.go` — `engramAPI.UnderstandQuery` Connect handler
- `internal/server/connectcsrf_test.go` — `UnderstandQuery` added to the read-Procedure allowlist and `TestReadRPCsCSRFExempt`

## Decisions Made

**Task 1 checkpoint (already resolved before dispatch): option-a, no item-level changes (2026-09-28).** Full chosen contract, recorded verbatim per the plan's acceptance criterion:

> A (recommended): the bundle in this plan's Artifacts table, plus these semantics. (1) Names and numbers exactly as the Artifacts table (research sketch, RelatedEdge-style oneof named `kind`; enum values prefixed `SUGGESTION_SOURCE_` per buf lint). (2) time_window: created_after is the UTC midnight of the request day minus the bucket span — today 0 days, past_week 7 days, past_month one calendar month (AddDate(0,-1,0)), past_year one calendar year — formatted RFC3339 as YYYY-MM-DDT00:00:00Z; created_before is always empty; label is "today", "past week", "past month" or "past year". This is byte-identical to what FacetStrip's created-after date input writes, so an accepted time chip is indistinguishable from a manual one (D-11), and the response is stable across a day (cache-friendly). (3) Suggestion order: categories in console order (convention, gotcha, decision, preference), then time_window, then scope, then tags in ListTags order (count descending, tag ascending); at most 8 tag suggestions. (4) Request validation in the shared core: categories or tags longer than 1000 entries reject with field=categories|tags hint=out_of_range (rejectOverMaximumCount); an empty or whitespace query is not an error (enabled true, zero suggestions); the query is truncated to 2000 runes before the decision call; scope and the window bounds are used only for "is it applied" checks and are not parsed. (5) cross_spine is carried per D-03 but only a non-empty scope counts as "scope applied".

Cons acknowledged and accepted: a "past week" chip starts at a midnight up to 24h earlier than now-7d; the day granularity is coarser than D-07's "relative to request time" read literally. This plan (06-01) implements only the category-suggestion slice (D-05/D-06); time-window/scope/tag suggestions (items 2-5 above) are scoped to plan 06-02 per the plan's own objective.

No other deviations — plan executed exactly as written.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. `ENGRAM_SEARCH_UNDERSTANDING` remains unset in any existing deployment unless `ENGRAM_DECISIONS_PROVIDER=jev` is already configured (D-01 default-follows-provider behavior), matching the plan's threat mitigation for T-06-01.

## Next Phase Readiness

- The wire contract, config resolution, dedicated client, `internal/understand` package, shared core, and Connect handler are all in place and tested — plan 06-02 can add time-window, scope, and tag suggestions on the same seams without an architectural change.
- No blockers. `buf breaking` is clean against `main`, so the proto additions are safe for the generated TS client plan 06-03 will consume.

## TDD Note

Task 3 carries `tdd="true"` but tests only functionality already implemented and committed in Task 2 (the tracer task) — per the plan's own design, Task 2 ships production-quality behavior first and Task 3 backfills resolver/mapping edge-case unit tests. No RED phase was applicable (the implementation already existed), so Task 3 was committed as a single `test(...)` commit exactly as the plan's action text directs.

---

## Self-Check: PASSED

- FOUND: internal/understand/understand.go
- FOUND: internal/understand/understand_test.go
- FOUND: internal/server/understand.go
- FOUND: internal/server/understand_test.go
- FOUND commit d0e60e95 (git log --oneline --all)
- FOUND commit b85bdd75 (git log --oneline --all)
- All plan `<verification>` commands re-run and green: `go test ./internal/understand/ ./internal/config/ -count=1` (ok), `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1` (ok), `task proto:lint`/`task proto:gen` (no drift), `go tool buf breaking --against '.git#branch=main'` (clean), `go test ./internal/surfaces/ -run '^TestSurfaceConformanceProseFiles$'` (PASS)
- All task-level `<acceptance_criteria>` commands re-run and match expected output (see Task 2/3 acceptance criteria checks above)

---
*Phase: 06-query-understanding*
*Completed: 2026-09-28*
