# Phase 4: Jev Reranker & Per-Hit Relevance Signal - Context

**Gathered:** 2026-09-24
**Status:** Ready for planning

<domain>
## Phase Boundary

`search_memory` (and `search_discovery`) can reorder candidates by Jev relevance, opt-in, through
the `store.rankCandidates` seam Phase 1 shipped (RANK-03); with the Jev ranker enabled every
surface (MCP, Connect, CLI) carries a per-hit relevance probability (RANK-04); the decision state
stays inside Jev's 32k-token context for candidate pools up to the recall maximum (RANK-05). A
decision error or timeout never fails the search — it falls back to today's shipped order.

Not in this phase: changing the default ranker (lexical stays default), a response-level
"nothing relevant" flag, reranking `list_memory`, write-time hints.

</domain>

<decisions>
## Implementation Decisions

### Enable & ship gate (RANK-03)
- **D-01:** A registered **ranker enum `ENGRAM_SEARCH_RANKER` = `lexical` (default) | `jev`**.
  `jev` requires `ENGRAM_DECISIONS_PROVIDER` to be set; config validation fails clearly
  otherwise (existing `Config.Validate` style). — **Reversibility:** costly — registered operator
  config key.
- **D-02:** **No eval bar for shipping** — the Jev ranker ships as an opt-in regardless of the
  numbers. The Phase 1 retrieval eval's disabled Jev slot is **enabled** (runs when a decisions
  provider is configured) and the live Jev numbers (paraphrase recall@k/MRR, #261 rank) are
  recorded as a phase artifact next to lexical/vector for operators to judge.

### Composition with lexical
- **D-03:** With `ranker=jev`: fetch as today (vector at `CandidateK`), apply the lexical rank
  step, then **stable-sort by Jev P(relevant)** — ties and any Jev failure keep the lexical
  order, so the fallback is exactly today's shipped order.
- **D-04:** The **whole `CandidateK` pool (32–100)** goes to Jev in **one** Decisions request
  (one Noul question per candidate, spike 004 criteria), then truncate to k.

### Relevance signal surface (RANK-04)
- **D-05:** Per-hit **`relevance` float (0–1), omitempty** — present only when the Jev ranker ran
  and succeeded for that search; absent otherwise. Sits beside the existing cosine `score`. MCP
  JSON field, an **additive optional Connect proto field** (buf-generated, gen/ tree regenerated),
  and a CLI column/field in `engram search`.
- **D-06:** **Per-hit only** — no response-level "nothing relevant" flag or threshold; the caller
  decides from the per-hit values.
- **D-07:** Scope: **everything routed through `SearchReranked` (`search_memory` MCP, Connect
  `SearchMemories`, CLI `engram search`, with or without `cross_spine`) plus `search_discovery`**.
  `list_memory` / `get_memory` unchanged.

### Budget & latency (RANK-05)
- **D-08:** Candidate state reuses Phase 3's **`internal/verdict` state construction (summary +
  content head)** with a smaller per-candidate budget (≈600 chars), plus a **total-state guard**
  that shrinks the per-candidate budget so query + up to 100 candidates stay under ≈28k tokens
  (chars/4 estimate with headroom). Candidate content must be fetched through the bounded-read
  primitives (search results may carry only summaries in the default view).
- **D-09:** A **dedicated `ENGRAM_SEARCH_RERANK_TIMEOUT`** (default ≈2s) with **no retry on the
  search path**; on timeout/error the call falls back to lexical order and still succeeds. Sweeps
  (consolidate) keep the decisions timeout and single retry.

### Claude's Discretion
- Exact per-candidate budget and token-estimate constants within D-08; Noul question wording
  (start from spike 004's criteria); how `search_discovery`'s ranking path is threaded to the same
  step; CLI column formatting; OTel attributes for the rerank span (reuse the `decide` span).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/ROADMAP.md` §"Phase 4: Jev Reranker & Per-Hit Relevance Signal"
- `.planning/REQUIREMENTS.md` — RANK-03, RANK-04, RANK-05

### Spike blueprint
- `.claude/skills/spike-findings-engram/references/recall-rerank.md` — Noul-per-candidate request shape, stable sort, no-answer ≈0.02 signal, 32k budget, latency, "don't put Jev on the sync path without a timeout fallback"
- `.claude/skills/spike-findings-engram/references/decision-transport.md`

### Prior phases
- Phase 1 (`.planning/phases/01-eval-foundation-lexical-reranker-fix/`): `store.SearchReranked` → `rankCandidates` seam (D-08 there), `CandidateK`, the named-ranker eval with the disabled Jev stub (`internal/retrievaleval/rankers.go`), `TestRankCandidatesIsTheD05Winner`, `TestRerankParityMCPAndConnect`
- Phase 2 (`.planning/phases/02-decision-interface-jev-backend/`): `internal/decide` (`Decider`, Noul, named errors, `decide.Status`), `jev` client, `deciderFromConfig`, `ENGRAM_DECISIONS_*`
- Phase 3 (`.planning/phases/03-curation-verdicts/`): `internal/verdict` state/truncation, `StoreAndDeciderFromEnv`, bounded `RecordStates` fetch
- engram record `xn69k3mcnz` — corrected rerank facts (lexical beats vector on the live blind eval)

### Code
- `internal/store/store.go` (`SearchReranked`), `internal/store/rerank.go` (`rankCandidates`, `RerankHits`, `CandidateK`)
- `internal/server/tools.go` (MCP `searchMemory`, `search_discovery`), `internal/server/connectapi.go` (`SearchMemories`)
- `proto/engram/v1/` + `gen/` (buf; CI drift check)
- `cmd/engram` search verb

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `rankCandidates` seam (single place both MCP and Connect rank).
- `internal/decide` Noul questions, `decide.Status` for failure class.
- `internal/verdict` state truncation (summary + head).
- Retrieval eval named-ranker roster with a Jev stub already wired to report "disabled".

### Established Patterns
- Authz filtering happens in `Search` before any ranking — ranking never widens visibility.
- Additive-only proto changes with committed `gen/` and `buf` CI drift check.
- Registry-driven docs gates for new `ENGRAM_*` keys.

### Integration Points
- The server's decider (Phase 2 `deps.decider`) must reach the store rank step without making `internal/store` import `internal/decide` if that breaks layering — planner decides the injection point.

</code_context>

<specifics>
## Specific Ideas

- No-answer queries in the Phase 1 paraphrase corpus (the 4 no-answer topics) are exactly the per-hit-relevance demonstration: record their Jev relevance values in the eval artifact.

</specifics>

<deferred>
## Deferred Ideas

- Response-level `no_relevant_results` flag (D-06 declined for now).
- An eval bar gating the Jev option (D-02: opt-in regardless).

</deferred>

---

*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Context gathered: 2026-09-24*
