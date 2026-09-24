---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 04
subsystem: search
tags: [jev, reranker, relevance, mcp, cli, connect, d-05, d-06, d-07]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 01
    provides: store.Memory.Relevance *float64, engram.v1.Memory.relevance (proto field 31), memoryToProto's relevance mapping, store.RankHook seam
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 03
    provides: ENGRAM_SEARCH_RANKER-gated production rankHook wiring (unedited by this plan; only its test-time scripted-hook substitution is exercised here)
provides:
  - recallView.Relevance and its toRecallView population — MCP compact search_memory results now carry per-hit relevance
  - renderMemoryTable's data-derived RELEVANCE column (engram search text output) and cli.md documentation of both the column and the JSON field
  - TestRerankParityMCPAndConnect jev-hook subtests proving MCP/Connect order+relevance parity, identical fallback, cross_spine parity, no cross-owner leak through the rank hook, and no response-level flag
affects: [04-05 (search_discovery reranker can reuse this plan's parity-proof shape), 04-06/04-07/04-08 (docs, security review, Helm values)]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 5984
  tasks: 3
  commits: 3
plan_head_before: 317d2cd01b6510ef9821f25b7e41fb58f4735239

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "recallView is a hand-written allow-list (documented in its own doc comment): a new store.Memory field needs BOTH the struct field AND toRecallView's literal touched, or it never reaches the compact MCP shape — Relevance follows this exactly, mirroring AccessCount/LastAccessedAt."
    - "renderMemoryTable's RELEVANCE column is data-derived, not flag-derived: withRelevance = withScore AND slices.ContainsFunc(mems, presence), so a lexical-only response renders byte-identical to before this column existed, and list (withScore=false) can never show it even if a caller somehow attached relevance to a list result."
    - "Rank-hook substitution in tests: d.rankHook is a lowercase deps field, directly settable from the same-package _test.go file for the duration of one subtest via a save/restore t.Cleanup, with no test-only production seam needed."

key-files:
  created: []
  modified:
    - internal/server/summary.go
    - internal/server/summary_test.go
    - cmd/engram/client_common.go
    - cmd/engram/client_search_test.go
    - docs-site/src/content/docs/guides/cli.md
    - internal/server/connectapi_test.go

key-decisions:
  - "TestClientSearchJSONCarriesRelevance decodes each memory as map[string]json.RawMessage rather than a fixed struct, so presence/absence of the relevance key is checked directly per-memory — a whole-line substring check is unsound here because renderJSON emits one single-line JSON object per invocation (Multiline: false), so a per-record line search would false-positive whenever any other record in the same response carries relevance."
  - "The five new jev-hook parity subtests reuse the existing fixture (recTHigh/recFmt/recCI/recOldPython, scope, query) and the existing mcpCtx/actx/mcpCaller/connectIDs closures unmodified — appended as the LAST subtests in TestRerankParityMCPAndConnect, after no_cross_owner_leak_through_reranked_path, so the added actor-B fixture record (seeded only inside its own subtest with a per-subtest t.Cleanup) can never pollute an earlier subtest's result set."
  - "The no-cross-owner-leak subtest asserts the hook was never HANDED actor-B's record (via a seen-ids slice the scripted hook appends to), not merely that it doesn't appear in the output — proving the authz filter runs before the candidate pool ever reaches RankHook, not merely before the response is built."

requirements-completed: [RANK-04, RANK-03]

coverage:
  - id: D1
    description: "MCP search_memory carries relevance on compact (recallView) and full (verbatim store.Memory) results exactly when store.Memory carries it, including a genuine 0"
    requirement: RANK-04
    verification:
      - kind: unit
        ref: "internal/server/summary_test.go#TestToRecallViewCarriesRelevance"
        status: pass
      - kind: unit
        ref: "internal/server/summary_test.go#TestShapeRecallRelevanceFullAndCompact"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram search text output gains a data-derived RELEVANCE column (present only when at least one returned memory carries relevance, printed to four decimals); with no relevance anywhere output is byte-identical to before; engram list never shows it"
    requirement: RANK-04
    verification:
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchTextOutputRelevanceColumn"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientListNeverShowsRelevance"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchTextOutputStateColumn"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram search --output json carries relevance per memory exactly when the server set it, omitting the key otherwise"
    requirement: RANK-04
    verification:
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchJSONCarriesRelevance"
        status: pass
    human_judgment: false
  - id: D4
    description: "cli.md documents the RELEVANCE column and the relevance JSON field, gated on ENGRAM_SEARCH_RANKER=jev, linking the configure guide"
    requirement: RANK-04
    verification:
      - kind: other
        ref: "task lint:markdown"
        status: pass
    human_judgment: true
    rationale: "A markdown lint pass proves the doc is well-formed, not that its content is accurate or complete prose — a human reviewing the rendered guide is the real check for documentation quality."
  - id: D5
    description: "MCP and Connect agree on order and per-hit relevance under the jev hook, both scope-confined and with cross_spine; a failing hook gives both lanes the identical lexical order with no relevance; the hook never surfaces another owner's record; no response-level relevance flag is added"
    requirement: RANK-03
    verification:
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect/jev_hook:_MCP_and_Connect_agree_on_order_and_relevance"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect/jev_hook_error:_both_fall_back_to_the_identical_lexical_order"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect/jev_hook_with_cross_spine"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect/jev_hook_never_surfaces_another_owner's_record"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect/jev_hook_adds_no_response-level_flag"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 4: Per-Hit Relevance on MCP Compact Results, CLI, and Lane Parity Summary

**`recallView` and `engram search` now surface the Jev reranker's per-hit relevance (MCP compact JSON key and a data-derived CLI RELEVANCE column/JSON field), with five new parity subtests proving MCP and Connect agree on order, relevance, fallback, cross_spine, and no cross-owner leak under a scripted rank hook.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-24T14:53:00Z (approximate — immediately after 04-03's completion)
- **Completed:** 2026-09-24T15:00:45Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- `recallView` gained `Relevance *float64 json:"relevance,omitempty"`, populated in `toRecallView` alongside `AccessCount`/`LastAccessedAt` — MCP's compact `search_memory` shape now carries relevance whenever `store.Memory` carries it, including a genuine `0`, and omits the key when nil. `shapeRecall`'s `full=true` path already carried it for free (verbatim `store.Memory`).
- `renderMemoryTable` (used by `engram search`, never `engram list`) gained a data-derived `withRelevance` gate (`withScore` AND at least one memory carries a non-nil `Relevance`): the RELEVANCE column appears between SCORE and SUMMARY only then, each value printed to four decimals, `-` for a memory without it (unreachable under plan 04-01's all-or-nothing rule, kept defensive per the plan's flagged assumption). Every other header/row shape is untouched — a lexical-only response renders byte-identical to before this column existed. The JSON lane needed no code change: `protojson` already omits an unset `optional double` field.
- `cli.md`'s Output contract gained a paragraph documenting the RELEVANCE column and JSON field, gated on `ENGRAM_SEARCH_RANKER=jev`, linking `/guides/configure/#search-reranking-jev`.
- `TestRerankParityMCPAndConnect` gained five `"jev hook..."` subtests (appended last, existing subtests/fixtures untouched): order+relevance parity between MCP and Connect under a scripted hook; identical lexical fallback (no relevance) when the hook errors; parity holds under `cross_spine`; the hook is never handed (not merely never returns) another owner's private record; and the Connect response's populated top-level field set is unchanged with the hook active (no response-level "nothing relevant" flag, per D-06).

## Task Commits

Each task was committed atomically:

1. **Task 1: MCP compact recall carries per-hit relevance** — `ea167dee` (feat, tdd — RED observed: both new tests failed on the missing `relevance` JSON key before `recallView`/`toRecallView` were touched)
2. **Task 2: engram search shows a RELEVANCE column and JSON field; list unchanged; CLI guide** — `bec16177` (feat, tdd — RED observed: the relevance-column test's header assertion failed against the unmodified `renderMemoryTable` before the `withRelevance` branch existed)
3. **Task 3: MCP and Connect agree under the jev hook** — `77644d56` (test — the five subtests were authored directly against the plan's fully-specified `<behavior>`/subtest names; verified via the acceptance-criteria grep gate and the full parent-test run rather than a separate compile-fail RED, since no prior symbol collided)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/server/summary.go` — `recallView.Relevance`, `toRecallView` population
- `internal/server/summary_test.go` — `TestToRecallViewCarriesRelevance`, `TestShapeRecallRelevanceFullAndCompact`
- `cmd/engram/client_common.go` — `renderMemoryTable`'s data-derived RELEVANCE column
- `cmd/engram/client_search_test.go` — `TestClientSearchTextOutputRelevanceColumn`, `TestClientSearchJSONCarriesRelevance`, `TestClientListNeverShowsRelevance`
- `docs-site/src/content/docs/guides/cli.md` — Output contract paragraph on `relevance`
- `internal/server/connectapi_test.go` — five `"jev hook..."` subtests of `TestRerankParityMCPAndConnect`, plus `withRankHook`/`newScriptedRankHook`/`idsOfMCP`/`idsOfConnect` local helpers

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## Deviations from Plan

None — plan executed exactly as written, including the flagged assumption (CLI column formatting: header `RELEVANCE`, four decimals like SCORE, `-` for a memory without relevance inside a relevance-bearing response) as the plan's own explicit, defensive resolution.

## Known Stubs

None.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-02, T-04-09, T-04-10), which this plan's new tests directly exercise: `jev hook never surfaces another owner's record` (T-04-02, both lanes plus the hook's own input), `jev hook adds no response-level flag` (T-04-09, D-06's no-threshold/no-flag rule).

## Self-Check: PASSED

- `internal/server/summary.go` — FOUND
- `internal/server/summary_test.go` — FOUND
- `cmd/engram/client_common.go` — FOUND
- `cmd/engram/client_search_test.go` — FOUND
- `docs-site/src/content/docs/guides/cli.md` — FOUND
- `internal/server/connectapi_test.go` — FOUND
- Commit `ea167dee` — FOUND (`git log --oneline --all | grep ea167dee`)
- Commit `bec16177` — FOUND (`git log --oneline --all | grep bec16177`)
- Commit `77644d56` — FOUND (`git log --oneline --all | grep 77644d56`)
- `go build ./...` — PASS
- `go test ./internal/server/ ./cmd/engram/ -count=1` — PASS
- `task lint:markdown` — PASS (126 files, no issues)
- `golangci-lint run ./internal/server/... ./cmd/engram/...` — 0 issues
- `task license:check` — PASS (2289 files, 0 invalid)
- `rg -o -F 'json:"relevance,omitempty"' internal/server/summary.go | wc -l` — 1
- `rg -o -e '[.]Relevance != nil' cmd/engram/client_common.go | wc -l` — 2 (>= 1 required)
- `rg -o -e 't[.]Run[(]"jev hook' internal/server/connectapi_test.go | wc -l` — 5

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. Relevance surfacing is entirely response-shaping; it activates only when `ENGRAM_SEARCH_RANKER=jev` is already configured per plan 04-03.

## Next Phase Readiness

Ready for 04-05 (`search_discovery` reranker). MCP, Connect, and CLI now agree end-to-end on per-hit relevance for `search_memory`; `TestRerankParityMCPAndConnect`'s jev-hook subtests are a reusable shape for 04-05's own discovery-path parity proof.

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
