# Phase 9: Retrieval Eval Harness & Ranking Precision - Research

**Researched:** 2026-07-09
**Domain:** Go retrieval evaluation harness (recall@k/MRR) + Qdrant dense/hybrid vector search
**Confidence:** HIGH (code touchpoints, CI constraints, Qdrant Go client API all verified against source/current docs)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Similarity Score Exposure (REQ-search-similarity-scores):**
- **D-01:** Keep the similarity score **always-on** as currently shipped. Do NOT add an opt-in flag and do NOT hide it by default. Score stays raw Qdrant cosine similarity (higher = closer), `omitempty` (zero/omitted on unranked `list`/`get`).
- **D-02:** Document the always-on score in (a) the `search_memory` tool `Description` + the result `jsonschema` (`internal/server/tools.go`), and (b) the memory-contract docs (`CLAUDE.md` "Memory contract" + the docs-site recall docs).
- **D-03:** The eval (REQ-retrieval-eval) **asserts score separation** between the target record and its sticky topical neighbors.
- **D-04:** This **supersedes** the ROADMAP Phase-9 success-criterion wording "search_memory *can* return a per-result similarity score (**opt-in**)". The shipped reality (always-on) is accepted as correct and better DX; record the supersession explicitly.

**Ranking-Fix Appetite & Guardrails (REQ-ranking-precision):**
- **D-05a:** The eval must **baseline against the current prod config** (qwen3-embedding-8b @4096, with `ENGRAM_EMBED_QUERY_INSTRUCTION` already applied via PR #262), so it measures the *remaining* ranking gap after PR #262 — not a naive symmetric baseline.
- **D-05:** The ranking approach is **chosen by the eval numbers**. Every escalation is **gated on eval evidence**.
- **D-06:** **Try light levers first:** higher default `k`, retrieval tuning, and an **in-process, dependency-free heuristic rerank** (lexical-overlap boost / MMR diversification / score-gap re-scoring). Negligible latency, no new dependency.
- **D-07:** **Hybrid dense+lexical (BM25) fusion is IN SCOPE for Phase 9** if the eval shows it is the winning fix. Requires adding a sparse vector to the collection schema + a reindex/backfill of existing records.
- **D-08:** **A cross-encoder reranker model is ALLOWED in Phase 9** if heuristics + hybrid still miss the eval bar. May add an extra gateway round-trip and a new **opt-in** `ENGRAM_`-prefixed config surface. Keep off the default hot path unless the eval justifies default-on.

### Claude's Discretion

- **Eval harness form & dataset home** — Lean: mirror the established env-gated Go-test eval pattern (`eval:summary`). Make the #261 miss a **permanent regression fixture**; report `recall@k`/MRR. Decide dataset location (Go `testdata/` fixtures vs a new `eval/` corpus) and whether it needs a live Qdrant+embedder or hermetic/recorded fixtures.
- **CI regression gating** — Hard constraint: `protect-main` requires **8 exact-named** status checks and a *skipped* required workflow blocks merge forever. A required eval gate MUST be hermetic (or use a service container); otherwise make it a **non-required** job or local-only `task eval:retrieval` (optionally nightly). Never add a skipped required workflow, never rename a required job.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. Adjacent recall-quality work is already routed: embedder asymmetry → Phase 10 (#305), async summaries → Phase 11 (#320), usage signals → Phase 12 (#317). Usage signals must **never** affect ranking (Phase 12 constraint) — Phase 9's ranking work must not pre-empt that.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-------------------|
| REQ-retrieval-eval | A reproducible retrieval-quality eval — labeled query→expected-record dataset (incl. #261 regression fixture), `recall@k`/MRR, `task eval:retrieval` | `eval:summary` precedent mirrored exactly (env-gated Go test); `store_test.go` `TestMain` gives a proven, already-CI-green Qdrant testcontainer pattern to reuse for realistic vector search; dataset-location and gating recommendations below |
| REQ-search-similarity-scores | `search_memory` returns a per-result similarity score; eval asserts score separation | Score plumbing confirmed already shipped end-to-end; exact file:line doc touchpoints identified; systemic `Out=any` MCP tool pattern documented (affects how "result jsonschema" can concretely be satisfied) |
| REQ-ranking-precision | Eliminate phrasing-sensitive ranking via the approach the eval numbers select | Full escalation ladder mapped to Go/Qdrant mechanics: heuristic rerank → hybrid dense+lexical fusion (Qdrant Go client `Prefetch`+`Fusion` API, self-hosted BM25 via `Modifier_Idf`) → cross-encoder rerank (Cohere/Jina-compatible `/v1/rerank` HTTP) |
</phase_requirements>

## Summary

The similarity-score requirement (REQ-search-similarity-scores) is nearly free: `Memory.Score` already flows from Qdrant's `ScoredPoint.Score` through `recallView.Score` to the Connect API. The only real work is (1) prose documentation in three places and (2) an eval that asserts the score is *meaningful* (separates the target from its neighbors) — there is no new API surface to design.

The eval harness (REQ-retrieval-eval) should be a straight structural copy of the `eval:summary` pattern already in the codebase: an env-gated (`ENGRAM_RETRIEVAL_EVAL=1`) `go test` that skips (not fails) when the gate is unset. This is not a new invention — `internal/store/store_test.go`'s `TestMain` already boots a pinned Qdrant testcontainer (`qdrant/qdrant:v1.18.2`) for the package's integration suite, and that suite already runs inside CI's required `test` job (`go test ./...`) today. The retrieval eval can reuse that exact provisioning path. What it *cannot* get from CI is a live embedder gateway — `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY` are not configured in `ci.yaml`, mirroring why `eval:summary` self-skips in CI. This resolves the CI-gating discretion cleanly: gate the whole eval on `ENGRAM_RETRIEVAL_EVAL=1` (same as summary), which means it participates in the already-required `test` job as a no-op skip (satisfies protect-main's exact-8-checks constraint with zero new job surface) while running for real via `task eval:retrieval` locally or on a separate non-required/nightly workflow.

The ranking-precision escalation ladder (REQ-ranking-precision) has three concrete rungs, each mapped to specific Go/Qdrant mechanics:

1. **Heuristic rerank (D-06)** — pure in-process Go at the `searchMemory` handler or `Store.Search`; no new dependency.
2. **Hybrid dense+lexical fusion (D-07)** — the current Qdrant Go client (`github.com/qdrant/go-client v1.18.3`) supports this natively via `QueryPoints.Prefetch` ( `[]*qdrant.PrefetchQuery` ) fused by `qdrant.NewQueryFusion(qdrant.Fusion_RRF)`. **Critical finding:** Qdrant's server-side BM25 text embedding (`qdrant.NewQueryDocument(&qdrant.Document{Model: "qdrant/bm25"})`) is a **Cloud Inference** feature and is **not available for self-hosted/OSS Qdrant** (confirmed via Qdrant docs: "For Qdrant open-source or self-hosted deployments, Cloud Inference is not available"). Since engram's Helm chart deploys OSS Qdrant, sparse vectors must be generated **client-side in Go** (tokenize → hash terms to indices → raw term-frequency values) and uploaded as `SparseVector{Indices, Values}`; Qdrant's `Modifier_Idf` on `SparseVectorParams` then computes IDF weighting server-side — this is Qdrant's documented self-hosted BM25 pattern and requires no new heavyweight dependency, only a small deterministic tokenizer.
3. **Cross-encoder rerank (D-08)** — an HTTP call to a Cohere/Jina-compatible `/v1/rerank` endpoint (vLLM supports this natively for cross-encoder models like `BAAI/bge-reranker-base`), mirroring `internal/embed`'s existing HTTP client shape. Opt-in `ENGRAM_`-prefixed koanf config.

**Primary recommendation:** Build the eval first as an env-gated Go test reusing the `store_test.go` Qdrant-testcontainer + `EmbedQuery`/`Embed` live-gateway pattern (mirroring `eval:summary`'s CI-safe skip behavior exactly), encode the #261 Query A/B → Record T + sticky-neighbor scenario as a permanent fixture, run it against the current prod baseline (qwen3 @4096 + query instruction) to get real recall@k/MRR numbers, then implement only the ladder rung(s) the numbers require — starting with D-06 (free, no schema change) before considering D-07's schema migration + reindex cost.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Eval harness execution (recall@k/MRR computation) | API/Backend (Go test binary) | — | Pure Go computation over `Store.Search` results; no new tier |
| Eval dataset (labeled query→record fixtures) | API/Backend (in-repo Go) | — | Convention: no `testdata/` dir precedent in this repo (per TESTING.md); inline Go slices match `fidelityCases` pattern |
| Similarity score exposure | API/Backend | — | Already fully plumbed store→server→Connect; doc-only work remains |
| Heuristic rerank (D-06) | API/Backend (`searchMemory` handler / `Store.Search`) | — | In-process, no new service boundary |
| Hybrid sparse-vector generation (D-07) | API/Backend (Go tokenizer) | Database/Storage (Qdrant IDF modifier) | Tokenization happens in Go at store/query time; IDF weighting is server-side Qdrant math via `Modifier_Idf` |
| Hybrid collection schema (D-07) | Database/Storage (Qdrant `CreateCollection`) | — | `SparseVectorsConfig` alongside existing dense `VectorsConfig` |
| Cross-encoder rerank (D-08) | API/Backend → external gateway | — | Extra HTTP round-trip to an OpenAI/Cohere-compatible rerank endpoint, same shape as `internal/embed`'s HTTP client |
| CI regression gate | CI/CD (GitHub Actions) | — | Must reuse the existing required `test` job's env-gated-skip pattern, not a new required job |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|---------------|
| `github.com/qdrant/go-client` | v1.18.3 [VERIFIED: go.mod] | Vector DB client — dense search today; `Prefetch`/`Fusion`/`SparseVectorParams` for hybrid (D-07) | Already the project's sole vector-DB client; no alternative needed |
| `github.com/testcontainers/testcontainers-go/modules/qdrant` | v0.43.0 [VERIFIED: go.mod] | Ephemeral Qdrant for integration tests | Already used by `internal/store/store_test.go`'s `TestMain`; directly reusable by the eval for realistic vector search without a shared dev instance |
| Go standard `testing` | stdlib | Eval harness runner (`go test -run TestRetrievalEval`) | Matches `eval:summary`'s `TestSummaryFidelity` precedent exactly; project convention is "no testify, no assertion DSL" (TESTING.md) |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| none new for D-06 (heuristic rerank) | — | Lexical-overlap scoring / MMR / score-gap re-scoring | Pure Go string/slice ops over already-fetched `store.Memory` results |
| none new for D-07 (hybrid, self-hosted) | — | Client-side BM25 tokenizer | A ~30-line deterministic tokenizer (lowercase, split, hash-to-index, term-frequency count) — Qdrant's `Modifier_Idf` computes the actual IDF weighting server-side, so this is NOT a from-scratch BM25 reimplementation |
| none new for D-08 (cross-encoder) | — | HTTP client to `/v1/rerank` | Reuse `internal/embed`'s existing `net/http` + OpenAI-compatible-gateway client shape |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Client-side BM25 tokenizer + `Modifier_Idf` | Qdrant `qdrant/bm25` server-side `Document` inference | **Cloud Inference only** — confirmed unavailable for self-hosted/OSS Qdrant (the Helm-deployed backend). Not viable for engram. |
| Client-side BM25 tokenizer + `Modifier_Idf` | A Python FastEmbed sidecar service for sparse-vector generation | Adds a new runtime/service dependency and a network hop; against the project's single-Go-binary posture; only justify if the hand-rolled tokenizer proves inadequate on the eval |
| Env-gated `go test` eval (mirrors `eval:summary`) | A separate CLI eval tool / Python notebook | Breaks the established precedent, adds a second test-invocation mechanism, complicates CI wiring for no benefit |
| Cohere/Jina-compatible `/v1/rerank` HTTP | A local ONNX cross-encoder embedded in the Go binary | No mature pure-Go ONNX cross-encoder runtime exists in this ecosystem at production quality; HTTP-to-gateway matches the existing embed/summarize architecture (D-08 already anticipates "extra gateway round-trip") |

**Installation:** No new packages required for this phase — `qdrant/go-client` and `testcontainers-go/modules/qdrant` are already direct dependencies (see go.mod lines 14, 17). If D-08 is triggered, no new Go dependency is needed either (plain `net/http`), only a new `ENGRAM_RERANK_*` config surface.

**Version verification:**
```
$ grep qdrant/go-client go.mod
	github.com/qdrant/go-client v1.18.3
```
`[VERIFIED: go.mod]` — current in-repo pin. The Context7-fetched Qdrant Go client docs (library ID `/qdrant/go-client`, reputation High) confirm `Prefetch`, `NewQueryFusion`, `Modifier_Idf`, and `NewSparseVectorsConfig` are all present in the current client generation (these are long-shipped Qdrant Query API features, stable since Qdrant ~1.10+; the repo's pinned server image is `qdrant/qdrant:v1.18.2`, well past that). `[CITED: github.com/qdrant/go-client docs via Context7]`

## Package Legitimacy Audit

No new external packages are required for this phase's core work (eval harness, D-06 heuristic rerank, D-07 hybrid via existing `qdrant/go-client`, D-08 cross-encoder via plain `net/http`).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|--------------|---------|-------------|
| `github.com/qdrant/go-client` | Go modules | already a direct dep (v1.18.3 pinned) | n/a (Go modules) | github.com/qdrant/go-client | OK | Reused, no change |
| `github.com/testcontainers/testcontainers-go/modules/qdrant` | Go modules | already a direct dep (v0.43.0 pinned) | n/a | github.com/testcontainers/testcontainers-go | OK | Reused, no change |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*If, during planning, a client-side BM25/tokenizer library or a Go rerank-client library is proposed instead of hand-rolling, it MUST go through the Package Legitimacy Gate (`gsd-tools query package-legitimacy check`) before being added to the plan — none is pre-approved by this research.*

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────┐
                         │   task eval:retrieval /       │
                         │   ENGRAM_RETRIEVAL_EVAL=1     │
                         │   go test ./internal/eval/... │
                         └───────────────┬────────────────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    │ (skip if gate unset — CI required "test" job path)
                    ▼
     ┌──────────────────────────┐        ┌──────────────────────────┐
     │ labeled fixtures (Go)     │        │ Qdrant testcontainer      │
     │ query → expected record   │        │ (qdrant/qdrant:v1.18.2,   │
     │ incl. #261 Query A/B →    │        │  reused TestMain pattern) │
     │ Record T + sticky-        │        └───────────┬───────────────┘
     │ neighbor distractors      │                    │
     └──────────────┬────────────┘                    │
                    │ seed via Store.Upsert (EmbedText) │
                    ▼                                   ▼
        ┌─────────────────────────────────────────────────────┐
        │           internal/store.Store (test instance)        │
        │  EnsureCollection → dense (+ sparse if D-07 chosen)    │
        └───────────────────────┬─────────────────────────────┘
                                │
              query embeds via  │ EmbedQuery (live gateway,
              d.em.EmbedQuery   │ ENGRAM_OPENAI_BASE_URL,
              (prod parity:     │ qwen3-embedding-8b @4096,
              D-05a baseline)   │ ENGRAM_EMBED_QUERY_INSTRUCTION)
                                ▼
                     ┌───────────────────────┐
                     │  Store.Search (Query)   │
                     │  dense-only today       │
                     │  → hybrid (D-07):        │
                     │    Prefetch[dense,       │
                     │    sparse] + Fusion(RRF) │
                     │  → +heuristic rerank      │
                     │    (D-06) post-fetch      │
                     │  → +cross-encoder          │
                     │    (D-08) HTTP call        │
                     └────────────┬────────────┘
                                 │ []store.Memory{Score: ...}
                                 ▼
                     ┌───────────────────────┐
                     │  recall@k / MRR scorer  │
                     │  + score-separation     │
                     │  assertion (D-03)       │
                     └────────────┬────────────┘
                                 ▼
                        t.Logf / t.Errorf
                     (regression = test fails
                      when gate is set)
```

Production path (unchanged shape, for contrast): MCP client → `search_memory` tool → `deps.searchMemory` (`internal/server/tools.go:704`) → `EmbedQuery` → `Store.Search` (`internal/store/store.go:544`) → `shapeRecall`/`toRecallView` (`internal/server/summary.go:76,89`) → JSON result with `score` (`json:"score,omitempty"`).

### Recommended Project Structure

Given the "no `testdata/` directories" convention (TESTING.md) and the `eval:summary` precedent (fixtures as an inline Go slice in `internal/summarize/fidelity_test.go`), the eval should live co-located with its subject package rather than in a new top-level `eval/` corpus:

```
internal/store/                     # OR a new internal/retrievaleval/ package if
├── retrieval_eval_test.go          # cross-cutting (embed+store+server) — see Open Question 1
│   ├── TestRetrievalEval (gated on ENGRAM_RETRIEVAL_EVAL=1)
│   ├── retrievalFixtures []retrievalCase (query, scope, wantRecordID, distractors)
│   └── recallAtK / mrr helper functions
```

**Recommendation:** place it as its own small package (e.g. `internal/retrievaleval/`) rather than inside `internal/store` — the eval needs BOTH `internal/store` (Qdrant) AND `internal/embed` (live gateway) AND ideally `internal/server`'s `searchArgs`/handler shape to test the real code path end-to-end, not just `Store.Search` in isolation. A new package avoids import-cycle risk and keeps `store_test.go` (already large, per TESTING.md's file listing) from growing further. This mirrors how `internal/summarize`'s eval lives in the same package as the summarizer it evaluates — here the "subject" spans three packages, so a thin dedicated eval package is the closer analogy to "co-located with source."

### Pattern 1: Env-gated live-integration eval test (established precedent)

**What:** A `go test` function that self-skips unless an explicit env var is set, so it safely participates in `go test ./...` (and therefore CI's required `test` job) as a no-op.
**When to use:** Any eval requiring a live external dependency (embedder gateway, in this case) that CI cannot provide.
**Example:**
```go
// Source: internal/summarize/fidelity_test.go:38-46 (existing code, verified in-repo)
func TestSummaryFidelity(t *testing.T) {
	if os.Getenv("ENGRAM_SUMMARY_EVAL") != "1" {
		t.Skip("set ENGRAM_SUMMARY_EVAL=1 (and the gateway/model env) to run the fidelity eval")
	}
	maxChars, _ := strconv.Atoi(os.Getenv("ENGRAM_SUMMARY_MAX_CHARS"))
	if maxChars <= 0 {
		maxChars = 280
	}
	c := New(os.Getenv("ENGRAM_OPENAI_BASE_URL"), os.Getenv("ENGRAM_OPENAI_API_KEY"), os.Getenv("ENGRAM_SUMMARY_MODEL"), maxChars)
	// ... iterate fixtures, assert, t.Logf a pass/fail summary
}
```
The retrieval eval should follow this shape exactly, gated on a new `ENGRAM_RETRIEVAL_EVAL=1`, reading `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`/`ENGRAM_EMBED_MODEL`/`ENGRAM_EMBED_DIM`/`ENGRAM_EMBED_QUERY_INSTRUCTION` to reconstruct the exact prod embedder config (D-05a).

### Pattern 2: Qdrant testcontainer provisioning (established precedent, directly reusable)

**What:** `TestMain` boots a pinned ephemeral Qdrant (or reuses `ENGRAM_QDRANT_TEST_ADDR` if set) so integration tests get a real backend without a shared dev instance.
**When to use:** Any test package needing a live Qdrant.
**Example:**
```go
// Source: internal/store/store_test.go:28,48-72 (existing code, verified in-repo)
const qdrantImageTag = "qdrant/qdrant:v1.18.2"

func TestMain(m *testing.M) {
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
	if err != nil {
		// ... integration tests skip with a clear message; unit tests unaffected
	}
	testQdrantAddr, err = container.GRPCEndpoint(startCtx)
	// ... m.Run(), terminateQdrant(container)
}
```
If the eval lives in a new package (`internal/retrievaleval/`), it needs its own `TestMain` (or `ENGRAM_QDRANT_TEST_ADDR` override) — `TestMain` is package-scoped, not shared across packages.

### Pattern 3: Hybrid dense+lexical fusion query (D-07, Qdrant Go client — current API)

**What:** A single `Query` RPC that prefetches from both a dense and a sparse named vector, then fuses with Reciprocal Rank Fusion.
**When to use:** Only if the eval shows heuristic rerank (D-06) is insufficient.
**Example:**
```go
// Source: Qdrant Go client docs, https://qdrant.tech/documentation/search/hybrid-queries/
// (fetched via Context7 /qdrant/go-client + /websites/qdrant_tech, current as of this research)
client.Query(context.Background(), &qdrant.QueryPoints{
	CollectionName: "{collection_name}",
	Prefetch: []*qdrant.PrefetchQuery{
		{
			Query: qdrant.NewQuerySparse(indices, values), // client-computed BM25-style term vector
			Using: qdrant.PtrOf("sparse"),
			Limit: qdrant.PtrOf(uint64(20)),
		},
		{
			Query: qdrant.NewQueryDense(denseVec), // existing EmbedQuery output
			Using: qdrant.PtrOf("dense"),
			Limit: qdrant.PtrOf(uint64(20)),
		},
	},
	Query: qdrant.NewQueryFusion(qdrant.Fusion_RRF),
})
```

Collection schema change required (`CreateCollection`/`ensureCollection`, `internal/store/store.go:220-223`):
```go
// Source: Qdrant Go client docs (Context7), self-hosted BM25 pattern
client.CreateCollection(ctx, &qdrant.CreateCollection{
	CollectionName: name,
	VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
		"dense": {Size: dim, Distance: qdrant.Distance_Cosine},
	}),
	SparseVectorsConfig: qdrant.NewSparseVectorsConfig(map[string]*qdrant.SparseVectorParams{
		"sparse": {Modifier: qdrant.Modifier_Idf.Enum()},
	}),
})
```
**Critical:** moving from a single unnamed dense vector (current schema) to a named-vector map (`"dense"`) is itself a breaking collection-schema change requiring the existing `engram reindex --target <new-collection>` flow (new collection, re-embed, cutover) — it is NOT a live in-place migration. The reindex would need to compute **both** the dense embedding (existing `Embed`) and the new sparse term vector (new tokenizer) per record. `EmbedText`'s tag-folding (`internal/store/store.go:148`) already composes `content + tags` into the text handed to `Embed` — the same composed text should feed the sparse tokenizer so tags contribute to lexical matching too (consistent with the dense path).

### Pattern 4: Self-hosted BM25 sparse vector generation (D-07, the central unknown resolved)

**What:** Qdrant's `Modifier_Idf` on `SparseVectorParams` computes IDF weighting server-side from whatever raw term-frequency sparse vectors the client uploads. The client's only job is tokenization + term counting + a stable hash-to-index scheme — NOT computing IDF or full BM25 scoring itself.
**When to use:** Required for D-07 on self-hosted/OSS Qdrant (the `qdrant/bm25` `Document`-inference shortcut is Cloud-only and unavailable here — see Anti-Patterns below).
**Example:**
```go
// Source: Qdrant Go client docs (Context7 /qdrant/go-client, /websites/qdrant_tech)
client.CreateCollection(ctx, &qdrant.CreateCollection{
	CollectionName: "books",
	SparseVectorsConfig: qdrant.NewSparseVectorsConfig(
		map[string]*qdrant.SparseVectorParams{
			"text": {Modifier: qdrant.Modifier_Idf.Enum()},
		}),
})
```
The Go-side tokenizer (new, small, hand-rolled) produces `qdrant.SparseVector{Indices: []uint32, Values: []float32}` via `qdrant.NewVectorsSparse(indices, values)` — e.g. lowercase, split on non-alphanumeric, hash each token (FNV-1a) into a fixed index space (e.g. `% (1<<24)`), count term occurrences as raw values. Qdrant applies the IDF modifier automatically at both upsert-time normalization and query-time scoring.

### Pattern 5: Cross-encoder rerank via OpenAI-compatible gateway (D-08)

**What:** A second-stage HTTP call to a `/v1/rerank` endpoint (Cohere/Jina-API-compatible; vLLM implements this shape for self-hosted cross-encoder models like `BAAI/bge-reranker-base`) that re-scores the top-N candidates from the first-stage search.
**When to use:** Only if D-06 + D-07 still miss the eval bar.
**Example (request shape, HTTP — Go client would mirror `internal/embed`'s existing pattern):**
```
POST {ENGRAM_RERANK_BASE_URL}/v1/rerank
{
  "model": "{ENGRAM_RERANK_MODEL}",
  "query": "<original search query>",
  "documents": ["<candidate 1 content>", "<candidate 2 content>", ...]
}
```
`[CITED: vLLM OpenAI-Compatible Server docs — /rerank, /v1/rerank, /v2/rerank compatible with Jina AI and Cohere rerank API interfaces]` — no official Go SDK exists for this; a plain `net/http` client (same shape as `internal/embed.Client`) is the correct approach, not a new dependency.

### Anti-Patterns to Avoid

- **Using Qdrant's `qdrant.NewQueryDocument(&qdrant.Document{Model: "qdrant/bm25"})` server-side inference for D-07:** this is a **Cloud Inference** feature, confirmed unavailable on self-hosted/OSS Qdrant deployments (the Helm-chart-deployed backend engram uses). Following a tutorial example that uses this API against a self-hosted cluster will fail. Use client-side tokenization + `Modifier_Idf` instead (Pattern 4).
- **Making the retrieval eval a new *required* CI status check:** protect-main's ruleset requires exactly 8 named checks (`test`, `golangci-lint`, `commit-lint`, `license headers`, `helm chart`, `actionlint`, `python`, `ui vendored-asset drift`, confirmed via `gh api repos/.../rulesets`); a required check that has no way to run (no embedder secret in CI) would sit "Expected" forever and permanently block merges. Gate on env var inside the *existing* `test` job instead (Pattern 1).
- **Adding a `testdata/` fixture directory:** the codebase has none (confirmed: "No `testdata/` directories or golden files in the Go tree" — TESTING.md) and the `eval:summary` precedent uses an inline Go slice. Breaking this convention for Phase 9 alone is inconsistent; only deviate if the dataset genuinely needs to be large/binary (unlikely for a labeled query→id fixture set).
- **Adding the score to `recallView.Score`'s struct tag and assuming it produces an MCP output schema:** `search_memory`'s handler, like every other tool in `internal/server/tools.go`, is registered with the literal closure return type `(*mcp.CallToolResult, any, error)`. Per the `go-sdk`'s own doc comment on `AddTool`: *"If the Out type is 'any', the output schema is omitted."* This is a systemic pattern across all 13 tools in this file, not an oversight specific to `search_memory`. A `jsonschema` struct tag on `recallView.Score` will NOT surface in an auto-generated output schema under the current pattern — the durable documentation channel is the tool's `Description` string (prose, which IS sent to MCP clients) plus the docs-site. See Common Pitfalls below.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| BM25/IDF scoring math | A full from-scratch BM25 ranking formula in Go | Qdrant's `Modifier_Idf` on `SparseVectorParams` | Qdrant already implements correct, tested BM25-style IDF weighting server-side; client only tokenizes and counts terms — reimplementing the scoring formula risks subtle correctness bugs (saturation, length normalization) that Qdrant has already solved |
| Rank fusion (combining dense + sparse result lists) | A custom RRF/weighted-sum fusion algorithm | `qdrant.NewQueryFusion(qdrant.Fusion_RRF)` | Native Query API primitive; hand-rolling fusion means re-fetching both result sets client-side and merging them correctly (tie-breaking, score normalization across incompatible scales) — Qdrant does this in one round-trip |
| Cross-encoder inference | A local ONNX/embedded reranker model in Go | HTTP call to a Cohere/Jina-compatible `/v1/rerank` gateway endpoint (vLLM-hosted) | No mature production-grade pure-Go cross-encoder runtime exists; this mirrors the existing `internal/embed`/`internal/summarize` architecture of delegating model inference to an external OpenAI-compatible gateway |
| recall@k / MRR computation | (nothing to avoid — this genuinely should be hand-rolled) | Plain Go: `recall@k = |relevant ∩ top-k| / |relevant|` (here: 1 expected record per query, so recall@k is 0/1 per query, averaged); `MRR = mean(1/rank of first relevant hit)` | These are ~10-line functions with no edge-case complexity for a single-expected-record dataset; a dependency here would be over-engineering |

**Key insight:** the domain's genuine complexity (BM25 IDF math, rank fusion, cross-encoder inference) all lives inside Qdrant or an external gateway that engram already delegates model-hosting to. The Go-side work for Phase 9 is thin glue (tokenize, call the Query API, compute two simple metrics) — resist the temptation to build a bigger ranking framework than the eval numbers justify.

## Common Pitfalls

### Pitfall 1: Assuming the "result jsonschema" in D-02 means an auto-generated MCP output schema
**What goes wrong:** A planner adds a `jsonschema:"..."` tag to `recallView.Score` expecting it to appear in the tool's advertised output schema to MCP clients, then is surprised when it doesn't.
**Why it happens:** `search_memory`'s handler closure is typed `(*mcp.CallToolResult, any, error)` — same as all 13 other tools in `tools.go`. The `go-sdk`'s `AddTool` explicitly omits the output schema when `Out` is `any`.
**How to avoid:** Treat D-02's "result jsonschema" as satisfied by (a) the tool's `Description` prose (client-visible) and (b) code-level self-documentation (the `jsonschema` tag is harmless to add for readability, just not schema-emitting). If a real output schema is desired, it requires either changing the return type of `search_memory`'s closure to a concrete struct (touches the shared `AddTool` call, a bigger and riskier change affecting only this one tool inconsistently with its 12 siblings) or manually setting `mcp.Tool.OutputSchema` before calling `AddTool` (the SDK only auto-fills it when nil). Recommend the prose-only route unless the eval or a consumer genuinely needs machine-readable output schema — out of the locked decisions' evident intent.
**Warning signs:** Plan tasks that describe "add score to output schema" as if it were equivalent to the existing input-arg `jsonschema` tags on `searchArgs`.

### Pitfall 2: Reindex boundary confusion for hybrid schema migration (D-07)
**What goes wrong:** Assuming sparse vectors can be added to the *existing* collection in-place.
**Why it happens:** Qdrant allows adding new named sparse vectors to schema at collection-creation time, but the existing engram collection has a single **unnamed** dense vector (`qdrant.NewVectorsConfig(&qdrant.VectorParams{...})`, not `NewVectorsConfigMap`). Converting to a named-vector map (required to add a co-located sparse field) is itself a breaking schema change — Qdrant vector configuration is immutable per collection.
**How to avoid:** Route through the existing `engram reindex --target <new-collection>` flow (new collection at the new schema, re-embed **and** re-tokenize every record, verify with `--dry-run`, then cut over `ENGRAM_QDRANT_COLLECTION`). This is exactly the operator ergonomics precedent CONTEXT.md points at.
**Warning signs:** A plan task that calls `CreateFieldIndexCollection` or similar in-place mutation for the sparse vector instead of a target-collection reindex.

### Pitfall 3: Query-side vs document-side sparse vector asymmetry mirrors the dense embedder split
**What goes wrong:** Using the same tokenizer/instruction handling for both stored documents and incoming queries without realizing engram already has an established query/document asymmetry seam (`EmbedQuery` vs `Embed`, PR #262).
**Why it happens:** BM25/sparse retrieval is naturally symmetric (same tokenizer both sides) — easy to assume no asymmetry decision is needed, unlike the dense embedding instruction split.
**How to avoid:** Confirm in planning that the SAME tokenizer function (same hash space) is used for both document upsert and query-time sparse vector construction — asymmetric tokenizer changes here would be a correctness bug (mismatched index spaces), not a query-quality lever like `EmbedQuery`'s instruction wrapping. Unlike the dense document instruction, there is no legitimate reason for query/document sparse tokenization to differ.
**Warning signs:** Two different tokenizer implementations or hash functions for store-time vs search-time sparse vector construction.

### Pitfall 4: CI's `test` job already runs `go mod tidy` and `gofmt` checks — a new eval package must stay clean
**What goes wrong:** A hastily-added `internal/retrievaleval/` package with an unused import or unformatted file fails the *required* `test` job even though the eval itself self-skips.
**Why it happens:** `ci.yaml`'s `test` job runs `go build`-adjacent checks (`go mod tidy` diff, `gofmt -l .`) unconditionally, regardless of whether `ENGRAM_RETRIEVAL_EVAL` is set — these checks run at the Go-toolchain level, before any test executes.
**How to avoid:** Standard Go hygiene (gofmt, goimports, `go vet` via golangci-lint) applies to the new eval package like any other; no different from adding any new Go file to this repo.
**Warning signs:** N/A — this is a standard reminder, not a novel risk, but worth flagging since eval code is sometimes treated as "throwaway."

## Code Examples

Verified patterns from official sources and in-repo precedent (see Architecture Patterns above for the primary five patterns). Additional supporting example:

### recall@k / MRR computation (hand-rolled, no library — see Don't Hand-Roll)
```go
// New code — no existing precedent in-repo; standard IR metric definitions.
// For a dataset where each query has exactly one expected record (the #261 shape):
func recallAtK(results []store.Memory, wantID string, k int) bool {
	for i, m := range results {
		if i >= k {
			break
		}
		if m.ID == wantID {
			return true
		}
	}
	return false
}

func reciprocalRank(results []store.Memory, wantID string) float64 {
	for i, m := range results {
		if m.ID == wantID {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0 // not found within returned results
}
```

### #261 regression fixture shape (encoding Query A/B → Record T + sticky neighbors)
```go
// New code — models the CONTEXT.md-specified #261 scenario as a permanent fixture.
type retrievalCase struct {
	name          string
	seedRecords   []seedRecord // Record T + N sticky topical-neighbor distractors
	queries       []string     // Query A, Query B (near-verbatim restatements of Record T)
	wantRecordID  string       // Record T's seeded id
	wantScoreGap  float64      // minimum score separation vs best distractor (D-03 assertion)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-------------------|----------------|--------|
| Symmetric query/document embedding (raw text both sides) | Asymmetric query-side instruction wrapping (`EmbedQuery` + `ENGRAM_EMBED_QUERY_INSTRUCTION`) | PR #262 (`08a0b979`), already shipped before this phase | The eval MUST baseline against this post-#262 state (D-05a) — a symmetric baseline would overstate the remaining gap this phase needs to close |
| Dense-only Qdrant collection | (proposed, D-07-gated) hybrid dense + sparse named-vector collection with RRF fusion | Not yet shipped — gated on eval evidence | Requires a full reindex (breaking schema change, immutable vector config) |
| `search_memory` result score undocumented | Score already shipped in the wire format, just needs prose docs | This phase | No behavior change — pure documentation + eval-assertion work for REQ-search-similarity-scores |

**Deprecated/outdated:**
- Treating "opt-in similarity score" (the literal ROADMAP wording) as the target: superseded by D-04 — the shipped always-on behavior is correct and the ROADMAP wording is stale, not the implementation.
- `qdrant.NewQueryDocument(..., Model: "qdrant/bm25")` as a self-hosted BM25 recipe: this pattern appears in Qdrant's own tutorial docs but is Cloud Inference-only; do not copy it verbatim for engram's self-hosted deployment.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|----------|-----------------|
| A1 | Qdrant's `Modifier_Idf` self-hosted BM25 pattern (client tokenizes + counts, server applies IDF) is the correct, sufficient approach for D-07 without needing a more sophisticated sparse encoder (e.g. SPLADE) | Architecture Patterns Pattern 4, Don't Hand-Roll | If plain term-frequency + IDF proves insufficient on the eval numbers, the escalation path (a learned sparse encoder, or skipping straight to D-08 cross-encoder) needs re-evaluation — but D-05/D-06 already establish "try the cheaper lever first, let the eval decide," so this is a low-risk assumption structurally |
| A2 | No mature Go cross-encoder rerank client library exists, so D-08 should be plain `net/http` to a Cohere/Jina-compatible endpoint rather than a Go SDK | Standard Stack (Alternatives), Pattern 5 | If a well-maintained Go client for a specific rerank provider is discovered during planning, it should go through the Package Legitimacy Gate before adoption — this assumption only rules out *needing* one, not prohibits using one if it clears legitimacy checks |
| A3 | The retrieval eval should live in a new `internal/retrievaleval/` package rather than inside `internal/store` or `internal/server` | Recommended Project Structure | Low risk — this is a structural/discretionary call the planner can override; the alternative (co-locating in `internal/store`) works too, just grows an already-large test file and requires importing `internal/embed`+`internal/server` types into `internal/store`'s test package (import-direction risk) |

**If this table is empty:** N/A — see above.

## Open Questions

1. **Exact eval package location: `internal/retrievaleval/` (new) vs. extending `internal/store`'s existing integration suite?**
   - What we know: `internal/store/store_test.go` already has a working `TestMain` + Qdrant testcontainer; `eval:summary` lives inside the package it evaluates (`internal/summarize`).
   - What's unclear: whether the eval should exercise the full `search_memory` handler stack (`internal/server`, including `EmbedQuery` wiring and `shapeRecall`) or just `Store.Search` directly — the former is more end-to-end-realistic but requires cross-package test wiring; the latter is simpler but skips the handler-layer default-`k`-injection logic (`if a.K == 0 { a.K = 8 }`, `internal/server/tools.go:705-707`) that D-06's "higher default k" lever would actually change.
   - Recommendation: exercise `internal/server`'s `deps.searchMemory` (or an equivalent thin wrapper) directly, not just `Store.Search`, so the eval measures exactly what a real MCP client experiences (including the default-k logic) — place it in a new `internal/retrievaleval/` package that imports `internal/server`, `internal/store`, and `internal/embed`.

2. **What `k` and dataset size are needed for a statistically meaningful recall@k/MRR on a single #261 fixture?**
   - What we know: the #261 scenario is one query pair (A/B) → one target record (T) with "sticky topical neighbor" distractors; default `k`=8 (`internal/server/tools.go:705`).
   - What's unclear: whether the eval dataset should be *only* the #261 fixture (a strict regression guard) or should also include a broader synthetic labeled set to give recall@k/MRR real statistical meaning beyond a single pass/fail case — CONTEXT.md's Specific Ideas section anchors on #261 as "the acceptance anchor" but REQ-retrieval-eval's wording ("labeled query→expected-record dataset ... including the #261 miss as a regression fixture") implies #261 is one fixture among possibly others.
   - Recommendation: start with the #261 fixture as the mandatory regression case (blocking), and let the planner size additional synthetic fixtures (e.g. 5-10 more query/record pairs covering other phrasing-sensitivity patterns) as a stretch — the mandatory minimum satisfies REQ-retrieval-eval's letter; the stretch set makes recall@k/MRR meaningful as an ongoing metric rather than a single boolean.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Docker (for Qdrant testcontainer) | Eval harness integration test | assumed ✓ in CI (ubuntu-latest has Docker preinstalled; already relied on by `internal/store` integration tests today) | — | `ENGRAM_QDRANT_TEST_ADDR` override to a pre-existing Qdrant instance |
| Live OpenAI-compatible embedder gateway (`ENGRAM_OPENAI_BASE_URL`) | Eval harness (realistic embeddings, D-05a prod-parity baseline) | ✗ in CI (not configured in `ci.yaml`, same as `eval:summary`'s existing gap) | — | Gate the whole eval on `ENGRAM_RETRIEVAL_EVAL=1`; skips cleanly in CI, runs locally/nightly with real credentials |
| `qdrant/go-client` `Modifier_Idf`, `Prefetch`, `Fusion` API | D-07 (if selected) | ✓ — confirmed present in current client generation via Context7-fetched docs | client v1.18.3 (repo-pinned), server v1.18.2 (testcontainer-pinned) | — |
| Cohere/Jina-compatible `/v1/rerank` gateway (e.g. vLLM-hosted) | D-08 (if selected) | unknown — not yet part of engram's infra; would need to be stood up by the operator | — | Keep D-08 config opt-in per CONTEXT.md; if unavailable, the eval simply can't validate this rung and D-06/D-07 remain the shipped fix |

**Missing dependencies with no fallback:**
- None — every gap above has a documented fallback (env-gate skip, or D-08 simply staying unvalidated/unshipped if no rerank gateway exists).

**Missing dependencies with fallback:**
- Live embedder gateway in CI → env-gated skip (mirrors `eval:summary`).
- Rerank gateway (D-08) → stays opt-in and unshipped by default unless an operator configures it; the eval only needs to validate it locally when D-06/D-07 numbers don't clear the bar.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` (stdlib), no assertion library (project convention per TESTING.md) |
| Config file | none — `TestMain` in a new `internal/retrievaleval/` package provisions Qdrant directly (mirrors `internal/store/store_test.go`) |
| Quick run command | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/... -run TestRetrievalEval -v` |
| Full suite command | `task eval:retrieval` (new Taskfile target, mirrors `eval:summary` exactly) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|---------------|
| REQ-retrieval-eval | `task eval:retrieval` runs labeled dataset incl. #261 fixture, reports recall@k/MRR | integration (live Qdrant testcontainer + live embedder gateway) | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v` | ❌ Wave 0 — new package |
| REQ-search-similarity-scores | Score already returned; eval asserts separation between target and sticky neighbors | integration (part of the same eval test, D-03) | same command as above, asserting `wantScoreGap` | ❌ Wave 0 — same new test; also a `go test ./internal/server/...` unit check that `recallView.Score` round-trips (likely already covered by existing `summary_test.go`, verify during planning) |
| REQ-ranking-precision | #261 Query A/B surface Record T within default `k` | integration (same eval, the #261 fixture is the assertion) | same command; test fails (not skips) on regression when `ENGRAM_RETRIEVAL_EVAL=1` is set | ❌ Wave 0 — same new test |

### Sampling Rate
- **Per task commit:** unit-level tests only (`task test:short` or targeted `go test ./internal/server/... ./internal/store/...`) — the live-gateway eval is too slow/costly for every commit.
- **Per wave merge / local validation:** `task eval:retrieval` (full live run) before considering a ranking-fix wave complete — this is the actual acceptance gate for D-05/D-06/D-07/D-08 decisions (the eval numbers ARE the escalation-ladder decision mechanism per D-05).
- **Phase gate:** `task eval:retrieval` green (recall@k/MRR at or above the agreed target, #261 fixture passing) before `/gsd-verify-work`; `task` (lint + `go test ./...`, which includes the self-skipping eval in CI) stays green throughout.

### Wave 0 Gaps
- [ ] `internal/retrievaleval/retrieval_eval_test.go` — new package, covers REQ-retrieval-eval, REQ-search-similarity-scores, REQ-ranking-precision
- [ ] `internal/retrievaleval/fixtures.go` (or inline in the test file, matching `fidelityCases` precedent) — the #261 Query A/B → Record T + sticky-neighbor dataset
- [ ] `Taskfile.yaml` `eval:retrieval` target — mirrors `eval:summary` (~line 52)
- [ ] Framework install: none — stdlib `testing` + already-vendored `testcontainers-go/modules/qdrant`

*(If any of the above are found to already partially exist during planning, e.g. a shared testcontainer helper worth extracting from `internal/store/store_test.go`, adjust the gap list accordingly — this research did not find such an extraction point today.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|-----------------|---------|---------------------|
| V2 Authentication | no | Eval runs against a test-only Qdrant + a test-scope owner; no new auth surface |
| V3 Session Management | no | N/A — not a session-facing change |
| V4 Access Control | no | `Store.Search`'s existing owner/scope isolation is unchanged by this phase; hybrid fusion and rerank operate strictly after the existing `ownerScopeFilter` is applied, so authz boundaries are unaffected |
| V5 Input Validation | yes (existing, unchanged) | `search_memory`'s existing `searchArgs` validation (`parseRFC3339`, etc.) is untouched; a new client-side tokenizer (D-07) processes already-trusted stored/query text, not external untrusted input beyond what `Embed`/`EmbedQuery` already handle |
| V6 Cryptography | no | N/A |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| Cross-encoder rerank (D-08) sends candidate record *content* to an external gateway | Information Disclosure | Same trust boundary as the existing `Embed`/`Summarize` gateway calls (`ENGRAM_OPENAI_BASE_URL`) — content already leaves the process for embedding/summarization today; D-08 doesn't introduce a new class of exposure, but the plan should note it explicitly (rerank sends the FULL candidate content, not just a query, to the gateway) so an operator choosing D-08 understands the data-flow implication |
| A `SUS`/untrusted rerank gateway returning manipulated scores | Tampering | Rerank output only affects ordering within already-owner-scoped, already-authorized results — it cannot surface another actor's records (authz filter runs before rerank in the pipeline) |

## Sources

### Primary (HIGH confidence)
- `internal/store/store.go` (in-repo, read directly) — `CreateCollection`/`ensureCollection` (~L212-230), `Search` (L544-573), `memoriesFromPoints` (L579-587), `Memory.Score` (L139), `EmbedText` (L148-151)
- `internal/server/tools.go` (in-repo) — `searchArgs` (L343-351), `deps.searchMemory` (L704-728), `search_memory` tool registration (L937-941), all 13 `AddTool` closures confirmed uniformly typed `(*mcp.CallToolResult, any, error)`
- `internal/server/summary.go` (in-repo) — `recallView.Score` (L52), `shapeRecall`/`toRecallView` (L76-96)
- `internal/server/connectapi.go` (in-repo) — `Score: m.Score` (L41)
- `internal/embed/embed.go` (in-repo) — `EmbedQuery`/`Embed` asymmetry, `queryInstruction` (L33-36, L144-154)
- `internal/store/store_test.go` (in-repo) — `TestMain`, `qdrantImageTag` (v1.18.2), testcontainer provisioning (L28-113)
- `internal/summarize/fidelity_test.go` (in-repo) — `TestSummaryFidelity`, `ENGRAM_SUMMARY_EVAL` gate pattern (L36-65)
- `Taskfile.yaml` (in-repo) — `eval:summary` target (L52-55)
- `.github/workflows/ci.yaml` (in-repo) — required `test` job structure, confirms no `ENGRAM_OPENAI_*` secrets configured
- `gh api repos/seanb4t/engram/rulesets/17228701` (live query) — confirmed exact 8 required status-check names: `test`, `golangci-lint`, `commit-lint`, `license headers`, `helm chart`, `actionlint`, `python`, `ui vendored-asset drift`
- `go.mod` (in-repo) — `github.com/qdrant/go-client v1.18.3`, `github.com/testcontainers/testcontainers-go/modules/qdrant v0.43.0`
- Context7 `/qdrant/go-client` (High reputation) — `PrefetchQuery`, `NewQueryFusion`, `Modifier_Idf`, `NewSparseVectorsConfig`, `SparseVectorCreationConfig`
- Context7 `/websites/qdrant_tech` (High reputation) — hybrid-queries and text-search guides with current Go code examples; "Cloud Inference is not available" for self-hosted/OSS Qdrant (Qdrant Fundamentals FAQ)
- `go-sdk` (`github.com/modelcontextprotocol/go-sdk@v1.6.1`) `mcp/server.go` (module cache, read directly) — `AddTool` doc comment confirming Out=any omits output schema (L490-502)

### Secondary (MEDIUM confidence)
- `docs-site/src/content/docs/guides/embedding-instructions.md` (in-repo) — already documents "search_memory results now carry the Qdrant similarity score" (L126-128); `docs-site/src/content/docs/reference/tools.md` `search_memory` section (L85-100) does NOT currently document the score field — confirmed gap for D-02
- WebSearch: vLLM OpenAI-Compatible Server rerank endpoint docs — `/rerank`, `/v1/rerank`, `/v2/rerank` Jina/Cohere-API-compatible

### Tertiary (LOW confidence)
- None used as load-bearing claims; all findings above were cross-checked against in-repo source or official docs.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; existing pins verified via go.mod
- Architecture: HIGH — hybrid-fusion API confirmed against current Qdrant Go client docs (Context7, High-reputation source) and cross-checked against the repo's actual pinned versions
- Pitfalls: HIGH — the Out=any output-schema finding and the Cloud-Inference-only BM25 finding are both directly verified (SDK source read, official Qdrant FAQ), not inferred

**Research date:** 2026-07-09
**Valid until:** 30 days (stable domain — Qdrant Query API and the MCP go-sdk's AddTool behavior are not fast-moving; re-verify if `qdrant/go-client` or `modelcontextprotocol/go-sdk` are bumped before planning executes)
