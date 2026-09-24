# Phase 4: Jev Reranker & Per-Hit Relevance Signal - Pattern Map

**Mapped:** 2026-09-24
**Files analyzed:** 15 (12 modified existing files, ~4 new test files; no wholly new production files — this phase composes existing seams)
**Analogs found:** 15 / 15 (every file's analog is either itself, a directly adjacent sibling in the same file, or a structurally identical existing block elsewhere in the tree)

All files touched by this phase already exist and are git-tracked in the main
repo (verified via `git ls-files`) — there is no capability-mirror path risk
here. Because RANK-03/04/05 are pure composition over Phases 1–3's seams (per
RESEARCH.md's own framing), most "analogs" are the *existing neighboring code
in the same file* that the new code must extend without disturbing — these
are called out explicitly below rather than pointed at an unrelated file.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/rerank.go` | service (pure ranking transform) | transform | itself — `rankCandidates`/`RerankHits` (pinned, do not edit body) | exact — new `stableSortByRelevance` helper sits beside `RerankHits`/`VectorOrder` |
| `internal/store/store.go` (`SearchReranked`, `SearchOptions`, `Memory`) | service (store layer) | CRUD / request-response | itself — `SearchReranked` (store.go:1325-1343), `Memory.Score` field (store.go:281-284), `SearchOptions.Full` (store.go:1160-1165) | exact — same file, same struct, sibling field pattern |
| `internal/store/store.go` (`SearchDiscoveryReranked`, new) | service (store layer) | CRUD / request-response | `SearchReranked` (store.go:1325-1343) — shape to mirror; `SearchDiscovery` (store.go:1352-1428) — the byte-identical default path it must NOT disturb | role-match (new method modeled on a sibling method in the same file) |
| `internal/server/tools.go` (`searchMemory`, `searchDiscovery`) | controller (MCP tool handler) | request-response | itself — `deps.searchMemory` (tools.go:1893-1920), `deps.searchDiscovery` (tools.go:2054-2087) | exact — same functions gain a `RankHook`-populated `SearchOptions` field |
| `internal/server/decider.go` (second no-retry client) | service (provider client construction) | request-response | itself — `deciderFromConfig` (decider.go:35-52), the `decisionsTimeout`/`decisionsMaxTimeout`/... resolver family (decider.go:54-144) | exact — new `searchDeciderFromConfig`-shaped sibling function in the same file |
| `internal/server/connectapi.go` (`memoryToProto`) | controller (Connect API handler / proto shaper) | request-response | itself — `memoryToProto` (connectapi.go:50-107) | exact — one more field in the same allow-list struct literal |
| `internal/server/summary.go` (`recallView`, `toRecallView`) | service (response-shaping transform) | transform | itself — `recallView` struct + `toRecallView` (summary.go:40-105), doc comment explicitly warns new fields need both sites touched | exact — same file, same documented allow-list pattern |
| `internal/config/registry.go` (2 new rows) | config | config | itself — the `decisions.*` block (registry.go:99-130) | exact — identical row shape, new `search.*` key namespace |
| `internal/config/validate.go` (new block) | config | config | itself — `Decisions.Provider` unconditional check + `Decisions.Provider == "jev"` gated block (validate.go:284-368) | exact — same two-tier (unconditional enum + gated cross-field) shape |
| `internal/config/config.go` (`SearchConfig`, new) | config | config | `DecisionsConfig` struct (config.go:230-248) | exact — new sibling `koanf`-tagged struct + `Config` field |
| `internal/decide/jev/jev.go` (`WithNoRetry` Option) | service (API client) | request-response | itself — the `Option` family (jev.go:101-166, e.g. `WithTimeout`/`WithConcurrency`), the retry branch it gates (jev.go:300-316) | exact — same functional-options pattern, one more `Option` |
| `internal/retrievaleval/rankers.go` (jev roster entry) | utility (eval harness) | batch | itself — the `lexical`/`vector-only` roster entries (rankers.go:80-99), `jevDisabledReason` stub (rankers.go:123-128) | exact — same `namedRanker`/`rankerFunc` shape, roster entry gains a `rank` closure |
| `proto/engram/v1/engram.proto` (`Memory.relevance`) | schema | schema | itself — `optional string superseded_by = 23;` / `optional uint32 schema_version = 28;` (engram.proto:44,52), `float score = 17;` (engram.proto:32) | exact — next free field number (31), same `optional` presence idiom |
| `cmd/engram/client_common.go` (`renderMemoryTable`) | utility (CLI table renderer) | transform | itself — the `withScore` conditional column (client_common.go:506-536) | exact — same conditional-column pattern, gated the same way `withScore` already is |
| `internal/decide/decide.go` / `internal/verdict/verdict.go` (Request-building reference, no edit) | service (request builder, reference only) | request-response | `verdict.NewRequest`/`verdict.State` (verdict.go:83-92, 147-159) | exact — reused verbatim (truncation), imitated in shape (Request building), never edited |

## Pattern Assignments

### `internal/store/rerank.go` (service, transform) — new `stableSortByRelevance` helper

**Analog:** the file's own `VectorOrder`/`RerankHits` (same file, lines 80-146) — a pure, deterministic, stably-sorted transform over `[]Memory`.

**Existing pin — DO NOT touch `rankCandidates`'s body:**
```go
// Source: internal/store/rerank.go:109-124 (verbatim)
// rankCandidates is the single rank step SearchReranked applies to its
// already authz-filtered candidate pool — its final call before truncation
// to the caller's k. ... rankCandidates is also the single
// seam Phase 4's Jev reranker (RANK-03) plugs into (D-08).
//
// If a future live eval re-run selects a different winner, this function's
// body changes to match — and rerank_test.go's TestRankCandidatesIsTheD05Winner
// is the pin that must be updated deliberately, never silently.
func rankCandidates(query string, hits []Memory, k int) []Memory {
	return RerankHits(query, hits, k)
}
```
`TestRankCandidatesIsTheD05Winner` (`internal/store/rerank_test.go:73`) fails
if this body changes — the Jev step is a *new function added after it*, never
an edit to it.

**Core pattern to imitate — deterministic stable sort with an explicit tie-break, mirroring `RerankHits`:**
```go
// Source: internal/store/rerank.go:90-98 (verbatim — the sort shape to imitate,
// not the tie-break values themselves)
sort.SliceStable(ranked, func(i, j int) bool {
	if ranked[i].overlap != ranked[j].overlap {
		return ranked[i].overlap > ranked[j].overlap
	}
	if ranked[i].m.Score != ranked[j].m.Score {
		return ranked[i].m.Score > ranked[j].m.Score
	}
	return ranked[i].m.ID < ranked[j].m.ID
})
```
New `stableSortByRelevance(hits []Memory, rel map[string]float64) []Memory`
should follow this exact shape: `sort.SliceStable`, a relevance-descending
primary key, falling back to **the hits' current (lexical) order** for ties
or missing entries — never a secondary tie-break that could override the D-03
"ties keep lexical order" fallback contract. `VectorOrder` (rerank.go:133-146)
is the second sibling to mirror for the "PURE function, copies input, never
mutates" discipline.

---

### `internal/store/store.go` — `SearchReranked` composition + `SearchOptions.RankHook` + `Memory.Relevance`

**Analog:** the function/struct being extended, in the same file.

**Imports pattern** (file already imports everything needed — no new import required for the composition itself; `context`, `store.Memory` stay primitive, per the layering constraint):
```go
// Source: internal/store/store.go:1321-1324 (verbatim doc comment — the
// layering rule the RankHook type must satisfy)
// SearchReranked takes plain inputs (query text, query vector, k) and does NOT
// import internal/embed or internal/server (round-2 finding 7): embedding
// happens in the caller before this is invoked; this helper only reorders an
// already-fetched, already-authorized []Memory.
```

**Core pattern — the exact function to extend, in place:**
```go
// Source: internal/store/store.go:1325-1343 (verbatim, current shape)
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 {
		return nil, fmt.Errorf("%w: SearchReranked requires k > 0 (caller must apply its default before calling)", ErrInvalidArgument)
	}
	opts.Full = true
	hits, err := s.Search(ctx, scope, subj, vec, CandidateK(k), opts)
	if err != nil {
		return nil, err
	}
	return rankCandidates(query, hits, int(k)), nil
}
```
Add the `RankHook` step AFTER `rankCandidates(query, hits, len(hits))` (no
truncation — D-04 needs the full pool) and BEFORE the final `[:k]` slice, per
RESEARCH.md's Architecture Patterns section (already verified against this
exact code). `opts.Full = true` is retained unconditionally — this is why no
second fetch/`RecordStates` call is needed for Jev's candidate content (see
Common Pitfalls in RESEARCH.md, Pitfall 3).

**Sibling-field pattern for `SearchOptions.RankHook`** (add beside `Full`):
```go
// Source: internal/store/store.go:1160-1165 (verbatim — the field this
// mirrors structurally: a per-call knob SearchReranked's own doc comment
// must explain)
// Full selects the D-09 fetch phase's payload projection: the summary
// view by default (false), the full view when true. Governs ONLY
// Store.Search's own fetch phase — SearchReranked forces this on
// unconditionally for its own delegated call regardless of what the
// caller passed here; see SearchReranked's doc comment for why.
Full bool
```

**Transient (non-persisted) field pattern for `Memory.Relevance`** — direct precedent is `Score`:
```go
// Source: internal/store/store.go:281-284 (verbatim)
// Score is the Qdrant similarity score of this record for the query that
// returned it (higher = closer). Set only on Search results; zero on
// list/get. Lets callers see how close a near-miss ranked (GH#261).
Score float32 `json:"score,omitempty"`
```
Use a **pointer** (`*float64`), not a bare float, because `omitempty` on a
bare `float64` would also swallow a genuine near-zero "no-answer" signal
(~0.02) — the codebase's own precedent for exactly this "present-only-when-
known, legitimately zero" shape is `NotBefore`/`LastAccessedAt`:
```go
// Source: internal/store/store.go:218, 257 (verbatim — the pointer-field
// idiom to copy for Relevance)
NotBefore *time.Time `json:"not_before,omitempty"`
...
LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
```
Confirm at write time that `Relevance` is **never** added to the `p :=
map[string]any{...}` payload-builder block (store.go ~680-729) — `Score` is
the existing proof this is safe: it is assigned only post-fetch (store.go
lines 1276, 1298, 1424), never inside that builder.

**Error handling / fallback pattern:** `SearchReranked` never returns an
error from the rank step itself — a `RankHook` error or nil map is swallowed
(D-03's fallback), never propagated as a `SearchReranked` error. This mirrors
the existing shape where `rankCandidates` cannot fail either — the *only*
error return in this function is the pre-existing `k == 0` guard and `s.Search`'s
own error, both unchanged.

---

### `internal/store/store.go` — new `SearchDiscoveryReranked` (D-07 discretion, gated on `ranker=jev`)

**Analog:** `SearchReranked` (store.go:1325-1343) for the shape to mirror; `SearchDiscovery` (store.go:1352-1428) is the byte-identical default-case path that must stay untouched.

**Core pattern — what `SearchDiscovery` does today (do not change this for the default/off case):**
```go
// Source: internal/store/store.go:1400-1414 (verbatim, excerpt)
res, err := s.client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: s.collection, Query: qdrant.NewQuery(vec...),
    Filter: f, Limit: qdrant.PtrOf(k), WithPayload: qdrant.NewWithPayload(false),
})
...
fetched, err := s.fetchPayloadsByID(ctx, f, s.fullView(), ids)
...
out = append(out, m)  // no rankCandidates call anywhere in this method
```
`SearchDiscovery` fetches exactly `k` (no `CandidateK` over-fetch) and never
calls `rankCandidates`/`RerankHits`. Per RESEARCH.md's recommended Option B
(and to satisfy the phase boundary "lexical stays default"), build a
**separate** `SearchDiscoveryReranked` method — over-fetch via `CandidateK(k)`,
apply `rankCandidates` (full pool, no truncation), accept the same `RankHook`
shape, truncate to `k` — used by `deps.searchDiscovery` ONLY when
`ranker=jev` is configured; the plain `SearchDiscovery` call stays exactly as
it is today for the default case. See RESEARCH.md Architecture Patterns /
Common Pitfall 4 for the full rationale — this file mirrors that
recommendation but does not restate proof.

---

### `internal/server/tools.go` — `searchMemory` / `searchDiscovery`

**Analog:** the functions themselves, in the same file.

**Core pattern — where the new `RankHook` (built from `deps.decider`) plugs in:**
```go
// Source: internal/server/tools.go:1893-1920 (verbatim, current shape)
func (d *deps) searchMemory(ctx context.Context, c caller, req coreSearchRequest) ([]store.Memory, error) {
	if req.Query == "" {
		return nil, argErrf(classMalformed, HintRequired, "query", "query is required")
	}
	if err := rejectOverMaximumCount("k", req.K); err != nil {
		return nil, err
	}
	scope, err := effectiveSearchScope(req.Scope, req.CrossSpine)
	if err != nil {
		return nil, err
	}
	vec, err := d.em.EmbedQuery(ctx, req.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchReranked(ctx, scope, c.Subj, req.Query, vec, req.K, store.SearchOptions{
		Tags:              req.Tags,
		Categories:        req.Categories,
		CreatedAfter:      req.CreatedAfter,
		CreatedBefore:     req.CreatedBefore,
		IncludeArchived:   req.IncludeArchived,
		IncludeSuperseded: req.IncludeSuperseded,
		IncludeScheduled:  req.IncludeScheduled,
	})
}
```
Add `RankHook: d.rankHook` (or similarly named field, built once at `deps`
construction from `d.decider`/the second no-retry Jev client, nil when
`ranker != jev`) to the `store.SearchOptions{...}` literal — same shape as
every other field already threaded through here. `deps.searchDiscovery`
(tools.go:2054-2087) needs the equivalent branch: call
`d.st.SearchDiscoveryReranked` only when the hook is non-nil, else keep
calling `d.st.SearchDiscovery` exactly as today (see D-07 discretion above).

**`deps` struct field precedent for wiring the decider through:**
```go
// Source: internal/server/tools.go:90-99 (verbatim, doc comment on the
// existing decider field this phase's second client sits beside)
// decider is the typed-decision backend (internal/decide.Decider). nil
// ... buildDepsFromEnv through deciderFromConfig (D-01). This phase (02-05)
// adds no caller of decider.Decide/DecideMany: Phase 4's search_memory
// ... continues, never branching on decider being present as a correctness
// ...
decider decide.Decider
```
A new `searchDecider decide.Decider` (or a resolved `store.RankHook` field
directly) belongs beside this, populated by `buildDepsFromEnv` the same way
`decider` is (tools.go:331,347 — `dec, err := deciderFromConfig(cfg)` /
`decider: dec,`).

---

### `internal/server/decider.go` — second no-retry client construction

**Analog:** `deciderFromConfig` and the `decisionsTimeout`/`decisionsMaxTimeout`/`decisionsDrainBytes`/`decisionsDrainTimeout`/`decisionsConcurrency` resolver family, all in the same file.

**Imports pattern** (already present, reused verbatim):
```go
// Source: internal/server/decider.go:1-21 (verbatim)
import (
	"cmp"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/decide/jev"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/verdict"
)
```

**Core pattern — the client-construction shape to clone for a second, no-retry client:**
```go
// Source: internal/server/decider.go:35-52 (verbatim)
func deciderFromConfig(cfg *config.Config) (decide.Decider, error) {
	switch cfg.Decisions.Provider {
	case "":
		return nil, nil
	case "jev":
		apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)
		return jev.New(cfg.Decisions.BaseURL, apiKey, cfg.Decisions.Model,
			jev.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
			jev.WithTimeout(decisionsTimeout(cfg)),
			jev.WithMaxTimeout(decisionsMaxTimeout(cfg)),
			jev.WithDrainBytes(decisionsDrainBytes(cfg)),
			jev.WithDrainTimeout(decisionsDrainTimeout(cfg)),
			jev.WithConcurrency(decisionsConcurrency(cfg)),
		), nil
	default:
		return nil, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: unknown provider (want \"\" or \"jev\")", cfg.Decisions.Provider)
	}
}
```
A new `searchDeciderFromConfig(cfg *config.Config) (decide.Decider, error)`
should follow this exact shape gated on `cfg.Search.Ranker == "jev"` instead
of `cfg.Decisions.Provider`, reusing `cfg.Decisions.BaseURL`/`APIKey`/`Model`
(the search-path decider is still a Jev client — only the ranker enum and the
timeout/no-retry option differ) plus the new
`jev.WithNoRetry()` option and a `searchRerankTimeout(cfg)` resolver mirroring
`decisionsTimeout` exactly:
```go
// Source: internal/server/decider.go:59-69 (verbatim — the resolver shape
// to clone for searchRerankTimeout, reading ENGRAM_SEARCH_RERANK_TIMEOUT
// instead of ENGRAM_DECISIONS_TIMEOUT, default ~2s instead of 10s)
func decisionsTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Decisions.Timeout)
	if err != nil || d < 0 {
		if cfg.Decisions.Timeout != "" {
			slog.Warn("ENGRAM_DECISIONS_TIMEOUT is set but unparseable or negative; using default 10s",
				"value", cfg.Decisions.Timeout)
		}
		return 10 * time.Second
	}
	return d
}
```

**Logging pattern** for the "feature enabled" startup Info line (Security
Domain in RESEARCH.md recommends this for the always-on egress path):
```go
// Source: internal/server/decider.go:245-269 (verbatim — logDeciderEnabled,
// the pattern to clone for a searchRankerEnabled-style log line; never logs
// the API key itself, only which env var supplied it)
func logDeciderEnabled(cfg *config.Config) {
	var host string
	if u, err := url.Parse(cfg.Decisions.BaseURL); err == nil {
		host = u.Host
	}
	apiKeySource := "none"
	switch {
	case cfg.Decisions.APIKey != "":
		apiKeySource = "ENGRAM_DECISIONS_API_KEY"
	case cfg.OpenAI.APIKey != "":
		apiKeySource = "ENGRAM_OPENAI_API_KEY"
	}
	slog.Info("typed decisions enabled",
		"provider", cfg.Decisions.Provider,
		"model", cfg.Decisions.Model,
		"endpoint_host", host,
		"api_key_source", apiKeySource,
	)
}
```

---

### `internal/decide/jev/jev.go` — new `WithNoRetry()` Option

**Analog:** the existing `Option` family in the same file.

**Core pattern — an `Option` to clone:**
```go
// Source: internal/decide/jev/jev.go:113-119 (verbatim — WithTimeout,
// the simplest Option to model WithNoRetry on: sets one Client field)
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}
```
`WithNoRetry()` should set a new `noRetry bool` field on `Client`, checked at
the single retry-gating site:
```go
// Source: internal/decide/jev/jev.go:299-316 (verbatim — the exact branch
// WithNoRetry's field must gate; today unconditional)
resp, err = c.attempt(ctx, body, req)
if err != nil && isRetryable(err) {
	if dl, ok := ctx.Deadline(); ok {
		d := c.retryDelay()
		if time.Until(dl) > d {
			timer := time.NewTimer(d)
			select {
			case <-timer.C:
				resp, err = c.attempt(ctx, body, req)
			case <-ctx.Done():
				timer.Stop()
			}
		}
	}
}
return resp, err
```
Change the guard to `if err != nil && !c.noRetry && isRetryable(err)`. Do not
add a second retry-loop elsewhere (RESEARCH.md Don't Hand-Roll table) — the
existing `isRetryable`/`classify.go` machinery stays the single source of
truth for what is retryable at all.

**Test analog:** `internal/decide/jev/jev_test.go:35`
(`TestJevRetryAndBounds`) and `internal/decide/jev/classify_test.go:150-165`
(`isRetryable` table) are the direct test-shape precedents for a new
`TestJevNoRetryOption`-style test asserting exactly one HTTP call with
`WithNoRetry()` set against a retryable failure, versus two without it.

---

### `internal/config/registry.go` — `search.ranker` / `search.rerank_timeout`

**Analog:** the `decisions.*` block, same file.

**Core pattern (verbatim structure, values adapted):**
```go
// Source: internal/config/registry.go:120-130 (verbatim — decisions.* block
// shape to clone for search.*)
{Key: "decisions.provider", Env: "ENGRAM_DECISIONS_PROVIDER"},
{Key: "decisions.base_url", Env: "ENGRAM_DECISIONS_BASE_URL"},
{Key: "decisions.api_key", Env: "ENGRAM_DECISIONS_API_KEY"},
{Key: "decisions.model", Env: "ENGRAM_DECISIONS_MODEL", Default: "typesafe/jev-1.13"},
{Key: "decisions.timeout", Env: "ENGRAM_DECISIONS_TIMEOUT", Default: "10s"},
```
New rows follow the identical `{Key, Env, Default}` shape with no `Legacy`
(brand-new keys) and no `Flag` (deployment-topology value, per the same
rationale documented at registry.go:99-119 for the `decisions.*` block):
```go
{Key: "search.ranker", Env: "ENGRAM_SEARCH_RANKER", Default: "lexical"},
{Key: "search.rerank_timeout", Env: "ENGRAM_SEARCH_RERANK_TIMEOUT", Default: "2s"},
```

---

### `internal/config/config.go` — new `SearchConfig` struct + `Config.Search` field

**Analog:** `DecisionsConfig`, same file.

```go
// Source: internal/config/config.go:230-248 (verbatim — DecisionsConfig,
// the struct shape to clone for SearchConfig)
type DecisionsConfig struct {
	Provider     string `koanf:"provider"`
	BaseURL      string `koanf:"base_url"`
	APIKey       string `koanf:"api_key"`
	Model        string `koanf:"model"`
	Timeout      string `koanf:"timeout"`
	...
}
```
New `SearchConfig` (two fields: `Ranker string \`koanf:"ranker"\`` and
`RerankTimeout string \`koanf:"rerank_timeout"\``) is added to the `Config`
struct's field list the same way `Decisions DecisionsConfig
\`koanf:"decisions"\`` is listed (config.go:30).

---

### `internal/config/validate.go` — ranker enum + cross-field check

**Analog:** the `Decisions.Provider` validation block, same file.

**Core pattern (verbatim structure, adapted per Code Examples in RESEARCH.md — already verified against this exact code this session):**
```go
// Source: internal/config/validate.go:284-296 (verbatim — the two-tier
// shape to clone: unconditional enum check, then a gated cross-field check)
// decisions.provider (D-01): checked unconditionally, unlike every other
// decisions.* field below — a typo in the provider enum must fail startup
// even when the feature is otherwise off.
if c.Decisions.Provider != "" && c.Decisions.Provider != "jev" {
	errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: must be empty (off) or \"jev\"", c.Decisions.Provider))
}

// Gated like Summarize.Model above (D-01): a deployment that never sets
// ENGRAM_DECISIONS_PROVIDER validates byte-identically to before this
// block existed. ...
if c.Decisions.Provider == "jev" {
	...
}
```
New block:
```go
if c.Search.Ranker != "" && c.Search.Ranker != "lexical" && c.Search.Ranker != "jev" {
    errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RANKER %q: must be empty, \"lexical\", or \"jev\"", c.Search.Ranker))
}
if c.Search.Ranker == "jev" && c.Decisions.Provider == "" {
    errs = append(errs, errors.New("ENGRAM_SEARCH_RANKER=jev requires ENGRAM_DECISIONS_PROVIDER to be set"))
}
```
Note D-01's exact wording is already this shape — no invention needed.
`ENGRAM_SEARCH_RERANK_TIMEOUT` gets its own duration-parse check mirroring
`decisions.timeout`'s block (validate.go:312-319: parse, reject negative,
non-fatal-defaulted-elsewhere — same shape).

---

### `internal/server/connectapi.go` — `memoryToProto` gains `Relevance`

**Analog:** the function itself, same file — a hand-written allow-list struct literal.

**Core pattern:**
```go
// Source: internal/server/connectapi.go:80-106 (verbatim, excerpt)
return &engramv1.Memory{
	Id: m.ID, Content: m.Content, Scope: m.Scope,
	...
	Score:          m.Score,
	ShortId:        m.ShortID,
	AccessCount:    m.AccessCount,
	LastAccessedAt: lastAccessed,
	...
}
```
Add `Relevance: m.Relevance` (a `*float64` maps directly to a proto3
`optional float` via a small nil-check helper, following the exact
`lastAccessed`/`notBefore`/`notAfter`/`archivedAt` nil-guard pattern already
in this function, lines 51-79 — e.g.:
```go
// Source: internal/server/connectapi.go:74-79 (verbatim — the nil-guard
// pattern to clone for a *float64 -> *float32 proto conversion)
var summaryEgressAt *timestamppb.Timestamp
if !m.SummaryEgressAt.IsZero() {
	summaryEgressAt = timestamppb.New(m.SummaryEgressAt)
}
```
For a scalar `optional float`, `proto.Float32` (mirroring the existing
`proto.Uint32(...)`/`proto.String(...)` calls at connectapi.go:103-104) over
a nil-checked `*m.Relevance` is the idiom — but unlike `SchemaVersion`
(always set, D-14 §3), `Relevance` is conditionally nil, so it follows the
`lastAccessed`-style `if m.Relevance != nil { ... }` guard, not the
unconditional `proto.Uint32` one.

---

### `internal/server/summary.go` — `recallView` / `toRecallView` gain `Relevance`

**Analog:** the struct/function themselves, same file — this file's own doc comment names exactly this failure mode.

**Core pattern:**
```go
// Source: internal/server/summary.go:50-59 (verbatim — the doc comment
// explicitly warning that a new field needs BOTH the struct and
// toRecallView touched)
// Score is the Qdrant similarity score on search results (higher = closer);
// omitted (zero) on list results, which are not ranked.
Score float32 `json:"score,omitempty"`
// AccessCount / LastAccessedAt are the D-07 read-only usage-signal curation
// fields (Phase 12). recallView is a hand-written allow-list — these must
// be explicitly added here AND populated in toRecallView to surface on the
// compact list/search shape; store.Memory carrying the fields alone is not
// enough.
AccessCount    uint64     `json:"access_count"`
LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
```
```go
// Source: internal/server/summary.go:96-104 (verbatim — toRecallView, the
// population site that must be touched too)
func toRecallView(m store.Memory, maxChars int) recallView {
	summary, truncated := summaryOrTruncation(m, maxChars)
	return recallView{
		ID: m.ID, ShortID: m.ShortID, Summary: summary, SummarySource: string(m.SummarySource), Truncated: truncated,
		Scope: m.Scope, Category: m.Category, Tags: m.Tags, CreatedAt: m.CreatedAt,
		Score:          m.Score,
		AccessCount:    m.AccessCount,
		LastAccessedAt: m.LastAccessedAt,
	}
}
```
Add `Relevance *float64 \`json:"relevance,omitempty"\`` to the struct and
`Relevance: m.Relevance,` to the literal in `toRecallView` — both sites,
matching `LastAccessedAt`'s pointer/omitempty precedent exactly (same file).

---

### `cmd/engram/client_common.go` — `renderMemoryTable` gains a relevance column

**Analog:** the function itself, same file — the existing `withScore`-gated column is the direct template.

**Core pattern:**
```go
// Source: cmd/engram/client_common.go:506-531 (verbatim, current shape)
func renderMemoryTable(w io.Writer, mems []*engramv1.Memory, withScore bool) error {
	...
	if withScore {
		writeLine("SHORT_ID\tSCOPE\tCATEGORY\tSTATE\tSCORE\tSUMMARY\n")
	} else {
		writeLine("SHORT_ID\tSCOPE\tCATEGORY\tSTATE\tSUMMARY\n")
	}
	now := time.Now()
	for _, m := range mems {
		summary := truncateSummary(m.GetSummary(), 80)
		state := memoryStateCell(m, now)
		if withScore {
			writeLine("%s\t%s\t%s\t%s\t%.4f\t%s\n",
				m.GetShortId(), m.GetScope(), m.GetCategory(), state, m.GetScore(), summary)
		} else {
			writeLine("%s\t%s\t%s\t%s\t%s\n",
				m.GetShortId(), m.GetScope(), m.GetCategory(), state, summary)
		}
	}
	...
}
```
`renderMemoryTable` is called with `withScore=true` only from `engram search`
(`cmd/engram/client_search.go:79`) — `engram list` always passes `false`
(`client_list.go:80`). A `relevance` column should follow the exact same
conditional-column shape (either folded into the existing `withScore` bool's
branch when relevance is present in the proto response, or a new parallel
`withRelevance`/derived-from-`m.HasRelevance()` check) — CLI column
formatting is explicitly Claude's Discretion per CONTEXT.md, but the
mechanism (a boolean-gated extra `\t%v` column + header cell) must match this
existing idiom, not invent a new table-rendering approach.

**Call-site precedent (all 3 existing callers, unchanged signatures to extend, not replace):**
```
cmd/engram/client_list.go:80    renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), false)
cmd/engram/client_search.go:79  renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), true)
```

---

### `internal/retrievaleval/rankers.go` — jev roster entry gains a `rank` closure

**Analog:** the `lexical`/`vector-only` roster entries, same file.

**Core pattern:**
```go
// Source: internal/retrievaleval/rankers.go:80-99 (verbatim, excerpt — the
// namedRanker shape a live jev entry must match)
{
	name:       "vector-only",
	family:     "vector-only",
	simplicity: 0,
	rank: func(_ string, pool []store.Memory, k int) []store.Memory {
		return store.VectorOrder(pool, k)
	},
},
{
	name:       "lexical",
	family:     "lexical",
	simplicity: 10,
	rank:       store.RerankHits,
},
```
```go
// Source: internal/retrievaleval/rankers.go:123-128 (verbatim — the
// currently-disabled stub this phase's rank closure replaces)
return append(rankers, namedRanker{
	name:           "jev",
	family:         "jev",
	simplicity:     99,
	disabledReason: jevDisabledReason,
})
```
`rankerFunc` is `func(query string, pool []store.Memory, k int) []store.Memory`
— **no context, no error return** (rankers.go:56). Per RESEARCH.md's Open
Question 3, the jev roster closure must own its own bounded
`context.WithTimeout` internally and fall back to
`store.RerankHits(query, pool, k)` (never `store.VectorOrder`, to match D-03's
"ties/failure keep lexical order" contract) on any decide failure — silently,
since the type signature cannot propagate an error. `evalRankers()` must gain
a `decide.Decider` parameter (or read one from a package-level/injected
value) to construct this closure — a signature change, not a body rewrite of
the function's existing shape.

---

### `proto/engram/v1/engram.proto` — `Memory.relevance`

**Analog:** `superseded_by` / `schema_version` fields in the same message.

**Core pattern (verbatim, the two nearest `optional` precedents plus the field-numbering context):**
```protobuf
// Source: proto/engram/v1/engram.proto:30-32, 44, 52 (verbatim, excerpts)
// Qdrant similarity score on search results (higher = closer); 0 on
// list/get results, which are not ranked.
float score = 17;
...
optional string superseded_by = 23;
...
optional uint32 schema_version = 28;
```
Next free field number in `Memory` is **31** (last used is `summary_egress_at
= 30`). Add:
```protobuf
// Jev decision-provider's P(this record answers the query); set only when
// ranker=jev ran and succeeded for this search. Unset otherwise — never a
// computed-but-zero float, since a genuine near-zero "no-answer" signal
// must still be distinguishable from "not scored."
optional float relevance = 31;
```
`float` (not `double`) matches `score`'s existing precision choice for a
[0,1]-ish ranking signal on this exact wire. Regenerate via `task proto:gen`
— the committed `gen/` tree is CI-checked for drift (`buf` job per
CLAUDE.md), and `go tool buf breaking` must pass since this is a purely
additive `optional` field (same class as `superseded_by`/`schema_version`
before it).

## Shared Patterns

### Transient (non-persisted) `Memory` field
**Source:** `internal/store/store.go:281-284` (`Score`), reinforced by
`NotBefore`/`LastAccessedAt` (store.go:218, 257)
**Apply to:** `internal/store/store.go` (`Memory.Relevance`),
`internal/server/connectapi.go` (`memoryToProto`),
`internal/server/summary.go` (`recallView`/`toRecallView`),
`cmd/engram/client_common.go` (`renderMemoryTable`)
```go
Score float32 `json:"score,omitempty"`
```
Every one of the three hand-written response shapers (`memoryToProto`,
`recallView`/`toRecallView`, `renderMemoryTable`) must be touched
independently — there is no reflection-based "add once, flows everywhere"
mechanism in this codebase (RESEARCH.md Common Pitfall 5, `recallView`'s own
doc comment names this explicitly).

### Registry-driven `ENGRAM_*` config addition (two-tier: row + validation)
**Source:** `internal/config/registry.go:120-130` (rows) +
`internal/config/validate.go:284-368` (validation) +
`internal/config/decisions_docs_test.go` (docs gate)
**Apply to:** `internal/config/registry.go`, `internal/config/validate.go`,
`internal/config/config.go`, plus a new
`internal/config/search_docs_test.go` mirroring `decisions_docs_test.go`'s
registry-driven docs-gate shape (grep every `ENGRAM_SEARCH_*` row, assert it
appears in `docs-site/src/content/docs/guides/configure.md`).
```go
// registry.go
{Key: "search.ranker", Env: "ENGRAM_SEARCH_RANKER", Default: "lexical"},
{Key: "search.rerank_timeout", Env: "ENGRAM_SEARCH_RERANK_TIMEOUT", Default: "2s"},

// validate.go
if c.Search.Ranker != "" && c.Search.Ranker != "lexical" && c.Search.Ranker != "jev" {
    errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RANKER %q: must be empty, \"lexical\", or \"jev\"", c.Search.Ranker))
}
if c.Search.Ranker == "jev" && c.Decisions.Provider == "" {
    errs = append(errs, errors.New("ENGRAM_SEARCH_RANKER=jev requires ENGRAM_DECISIONS_PROVIDER to be set"))
}
```

### Fallback-never-fails-the-call (D-03)
**Source:** `SearchReranked`'s existing error contract (store.go:1325-1343
— the only errors it returns are the `k==0` guard and `s.Search`'s own
error) plus `verdict.Unavailable`/`ErrorClass` (verdict.go:216-230) as the
"classify but never crash the caller" idiom for a decide failure.
**Apply to:** the new `RankHook` invocation inside `SearchReranked` /
`SearchDiscoveryReranked`, and the jev roster closure in
`internal/retrievaleval/rankers.go`.
A `RankHook` error or nil map is swallowed — the search still returns
`200`/success with lexical order, never an error surfaced to MCP/Connect/CLI
callers. This is the same shape `verdict.Unavailable(err)` establishes for
consolidate's per-pair verdicts (classify the failure, never fail the whole
operation) — but here even the classification stays server-internal (a slog
line, not a per-hit field), since D-06 forbids a response-level flag.

### Second, dedicated no-retry client instance for the search path (D-09)
**Source:** `deciderFromConfig` (decider.go:35-52) as the client-construction
shape to clone; `jev.Client`'s functional-options family (jev.go:101-166) as
the `Option` shape for `WithNoRetry()`.
**Apply to:** `internal/server/decider.go` (new
`searchDeciderFromConfig`/`searchRerankTimeout`), `internal/decide/jev/jev.go`
(new `WithNoRetry()` Option + `noRetry` field gating the retry branch at
jev.go:300).
Never share the consolidate-path `*jev.Client` for search reranking — a
shared client's retry would double request volume under load (Security
Domain / Known Threat Patterns, RESEARCH.md) and cannot honor "no retry on
the search path" without also changing consolidate's behavior.

### Per-candidate truncation reused from Phase 3, never refetched
**Source:** `internal/verdict.State`/`truncateRunes` (verdict.go:83-108),
`verdict.NewRequest` (verdict.go:147-159) as the `decide.Request`-building
shape (State map + Questions map, one call per candidate batch).
**Apply to:** the new rerank-request builder invoked from the `RankHook`
closure (wherever it is constructed in `internal/server`).
Call `verdict.State(m.Summary, m.Content, maxChars)` directly on each
already-fetched `Memory` from `SearchReranked`'s `hits` slice — never
`Store.RecordStates` (a second, wasted Qdrant round trip; RESEARCH.md
Pitfall 3). One `decide.Request` batches every candidate's Noul question
(D-04) — `internal/decide/validate.go`'s `MaxChoices = 255` bounds a single
*choice* question's option count only, never the number of `Questions` in a
`Request`, so up to 100 Noul questions in one call is structurally valid
(no new validation to add).

## No Analog Found

None — every file this phase touches is an existing, git-tracked file being
extended in place, and every extension has a directly analogous sibling
block in the same file or an immediately adjacent file from Phases 1-3. New
test files (below) have direct analog *test files* to model, not code files.

### New test files (Wave 0 gaps, from RESEARCH.md's Validation Architecture) — analog test files to model

| New test file | Role | Analog |
|---|---|---|
| `internal/store/rerank_jev_test.go` | test | `internal/store/rerank_test.go` (`TestCandidateK`, `TestSearchRerankedRejectsZeroK`) — same package, same table-driven/hermetic-guard style |
| `internal/config/search_config_test.go` | test | existing `internal/config/decisions_config_test.go`-style table test (registry defaults, jev-requires-provider validation) — grep `internal/config/*_test.go` for the exact sibling file name before creating |
| `internal/config/search_docs_test.go` | test | `internal/config/decisions_docs_test.go` (verbatim registry-driven docs-gate pattern — read in full above) |
| `internal/decide/jev/noretry_test.go` (or extend `jev_test.go`) | test | `internal/decide/jev/jev_test.go:35` (`TestJevRetryAndBounds`), `internal/decide/jev/classify_test.go:150-165` (`isRetryable` table) |
| extensions to `internal/server/summary_test.go`, `connectapi_test.go`, `cmd/engram/client_search_test.go` | test | `internal/server/connectapi_test.go:367` (`TestRerankParityMCPAndConnect`) — extend with a jev-enabled fake-decider subtest, do not replace |

## Metadata

**Analog search scope:** `internal/store/`, `internal/server/`,
`internal/config/`, `internal/decide/`, `internal/decide/jev/`,
`internal/verdict/`, `internal/retrievaleval/`, `proto/engram/v1/`,
`cmd/engram/` — every directory RESEARCH.md's "Recommended Project
Structure" names, confirmed git-tracked.
**Files scanned:** 15 production files + 5 test-analog files (all read this
session via `Read`/`Bash sed`/`Bash rg`, exact line numbers cited above)
**Pattern extraction date:** 2026-09-24
