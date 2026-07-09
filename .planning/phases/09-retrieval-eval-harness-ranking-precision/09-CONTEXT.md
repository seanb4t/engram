# Phase 9: Retrieval Eval Harness & Ranking Precision - Context

**Gathered:** 2026-07-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Make engram's recall **measurable**, **legible**, and **correct** enough that a
near-verbatim restatement of a stored record reliably surfaces that record within
default `k` — closing the GitHub #261 failure where phrasing-sensitive misses and
"sticky topical neighbor" crowding drove `search-before-store` to produce
duplicates.

Three scoped requirements:

- **REQ-retrieval-eval** — a reproducible retrieval-quality eval: a labeled
  query→expected-record dataset (with the #261 miss as a regression fixture),
  `recall@k` / MRR metrics, runnable via `task eval:retrieval`, so ranking and
  embedding changes are **measured, not guessed**.
- **REQ-search-similarity-scores** — `search_memory` surfaces a per-result
  similarity score so callers/agents can gauge a near-miss and the eval can
  assert score separation.
- **REQ-ranking-precision** — eliminate phrasing-sensitive ranking so a
  near-verbatim restatement surfaces its target within default `k`, via the
  approach **the eval numbers select**.

**In scope:** the eval harness + dataset; documenting/asserting the similarity
score; and a ranking fix chosen by eval evidence (light levers first, escalating
to hybrid and/or cross-encoder rerank if the numbers demand it).

**Out of scope (other phases / deferred):** embedder query/document asymmetry
(Phase 10, #305), async-on-write summaries (Phase 11, #320), per-memory usage
signals (Phase 12, #317). Usage signals must **never** affect ranking (Phase 12
constraint) — Phase 9's ranking work must not pre-empt that.

</domain>

<decisions>
## Implementation Decisions

### Similarity Score Exposure (REQ-search-similarity-scores)

**Baseline verified against code (2026-07-09):** the similarity score is *already
shipped end-to-end and always-on* — `store.Memory.Score` is populated from
Qdrant's `ScoredPoint.Score` (`internal/store/store.go` `memoriesFromPoints`),
carried onto the compact recall DTO `recallView.Score` (`json:"score,omitempty"`,
`internal/server/summary.go`), and mapped on the Connect API
(`internal/server/connectapi.go`). It is **undocumented** in the `search_memory`
tool description/schema. REQ-search-similarity-scores is therefore mostly a
document-and-test task, not new API surface.

- **D-01:** Keep the similarity score **always-on** as currently shipped. Do NOT
  add an opt-in flag and do NOT hide it by default — an opt-in gate would be a
  behavior regression. Score stays raw Qdrant cosine similarity (higher = closer),
  `omitempty` (zero/omitted on unranked `list`/`get`).
- **D-02:** Document the always-on score in (a) the `search_memory` tool
  `Description` + the result `jsonschema` (`internal/server/tools.go`), and
  (b) the memory-contract docs (`CLAUDE.md` "Memory contract" + the docs-site
  recall docs).
- **D-03:** The eval (REQ-retrieval-eval) **asserts score separation** between the
  target record and its sticky topical neighbors — this is the machine-checkable
  proof the score is meaningful, per ROADMAP success criterion 2.
- **D-04:** This **supersedes** the ROADMAP Phase-9 success-criterion wording
  "search_memory *can* return a per-result similarity score (**opt-in**)". The
  shipped reality (always-on) is accepted as correct and better DX; record the
  supersession explicitly so #261 traceability stays honest. Planner should note
  this reconciliation in the plan.

### Ranking-Fix Appetite & Guardrails (REQ-ranking-precision)

**Prior #261 work already shipped (verified baseline — do NOT re-solve blindly):**
PR #262 (commit `08a0b979`) landed a *first* #261 phrasing-sensitivity
mitigation: `embed.go` `EmbedQuery` now applies an **asymmetric query-side
instruction** (`ENGRAM_EMBED_QUERY_INSTRUCTION`; empty = raw; `{query}` = literal
template; else `Instruct:<v>\nQuery:<t>`), **query-only so no reindex**. Production
embeds via **qwen3-embedding-8b @4096** (NOT the bge-m3 chart default). #261 was
still deemed open enough to warrant this phase's eval + ranking work. Implication:

- **D-05a:** The eval must **baseline against the current prod config** (qwen3
  @4096, with the query instruction already applied), so it measures the
  *remaining* ranking gap after PR #262 — not a naive symmetric baseline. Any
  ranking fix is judged as an *increment over* the shipped query-instruction
  mitigation, not a from-scratch solution.

- **D-05:** The ranking approach is **chosen by the eval numbers**, per ROADMAP —
  nothing is added speculatively. Every escalation below is **gated on eval
  evidence** (the eval must first show the lighter lever is insufficient).
- **D-06:** **Try light levers first:** higher default `k`, retrieval tuning, and
  an **in-process, dependency-free heuristic rerank** (lexical-overlap boost / MMR
  diversification / score-gap re-scoring). Negligible latency, no new dependency.
- **D-07:** **Hybrid dense+lexical (BM25) fusion is IN SCOPE for Phase 9** if the
  eval shows it is the winning fix. Qdrant is currently **dense-only** (single
  unnamed Cosine vector, `internal/store/store.go` `CreateCollection`), so this
  entails adding a **sparse vector to the collection schema + a reindex/backfill**
  of existing records to populate it. Precedent exists: the `engram reindex`
  command (embedder migration). The reindex boundary/operator ergonomics must be
  respected.
- **D-08:** **A cross-encoder reranker model is ALLOWED in Phase 9** if heuristics
  + hybrid still miss the eval bar. It may add an extra gateway round-trip per
  search and a new **opt-in** `ENGRAM_`-prefixed config surface (koanf registry).
  Keep it opt-in / off the default hot path unless the eval justifies default-on.

### Claude's Discretion

Two gray areas were deliberately **not** locked in discussion — resolve them in
research/planning, honoring the leanings below:

- **Eval harness form & dataset home** → researcher/planner decide.
  **Lean:** match the established env-gated Go-test eval pattern
  (`eval:summary` → `ENGRAM_SUMMARY_EVAL=1 go test ./internal/summarize/ -run
  TestSummaryFidelity`); make the #261 miss a **permanent regression fixture**;
  report `recall@k` / MRR. Decide dataset location (Go `testdata/` fixtures vs a
  new `eval/` corpus) and whether it needs a live Qdrant+embedder (integration,
  like `eval:summary`) or hermetic/recorded fixtures.
- **CI regression gating** → researcher/planner decide.
  **Hard constraint:** `protect-main` requires **8 exact-named** status checks and
  a *skipped* required workflow blocks merge forever. So a required eval gate MUST
  be hermetic (or use a service container); otherwise make it a **non-required**
  job or local-only `task eval:retrieval` (optionally nightly). Never add a
  skipped required workflow, and never rename a required job.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase requirements & goal
- `.planning/ROADMAP.md` — Phase 9 section (goal, 3 success criteria, `Depends on:
  Phase 2, Phase 6`). Note the success-criterion "opt-in" wording is superseded by
  D-04.
- `.planning/REQUIREMENTS.md` — `REQ-retrieval-eval`, `REQ-search-similarity-scores`,
  `REQ-ranking-precision` (lines ~90-92).
- GitHub issue **#261** — the source failure (query A/B near-verbatim restatements
  fail to surface record T within default `k`). **PR #262** (`08a0b979`) shipped a
  first mitigation (asymmetric query-side instruction); read it to understand what
  is already addressed vs what ranking work remains.
- `docs-site/guides/embedding-instructions` — per-model query-instruction guidance
  (from PR #262); relevant to how the eval configures the query side.

### Retrieval code touchpoints (verified baseline)
- `internal/embed/embed.go` — `EmbedQuery` (asymmetric query-side instruction,
  `ENGRAM_EMBED_QUERY_INSTRUCTION`, query-only, PR #262) vs the symmetric document
  path; the eval's query embedding MUST go through `EmbedQuery` for realism.
- `internal/store/store.go` — `Store.Search` (Qdrant `Query`), `memoriesFromPoints`
  (`m.Score = p.Score`), `Memory.Score` field (~line 136-139), `CreateCollection`
  (dense-only `VectorParams`, ~line 220-223), `EmbedText` (tags folded into the
  embedded document).
- `internal/server/tools.go` — `searchArgs` struct (`K`, `Full`, `Tags`), the
  `search_memory` tool registration/description (~line 937), `searchMemory` handler
  (~line 704), `shapeRecall` call site.
- `internal/server/summary.go` — `recallView` DTO (`Score float32
  json:"score,omitempty"`), `shapeRecall`, `toRecallView`.
- `internal/server/connectapi.go` — `Score: m.Score` mapping (~line 41) for the
  Connect read API.
- `Taskfile.yaml` — `eval:summary` target (~line 52) as the eval pattern precedent;
  `task` = lint + test.

### Conventions / guardrails
- `CLAUDE.md` — "Memory contract (stable)" + conventions (buf/proto codegen, SPDX
  headers, `task lint`/`fmt`, koanf `ENGRAM_` config, protect-main branch+PR).
- `.planning/codebase/STACK.md`, `.planning/codebase/ARCHITECTURE.md`,
  `.planning/codebase/TESTING.md` — stack, arch, and testing conventions.
- Qdrant hybrid/sparse-vector + fusion (RRF) capabilities and any cross-encoder
  reranker options are **research territory** — the researcher should fetch current
  Qdrant Go client docs (sparse vectors, `Prefetch`, `Fusion`) and reranker
  approaches. No local doc exists yet.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Similarity score plumbing is done** — `Memory.Score` → `recallView.Score` →
  Connect. REQ-search-similarity-scores reuses this; only docs + eval assertions
  are new.
- **`engram reindex` command** — existing embedder-migration reindex path; the
  hybrid sparse-vector backfill (D-07) can model its operator ergonomics on it.
- **`eval:summary` Go-test pattern** — env-gated `go test` eval is the shape to
  mirror for `eval:retrieval`.
- **OTLP telemetry seam (Phase 6)** — the eval may reuse it for instrumentation
  (ROADMAP `Depends on: Phase 6`).
- **`EmbedText` tag-folding** — tags are already folded into the embedded document,
  so lexical/BM25 term matching interacts with tags; keep in mind for hybrid.

### Established Patterns
- **Dense-only Qdrant** (single unnamed Cosine vector) — hybrid requires a schema
  change + reindex; this is the central cost driver behind D-07.
- **Recall shaping** via `shapeRecall`/`toRecallView` (compact summary default,
  `full=true` opt-in) — any score/rank change flows through here.
- **Config** via `internal/config` koanf registry, `ENGRAM_` prefix — new reranker
  knobs (D-08) live here, not viper.
- **Buf/Connect API** — `Score` is already on the Connect wire; a proto/schema
  touch would go through `proto/` + `task proto:gen` (committed `gen/`).

### Integration Points
- `search_memory` handler (`internal/server/tools.go`) — where higher-`k`,
  heuristic rerank, hybrid fusion, and/or cross-encoder rerank hook in.
- `Store.Search` / `CreateCollection` (`internal/store/store.go`) — where the
  sparse-vector schema + hybrid query (Qdrant `Prefetch` + `Fusion`) land.
- `Taskfile.yaml` — new `eval:retrieval` target; CI wiring under `.github/workflows/`
  (respect protect-main's exact job names).

</code_context>

<specifics>
## Specific Ideas

- The **#261 concrete scenario** is the acceptance anchor: Query A and Query B are
  near-verbatim restatements of Record T; today T does not surface within default
  `k` because sticky topical neighbors crowd it out. The eval must encode exactly
  this as a labeled fixture, and the ranking fix must make T surface within default
  `k` for both queries — with the eval, not intuition, proving it.
- Prefer the eval to also **assert score separation** for the #261 case (target T's
  score vs its crowding neighbors), tying REQ-search-similarity-scores and
  REQ-ranking-precision to one fixture.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (Adjacent recall-quality work is
already routed: embedder asymmetry → Phase 10 #305, async summaries → Phase 11
#320, usage signals → Phase 12 #317.)

</deferred>

---

*Phase: 09-retrieval-eval-harness-ranking-precision*
*Context gathered: 2026-07-09*
