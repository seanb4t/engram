# Requirements: engram — Milestone `2026-09-22.01` Typed Decisions & Recall Ranking

**Defined:** 2026-09-22
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** engram gains a provider-neutral, advisory-only typed-decision capability (Jev
as the first backend) that measurably improves curation and recall, and the lexical reranker's
paraphrase regression (#605) is fixed.

> **Research basis.** No domain research pass; spikes 001–004 (`.planning/spikes/`) validated the
> capability live, and the `spike-findings-engram` skill is the implementation blueprint. Jev is
> reachable at `https://openrouter.ai/api/alpha/decisions` (OpenRouter key) and at
> `https://llm.fzymgc.house/openrouter/alpha/decisions` (LiteLLM pass-through, per-key
> `allowed_passthrough_routes` grant; engram's key is granted — selfhosted-cluster PR #2227).

## v1 Requirements

### Decision interface

- [x] **DEC-01**: Operator can enable a typed-decision provider through `ENGRAM_` config; it is off by default, and with it off no decision call is made and behavior is byte-identical to today.
- [x] **DEC-02**: Callers use a provider-neutral Go interface speaking the System One contract (shared state plus batched Choice/Score/Noul questions in; typed answers with probabilities and confidence out), so a second backend (the chat-LLM emulator) can be added without changing callers.
- [x] **DEC-03**: The Jev backend calls `{base}/alpha/decisions` with its own base-URL, API-key, model (default pinned `typesafe/jev-1.13`) and timeout settings, each falling back to the shared OpenRouter values, and works against OpenRouter directly (`https://openrouter.ai/api`) and through the LiteLLM pass-through (`https://llm.fzymgc.house/openrouter`).
- [x] **DEC-04**: Decision calls are bounded (timeout, response bytes, drain); failures are classified by HTTP status into named errors (OpenRouter and LiteLLM error bodies differ), and a decision failure never fails the surrounding read or sweep.
- [x] **DEC-05**: The OpenRouter Go SDK for the Decisions API is evaluated (maintained upstream, current module path) and adopted or rejected with the rationale recorded, before any hand-written client.
- [x] **DEC-06**: Decision calls emit OTLP spans carrying latency, question count, input tokens and cost.

### Curation verdicts

- [ ] **CUR-01**: Operator running `engram spine-review consolidate` with decisions enabled sees, per candidate pair, a relation verdict (`duplicate` / `contradicts` / `updates` / `related` / `unrelated`), its probability, and a same-subject probability in `--output json` and the text view.
- [ ] **CUR-02**: Verdicts below a configurable confidence threshold (default 0.9) are marked needs-review, and consolidate never mutates a record because of a verdict.
- [ ] **CUR-03**: The relation question set (including `updates`) is measured on a labeled pair eval that commits no verbatim spine content, reporting accuracy by confidence bucket and a Brier score.
- [x] **CUR-04**: Operator running `consolidate` with neither `--scope` nor `--all-scopes` gets the scope-or-all-scopes rule error instead of a silent zero-candidate report (#508).

### Ranking

- [x] **RANK-01**: The retrieval eval includes an independently written paraphrase case alongside #261, reporting recall@k and MRR for vector-only, lexical, and (when enabled) Jev ordering (#605).
- [x] **RANK-02**: The default ranking does not regress the paraphrase case versus vector-only order and keeps #261's target at rank 1 — the lexical reranker is kept, demoted, or replaced on the RANK-01 numbers (#605).
- [ ] **RANK-03**: Agent calling `search_memory` with the Jev reranker enabled gets candidates reordered by relevance probability; on decision error or timeout the results fall back to the default order and the call still succeeds.
- [ ] **RANK-04**: With the reranker enabled, search results carry a per-hit relevance probability across MCP, Connect and the CLI, so a caller can tell when no hit answers the query.
- [ ] **RANK-05**: The reranker keeps its decision state within Jev's 32k-token context for candidate sets up to the recall maximum (summaries or per-candidate truncation).

### Eval fixes

- [x] **EVAL-01**: The embedding differ gate compares with a cosine epsilon, not `reflect.DeepEqual` on `[]float32` (#353).
- [x] **EVAL-02**: The retrieval-eval skip guard reads the resolved koanf config, not raw `os.Getenv` (#354).

### Operator correctness

- [ ] **OPS-01**: The exit-code baseline test passes with `ENGRAM_REINDEX_TARGET` / `ENGRAM_MIGRATE_OWNER` set in the environment (#476).
- [ ] **OPS-02**: The bare nested-object branch in `viewFields` is covered by a test, or removed if unreachable (#504).
- [ ] **OPS-03**: `ParsePlanKeyLinks` emits no empty key-link for a fieldless list item, matching its doc comment (#502).
- [ ] **OPS-04**: A test covers a record inserted mid-sweep whose id sorts below the migrate cursor (#501).
- [ ] **OPS-05**: `docs-site` `guides/cli.md` lists `migrate`, `migrate status` and `migrate revert` among the operator commands (#503).

## Future Requirements

Deferred. Tracked but not in this roadmap.

### Decisions

- **DEC-F1**: Write-time advisory hints on store (likely-junk, suggested category, rule candidate, possible contradiction → suggest `supersede_memory`).
- **DEC-F2**: Chat-LLM emulator backend (Go port of the MIT `system-one-adapter-python` contract over `openai.chat_*`).
- **DEC-F3**: Classifier-backed rule-candidate detection (#351).

## Out of Scope

| Feature | Reason |
|---------|--------|
| Any automatic action on a verdict (supersede, delete, archive, reject a write) | Design intent: explicit, zero-junk, never automatic — verdicts are advisory |
| Jev-generated summaries | Jev does not generate text; summaries stay with the chat summarizer |
| LiteLLM pass-through route and key grants | Owned by selfhosted-cluster (live; PR #2227), not engram |
| Committing eval fixtures with verbatim spine content | Public repo |

## Traceability

Which phases cover which requirements. Filled during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DEC-01 | Phase 2 | Complete |
| DEC-02 | Phase 2 | Complete |
| DEC-03 | Phase 2 | Complete |
| DEC-04 | Phase 2 | Complete |
| DEC-05 | Phase 2 | Complete |
| DEC-06 | Phase 2 | Complete |
| CUR-01 | Phase 3 | Pending |
| CUR-02 | Phase 3 | Pending |
| CUR-03 | Phase 3 | Pending |
| CUR-04 | Phase 3 | Complete |
| RANK-01 | Phase 1 | Complete |
| RANK-02 | Phase 1 | Complete |
| RANK-03 | Phase 4 | Pending |
| RANK-04 | Phase 4 | Pending |
| RANK-05 | Phase 4 | Pending |
| EVAL-01 | Phase 1 | Complete |
| EVAL-02 | Phase 1 | Complete |
| OPS-01 | Phase 5 | Pending |
| OPS-02 | Phase 5 | Pending |
| OPS-03 | Phase 5 | Pending |
| OPS-04 | Phase 5 | Pending |
| OPS-05 | Phase 5 | Pending |

**Coverage:**

- v1 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-22*
*Last updated: 2026-09-22 after roadmap creation (5 phases, 22/22 requirements mapped)*
