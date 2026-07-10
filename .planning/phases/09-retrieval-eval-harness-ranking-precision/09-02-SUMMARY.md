---
phase: 09-retrieval-eval-harness-ranking-precision
plan: 02
subsystem: docs
tags: [mcp-tools, search-memory, qdrant, ranking, documentation]

# Dependency graph
requires:
  - phase: 09-retrieval-eval-harness-ranking-precision (plan 01)
    provides: internal/retrievaleval package (labeled retrieval eval harness) — untouched by this plan
provides:
  - Documented always-on search_memory `score` field across all three client-visible surfaces (tool Description, CLAUDE.md, docs-site reference)
  - Recorded D-04 supersession of ROADMAP Phase-9 success-criterion 2 "opt-in" wording
affects: [09-03 (reranking — must extend reference/tools.md:95 order caveat without walking back this plan's order-agnostic score wording)]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - CLAUDE.md
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "D-04 supersession recorded: ROADMAP Phase-9 success-criterion 2 wording 'search_memory can return a per-result similarity score (opt-in)' is superseded — the shipped behavior is always-on (score present whenever non-zero), which is accepted as correct and better DX. No ROADMAP text was rewritten; the supersession is captured here for #261 traceability."
  - "Score FIELD semantics only were documented (raw Qdrant cosine similarity, higher = closer); result ORDER-is-score-descending was deliberately NOT asserted, per round-2 finding 5 — that caveat is owned by 09-03 (introduces reranking, updates reference/tools.md:95 'ranked by vector similarity')."

patterns-established: []

requirements-completed: [REQ-search-similarity-scores]

coverage:
  - id: D1
    description: "search_memory tool Description (MCP client-visible prose) documents the always-on score field: cosine similarity, higher = closer, present when non-zero, omitted on unranked list/get results."
    requirement: "REQ-search-similarity-scores"
    verification:
      - kind: other
        ref: "go build ./... && rg -n 'score' internal/server/tools.go (confirms cosine/similarity wording present; handler closure signature unchanged)"
        status: pass
    human_judgment: false
  - id: D2
    description: "CLAUDE.md Memory-contract section and docs-site reference/tools.md search_memory section both document the score field consistently with the tool Description and the existing guides/embedding-instructions.md mention."
    requirement: "REQ-search-similarity-scores"
    verification:
      - kind: other
        ref: "rg -n 'score' docs-site/src/content/docs/reference/tools.md CLAUDE.md; golangci-lint run ./internal/server/...; task license:check"
        status: pass
    human_judgment: false

# Metrics
duration: 15min
completed: 2026-07-09
status: complete
---

# Phase 9 Plan 02: Document Always-On search_memory Score Summary

**Closed the search_memory score documentation gap across the tool Description, CLAUDE.md, and the docs-site reference — pure prose, zero code-behavior change — and recorded the D-04 ROADMAP supersession.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-09
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Extended the `search_memory` tool `Description` string in `internal/server/tools.go` to state that each result carries a `score` (raw Qdrant cosine similarity, higher = closer, present when non-zero; unranked `list_memory`/`get_memory` results have a zero/omitted score) — the only durable client-visible channel since the tool closure is `Out=any` and go-sdk omits an auto-generated output schema.
- Added the same score semantics to `CLAUDE.md`'s "Memory contract (stable)" section, next to the existing Recall sentence.
- Extended `docs-site/src/content/docs/reference/tools.md`'s `search_memory` section (after the "Returns" line) with the score field description, closing the confirmed gap there and matching the informal mention already present in `guides/embedding-instructions.md`.
- Recorded the D-04 supersession: ROADMAP Phase-9 success-criterion 2 ("search_memory can return a per-result similarity score (opt-in)") is superseded by the shipped always-on behavior. The ROADMAP text itself was not edited — the supersession is on the record here for GitHub #261 traceability.

## Task Commits

Each task was committed atomically:

1. **Task 1: Document the always-on score in the search_memory tool Description** - `d4e9b19` (docs)
2. **Task 2: Document the score in CLAUDE.md and the docs-site tools reference; record the D-04 supersession** - `ad631cd` (docs)

**Plan metadata:** (final commit follows this SUMMARY)

## Files Created/Modified
- `internal/server/tools.go` - Extended `search_memory` tool `Description` string with score-field semantics (prose only; handler closure signature unchanged: `func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error)`)
- `CLAUDE.md` - Memory-contract section now states `search_memory` results carry an always-on `score`
- `docs-site/src/content/docs/reference/tools.md` - `search_memory` section documents the returned `score` field, mirroring `guides/embedding-instructions.md`

## Decisions Made
- D-04 supersession recorded (see `key-decisions` above): ROADMAP "opt-in" wording is superseded by shipped always-on behavior, accepted as correct.
- Deliberately stayed order-agnostic: this plan documents the `score` FIELD only, not that result ORDER is always score-descending. That caveat (post-rerank order may be non-monotonic relative to `score`) is explicitly deferred to 09-03, which introduces the reranker and will update `reference/tools.md:95`'s "ranked by vector similarity" wording. This avoids 09-03 having to walk back 09-02 prose.

## Deviations from Plan

None - plan executed exactly as written. No new score plumbing was added (it already ships end-to-end: `store.Memory.Score` → `recallView.Score` `json:"score,omitempty"` → the Connect read API); this plan was documentation-only per its explicit prohibitions.

## Issues Encountered

None. `go build ./...`, `golangci-lint run ./internal/server/...`, and `task license:check` all passed clean on the first attempt. `rumdl check` on `CLAUDE.md` passed with zero issues; `docs-site/` is intentionally excluded from `rumdl` (`.rumdl.toml`: "Astro/Starlight site — MDX + generated output, not plain prose"), so no markdown-lint gate applies to the docs-site file touched here. A full `task lint` run surfaces 132 pre-existing markdown-lint issues in unrelated files (`09-CONTEXT.md`, `09-PATTERNS.md`, `09-RESEARCH.md`, `09-01-SUMMARY.md` from the prior plan) — confirmed pre-existing (identical count with this plan's diff stashed out) and out of scope per the deviation-rules scope boundary; logged here rather than fixed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- REQ-search-similarity-scores documentation half is closed; the score is legible to MCP clients and consistent across all three doc surfaces.
- 09-03 (reranking, wave 2, `depends_on: [09-01, 09-02]`) can proceed: it owns the `reference/tools.md:95` "ranked by vector similarity" → post-rerank order caveat, building on this plan's order-agnostic score wording without conflict.

---
*Phase: 09-retrieval-eval-harness-ranking-precision*
*Completed: 2026-07-09*

## Self-Check: PASSED

All created/modified files verified present on disk; both task commit hashes (`d4e9b19`, `ad631cd`) verified in `git log`.
