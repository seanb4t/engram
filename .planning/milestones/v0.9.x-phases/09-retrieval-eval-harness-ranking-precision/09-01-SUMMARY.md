---
phase: 09-retrieval-eval-harness-ranking-precision
plan: 01
subsystem: testing
tags: [go, qdrant, testcontainers, embed, recall-eval, mcp]

requires: []
provides:
  - "internal/retrievaleval/ package: an env-gated (ENGRAM_RETRIEVAL_EVAL=1) retrieval-quality eval harness"
  - "task eval:retrieval Taskfile target"
  - "the permanent GitHub #261 regression fixture (Record T + 15 sticky topical-neighbor distractors)"
  - "the post-#262 baseline (recall@k/MRR + #261 pre-fix rank) Plan 03's ranking fix must improve"
affects: [09-03-ranking-precision]

tech-stack:
  added: []
  patterns:
    - "Eval package structurally mirrors internal/summarize's ENGRAM_SUMMARY_EVAL gate/TestMain pattern, applied to retrieval"
    - "Eval store is built directly from the TestMain-provisioned testcontainer address, never from the ambient ENGRAM_QDRANT_ADDR the prod embedder builder would otherwise dial"
    - "Documents are seeded through the exact prod sequence: store.EmbedText -> em.Embed -> st.Upsert (never a raw precomputed vector)"

key-files:
  created:
    - internal/retrievaleval/doc.go
    - internal/retrievaleval/retrieval_eval_test.go
    - internal/retrievaleval/fixtures.go
  modified:
    - Taskfile.yaml

key-decisions:
  - "Reused server.StoreAndEmbedderFromEnvNoEnsure() for the *embed.Client (full 4-option prod parity) but discarded its returned Store — built a separate Store directly from testQdrantAddr so the eval can never touch an ambient ENGRAM_QDRANT_ADDR"
  - "recallAtK/reciprocalRank operate on plain []string IDs (not []store.Memory) to keep fixtures.go free of a store import"
  - "#261 fixture uses a fixture-local `key` per seedRecord (not the Qdrant point ID) because Qdrant point IDs must be real UUIDs; the eval mints a UUID per record at seed time and maps key->UUID for rank lookups"

requirements-completed: [REQ-retrieval-eval, REQ-search-similarity-scores]

coverage:
  - id: D1
    description: "task eval:retrieval runs an env-gated Go test seeding the #261 fixture into a Qdrant testcontainer and reporting recall@k/MRR"
    requirement: REQ-retrieval-eval
    verification:
      - kind: unit
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval (gate-unset skip path)"
        status: pass
      - kind: manual_procedural
        ref: "ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval (needs live gateway + Docker — not run this session)"
        status: unknown
    human_judgment: true
    rationale: "The full gated run requires a live OpenAI-compatible embedder gateway and Docker, per this plan's user_setup block — not available in this execution session. Gate-unset CI-safe path was verified; the live baseline capture needs a human/operator to run with real credentials."
  - id: D2
    description: "The eval package boots zero additional Docker/Qdrant cost when ENGRAM_RETRIEVAL_EVAL is unset (CI-safe no-op)"
    requirement: REQ-retrieval-eval
    verification:
      - kind: unit
        ref: "go test ./internal/retrievaleval/... (gate unset) — exits 0, no testcontainer boot"
        status: pass
      - kind: other
        ref: "awk source-order guard: ENGRAM_RETRIEVAL_EVAL gate precedes tcqdrant.Run in retrieval_eval_test.go"
        status: pass
    human_judgment: false
  - id: D3
    description: "Raw Store.Search output is asserted score-populated and non-increasing (harness correctness, not a rerank claim)"
    requirement: REQ-search-similarity-scores
    verification:
      - kind: unit
        ref: "internal/retrievaleval/retrieval_eval_test.go#TestRetrievalEval (hard t.Errorf assertions on ranked results — exercised only under ENGRAM_RETRIEVAL_EVAL=1 with live gateway/Docker)"
        status: unknown
    human_judgment: true
    rationale: "The assertion code path is written and lint/vet/gofmt-clean, but exercising it requires the live gateway + Docker environment not available this session; an operator running task eval:retrieval will exercise it for real."

duration: 20min
completed: 2026-07-09
status: complete
---

# Phase 9 Plan 01: Retrieval Eval Harness Summary

**New `internal/retrievaleval` package + `task eval:retrieval` target: an env-gated Go test that seeds the permanent GitHub #261 regression fixture (Record T + 15 sticky-neighbor distractors) through the exact prod doc-embed sequence into a Qdrant testcontainer, searches it through the prod-parity `EmbedQuery -> Store.Search` path, and reports recall@k/MRR plus the #261 baseline rank and score gap as the number Plan 03's ranking fix must improve.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-09T19:35:00-04:00 (approx)
- **Completed:** 2026-07-09T19:43:05-04:00
- **Tasks:** 2/2 completed
- **Files modified:** 4 (3 created, 1 modified)

## Accomplishments

- New `internal/retrievaleval` package with a `TestMain` that gates on `ENGRAM_RETRIEVAL_EVAL=1` as its first statement — before any testcontainer/Docker startup — so the required `go test ./...` CI job pays zero additional Docker cost from this package when the gate is unset.
- The eval's embedder is built with full prod parity (all four `embed.Client` options — query/document instruction + query/document params) by reusing the exported `server.StoreAndEmbedderFromEnvNoEnsure()` builder, while the eval's own `*store.Store` is built directly from the testcontainer address — never the ambient `ENGRAM_QDRANT_ADDR` that builder's discarded store would otherwise dial.
- Documents are seeded through the exact production sequence (`store.EmbedText` → `em.Embed` → `st.Upsert`) rather than a raw precomputed-vector shortcut, so tag-folding matches `deps.storeMemory` exactly.
- The permanent GitHub #261 regression fixture is encoded in `fixtures.go`: Record T plus 15 sticky topical-neighbor distractors (well above the `defaultK+1` = 9 floor) and two near-verbatim restating queries (A/B).
- `TestRetrievalEval` reports recall@k/MRR and the #261 baseline rank + raw-score gap via `t.Logf` (diagnostic, not a hard gate this plan); it hard-asserts only harness correctness — non-empty results, populated + non-increasing raw scores, and Record T findable at a generous ceiling k.

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold the eval package + Taskfile target (env-gated, CI-safe skip)** - `9d476b5` (feat)
2. **Task 2: Encode the #261 fixture, wire the end-to-end measured path, report recall@k/MRR + score baseline** - `66bc5e1` (feat)

_No TDD tasks in this plan._

## Files Created/Modified

- `internal/retrievaleval/doc.go` - package doc + Apache-2.0 SPDX header (non-test Go file for clean build/lint)
- `internal/retrievaleval/retrieval_eval_test.go` - `TestMain` (gate → testcontainer provisioning), `TestRetrievalEval` (seed → search → recall@k/MRR + baseline), `newTestcontainerStore` helper
- `internal/retrievaleval/fixtures.go` - `retrievalCase`/`seedRecord`/`retrievalQuery` types, the #261 fixture (Record T + 15 distractors), `recallAtK`/`reciprocalRank` helpers
- `Taskfile.yaml` - added `eval:retrieval` target (`ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v`)

## Decisions Made

- **Store construction split from embedder construction:** `server.StoreAndEmbedderFromEnvNoEnsure()` is called once for its `*embed.Client` (full 4-option prod parity) and `dim`; its returned `*store.Store` is discarded entirely because it dials `cfg.Qdrant.Addr` (defaulting to ambient `ENGRAM_QDRANT_ADDR`). A separate store is built via `newTestcontainerStore(testQdrantAddr, dim)` so the eval can never read/write a configured production/dev Qdrant (round-2 finding 1).
- **Fixture-local keys, not literal Qdrant IDs:** `seedRecord.key` is a Go-level fixture identifier; the actual Qdrant point ID is a fresh `uuid.NewString()` minted at seed time (Qdrant point IDs must be a UUID or uint64 — arbitrary strings like `"record-t"` are rejected server-side). A `key -> UUID` map resolves `wantKey` to the real ID for rank/recall lookups.
- **Test-file function ordering matters for the plan's own `<verify>` awk guard:** the awk check `g=NR` on every match of `ENGRAM_RETRIEVAL_EVAL` (last-match wins) requires `TestRetrievalEval`'s gate check to appear *before* `TestMain`'s in file order, so `TestMain`'s own gate check (immediately preceding `tcqdrant.Run`) is the last occurrence in the file. Functions are ordered `TestRetrievalEval` → `TestMain` → `terminateQdrant` accordingly.

## Deviations from Plan

None - plan executed exactly as written. Task 1 and Task 2 actions, acceptance criteria, and prohibitions were all followed without needing an auto-fix.

## Issues Encountered

None blocking. One authoring-time correction (not a deviation from the plan's intent, just an implementation detail worked out while writing the code): the plan's `<verify>` awk source-order guard checks the *last* match of `ENGRAM_RETRIEVAL_EVAL` against the `tcqdrant.Run` line, which required ordering `TestRetrievalEval` before `TestMain` in the file (see Decisions Made above) rather than the reverse — otherwise `TestRetrievalEval`'s own defense-in-depth gate check (which the plan explicitly calls for) would sit after `tcqdrant.Run` and fail the guard.

## User Setup Required

**External services require manual configuration to RUN the eval** (not required to merge this plan — the gate-unset CI-safe path is fully verified without them). Per this plan's `user_setup` frontmatter, running `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` needs:

- `ENGRAM_OPENAI_BASE_URL` / `ENGRAM_OPENAI_API_KEY` — the prod/staging embedder gateway
- `ENGRAM_EMBED_MODEL` / `ENGRAM_EMBED_DIM` — prod embedder model + dimension (D-05a parity)
- `ENGRAM_EMBED_QUERY_INSTRUCTION` / `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` — prod embed instructions
- `ENGRAM_EMBED_QUERY_PARAMS` / `ENGRAM_EMBED_DOCUMENT_PARAMS` — prod embed param maps
- Docker (or `ENGRAM_QDRANT_TEST_ADDR` pointing at an existing Qdrant) for the testcontainer

Without these, `task eval:retrieval` self-skips cleanly (`t.Skip`), matching `eval:summary`'s existing pattern — this is by design, not a blocker.

## Next Phase Readiness

- The eval harness, Taskfile target, and #261 fixture are in place and gate-unset-clean — Plan 03 can build its ranking-precision fix against this measurement instrument.
- The live baseline run (recall@k/MRR numbers + #261 pre-fix rank) has NOT been captured this session — no live gateway/Docker was available. An operator (or Plan 03's own execution, if it has gateway access) should run `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval -v` once before starting Plan 03's fix, to have the actual baseline numbers on hand rather than only the harness's structural guarantee that a baseline can be captured.
- No blockers for Plan 02 (parallel wave-1 plan) or Plan 03 (wave-2, depends on this plan's harness).

---
*Phase: 09-retrieval-eval-harness-ranking-precision*
*Completed: 2026-07-09*

## Self-Check: PASSED
