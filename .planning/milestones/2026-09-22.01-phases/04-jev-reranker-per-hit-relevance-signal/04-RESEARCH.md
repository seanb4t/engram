# Phase 4: Jev Reranker & Per-Hit Relevance Signal - Research

**Researched:** 2026-09-24
**Domain:** In-repo Go architecture (recall ranking seam, typed-decision client composition, Connect/proto wire evolution) — no new external library
**Confidence:** HIGH (every claim below is grounded in a `Read` of the actual file this session, cited by path and line range; no web research was needed because this phase composes packages Phases 1–3 already built)

## Summary

Phase 4 does not introduce a new subsystem — it composes three subsystems Phases 1–3 already shipped: the D-05 lexical rank step (`store.rankCandidates`, Phase 1), the provider-neutral `decide.Decider` (Phase 2), and the truncated-state pattern from curation verdicts (`internal/verdict`, Phase 3). The single most important finding is that **`store.SearchReranked` already forces `opts.Full = true`** on its internal fetch (`internal/store/store.go:1337`) specifically because "this input contract... is retained unconditionally for Phase 4's content-reading Jev reranker (D-08)" — the candidates handed to the rank step already carry full `Content`/`Summary`, fetched in a single two-phase Qdrant round trip. This means Phase 4 needs **no second fetch, no `Store.RecordStates` call, and no new Qdrant round trip** — only a truncation step (reusing `internal/verdict.State`'s truncation logic, not its fetch) applied to content already in hand.

The second load-bearing finding: `rankCandidates` (`internal/store/rerank.go:122`) is pinned by `TestRankCandidatesIsTheD05Winner` as the D-05-approved lexical winner and must not change its own behavior. D-03's "fetch as today, apply the lexical rank step, then stable-sort by Jev P(relevant)" therefore cannot be implemented by editing `rankCandidates` — it requires a **new composition step** in `SearchReranked` that calls the existing lexical step *without truncation* (`rankCandidates(query, hits, len(hits))`, which returns every hit reordered per `RerankHits`'s own documented contract), then optionally re-sorts by a Jev-supplied relevance map, *then* truncates to `k`. `internal/store` never needs to import `internal/decide`: a primitive `func(ctx, query, hits)([]Memory /* or map[string]float64 */, error)` hook type — using only `context`, `store.Memory`, and stdlib types — is enough, mirroring the existing precedent that `SearchReranked` "does NOT import `internal/embed` or `internal/server`... this helper only reorders an already-fetched, already-authorized `[]Memory`" (`internal/store/store.go:1321-1324`).

The third finding materially affects scope: `store.SearchDiscovery` (`internal/store/store.go:1352-1428`) does **not** go through `rankCandidates`/`RerankHits` at all today — it is a separate method that fetches exactly `k` (no `CandidateK` over-fetch) in pure vector order. D-07 lists `search_discovery` in scope, but CONTEXT.md explicitly leaves "how `search_discovery`'s ranking path is threaded to the same step" to Claude's Discretion. This research recommends a **new, separate `SearchDiscoveryReranked`** path used only when `ranker=jev`, leaving today's `SearchDiscovery` (used when the ranker is off/lexical, the default) byte-identical — see Architecture Patterns.

The fourth finding affects D-09: `jev.Client.Decide` (`internal/decide/jev/jev.go:220-318`) bakes in D-11's single retry **unconditionally** inside the client — there is no `Option` to disable it. Satisfying "no retry on the search path" therefore requires either a new `jev.Option` (e.g. `WithNoRetry()`) or a second `*jev.Client` instance dedicated to search reranking. A second client is the additive, lower-risk choice, and is what the phase description's own hint ("a second client") points to.

**Primary recommendation:** Add a `RankHook` func-type field to `store.SearchOptions` (nil by default, byte-identical behavior when unset); have `SearchReranked` call the existing `rankCandidates` with no truncation to get the full lexically-ordered `CandidateK` pool, invoke the hook if present, stable-sort by the returned relevance map (ties/errors keep lexical order), then truncate to `k` and stamp `Memory.Relevance` on the survivors. Wire the hook in `internal/server` from `deps.decider`, using a *second*, retry-disabled `jev.Client` built from two new registry keys (`ENGRAM_SEARCH_RANKER`, `ENGRAM_SEARCH_RERANK_TIMEOUT`). Add `optional float relevance = 31;` to the `Memory` proto message (next free field number) and a matching `Relevance *float64 json:"relevance,omitempty"` on `store.Memory` (transient — never written to the Qdrant payload, mirroring `Score`'s existing precedent).

## User Constraints

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Enable & ship gate (RANK-03)**
- **D-01:** A registered ranker enum `ENGRAM_SEARCH_RANKER` = `lexical` (default) | `jev`. `jev` requires `ENGRAM_DECISIONS_PROVIDER` to be set; config validation fails clearly otherwise (existing `Config.Validate` style). — Reversibility: costly — registered operator config key.
- **D-02:** No eval bar for shipping — the Jev ranker ships as an opt-in regardless of the numbers. The Phase 1 retrieval eval's disabled Jev slot is enabled (runs when a decisions provider is configured) and the live Jev numbers (paraphrase recall@k/MRR, #261 rank) are recorded as a phase artifact next to lexical/vector for operators to judge.

**Composition with lexical**
- **D-03:** With `ranker=jev`: fetch as today (vector at `CandidateK`), apply the lexical rank step, then stable-sort by Jev P(relevant) — ties and any Jev failure keep the lexical order, so the fallback is exactly today's shipped order.
- **D-04:** The whole `CandidateK` pool (32–100) goes to Jev in one Decisions request (one Noul question per candidate, spike 004 criteria), then truncate to k.

**Relevance signal surface (RANK-04)**
- **D-05:** Per-hit `relevance` float (0–1), omitempty — present only when the Jev ranker ran and succeeded for that search; absent otherwise. Sits beside the existing cosine `score`. MCP JSON field, an additive optional Connect proto field (buf-generated, gen/ tree regenerated), and a CLI column/field in `engram search`.
- **D-06:** Per-hit only — no response-level "nothing relevant" flag or threshold; the caller decides from the per-hit values.
- **D-07:** Scope: everything routed through `SearchReranked` (`search_memory` MCP, Connect `SearchMemories`, CLI `engram search`, with or without `cross_spine`) plus `search_discovery`. `list_memory` / `get_memory` unchanged.

**Budget & latency (RANK-05)**
- **D-08:** Candidate state reuses Phase 3's `internal/verdict` state construction (summary + content head) with a smaller per-candidate budget (≈600 chars), plus a total-state guard that shrinks the per-candidate budget so query + up to 100 candidates stay under ≈28k tokens (chars/4 estimate with headroom). Candidate content must be fetched through the bounded-read primitives (search results may carry only summaries in the default view).
- **D-09:** A dedicated `ENGRAM_SEARCH_RERANK_TIMEOUT` (default ≈2s) with no retry on the search path; on timeout/error the call falls back to lexical order and still succeeds. Sweeps (consolidate) keep the decisions timeout and single retry.

### Claude's Discretion
- Exact per-candidate budget and token-estimate constants within D-08; Noul question wording (start from spike 004's criteria); how `search_discovery`'s ranking path is threaded to the same step; CLI column formatting; OTel attributes for the rerank span (reuse the `decide` span).

### Deferred Ideas (OUT OF SCOPE)
- Response-level `no_relevant_results` flag (D-06 declined for now).
- An eval bar gating the Jev option (D-02: opt-in regardless).
</user_constraints>

## Phase Requirements

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RANK-03 | `search_memory` with the Jev reranker enabled reorders candidates by relevance probability; a decision error/timeout falls back to default order, call still succeeds | `store.SearchReranked`'s forced `Full` fetch (store.go:1337) means no second fetch is needed; `rankCandidates` (D-05 pin) must stay unmodified — composition happens via a new `RankHook` step called with `k=len(hits)` (no truncation) before the final truncate; fallback-on-error is a `nil`/`err != nil` check on the hook's return, never propagated as a `SearchReranked` error |
| RANK-04 | Every surface (MCP, Connect, CLI) carries a per-hit relevance probability when the reranker is enabled | New `store.Memory.Relevance *float64` (transient, mirrors `Score`'s payload-exclusion precedent) flows through `memoryToProto` (connectapi.go:80-107), the MCP `recallView`/`toRecallView` (summary.go:40-105), and `renderMemoryTable` (client_common.go:506-536) — all three are hand-written allow-list shapers that each need an explicit added field, confirmed by the identical existing pattern for `Score`/`AccessCount` |
| RANK-05 | Decision state stays within Jev's 32k-token context for candidate sets up to the recall maximum | `store.CandidateK` hard-caps the pool at 100 regardless of `k` (rerank.go:17-26; `MaxRecallLimit` of 1000 is irrelevant here) — the token budget only ever needs to cover 100 candidates, not 1000; `internal/verdict.State`/`truncateRunes` (verdict.go:83-108) is the exact reusable truncation primitive, applied to content already resident in the fetched `[]Memory` (no new store round trip) |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Vector fetch + authz filter (`CandidateK` over-fetch) | Database/Storage (`internal/store`) | — | Unchanged; `Store.Search`'s existing two-phase query + `fetchPayloadsByID` (store.go:1237-1284) |
| Lexical rank step (`rankCandidates`) | Database/Storage (`internal/store`) | — | Pinned pure function; stays exactly as-is (rerank.go:109-124) |
| Jev relevance scoring (network call to Decisions API) | API/Backend (`internal/server`, via `internal/decide/jev`) | — | `internal/store` must not gain an `internal/decide` import for an I/O client — precedent: `SearchReranked` doc comment explicitly keeps embedding out of `internal/store` (store.go:1321-1324); the network call belongs where `deps.decider` already lives (tools.go:90-99) |
| Stable-sort-by-relevance + truncate-to-k | Database/Storage (`internal/store`) | — | Must run on the FULL `CandidateK` pool before truncation (D-04) — only `SearchReranked` has that pool; a `RankHook` closure lets the sort/truncate logic stay in `internal/store` while the network call stays in `internal/server` |
| Per-hit relevance wire exposure (MCP JSON, Connect proto, CLI table) | API/Backend + Browser/Client boundary | — | Three independent hand-written shapers (`recallView`, `memoryToProto`, `renderMemoryTable`) each need the new field added explicitly — none of them derive fields by reflection |
| Operator config (`ENGRAM_SEARCH_RANKER`, `ENGRAM_SEARCH_RERANK_TIMEOUT`) | API/Backend (`internal/config`) | — | Registry-driven, `env`-first, consistent with every other `ENGRAM_*` knob (registry.go) |
| Retrieval eval Jev slot | API/Backend (`internal/retrievaleval`) | — | Roster entry already scaffolded, disabled (`rankers.go:123-128`); enabling it needs a real `decide.Decider` wired into the eval's `rankerFunc` closure |

## Standard Stack

No new external package is introduced by this phase. Every dependency the reranker needs is already vendored and wired by Phase 2 (`internal/decide`, `internal/decide/jev`) and Phase 3 (`internal/verdict`).

### Core (already in the module, reused)
| Package | Purpose | Why reused, not new |
|---------|---------|---------------------|
| `internal/decide` | Provider-neutral `Decider` interface, `Request`/`Response`/`Question` types | DEC-02's whole point — a second consumer (search rerank) must not need a new abstraction |
| `internal/decide/jev` | The Jev HTTP client (`Client.Decide`) | Same backend Phase 2/3 shipped; needs one new capability (a way to disable D-11's retry) — see Common Pitfalls |
| `internal/verdict` | `State`/`truncateRunes` per-candidate truncation | D-08 explicitly says "reuses Phase 3's `internal/verdict` state construction" — the truncation function only, not `Store.RecordStates` (see Summary) |
| `internal/store` | `rankCandidates`, `CandidateK`, `SearchReranked`, `Memory` | The seam this phase plugs into (D-08 of Phase 1's CONTEXT, referenced verbatim in this phase's canonical refs) |

### Package Legitimacy Audit

Not applicable — this phase adds no new `go.mod` dependency (confirmed: every type/function cited above already exists in the current tree, verified by `Read` this session). No `npm view`/`pip index`/`cargo search` check is needed.

## Architecture Patterns

### System Architecture Diagram

```
                     MCP search_memory / Connect SearchMemories / CLI `engram search`
                                          │
                                          ▼
                          deps.searchMemory (tools.go:1893)  ── embeds query, resolves scope
                                          │
                                          ▼
                    store.SearchReranked (store.go:1325)  ── k==0 guard; opts.Full = true
                                          │
                                          ▼
          store.Search (CandidateK(k) over-fetch, authz filter, two-phase fetch)  ── 32–100 hits, FULL content already attached
                                          │
                                          ▼
       rankCandidates(query, hits, len(hits))  ── D-05 lexical step, NO truncation yet (full pool reordered)
                                          │
                                          ▼
            ┌─────────────────────────────────────────────────────────┐
            │  opts.RankHook != nil? (server-wired closure)            │
            │      │                                                   │
            │      ▼                                                   │
            │  per-candidate truncate (internal/verdict.State reuse)   │
            │      │                                                   │
            │      ▼                                                   │
            │  ONE decide.Request: {query, c00..cNN} + N Noul questions│
            │      │                                                   │
            │      ▼                                                   │
            │  second jev.Client (no retry, ENGRAM_SEARCH_RERANK_TIMEOUT)
            │      │  success ──▶ map[id]P(relevant)                   │
            │      │  error/timeout ──▶ nil map, err                   │
            └─────────────────────────────────────────────────────────┘
                                          │
                                          ▼
       stable-sort by relevance (ties / hook error ⇒ keep lexical order) ── D-03 fallback
                                          │
                                          ▼
                         truncate to k, stamp Memory.Relevance where scored
                                          │
                                          ▼
        memoryToProto / toRecallView / renderMemoryTable  ── relevance surfaces on all 3 wires
```

### Recommended composition inside `store.SearchReranked`

`internal/store/rerank.go` and `internal/store/store.go` are the two files this touches. `rankCandidates` itself (rerank.go:109-124) must NOT change — `TestRankCandidatesIsTheD05Winner` (rerank_test.go:73) pins it to the D-05 lexical winner and its own doc comment says a future re-tune "changes... deliberately, never silently." The Jev composition is a **new** step layered on top, inside `SearchReranked`:

```go
// Source: internal/store/store.go:1325-1343 (existing, VERIFIED this session)
// SearchReranked today:
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 {
		return nil, fmt.Errorf("%w: SearchReranked requires k > 0 ...", ErrInvalidArgument)
	}
	opts.Full = true
	hits, err := s.Search(ctx, scope, subj, vec, CandidateK(k), opts)
	if err != nil {
		return nil, err
	}
	return rankCandidates(query, hits, int(k)), nil
}
```

Recommended shape after this phase (illustrative — not yet in the tree, so unmarked claims below are a design recommendation, not a verified fact):

```go
// RankHook optionally scores rankCandidates' lexically-ordered CandidateK
// pool with a per-ID relevance probability before SearchReranked truncates
// to k. A nil map or non-nil err means "no scores": SearchReranked keeps
// the lexical order unchanged and attaches no relevance (D-03 fallback).
// Deliberately built from primitive types only (context, store.Memory, a
// plain map/error) so internal/store never imports internal/decide.
type RankHook func(ctx context.Context, query string, hits []Memory) (relevance map[string]float64, err error)

func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 { /* unchanged guard */ }
	opts.Full = true
	hits, err := s.Search(ctx, scope, subj, vec, CandidateK(k), opts)
	if err != nil {
		return nil, err
	}
	// D-04: rankCandidates over the FULL pool, no truncation (k=len(hits))
	// — RerankHits already documents "k <= 0 || k >= len(hits) returns
	// every hit reordered" (rerank.go:73), so this is a supported call
	// shape, not a new code path in RerankHits itself.
	ordered := rankCandidates(query, hits, len(hits))
	if opts.RankHook != nil {
		if rel, herr := opts.RankHook(ctx, query, ordered); herr == nil && rel != nil {
			ordered = stableSortByRelevance(ordered, rel) // ties/missing keep lexical order
		}
	}
	if k >= uint64(len(ordered)) {
		return ordered, nil
	}
	return ordered[:k], nil
}
```

`SearchOptions.RankHook RankHook` (nil zero value) is the injection point — a bare `&deps{}`/zero-value `SearchOptions{}` literal (used throughout the existing test suite, e.g. `internal/store/rerank_test.go`) stays byte-identical, and every EXISTING `SearchReranked` caller that doesn't set it is unaffected. This satisfies the layering constraint the phase context calls out directly: "The server's decider (Phase 2 `deps.decider`) must reach the store rank step without making `internal/store` import `internal/decide`."

### The `search_discovery` divergence (D-07 discretion — resolve explicitly in planning)

`Store.SearchDiscovery` (store.go:1352-1428) is a **separate** method from `Store.Search`/`SearchReranked` — verified by reading its full body this session:

```go
// Source: internal/store/store.go:1400-1414 (excerpt, VERIFIED this session)
res, err := s.client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
    Filter: f, Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(false),
})
...
fetched, err := s.fetchPayloadsByID(ctx, f, s.fullView(), ids)
...
out = append(out, m)  // no rankCandidates call anywhere in this method
```

It fetches exactly `k` (no `CandidateK` over-fetch) and returns pure vector order — `rankCandidates`/`RerankHits` is never invoked for discoveries today. D-07 says relevance must surface on `search_discovery` too, but the phase boundary explicitly excludes "changing the default ranker (lexical stays default)." Two structurally different resolutions exist:

- **Option A (not recommended):** Route `search_discovery` through the same `SearchReranked`-shaped over-fetch + lexical + Jev pipeline unconditionally. This silently introduces the D-05 lexical reranker into discovery ordering for EVERY caller, even with `ranker=lexical` (today's default) — a default-ranking change to a surface the phase boundary says must not change.
- **Option B (recommended):** Leave `Store.SearchDiscovery` byte-identical for the default case. Add a **new** `Store.SearchDiscoveryReranked` (mirroring `SearchReranked`'s exact shape: `CandidateK(k)` over-fetch via the existing discovery vector query, `rankCandidates` full-pool reorder, `RankHook`, truncate) that `deps.searchDiscovery` (tools.go:2054-2087) calls **only when `ranker=jev` is configured**. When `ranker=lexical` (the default), `deps.searchDiscovery` keeps calling `d.st.SearchDiscovery` exactly as today. This keeps the "lexical stays default" boundary intact while still giving `search_discovery` the Jev relevance signal when opted in.

### Recommended Project Structure (files touched, not a new tree)
```
internal/store/
├── rerank.go          # unchanged (rankCandidates stays pinned); may gain stableSortByRelevance helper
├── store.go            # SearchReranked composition change; new SearchOptions.RankHook field; new store.Memory.Relevance field
├── verdictstate.go      # unchanged — NOT reused for fetch (content already in hand); only internal/verdict.State's truncation logic is reused
internal/server/
├── tools.go             # searchMemory/searchDiscovery unchanged signatures; RankHook closure built here from d.decider
├── decider.go           # new: second jev.Client construction (no-retry, ENGRAM_SEARCH_RERANK_TIMEOUT) alongside existing deciderFromConfig
├── connectapi.go        # memoryToProto gains Relevance field mapping
├── summary.go            # recallView gains Relevance field + toRecallView population
internal/config/
├── registry.go          # two new rows: search.ranker, search.rerank_timeout
├── validate.go           # new block: search.ranker enum + jev-requires-decisions-provider check
internal/decide/jev/
├── jev.go               # new Option (e.g. WithNoRetry) OR accept a second full New() call — see Common Pitfalls
internal/retrievaleval/
├── rankers.go           # jev roster entry gains a rank function (still the pure rankerFunc shape)
proto/engram/v1/
├── engram.proto          # optional float relevance = 31; on Memory
gen/                      # regenerated via task proto:gen
cmd/engram/
├── client_search.go       # no change (already forwards full proto response)
├── client_common.go       # renderMemoryTable gains a relevance column/param
docs-site/src/content/docs/
├── guides/configure.md    # new config section (mirrors "## Typed decisions (Jev)")
├── reference/tools.md     # search_memory doc: relevance field description
```

### Pattern: transient (non-persisted) Memory field — exact precedent already in the codebase

`store.Memory.Score` is the direct precedent for `Relevance`: set only after the Qdrant fetch, never written into the payload map. Verified by reading the full payload-building block (store.go:680-729) and confirming `.Score` is assigned only at lines 1276/1298/1424 (post-fetch), never inside the `p := map[string]any{...}` builder. `Relevance` should follow the identical rule:

```go
// Source: internal/store/store.go:281-284 (VERIFIED this session, quoted verbatim)
// Score is the Qdrant similarity score of this record for the query that
// returned it (higher = closer). Set only on Search results; zero on
// list/get. Lets callers see how close a near-miss ranked (GH#261).
Score float32 `json:"score,omitempty"`
```

Recommended sibling field (design, not yet in tree):
```go
// Relevance is the Jev decision-provider's P(this record answers the
// query), set only when ranker=jev ran and succeeded for this search.
// nil otherwise (never a computed-but-zero 0.0 collapsing to omitted —
// a near-zero "no-answer" signal, ~0.02, must still serialize). Never
// persisted: payload() must not write this key, mirroring Score.
Relevance *float64 `json:"relevance,omitempty"`
```
A pointer (not a bare `float64`) is required because `omitempty` on a bare `float64` would also omit a genuine `0.02`-class "nothing answers this" signal if it ever rounded to `0.0` — the codebase's own established idiom for exactly this "present-only-when-known, value can legitimately be zero" case is a pointer (`NotBefore *time.Time`, `LastAccessedAt *time.Time` — store.go:218, 257).

### Pattern: `internal/decide.Request` shape for the one-request-N-questions call (D-04)

Verified from `internal/decide/decide.go` (Request/Question types) and `decision-transport.md`'s live-verified wire shape: ONE `Request` carries a `State` map and a `Questions` map answered in parallel against that shared state — this is exactly D-04's "whole CandidateK pool... in ONE Decisions request... one Noul question per candidate." No cap on the number of `Questions` in a `Request` exists in `Request.Validate()` (`internal/decide/validate.go:21-59` — only `MaxChoices=255` bounds a single *choice* question's option count, never Noul question count), so up to 100 Noul questions in one `Request` is structurally valid.

```go
// Illustrative shape (design, following verdict.NewRequest's precedent at
// internal/verdict/verdict.go:147-159 — a real Request builder for a
// different question set, VERIFIED this session):
req := decide.Request{
    State: decide.State{"query": query, "c00": truncated0, "c01": truncated1 /* ... */},
    Questions: map[string]decide.Question{
        "c00": decide.Noul("Does record c00 directly answer the query?", whenTrue, whenFalse),
        "c01": decide.Noul(...),
        // ...
    },
}
resp, err := searchDecider.Decide(ctx, req)
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-candidate content truncation for the Jev request | A new truncation function | `internal/verdict.State`/`truncateRunes` (verdict.go:83-108) | Already handles UTF-8-safe rune truncation with the exact summary+content-head shape D-08 asks for; a second copy would drift |
| Fetching candidate content for ranking | A new bounded-read call (e.g. reusing `Store.RecordStates`) | The `[]Memory` `SearchReranked` already has in hand (forced `Full`, store.go:1337) | `RecordStates` issues its own `fetchPayloadsByID` round trip (verdictstate.go:30-40) — calling it here would be a wasted second Qdrant fetch for content already resident |
| Retry/backoff for the search-path Jev call | A hand-rolled retry loop in the server layer | Configure a Client that simply doesn't retry (new `jev.Option`, or accept the existing single retry is bounded by a tight `ENGRAM_SEARCH_RERANK_TIMEOUT`) | `jev.Client.Decide` already owns D-11's single retry end-to-end (jev.go:288-317); duplicating retry logic outside the client would double-retry |
| HTTP transport / error classification for the Jev call | A second HTTP client implementation | The existing `jev.Client` (`internal/decide/jev`), constructed a second time with different options | `classifyTransport`/`classifyStatus` (jev/classify.go) already implement DEC-04's HTTP-status-to-named-error mapping; a bespoke client for search would silently diverge from consolidate's classification |
| Proto field presence for a "maybe absent" float | A wrapper message or a sentinel value (`-1`) | `optional float relevance = 31;` (proto3 explicit presence) | The existing `Memory` message already uses `optional` for exactly this "present only when known" case (`optional string superseded_by = 23;`, `optional uint32 schema_version = 28;`) |

**Key insight:** Every piece this phase needs — truncation, decision transport, error classification, retry — was already built correctly by Phases 1–3. The actual engineering surface of Phase 4 is composition and wiring (a new step in `SearchReranked`, three new field-shapers, two new config rows, one new proto field), not new algorithms.

## Common Pitfalls

### Pitfall 1: Editing `rankCandidates` directly to call Jev
**What goes wrong:** `TestRankCandidatesIsTheD05Winner` (rerank_test.go:73-110) fails, and the function's own doc comment states the pin is deliberate — "the tuning-cannot-happen-here pin (T-01-18)."
**Why it happens:** It looks like the natural place to add ranking logic since its doc comment says "It was chosen by... rankCandidates is also the single seam Phase 4's Jev reranker (RANK-03) plugs into (D-08)" — but "plugs into" means "is composed with," not "gets its body replaced."
**How to avoid:** Add a new step AFTER calling `rankCandidates` (with no truncation), never inside it.
**Warning signs:** `TestRankCandidatesIsTheD05Winner` goes red on an unrelated PR.

### Pitfall 2: Assuming `jev.Client` retry is disable-able via an existing option
**What goes wrong:** D-09 requires "no retry on the search path," but `Client.Decide` (jev.go:288-317) performs the single retry unconditionally — there is no `WithNoRetry`/`WithRetry(bool)` option today (confirmed: `rg -n "isRetryable"` finds exactly one call site, inside `Decide` itself, with no option gating it).
**Why it happens:** The phase description's own hint ("per-call context deadline vs a second client") suggests a tight deadline alone might suffice, but the retry's jittered delay (100-400ms, jev.go:77-78, 302-304) is checked against `ctx.Deadline()`, not disabled — a generous `ENGRAM_SEARCH_RERANK_TIMEOUT` (e.g. 2s) leaves room for the retry to fire anyway, which is nondeterministic, not "no retry."
**How to avoid:** Add a genuine `jev.Option` (e.g. `WithNoRetry()`) that skips the `isRetryable` branch entirely, and construct a **second** `*jev.Client` for the search path with that option plus its own `ENGRAM_SEARCH_RERANK_TIMEOUT`-derived timeout — separate from the consolidate-path client's `ENGRAM_DECISIONS_TIMEOUT`.
**Warning signs:** A flaky test asserting "exactly one HTTP call" for a timed-out search-path decide.

### Pitfall 3: Reusing `Store.RecordStates` for candidate content
**What goes wrong:** An extra Qdrant round trip per search, defeating the latency goal (D-09's ~2s budget), and a second place candidate content could diverge from what `SearchReranked` already fetched.
**Why it happens:** D-08's own wording ("reuses Phase 3's `internal/verdict` state construction") reads as "reuse the whole verdict-state fetch pipeline," but `RecordStates` is a **fetch** function (verdictstate.go:30, its own `fetchPayloadsByID` call) — the reusable piece is only `verdict.State`'s **truncation** logic, applied to `Memory.Content`/`Memory.Summary` already sitting in the `[]Memory` slice `SearchReranked` built.
**How to avoid:** Call `verdict.State(m.Summary, m.Content, maxChars)` directly on each already-fetched `Memory`; never call `Store.RecordStates` from the search path.
**Warning signs:** A live-eval run shows two Qdrant fetches per search query in tracing.

### Pitfall 4: Threading `search_discovery` through the unconditional lexical+Jev pipeline
**What goes wrong:** Discovery search ordering silently changes for every caller (even with the default `ranker=lexical`), violating "Not in this phase: changing the default ranker (lexical stays default)."
**Why it happens:** D-07 groups `search_memory` and `search_discovery` together as "everything routed through `SearchReranked`... plus `search_discovery`," which reads as "use the exact same code path" — but `SearchDiscovery` never had a `rankCandidates` step to begin with, so routing it through the *lexical-first* composition is itself a new default-ranking change for discoveries, not an extension of an existing one.
**How to avoid:** Gate the discovery over-fetch+rerank path on `ranker=jev` being active (see Architecture Patterns, Option B); leave the plain vector-order `SearchDiscovery` untouched for the default/off case.
**Warning signs:** A discovery search's hit order changes with `ENGRAM_DECISIONS_PROVIDER` unset / `ENGRAM_SEARCH_RANKER` at its default.

### Pitfall 5: Forgetting all three hand-written shapers when adding `relevance`
**What goes wrong:** `relevance` appears on the Connect wire (full `store.Memory` JSON) but not on MCP's default compact `search_memory` response, or vice versa.
**Why it happens:** There is no single "add a field to Memory and it flows everywhere" mechanism — `recallView` (summary.go:40-60, "these must be explicitly added here AND populated in `toRecallView`... `store.Memory` carrying the fields alone is not enough" per its own doc comment on `AccessCount`), `memoryToProto` (connectapi.go:50-107), and `renderMemoryTable` (client_common.go:506-536) are three independent allow-list shapers, exactly the same shape that already trips up new fields (see the `Score`/`AccessCount` precedent comment in `recallView`).
**How to avoid:** Grep for every existing `Score` reference across `internal/server` and `cmd/engram` and add `Relevance` at each matching site — `Score` is the field with the most structurally identical lifecycle (present on search results, absent/zero on list/get).
**Warning signs:** `TestRerankParityMCPAndConnect`-style parity test passes for hit order but a manual `engram search --format json` shows no `relevance` key.

## Code Examples

### Registry rows to add (mirrors the existing `decisions.*` block exactly)
```go
// Source pattern: internal/config/registry.go:120-130 (VERIFIED this
// session — decisions.* block, quoted structure, not values invented)
{Key: "search.ranker", Env: "ENGRAM_SEARCH_RANKER", Default: "lexical"},
{Key: "search.rerank_timeout", Env: "ENGRAM_SEARCH_RERANK_TIMEOUT", Default: "2s"},
```
No `Legacy` (brand-new keys, nothing retired) and no `Flag` (deployment-topology values, following the `decisions.*` precedent's own stated rationale at registry.go:100-102: "provider-tuning values, never typed at a prompt").

### Validation block to add (mirrors `internal/config/validate.go:284-289`'s unconditional provider-enum check)
```go
// Source pattern: internal/config/validate.go:284-289 (VERIFIED this
// session, structure quoted, values adapted)
if c.Search.Ranker != "" && c.Search.Ranker != "lexical" && c.Search.Ranker != "jev" {
    errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RANKER %q: must be empty, \"lexical\", or \"jev\"", c.Search.Ranker))
}
if c.Search.Ranker == "jev" && c.Decisions.Provider == "" {
    errs = append(errs, errors.New("ENGRAM_SEARCH_RANKER=jev requires ENGRAM_DECISIONS_PROVIDER to be set"))
}
```
This follows the exact conditional-gating shape validate.go already uses for `decisions.provider == "jev"` (validate.go:296-368): the ranker check runs unconditionally (a typo must fail startup even when the feature is off), and the cross-field `ranker=jev requires decisions.provider` check runs only when ranker is jev.

### `internal/verdict.State` — the exact truncation primitive to reuse
```go
// Source: internal/verdict/verdict.go:77-92 (VERIFIED this session, quoted verbatim)
func State(summary, content string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = DefaultStateChars
	}
	full := content
	if summary != "" {
		full = summary + "\n\n" + content
	}
	return truncateRunes(full, maxChars)
}
```
Call this once per candidate with a computed `maxChars` (≈600 default per D-08, shrunk by a total-state guard for large candidate counts — exact constants are Claude's Discretion per CONTEXT.md).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `search_discovery` and `search_memory` both use only vector-then-lexical (or pure vector for discovery) ordering | `search_memory` (and, via a new discretely-gated path, `search_discovery`) can additionally stable-sort by a Jev relevance probability, opt-in | This phase (RANK-03/04/05) | First "absolute nothing-answers-this" signal (Jev P(relevant) ≈ 0.02 on no-answer queries per `recall-rerank.md`'s spike measurement) available to callers; cosine similarity alone cannot express this |
| `internal/retrievaleval`'s Jev roster row is a permanently-disabled stub (`jevDisabledReason = "Jev: disabled"`) | The row gets a real `rankerFunc` closure once a decisions provider is configured | This phase (D-02) | `task eval:retrieval` with `ENGRAM_DECISIONS_PROVIDER=jev` set now reports live Jev recall@k/MRR next to lexical/vector-only, with no bearing on which ranker ships by default |

**Deprecated/outdated:** None — nothing existing is removed or renamed this phase.

## Assumptions Log

> Every claim in this document is either `[VERIFIED: path:lines]` (grounded in a `Read` this session, quoted where load-bearing) or presented explicitly as "design/recommendation, not yet in the tree." Nothing here is `[ASSUMED]` in the sense of "unverified training-data guess about the codebase" — this phase required no external-library research. The table below lists the design choices that ARE genuinely open (left to Claude's Discretion by CONTEXT.md) and therefore need the planner's explicit resolution, not user reconfirmation.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `RankHook` as a `SearchOptions` field (vs. an explicit `SearchReranked` parameter, vs. a wrapping method) is the best injection point | Architecture Patterns | Low — CONTEXT.md explicitly defers this exact choice to the planner ("planner decides the injection point"); the alternative (explicit parameter) is a mechanical rename, not a design risk |
| A2 | `search_discovery`'s Jev path should be a new `SearchDiscoveryReranked` method gated on `ranker=jev`, not a change to `SearchDiscovery` itself | Architecture Patterns | Medium — if the planner instead threads discovery through the same unconditional lexical+Jev pipeline, discovery's default ordering changes for every caller, which the phase boundary explicitly forbids |
| A3 | A new `jev.Option` (e.g. `WithNoRetry`) is required to satisfy D-09's "no retry," rather than relying on a tight timeout alone | Common Pitfalls #2 | Medium — a tight-timeout-only approach is nondeterministic (retry may or may not fire depending on how fast the first attempt failed), which could make a fallback test flaky |
| A4 | `optional float relevance = 31;` (float32, not double) is the right proto type, matching the existing `score` field's precision | Code Examples / Don't Hand-Roll | Low — a double would also work; the existing `score` field (engram.proto:32) sets the float32 precedent for a [0,1]-ish ranking signal on this wire |

**If this table is empty:** N/A — see above; the four rows are open design choices explicitly deferred to the planner by CONTEXT.md's "Claude's Discretion" list, not unverified factual claims.

## Open Questions

1. **Exact per-candidate truncation budget and total-state guard formula (D-08)**
   - What we know: `CandidateK` caps the pool at 100 (rerank.go:17-26, verified); the target ceiling is "≈28k tokens" using a "chars/4" estimate; `internal/verdict.DefaultStateChars = 1500` is the Phase 3 precedent for a *different* (pairwise, 2-record) budget.
   - What's unclear: The exact per-candidate default (≈600 chars suggested in CONTEXT.md) and the shrink formula when many/large candidates would otherwise exceed budget.
   - Recommendation: Explicitly left to Claude's Discretion by CONTEXT.md — the planner should pin concrete constants (e.g. `DefaultSearchStateChars = 600`, a `totalBudgetChars` derived from `(28000 tokens * 4 chars/token) - fixed overhead`) and a deterministic shrink rule (e.g. `min(600, totalBudgetChars/len(candidates))`), then unit-test the boundary at exactly 100 candidates.

2. **Noul question wording per candidate**
   - What we know: `recall-rerank.md` (spike 004) gives verified wording: instructions "Does memory record cNN directly answer the query?", criteria true = "The record directly answers what the query asks," false = "The record is about something else, or only shares vocabulary with the query."
   - What's unclear: Whether this exact wording should be used verbatim or adapted; whether the "context" framing sentence pattern from `verdict.StateContext` (a fixed prose sentence, verdict.go:61-64) should also be applied here.
   - Recommendation: Start from spike 004's verified wording (CONTEXT.md's own instruction); add a fixed context sentence analogous to `verdict.StateContext` only if empirically useful — not required by any locked decision.

3. **Whether the retrieval eval's Jev roster closure needs its own timeout/fallback, given `rankerFunc`'s pure `(query, hits, k) []Memory` signature has no error return**
   - What we know: `evalRankers()` (rankers.go:79-129) defines `rankerFunc` with no `context.Context` or `error` in its signature — every existing entry is synchronous and infallible.
   - What's unclear: How the Jev roster entry's closure should handle a live decide failure/timeout without being able to return an error through `rankerFunc`.
   - Recommendation: The closure should own its own bounded context (e.g. `context.WithTimeout(context.Background(), ...)`) and, on any failure, fall back to `store.RerankHits(query, pool, k)` internally (silently), so the eval always gets a well-formed `[]Memory` — this mirrors D-03's fallback semantics even though the roster's type can't express an explicit error.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker (Qdrant testcontainer) | `task eval:retrieval` (D-02's live artifact run) | Not probed this session (no shell access to the target execution host at research time — sandboxed research environment) | — | Unit/integration tests use fakes (`decide.Decider` fake implementations already exist, e.g. `verdictThresholdFakeDecider`, `scriptedFakeDecider` in `cmd/engram/spine_review_consolidate_test.go:607-790`) — no live dependency needed for the code-level test suite |
| Live Jev/OpenRouter credentials (`ENGRAM_DECISIONS_PROVIDER=jev`, `ENGRAM_DECISIONS_BASE_URL`, key) | D-02's "live Jev numbers... recorded as a phase artifact" | Not probed this session | — | The unit-test suite (RANK-03/04) does not require live credentials — only the `task eval:retrieval` artifact-generation run does, exactly like Phase 1's `01-EVAL-SHIPPED.log`/`01-RANKING-DECISION.md` provenance pattern |

**Missing dependencies with no fallback:**
- None for the code-level implementation and its unit tests.

**Missing dependencies with fallback:**
- Live Qdrant + live Jev credentials for the D-02 eval artifact: this is an operator-run task (`ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v`, confirmed at `Taskfile.yaml:81-84`) that must be executed with real credentials at some point before the phase can close D-02 — flag this as a task requiring a `checkpoint:human-verify` or an explicit "run this when you have credentials" step in the plan, mirroring Phase 3's `03-EVAL-RESULTS.md` provenance artifact.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no third-party framework) |
| Config file | none — `go test` via `Taskfile.yaml` targets |
| Quick run command | `go test ./internal/store/... ./internal/server/... ./internal/config/... ./cmd/engram/... -run <Test...>` |
| Full suite command | `task test` (== `go test ./...` + python hook tests); `task` (== lint + test) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RANK-03 | `SearchReranked` reorders by Jev relevance when hook present; falls back to lexical order on hook error/timeout, call still succeeds | unit | `go test ./internal/store/ -run TestSearchRerankedWithRankHook -v` | ❌ Wave 0 (new test file, e.g. `internal/store/rerank_jev_test.go`) |
| RANK-03 | `rankCandidates` itself stays the D-05 winner, unmodified | unit (existing pin) | `go test ./internal/store/ -run TestRankCandidatesIsTheD05Winner -v` | ✅ (`internal/store/rerank_test.go:73`) — must keep passing unmodified |
| RANK-03 | MCP `search_memory` and Connect `SearchMemories` agree on relevance-influenced order (parity) | integration | `go test ./internal/server/ -run TestRerankParityMCPAndConnect -v` | ✅ existing test (`internal/server/connectapi_test.go:367`) — extend with a jev-enabled fake-decider subtest, do not replace |
| RANK-04 | Per-hit `relevance` present on MCP JSON, Connect proto, CLI table/JSON when ranker=jev succeeded; absent otherwise | unit + golden | New tests in `internal/server/summary_test.go`, `internal/server/connectapi_test.go`, `cmd/engram/client_search_test.go` | ❌ Wave 0 |
| RANK-04 | Connect proto field is additive (buf breaking check passes) | CI gate | `go tool buf breaking --against '.git#branch=main'` (existing CI job — mirrors CLAUDE.md's "buf job") | ✅ existing CI job, no new file needed |
| RANK-05 | Candidate state (query + up to 100 candidates) stays within the ≈28k-token budget | unit | New `TestSearchStateBudgetGuard`-style test asserting total char count under the configured ceiling at 100 candidates | ❌ Wave 0 |
| D-01 | `ENGRAM_SEARCH_RANKER=jev` without `ENGRAM_DECISIONS_PROVIDER` fails `Config.Validate` clearly | unit | Extend `internal/config/decisions_config_test.go`-style table (new `internal/config/search_config_test.go`) | ❌ Wave 0 |
| D-09 | Search-path decider does not retry; consolidate-path decider still does | unit | New test in `internal/decide/jev/` asserting exactly one HTTP call for a retryable failure on the no-retry client, two on the default client | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** targeted `go test ./internal/store/... ./internal/server/... ./internal/config/... ./internal/decide/jev/... -run <relevant tests>`
- **Per wave merge:** `task test` (full Go + Python suite)
- **Phase gate:** `task` (lint + test) green, plus `task proto:lint` / a `buf breaking` check for the new proto field, before `/gsd-verify-work`. The live `task eval:retrieval` (D-02 artifact) and `task eval:decisions` runs are operator-gated (need credentials) and are a separate, explicitly-flagged phase-closing step, not part of the automated gate.

### Wave 0 Gaps
- [ ] `internal/store/rerank_jev_test.go` — covers RANK-03 (`RankHook` composition, fallback-on-error, fallback-on-nil-map)
- [ ] `internal/config/search_config_test.go` — covers D-01 (registry defaults, jev-requires-provider validation)
- [ ] `internal/decide/jev/noretry_test.go` (or extend `jev_test.go`) — covers D-09 (no-retry option/client)
- [ ] Extensions to `internal/server/summary_test.go`, `connectapi_test.go`, `cmd/engram/client_search_test.go` — covers RANK-04 (relevance on all three wires)
- [ ] `internal/store/verdictstate_test.go`-style budget test for the new truncation guard — covers RANK-05

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Unchanged — reranking runs strictly after `ownerScopeFilter`'s authz-scoped `Query` (existing invariant, `SearchReranked`'s own doc comment: "reranking runs strictly AFTER ownerScopeFilter's authz-scoped Query, never widening visibility") |
| V4 Access Control | yes (no new surface, reaffirm) | The `RankHook`/Jev step must never be given more candidates than the already-authz-filtered `hits` — no separate fetch, no widened filter |
| V5 Input Validation | yes | `search.ranker` enum validated in `Config.Validate` (existing pattern); `ENGRAM_SEARCH_RERANK_TIMEOUT` parsed as a Go duration, same as `ENGRAM_DECISIONS_TIMEOUT` |
| V6 Cryptography | no | No new crypto surface |
| V9/V13 Data protection / API security (egress disclosure) | yes | Candidate `Content`/`Summary` text is sent to the external Jev/OpenRouter provider on every `search_memory`/`search_discovery` call once `ranker=jev` is enabled — this is a NEW, always-on egress path (unlike consolidate's per-invocation `--no-verdicts` opt-out), so it needs its own disclosure control |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Record content egress to a third-party decision provider on every search, not just an operator-invoked sweep | Information disclosure | Phase 3's precedent (`03-SECURITY.md` T-03-01: "Gated on provider + `--no-verdicts`; disclosure line before any send") does not directly transfer — `search_memory` has no interactive terminal to print a disclosure line to. Recommend: `logDeciderEnabled`-style one-time `slog.Info` at server startup naming that `ENGRAM_SEARCH_RANKER=jev` is active (mirroring `internal/server/decider.go:251-269`'s existing `logDeciderEnabled`, which already logs provider/model/host — never the key), so the fact is visible in server logs/ops runbooks even though no per-call disclosure is possible |
| DoS via unbounded candidate content sent to Jev | Denial of service | `CandidateK`'s existing 100-candidate hard cap (rerank.go:17-26) plus the new D-08 total-state guard bounds the request size regardless of `k`; mirrors Phase 3's `verdictStateRecordCeiling` DoS mitigation (T-03-10) |
| Retry storm from a shared jev.Client between consolidate and search paths | Denial of service (self-inflicted, amplified provider cost/latency) | D-09's separate no-retry search-path client (see Common Pitfalls #2) also prevents the search path's per-request-under-load Jev call volume from silently doubling via retries |
| Malformed/adversarial provider response (bad probability values, wrong candidate IDs) | Tampering | Reuse the same validate-before-trust pattern `verdict.FromResult` establishes (verdict.go:246-270): never trust a Jev answer key blindly — map `Answers["cNN"]` defensively, treat a missing/malformed answer for any candidate as "no score for that candidate" (falls back to lexical position for that one hit), never as a fatal error for the whole search |

## Sources

### Primary (HIGH confidence — all `Read` this session, this repository)
- `.planning/phases/04-jev-reranker-per-hit-relevance-signal/04-CONTEXT.md` — locked decisions, canonical refs
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md` — requirement IDs, milestone state
- `internal/store/rerank.go` (full file), `internal/store/store.go` (lines 1-300, 1100-1450, 680-729) — `SearchReranked`, `SearchOptions`, `Search`, `SearchDiscovery`, `Memory` struct, payload codec
- `internal/store/verdictstate.go`, `internal/store/boundedread.go` (lines 280-340) — `RecordStates`, view-sizing precedent
- `internal/decide/decide.go`, `internal/decide/errors.go`, `internal/decide/validate.go` — `Decider`, `Request`/`Question`/`Answer`, error classes, structural validation
- `internal/decide/jev/jev.go` (lines 1-340) — `Client`, `New`, `Decide`, retry/timeout mechanics
- `internal/verdict/verdict.go` (full file) — `State`, `truncateRunes`, `NewRequest`, `Probabilities`, `FromResult`
- `internal/server/decider.go` (full file), `internal/server/tools.go` (lines 1-130, 1800-2090) — `deciderFromConfig`, `deps` struct, `searchMemory`, `searchDiscovery`
- `internal/server/connectapi.go` (lines 1-260, 329-378), `internal/server/summary.go` (full file) — `memoryToProto`, `shapeProtoMemories`, `recallView`, `toRecallView`, `SearchMemories` handler
- `internal/config/registry.go` (full file), `internal/config/validate.go` (lines 260-380), `internal/config/decisions_docs_test.go` (lines 1-60) — registry/validate/docs-gate conventions
- `proto/engram/v1/engram.proto` (full file) — `Memory` message field numbers, `SearchMemoriesRequest/Response`
- `internal/retrievaleval/rankers.go` (full file) — roster, `rankerFunc`, disabled Jev stub
- `cmd/engram/client_search.go` (full file), `cmd/engram/client_common.go` (lines 500-550) — `engram search`, `renderMemoryTable`
- `internal/store/rerank_test.go` (lines 1-110), `internal/server/connectapi_test.go` (lines 340-430) — `TestRankCandidatesIsTheD05Winner`, `TestRerankParityMCPAndConnect`
- `docs-site/src/content/docs/guides/configure.md` (headings), `docs-site/src/content/docs/reference/tools.md` (lines 133-165) — docs structure to extend
- `Taskfile.yaml` (lines 1-95, 340-385) — `task eval:retrieval`/`eval:decisions`, `task proto:lint`/`proto:gen`, `task test`
- `.claude/skills/spike-findings-engram/references/recall-rerank.md`, `.../decision-transport.md` — spike-verified wire shapes, latency, token-budget constraints, Noul question wording

### Secondary (MEDIUM confidence)
- None — this phase required no external documentation lookup.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency; every reused package verified by direct read
- Architecture: HIGH for constraints (rankCandidates pin, forced Full fetch, SearchDiscovery divergence, jev retry baked-in) — MEDIUM for the specific composition design (RankHook shape), since that code does not exist yet and is a recommendation, not a verified fact
- Pitfalls: HIGH — each pitfall is derived from a concrete, cited code constraint, not speculation

**Research date:** 2026-09-24
**Valid until:** Stable — this research is tied to the current shape of `internal/store`, `internal/decide`, and `proto/engram/v1/engram.proto` in this repository; re-verify the cited line numbers if a phase lands between this research and Phase 4's planning that touches any of `store.go`, `rerank.go`, `jev.go`, or `engram.proto`.
