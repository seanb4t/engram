# Phase 6: Query Understanding - Research

**Researched:** 2026-09-28
**Domain:** Server-side, budgeted `internal/decide` consumer wired to one new Connect-only read RPC, plus a console-only advisory-chip UI row
**Confidence:** HIGH (every server-side pattern below is copied from a live, shipped consumer of the same package — `internal/relevance`/`internal/verdict`/`internal/server/decider.go` — read in full this session; the UI wiring is copied from the shipped `/search` page and `FacetStrip.svelte`, also read in full)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Phase Boundary:** A prose query on the console's `/search` route yields **suggested, unapplied
filter chips** (categories, time window, scope, tags) from a new server-side Connect RPC
`UnderstandQuery`, built on `internal/decide` under a bounded, no-retry timeout. Suggestions never
change results until the user accepts one. An accepted chip is exactly a manually added chip: same
`onchange` path, same URL params, same `SearchMemoriesRequest`. A decision failure or timeout
yields zero suggestions. It never fails the search and never delays it.

Out of scope: an MCP tool or CLI verb for query understanding (D-02), the chat-LLM emulator
backend (NLQ-05, v2), suggestions in the header search or the Cmd-K menu, auto-applying any chip,
and showing probabilities in the UI.

**Server contract & config:**
- **D-01 (default on, where possible):** new key `ENGRAM_SEARCH_UNDERSTANDING=off|jev`. When
  unset, resolves to `jev` if `ENGRAM_DECISIONS_PROVIDER=jev`, else `off`. Explicit `off` opts
  out. `Config.Validate` rejects explicit `jev` with no decisions provider configured, and rejects
  any other value. "Off" still means no decision call and search behaviour byte-identical to
  before this phase.
- **D-01a:** `ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT` (default `2s`), same parse/validate/default
  pattern as `searchRerankTimeout`: non-positive/invalid falls back to 2s; Validate rejects
  non-positive while understanding is on. A dedicated Jev client like `searchDeciderFromConfig`:
  `jev.WithTimeout(understandingTimeout)` + `jev.WithNoRetry()`. Never shares the sweep client.
- **D-02 (lanes):** Connect `UnderstandQuery` only. No MCP tool, no CLI verb. Deliberate exception
  to lane-symmetry (`avxpsyr28d`): suggestion chips are a console affordance; a second lane would
  add another path sending query text to the provider. Read RPC — no CSRF entry needed.
  Authenticated like every other read.
- **D-03 (response shape):** `UnderstandQueryResponse { bool enabled; repeated FilterSuggestion
  suggestions; }`, `FilterSuggestion { oneof kind { category | time_window{created_after,
  created_before, label} | scope | tag }; source: DECIDED | MATCHED; }`. One suggestion per chip.
  Same oneof style as `RelatedEdge` evidence. No `buf.validate` on the request (follows
  `eneecjyzxh`) — the shared core validates. The request carries query text plus currently-applied
  filters (scope/cross_spine/categories/tags/window) so the server can skip already-answered
  questions.
- **D-04 (capability discovery):** RPC always exists. When off, returns `{enabled:false}`
  immediately — no decision call, no ListScopes/ListTags work. On `enabled:false` the UI stops
  calling for the rest of the session. No new info/capabilities RPC.

**What gets suggested:**
- **D-05:** at most **one `Decide` call per query**. State is the query text only (never record
  content). All questions batched into that one Request (`internal/relevance` batching precedent).
  A suggestion is emitted only when its top probability is **≥ 0.9** (spike-003's calibration
  band, accuracy 1.00). Fixed constant, not a config knob.
- **D-06 (categories):** one `Noul` per console category (`convention`, `gotcha`, `decision`,
  `preference` — `CATEGORIES` in `ui/src/lib/queries.ts`). Several can be suggested.
- **D-07 (time window):** one `Choice` over `{none, today, past_week, past_month, past_year}`.
  Server converts the answer to `created_after`/`created_before` (RFC3339, UTC, relative to
  request time). `none` → no suggestion. `label` carries the bucket name for the chip text.
- **D-08 (scope):** one `Choice` over the caller's own `ListScopes` result plus `none`, so the
  model can only pick a scope the caller can read. Asked only when the caller has ≤ 254 readable
  scopes (the Jev 255-choice ceiling) and the request has no scope applied.
- **D-09 (tags):** no decide call. Server matches query tokens against the caller's `ListTags`
  vocabulary, case-insensitive, full token or hyphen-part match. `source: MATCHED`. Outside the
  advisory decide contract; already-applied tags are skipped.

**Console behaviour:**
- **D-10 (trigger):** fires on a committed `/search` query (URL `q` changes), never per keystroke.
  Only for prose: not UUID-/short_id-shaped (reuse `ui/src/lib/search/classify.ts`), no `#`/`tag:`
  operator, 2+ words. Runs in parallel with `SearchMemories`, never gates/awaits it. Nothing in
  header search or Cmd-K.
- **D-11 (rendering):** "Suggested" row directly under `FacetStrip`, dashed-outline chips meaning
  "not applied". Click accepts through the exact `onchange` → URL path a manual chip uses, so the
  resulting URL/request are identical. `×` dismisses. No "Apply all". No probabilities shown.
- **D-12 (filtering & failure):** already-applied suggestions are hidden. Dismissals held per `q`
  in memory, not the URL. On error/timeout/zero suggestions the row is absent — no error UI, search
  untouched.
- **D-13 (keyboard & a11y):** roving-tabindex toolbar after `FacetStrip`. Enter/Space accepts,
  Delete/Backspace dismisses. Each chip's `aria-label` is `Suggested filter, not applied: <label>`.
  Follows `engram-console-conventions`.

**Telemetry, audit & proof:**
- **D-14 (always-on telemetry):** `engram.understand.*` attributes on the RPC span: `outcome`,
  `fallback_class`, `suggestion_count`, `questions_asked`. Never query, scope names, or tags. Tier
  1 of the rerank telemetry scheme (`0gxjp4xwgf`).
- **D-15 (disclosure):** startup log line when on: `search understanding enabled: console query
  text is sent to <host>`, stating whether from default (provider=jev) or explicit. Follows
  `SearchRerankInfo`/`logSearchRerankAuditEnabled`.
- **D-16 (audit flag, NLQ-04):** `ENGRAM_SEARCH_UNDERSTANDING_AUDIT` (default `false`; Validate
  rejects non-boolean). When true, logs query text plus suggestion labels at info level, never
  content. Also logs a startup Warn disclosure, and warns when set while understanding is off.
- **D-17 (proof tests):** off → fake Decider never called, response `{enabled:false}`; slog
  capture — no query text without audit flag, present with it; error/timeout → zero suggestions
  within budget, never an RPC error; 0.9 threshold and ≤254-scope gate exercised; UI browser test —
  `SearchMemories` request unchanged until a chip is clicked, accepted chip gives same URL/request
  as manual chip; behaviour tests only, no tests of Jev itself, no tests of tests (`3p0zsqrhmb`/
  `m45p2b4bp7`).

### Claude's Discretion
Exact proto field numbers/names beyond D-03's shape; the package for the understanding core (e.g.
`internal/understand`, beside `internal/relevance`); Noul/Choice instruction wording; the
tokenization details for D-09; the chip copy for time-window labels.

### Deferred Ideas (OUT OF SCOPE)
- MCP `understand_query` tool and CLI verb (rejected for now by D-02; revisit if agents need it).
- Showing suggestion confidence in the UI.
- Chat-LLM emulator backend (NLQ-05, already v2).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| NLQ-01 | Query understanding controlled by `ENGRAM_SEARCH_UNDERSTANDING` on top of `ENGRAM_DECISIONS_PROVIDER`; unset follows provider; explicit `off` disables; on discloses at startup; off is byte-identical. | Config registry/Validate precedent (`search.ranker`/`ENGRAM_SEARCH_RANKER`), `searchDeciderFromConfig`/`logSearchRerankAuditEnabled` pattern in `internal/server/decider.go`. The "unset follows provider" shape has NO existing registry precedent (see Pitfall/Code Example below) — must be resolved by a hand-written function, not a registry `Default`. |
| NLQ-02 | Prose query yields suggested chips from server-side `UnderstandQuery` via `internal/decide` under a bounded no-retry timeout; failure/timeout yields zero suggestions, never fails/delays search. | `internal/relevance.Hook`/`applyRankHook` fallback-never-fails pattern; `internal/decide`'s Noul/Choice vocabulary and `MaxChoices=255`; `store.ListScopes`/`Store.ListTags` for D-08/D-09 option sets. |
| NLQ-03 | Suggested chips are unapplied; results never change until accepted; accepted chip indistinguishable from manual. | `FacetStrip.svelte`'s `onchange: (next: Partial<SearchParams>) => void` prop and `+page.svelte`'s `onchange={(partial) => navigate({ ...partial, sel: '' })}` wiring — the exact path a new SuggestedRow must reuse. |
| NLQ-04 | Query text never logged/exported unless explicit opt-in audit flag is set; content never logged. | `ENGRAM_SEARCH_RERANK_AUDIT`/`searchRerankAudit`/`rankReport.audit` in `internal/store/rerank_report.go` — the exact tier-1/tier-2 telemetry split to mirror for `engram.understand.*` + the audit log line. |
</phase_requirements>

## Summary

This phase adds exactly one new capability — a Connect-only, read-only `UnderstandQuery` RPC — and
has essentially no architectural discovery risk left: `internal/decide`'s vocabulary
(Noul/Choice/Score), the two-client search-timeout pattern, the tier-1/tier-2 telemetry split, and
the "hook failure never fails the caller" fallback contract are all already shipped and used twice
(`internal/relevance` for search rerank, `internal/verdict` for spine-review consolidate). This
phase's job is to write a **third** consumer (`internal/understand`, matching CONTEXT.md's
suggested package name) that follows the identical shape, wire it through a **new** Connect handler
with no MCP counterpart (a documented, precedented asymmetry — `ListScopes` is the existing example
of exactly this), and add a small, self-contained Svelte row under the already-shipped
`FacetStrip.svelte`.

The one genuinely new piece of design is the **config resolution rule** in D-01: "unset follows the
decisions provider." Every existing `search.*`/`decisions.*` config key uses a *static* registry
default (`Default: "2s"`, `Default: "lexical"`) — there is no precedent in `internal/config/registry.go`
for a default that depends on the value of a *different* key. This must be a hand-written resolver
function in `internal/server` (mirroring `searchRerankTimeout`), with `Config.Validate` doing its
own, independent copy of the same resolution logic (config cannot import server). This is flagged
explicitly in Common Pitfalls below because it is the one place a naive implementation ("just set
`Default: "jev"` conditionally") cannot work with the registry's current shape.

The second concrete risk is a **docs gate test that will start failing on prefix collision**:
`internal/config/search_docs_test.go`'s `searchRegistryEnvNames()` matches every registry `Env`
carrying the `ENGRAM_SEARCH_` prefix and hardcodes `len(envs) != 3` as a "positive control." Adding
`ENGRAM_SEARCH_UNDERSTANDING`/`_TIMEOUT`/`_AUDIT` (three more `ENGRAM_SEARCH_*` vars) will make that
count 6 and will also require those three new vars to have doc rows inside the **existing**
`## Search reranking (Jev)` heading's section (the only section the gate reads) unless the gate is
either narrowed or a second, understanding-scoped docs gate is authored. This is exercised and
cited below with the exact fix shape.

**Primary recommendation:** Build `internal/understand` as a line-for-line structural copy of
`internal/relevance` (batch Noul/Choice questions into one `decide.Request`, map the `decide.Response`
back with `FromResponse`-style strict validation, never call `DecideMany`), wire it through a new
Connect-only handler in `internal/server` that calls `callerFromConnectContext` +
`a.d.st.ListScopes`/`d.listTags` directly (mirroring `RelatedMemories`/`ListTags`'s existing thin-
adapter shape), and add config resolution as a new hand-written pair of functions
(`understandingEnabled`/`understandingTimeout`) in `decider.go` and a mirrored inline check in
`validate.go` — never a registry-level conditional default.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Query-to-filter-suggestion decision | API / Backend | — | Must run server-side only: the Jev API key/base URL are server config; a browser-side call would leak the key (Anti-Pattern already documented in `.planning/research/ARCHITECTURE.md`) |
| Scope/tag option sourcing (D-08/D-09) | API / Backend | Database / Storage | `ListScopes`/`ListTags` already enforce authz inside `internal/store` (Cedar-compiled Qdrant filter) — the handler only passes the caller's `Subject` through, never re-filters |
| Suggested-chip rendering, accept/dismiss | Browser / Client | — | Pure Svelte state (`$state` latch, in-memory dismissed-set); D-12 explicitly forbids URL persistence of dismissals |
| Config resolution (on/off/timeout/audit) | API / Backend | — | `internal/config` (registry + `Validate`) and `internal/server` (`decider.go`-style resolver) — never a client-visible toggle; the console has no settings surface for this (per UI-SPEC scope note) |
| Telemetry (span attrs + audit log) | API / Backend | — | Set on the ambient Connect RPC span via `trace.SpanFromContext(ctx)`, exactly like `rankReport.stamp` — never a client-visible signal |

## Standard Stack

No new external packages this phase (server or UI). Everything is built from already-vendored,
already-pinned dependencies: `internal/decide`/`internal/decide/jev` (Go, in-repo), `connectrpc.com/connect`
(already a dependency, used by every other RPC), `@tanstack/svelte-query` `^6.1.34` (already pinned,
`ui/package.json`), Svelte 5 (already pinned). See **Package Legitimacy Audit** below.

### Core (in-repo packages this phase builds on, not installs)
| Package | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/decide` | in-repo | Noul/Choice/Score question vocabulary, `Decider` interface, D-12 error-class sentinels | The one and only typed-decision contract in this codebase; `internal/relevance` and `internal/verdict` both already build on it |
| `internal/decide/jev` | in-repo | The Jev backend (`jev.New`, `jev.WithTimeout`, `jev.WithNoRetry`) | Same backend `searchDeciderFromConfig` already builds a dedicated client from |
| `connectrpc.com/connect` | already pinned (go.mod) | The Connect RPC transport | Every other RPC in `engramv1connect`/`connectapi.go` uses it |
| `@connectrpc/connect-web` | already pinned (`ui/package.json`) | Browser Connect client (`engram`/`engramWrite`) | `ui/src/lib/client.ts` |
| `@tanstack/svelte-query` | `^6.1.34` (pinned) | The `createQuery` hook for `UnderstandQuery` | Every other read on `/search` already uses it with the identical `queryKey`/`queryFn`/`meta.silent` shape |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| One `Noul` per category batched into one Request | A single `Choice` over all four categories | Rejected by D-06 itself ("several can be suggested" — a `Choice` is single-select by definition, so it cannot express "both `decision` and `gotcha`") |
| Server-side tag matching (D-09) | A fifth `Choice`/`Noul` question to the decider for tags | Rejected by D-09/ARCHITECTURE.md: tags are open-vocabulary, no finite option set fits `decide`'s closed question types |

**Installation:** none — no `go get`, no `npm install`/`pnpm add` this phase.

## Package Legitimacy Audit

Not applicable — no new external package (Go module or npm/pnpm package) is introduced by this
phase. Every dependency used is already vendored and pinned (see Standard Stack above).

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
Browser (/search route)
  │
  │  URL q= changes (committed query, D-10 gate: classifyInput
  │  is text, no operator chips, 2+ words)
  ▼
TanStack Query: ['understandQuery', q]  ──────────────┐
  │  (parallel, independent of SearchMemories;         │  meta.silent: true
  │   never gates/awaits the search request)           │  enabled: eligible && !latchedDisabled
  ▼                                                      │
engram.understandQuery({ query: q, applied: {...} })    │
  │  (read-only client — no CSRF, D-02)                  │
  ▼                                                      │
─────────────────── network boundary ───────────────────┘
  ▼
Connect RPC UnderstandQuery  (internal/server, engramAPI)
  │  otel span (ambient — engram.understand.* attrs land here, D-14)
  │  callerFromConnectContext(ctx) → caller{Subj, Actor}
  ▼
internal/understand.Suggest(ctx, dec, caller.Subj, req)
  │
  ├─ off (dec == nil / understanding disabled) ─────► return {enabled:false} immediately
  │                                                     (NO ListScopes/ListTags call — D-04)
  │
  ├─ on: build ONE decide.Request
  │     • up to 4 Noul questions (categories, D-06)
  │     • 1 Choice question (time window, D-07)
  │     • 1 Choice question (scope, D-08) — ONLY if
  │       st.ListScopes(ctx, subj) returns ≤254 scopes
  │       AND no scope already applied
  │  ▼
  │  dec.Decide(ctx, req)  [single dedicated no-retry
  │                         client, 2s default timeout]
  │     ├─ error/timeout ──► zero suggestions, span
  │     │                    outcome=fallback, fallback_class=<class>
  │     └─ success ──► map answers → suggestions at p≥0.9
  │
  └─ separately, no decide call (D-09):
        st.ListTags(ctx, subj, scope, limit) → token-match
        against query text → source:MATCHED suggestions
  ▼
UnderstandQueryResponse{ enabled, suggestions[] }
  ▼
Browser: SuggestedRow.svelte mounted under FacetStrip
  │  hides already-applied suggestions (reactive on `effective`)
  │  hides dismissed suggestions (in-memory Set, keyed per q)
  ▼
User clicks/Enters a chip
  ▼
SAME onchange={(partial) => navigate({ ...partial, sel: '' })}
path FacetStrip already uses  ──►  URL updates  ──►  SearchMemories
                                                       re-fires with the
                                                       new params (indistin-
                                                       guishable from a
                                                       manually-added chip)
```

### Recommended Project Structure
```
internal/understand/               # NEW package, sibling to internal/relevance
├── understand.go                  # NewRequest (batches Noul/Choice), FromResponse (≥0.9 gate), Suggest
├── understand_test.go
proto/engram/v1/engram.proto        # + FilterSuggestion/UnderstandQuery* messages, + 1 rpc line
internal/server/
├── decider.go                      # + understandingEnabled/understandingTimeout/understandingAudit
│                                    #   resolvers, + understandDeciderFromConfig (3rd dedicated client)
├── understand.go                   # NEW: deps.understandQuery core (mirrors related.go/tags.go shape)
├── connectapi.go                   # + engramAPI.UnderstandQuery handler
internal/config/
├── config.go                       # + SearchConfig.Understanding/UnderstandingTimeout/UnderstandingAudit
├── registry.go                     # + search.understanding / _timeout / _audit rows
├── validate.go                     # + the D-01/D-01a/D-16 validation block
├── search_config_test.go           # + 3 new searchFields rows, + TestSearchConfigValidate cases
├── understanding_docs_test.go      # NEW (see Pitfall below — do NOT extend search_docs_test.go as-is)
ui/src/lib/components/
├── FacetStrip.svelte               # UNCHANGED (reused, not modified)
├── SuggestedRow.svelte             # NEW: the one authored UI element (UI-SPEC E1)
ui/src/routes/search/
├── +page.svelte                    # + one createQuery + <SuggestedRow> mount under <FacetStrip>
├── search.browser.test.ts          # + assertions per D-17
docs-site/src/content/docs/guides/
├── configure.md                    # + "## Query understanding (Jev)" section, mirroring "## Search reranking (Jev)"
.claude/skills/engram-console-conventions/SKILL.md  # + the new roving-toolbar keyboard bindings (UI-SPEC obligation)
```

### Pattern 1: Third `internal/decide` consumer — copy `internal/relevance`'s shape exactly

**What:** A package with no I/O of its own beyond the `decide.Decider` it's handed: a pure
`NewRequest(query, appliedFilters, ...) decide.Request` builder, a pure `FromResponse(resp) (...)`
mapper that strictly validates every answer's type/probability range, and a thin orchestration
function that calls `dec.Decide` **exactly once** (never `DecideMany` — there is one State: the
query text, per D-05).

**When to use:** Any new `internal/decide` consumer. This is now the *third* time this shape is
used (`internal/relevance`, `internal/verdict`, this phase) — treat it as settled, not a design
question.

**Example (structure, not literal code — mirrors `internal/relevance/relevance.go:127-175,186-201`):**
```go
// Source: internal/relevance/relevance.go (read in full this session)
func buildRequest(query string, applied appliedFilters) decide.Request {
	state := decide.State{StateQuery: query} // never record content (D-05)
	questions := map[string]decide.Question{}
	for _, cat := range consoleCategories { // CATEGORIES from ui/src/lib/queries.ts, mirrored server-side
		if !applied.hasCategory(cat) {
			questions["category_"+cat] = decide.Noul(instructionsFor(cat), whenTrueFor(cat), whenFalseFor(cat))
		}
	}
	if !applied.hasWindow() {
		questions["time_window"] = decide.Choice(windowInstructions, timeWindowOptions)
	}
	if scopeEligible { // ≤254 readable scopes AND no scope applied (D-08)
		questions["scope"] = decide.Choice(scopeInstructions, scopeOptionsFromListScopes)
	}
	return decide.Request{State: state, Questions: questions}
}

// FromResponse mirrors relevance.FromResponse's strict-validation shape exactly:
// missing/mistyped/out-of-range answers are decide.ErrDecisionMalformedResponse,
// never silently skipped.
```

### Pattern 2: The two-client, no-retry search-timeout shape (D-01a)

**What:** A dedicated `decide.Decider` built by its own `xDeciderFromConfig` function, gated on
`cfg.Decisions.Provider` (never on the feature's own enable flag alone — mirrors
`searchDeciderFromConfig`'s own comment: "Gated on cfg.Decisions.Provider, NOT cfg.Search.Ranker").

**Example (verbatim precedent to copy):**
```go
// Source: internal/server/decider.go:198-224 (read in full this session)
func searchDeciderFromConfig(cfg *config.Config) (decide.Decider, error) {
	switch cfg.Decisions.Provider {
	case "":
		return nil, nil
	case "jev":
		apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)
		return jev.New(cfg.Decisions.BaseURL, apiKey, cfg.Decisions.Model,
			jev.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
			jev.WithTimeout(searchRerankTimeout(cfg)),
			jev.WithMaxTimeout(decisionsMaxTimeout(cfg)),
			jev.WithDrainBytes(decisionsDrainBytes(cfg)),
			jev.WithDrainTimeout(decisionsDrainTimeout(cfg)),
			jev.WithNoRetry(),
		), nil
	default:
		return nil, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: unknown provider (want \"\" or \"jev\")", cfg.Decisions.Provider)
	}
}
```
`understandDeciderFromConfig` is a **structural copy** of this function, substituting
`understandingTimeout(cfg)` for `searchRerankTimeout(cfg)`. This is the THIRD such client
(consolidate's `deciderFromConfig`, search's `searchDeciderFromConfig`, this phase's
`understandDeciderFromConfig`) — each with its own timeout knob, sharing every other
decisions.* setting (base URL, key fallback, model, max-timeout ceiling, drain bounds).

### Pattern 3: Connect-only handler with no MCP counterpart (D-02's precedent already exists)

**What:** `ListScopes` is already the documented example of exactly this shape in this codebase.

**Example (verbatim precedent):**
```go
// Source: internal/server/connectapi.go:176-193 (read in full this session)
// ListScopes is the ONE documented D-07 exception: no MCP-side
// deps.listScopes counterpart exists (read-only scope-count listing —
// research OQ2), so this handler still calls a.d.st.ListScopes directly.
func (a *engramAPI) ListScopes(ctx context.Context, _ *connect.Request[engramv1.ListScopesRequest]) (*connect.Response[engramv1.ListScopesResponse], error) {
	subj, err := subjectFromConnectContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	scopes, approx, err := a.d.st.ListScopes(ctx, subj)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	resp := &engramv1.ListScopesResponse{Approximate: approx}
	for _, sc := range scopes {
		resp.Scopes = append(resp.Scopes, &engramv1.ScopeCount{Scope: sc.Scope, Count: sc.Count})
	}
	return connect.NewResponse(resp), nil
}
```
`UnderstandQuery`'s handler should use `callerFromConnectContext` (not the narrower
`subjectFromConnectContext`) because it also needs `caller.Actor`-shaped context for `d.listTags`'s
existing signature — see Pattern 4. State the "no MCP tool" comment on the handler in the exact
same phrasing style, citing D-02 instead of "research OQ2".

### Pattern 4: Calling `ListScopes`/`ListTags` with the caller's own authz subject (D-08/D-09)

**What:** Both existing shared-core functions take `c caller` (or `subj store.Subject` directly)
and pass it straight into the store call — no post-filter, no ownership check above the store
(DEC-cgb).

**Example (verbatim precedent, `internal/server/tags.go:39-44`, read in full this session):**
```go
func (d *deps) listTags(ctx context.Context, c caller, a listTagsArgs) ([]store.TagCount, bool, error) {
	if err := rejectOverMaximumCount("limit", a.Limit); err != nil {
		return nil, false, err
	}
	return d.st.ListTags(ctx, c.Subj, a.Scope, a.Limit)
}
```
And `internal/server/related.go:37-53`'s `relatedMemories` for the equivalent by-id shape. The
understanding core should call `d.st.ListScopes(ctx, c.Subj)` (for D-08, gated on the returned
count being ≤254 and no scope already applied) and `d.listTags(ctx, c, listTagsArgs{Scope: ...})`
(for D-09, reusing the existing shared core rather than calling `d.st.ListTags` a second, divergent
way) — both already-shipped, already-tested authz paths. **Never** re-derive a scope/tag list with
a hand-rolled filter in the new handler (Pitfall 6 in `.planning/research/PITFALLS.md`).

### Pattern 5: Tier-1/Tier-2 telemetry split, and the ambient-span technique (D-14/D-16)

**What:** Tier 1 (always-on, bounded, no query text) is stamped onto the **ambient** span — the
RPC's own span from the interceptor chain, obtained via `trace.SpanFromContext(ctx)` — never a new
child span of the understanding package's own. Tier 2 (opt-in, query text) is a single `slog.InfoContext`
line gated on the audit config bool.

**Example (verbatim precedent, `internal/store/rerank_report.go:1-43,68-112`, and the call site,
`internal/store/store.go:1370-1375`, read in full this session):**
```go
// Attribute keys — set on the AMBIENT span (tool/search_memory, the Connect
// RPC span, etc.) — never a span of the store's own, so one span row answers
// the question with no join needed.
const (
	AttrRerankOutcome       = "engram.rerank.outcome"
	AttrRerankFallbackClass = "engram.rerank.fallback_class"
	// ...
)

// Call site: the caller obtains the ambient span itself.
ranked, rep := rankWithReport(ctx, query, hits, int(k), opts.RankHook)
rep.stamp(trace.SpanFromContext(ctx))
if opts.RankAudit {
	rep.audit(ctx, "search_memory", query, ownerOf(subj))
}
```
For D-14, the `UnderstandQuery` Connect handler (or `internal/understand.Suggest` handed the
handler's `ctx`) should call `trace.SpanFromContext(ctx).SetAttributes(...)` directly with
`engram.understand.outcome`/`fallback_class`/`suggestion_count`/`questions_asked` — the ambient span
here is the one `otelconnect.NewInterceptor()` already creates per RPC (`connectapi.go:693,714-721`,
"otel outermost (spans cover auth + logging)"), so no manual span creation is needed. For D-16's
audit log, mirror `rankReport.audit`'s exact shape: one `slog.InfoContext` call, query text plus
suggestion **labels only** (never content), gated on `understandingAudit(cfg)`.

### Pattern 6: The startup disclosure and audit-flag-while-off warning (D-15/D-16)

**Example (verbatim precedent, `internal/server/decider.go:168-196`, read in full this session):**
```go
func searchRerankAudit(cfg *config.Config) bool {
	b, err := strconv.ParseBool(cfg.Search.RerankAudit)
	if err != nil {
		if cfg.Search.RerankAudit != "" {
			slog.Warn("ENGRAM_SEARCH_RERANK_AUDIT is set but not a boolean; audit capture stays off",
				"value", cfg.Search.RerankAudit)
		}
		return false
	}
	return b
}

func logSearchRerankAuditEnabled(hookEnabled bool) {
	if !hookEnabled {
		slog.Warn("ENGRAM_SEARCH_RERANK_AUDIT is true but ENGRAM_SEARCH_RANKER is not jev; nothing is reranked, so nothing is audited")
		return
	}
	slog.Warn("search rerank audit capture enabled: every reranked search logs its query text and candidate ids (never content) at info level",
		"log_msg", "search rerank audit")
}
```
`understandingAudit(cfg)` and `logUnderstandingAuditEnabled(enabled bool)` are structural copies.
D-16 additionally requires the "warns when set while understanding is off" case, which is exactly
`logSearchRerankAuditEnabled`'s `!hookEnabled` branch above — reuse the branch shape verbatim.

D-15's own disclosure line (distinct from the audit line) mirrors `logSearchRankerEnabled`
(`decider.go:298-317`): one `slog.Info` naming the endpoint host only (never userinfo/path/query),
plus — per D-15's own text — whether the `jev` value came from the D-01 default or was set
explicitly (a new piece of state `understandingEnabled` must return alongside its bool, e.g. a
`(enabled bool, source string)` return, `source` being `"default"` or `"explicit"`).

### Recommended Project Structure — proto sketch

```protobuf
// Additive only; append after the existing ListTags* messages (proto/engram/v1/engram.proto:713),
// and the rpc line after ListTags (engram.proto:741) — buf breaking stays green (FILE-mode) as
// long as no existing field/message is renumbered (Pitfall: buf breaking, cited in
// .planning/research/PITFALLS.md's Integration Gotchas table).

message TimeWindowSuggestion {
  string created_after = 1;  // RFC3339, UTC
  string created_before = 2; // RFC3339, UTC
  string label = 3;          // e.g. "past week" — rendered verbatim, no client translation (UI-SPEC)
}

enum SuggestionSource {
  SUGGESTION_SOURCE_UNSPECIFIED = 0;
  SUGGESTION_SOURCE_DECIDED = 1; // categories, time_window, scope (D-05..D-08)
  SUGGESTION_SOURCE_MATCHED = 2; // tags (D-09, no decide call)
}

// FilterSuggestion is one unapplied chip (D-03). Exactly one oneof case is
// set. Same oneof-by-kind style as RelatedEdge (engram.proto:634-648).
message FilterSuggestion {
  oneof kind {
    string category = 1;                 // one of CATEGORIES
    TimeWindowSuggestion time_window = 2;
    string scope = 3;
    string tag = 4;
  }
  SuggestionSource source = 5;
}

// UnderstandQueryRequest carries the query text plus a snapshot of the
// currently-applied filters (D-03) so the server can skip already-answered
// questions. No buf.validate rule (follows eneecjyzxh) — the shared core
// validates.
message UnderstandQueryRequest {
  string query = 1;
  string scope = 2;
  bool cross_spine = 3;
  repeated string categories = 4;
  repeated string tags = 5;
  string created_after = 6;
  string created_before = 7;
}

// enabled is authoritative for the whole page session (D-04) — the UI
// latches on the first response and stops calling once false.
message UnderstandQueryResponse {
  bool enabled = 1;
  repeated FilterSuggestion suggestions = 2;
}
```
```protobuf
service EngramService {
  // ... existing rpcs unchanged ...
  // --- query understanding (milestone 2026-09-25.01 Phase 6, D-02: Connect-only, no MCP tool) ---
  rpc UnderstandQuery(UnderstandQueryRequest) returns (UnderstandQueryResponse);
}
```

### Anti-Patterns to Avoid
- **Registry-level conditional default for D-01:** `internal/config/registry.go`'s `field.Default`
  is always a static string (verified by reading every entry in the file this session — e.g.
  `{Key: "search.rerank_timeout", ..., Default: "2s"}`). There is no mechanism for "default to the
  value of another key." Do not attempt to encode D-01's "unset follows `ENGRAM_DECISIONS_PROVIDER`"
  rule as a registry default — resolve it in a hand-written function (see Common Pitfalls below).
- **A per-handler authz check "just to be safe":** Pitfall 6 in `.planning/research/PITFALLS.md`
  is a **locked** invariant (DEC-cgb) — the `UnderstandQuery` handler must call
  `d.st.ListScopes`/`d.listTags` with the caller's `Subject` and do nothing else; it must never
  re-filter the returned scopes/tags itself.
- **Computing the decision from the browser:** already a named anti-pattern in
  `.planning/research/ARCHITECTURE.md` — leaks the Jev API key. `UnderstandQuery` is
  server-side-only by construction (D-02's whole rationale).
- **Keying the TanStack query on the full merged `effective` params object:** the UI-SPEC's own
  "Query-key pitfall" section is explicit — the key is `['understandQuery', q]` (bare `q`, not
  `effective`), or a facet-chip toggle after the row renders would silently re-trigger the RPC,
  unwiring D-10's own firing rule.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Batching several category questions into one provider call | A custom multi-question wire format | `decide.Request{Questions: map[string]decide.Question{...}}` — `internal/decide`'s existing batching contract | This is literally what `Request.Questions` already exists for; `internal/relevance.buildRequest` already proves the pattern for a variable-length question set |
| Scope-option enumeration for the scope `Choice` | A second scope-listing code path | `d.st.ListScopes` — already authz-scoped, already returns counts | Any hand-rolled scope enumeration duplicates the authz composition Pitfall 6 warns about |
| Query-text truncation/size bounding before sending to Jev | A bespoke truncation function | `verdict.State`/`verdict.truncateRunes`-style rune-safe truncation, or the `relevance.MaxQueryChars` (2000) constant as a starting bound | `internal/verdict/verdict.go:84-101` already implements UTF-8-safe truncation (invalid byte → U+FFFD, never splits a multi-byte sequence) — re-deriving this is a well-known Unicode footgun |
| Error-class-to-telemetry-word mapping | A new switch statement on error text | `decide.Status(err)` — the shared D-12 classifier | `internal/relevance.errClass`/`internal/verdict.ErrorClass` both already delegate to it; re-switching on error text is exactly what D-12's sentinel design exists to prevent |

**Key insight:** every mechanical piece of this phase (question batching, response validation,
timeout/no-retry client construction, telemetry stamping, audit logging, startup disclosure) has a
byte-for-byte precedent already shipped in this repository. The only genuinely new code is the
question *content* (which categories/time buckets/scope options to ask) and the D-01 config
resolution function — everything else should be copied, not redesigned.

## Common Pitfalls

### Pitfall 1: D-01's "unset follows provider" has no registry precedent — will be implemented wrong if treated as a simple default
**What goes wrong:** An implementer sets `{Key: "search.understanding", Env: "ENGRAM_SEARCH_UNDERSTANDING", Default: "jev"}`
in the registry, then discovers this makes `ENGRAM_SEARCH_UNDERSTANDING` default to `"jev"`
**unconditionally**, even when `ENGRAM_DECISIONS_PROVIDER` is empty — violating "off is
byte-identical" for every deployment that has never touched `ENGRAM_DECISIONS_PROVIDER`.
**Why it happens:** Every existing `search.*`/`decisions.*` registry entry uses a static
`Default` string (confirmed by reading every row in `internal/config/registry.go` this session —
`search.ranker` defaults to `"lexical"`, `search.rerank_timeout` to `"2s"`, both static, both
provider-independent). There is no field-to-field conditional default mechanism in `field{}`
(`internal/config/registry.go:12-19`).
**How to avoid:** Set `Default: ""` for `search.understanding` (empty = genuinely unset). Add a
resolver function in `internal/server/decider.go`, e.g.:
```go
// understandingEnabled resolves D-01: an explicit off/jev wins; unset (empty)
// follows cfg.Decisions.Provider. Returns (enabled, source) where source is
// "default" or "explicit" for D-15's disclosure line.
func understandingEnabled(cfg *config.Config) (enabled bool, source string) {
	switch cfg.Search.Understanding {
	case "off":
		return false, "explicit"
	case "jev":
		return true, "explicit"
	default: // "" (unset)
		return cfg.Decisions.Provider == "jev", "default"
	}
}
```
`Config.Validate` needs its **own**, independent copy of this same three-way switch (it cannot call
the `internal/server` function — `internal/config` has no dependency on `internal/server`, and must
not gain one) to decide whether `ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT`/`_AUDIT` are gated on
"effectively on" vs. only on the explicit `jev` value. Comment both copies noting they must be kept
in sync, mirroring how `search.ranker`'s gating in `validate.go:380` and `searchRankHook`'s gating
in `decider.go:236` already independently re-check `cfg.Search.Ranker == "jev"` in each package.
**Warning signs:** A `Config.Validate` that only checks `Search.Understanding == "jev"` explicitly,
silently skipping validation of `UnderstandingTimeout`/`UnderstandingAudit` when the effective state
is "on via default" — the exact class of gap `search.rerank_timeout`'s validation already had to
get right for `search.ranker`.
**Phase to address:** This phase — Task-level checkpoint before implementation.

### Pitfall 2: The docs gate `TestSearchVarsDocumented` will fail (or silently pass wrong) if `ENGRAM_SEARCH_UNDERSTANDING*` vars are added without touching the gate
**What goes wrong:** `go test ./internal/config/...` fails with `searchRegistryEnvNames() returned
6 names, want 3` the moment the three new registry rows land — or, if that hardcoded count is
"fixed" by bumping it to 6 without also fixing the section scope, `TestSearchVarsDocumented`'s
"every registered var has a doc row" subtest fails because the new vars are being looked for inside
the **existing** `## Search reranking (Jev)` heading's section, where they don't belong.
**Why it happens:** `internal/config/search_docs_test.go:32-40`'s `searchRegistryEnvNames()`
matches **every** registry `Env` with the `ENGRAM_SEARCH_` prefix, not just the three rerank vars —
verified by reading the function this session. `search_docs_test.go:74` hardcodes
`if len(envs) != 3` as its own "positive control" comment ("an empty or short derivation must not
pass vacuously"). `extractSearchSection` (`search_docs_test.go:44-54`) only reads the body between
`## Search reranking (Jev)` and the next `## ` heading.
**How to avoid:** Do **not** extend `search_docs_test.go`/`## Search reranking (Jev)`. Instead:
(1) author a **new** `## Query understanding (Jev)` section in `configure.md` documenting the three
new vars plus the "What leaves your deployment"/"Failure behavior" disclosure prose (mirroring the
existing section's shape at `configure.md:220-260`, read in full this session); (2) author a
**new** `internal/config/understanding_docs_test.go` that is a structural copy of
`search_docs_test.go`, filtering on `ENGRAM_SEARCH_UNDERSTANDING` prefix (not bare
`ENGRAM_SEARCH_`) and scoped to the new heading. This keeps `search_docs_test.go`'s `!= 3` positive
control correct and untouched.
**Warning signs:** `go test ./internal/config/...` red immediately after adding the three registry
rows, before any docs or handler work is done.
**Phase to address:** This phase — the config task, before any handler work (fail fast).

### Pitfall 3: Existing `Config{}`/`SearchConfig{}` test literals will break if the new bool field is unconditionally validated
**What goes wrong:** `TestValidateHappyPath`, `TestValidateIgnoresSummaryWhenDisabled`, and
`TestServiceAuthValidate_*` (all of which construct `SearchConfig{RerankAudit: "false"}` without
setting the other two fields) start failing `Validate()` with `"ENGRAM_SEARCH_UNDERSTANDING_AUDIT
must be a boolean"` once `UnderstandingAudit` is validated unconditionally like `RerankAudit`
already is (mirroring `validate.go:409-411`'s pattern, where an empty string fails
`strconv.ParseBool`).
**Why it happens:** Verified by reading every `SearchConfig{...}` literal in the package this
session — `internal/config/validate_test.go:22`, `internal/config/config_test.go:253`,
`internal/config/service_auth_test.go:288` all write exactly `SearchConfig{RerankAudit: "false"}`,
leaving every other `SearchConfig` field at its Go zero value (`""`). This is the exact durable
gotcha already recorded in `.planning/STATE.md` (`s780vae1vr`): "Any new required internal/config
registry field must also be added to every full Config{} literal in that package's tests."
**How to avoid:** When adding `UnderstandingAudit string` to `SearchConfig` with unconditional
validation, update all three literal sites above (and any others `rg -n 'SearchConfig\{'
internal/config` surfaces at implementation time — re-run the search, this list is a snapshot) to
also set `UnderstandingAudit: "false"`. Alternatively, scope the new bool field's validation to only
fire when the effective understanding state is non-default-off (matching Pitfall 1's resolver) —
but D-16 explicitly says the audit flag must "warn when set while understanding is off," which
implies it IS read/parsed unconditionally (just not gated on the same on/off switch as the timeout)
— so the unconditional-parse shape (matching `RerankAudit`) is the one D-16 actually asks for, and
the literal-update path is the correct fix, not a validation-scope change.
**Warning signs:** `go test ./internal/config/...` red on the happy-path test, not the new tests.
**Phase to address:** This phase — same task as Pitfall 1/2, before moving to the handler.

### Pitfall 4: Query text sent to `decide.Request.State` is currently unbounded
**What goes wrong:** A very long pasted "query" (a user pastes a paragraph into the search box)
sends an unbounded string into `decide.Request.State`, which `internal/decide` itself does not
size-check (confirmed: `Request.Validate()` in `internal/decide/validate.go` performs zero token/byte
counting — the doc comment at `validate.go:19` says so explicitly: "No token counting happens here
... left to callers"). This risks `ErrDecisionContextTooLarge` on every long query, which — while
handled gracefully by D-05's "failure = zero suggestions" contract — burns the request's timeout
budget on every such query for no benefit.
**Why it happens:** D-10's trigger gate (2+ words, no operators) does not bound query *length*, only
word count and shape.
**How to avoid:** Apply a truncation bound to the query text before building the Request — reuse
`relevance.MaxQueryChars` (2000, the existing constant already used for exactly this purpose in the
sibling rerank consumer) or a smaller phase-local constant if 2000 chars is excessive for a search
box. This is a `Claude's Discretion` item per CONTEXT.md ("Noul/Choice instruction wording" is
listed, and truncation length is adjacent) — but leaving it fully unbounded is a real, demonstrable
gap versus the sibling consumer's own practice.
**Phase to address:** This phase, `internal/understand`'s request-builder task.

### Pitfall 5: `ListScopes`' `more`/`approx` return semantics must not be conflated with the D-08 ≤254 gate
**What goes wrong:** `store.ListScopes` returns `(out []ScopeCount, more bool, err error)`
(`internal/store/store.go:2081`) where `more` means "the result was truncated at some internal cap,"
not "the caller has more than 254 scopes." An implementer who gates D-08 on `!more` instead of
`len(scopes) <= 254` will silently mis-gate whenever the internal truncation cap differs from 254
(it is a different constant, unrelated to Jev's `MaxChoices`).
**Why it happens:** Both are booleans/counts returned from the same call, and both superficially
mean "too many" — but they bound different things (a store-side pagination/response-size cap vs.
Jev's own protocol-level 255-choice ceiling).
**How to avoid:** Gate D-08 explicitly on `len(scopes) <= 254` (leaving room for the `none` option
to bring the total to 255, `decide.MaxChoices`), not on the `more`/`approx` flag `ListScopes`
returns for an unrelated reason.
**Phase to address:** This phase, the D-08 scope-question task.

## Code Examples

### Fake-Decider test pattern (D-17: "off → fake Decider never called")
```go
// Source: cmd/engram/spine_review_consolidate_test.go:791-806 (read in full this session)
// countingFakeDecider only records how many times Decide/DecideMany were
// called — used to prove a code path that must never reach the decider
// (--no-verdicts, a state-fetch error) genuinely doesn't.
type countingFakeDecider struct {
	decideCalls, decideManyCalls int
}

func (d *countingFakeDecider) Decide(_ context.Context, _ decide.Request) (decide.Response, error) {
	d.decideCalls++
	return decide.Response{}, nil
}

func (d *countingFakeDecider) DecideMany(_ context.Context, reqs []decide.Request) []decide.Result {
	d.decideManyCalls++
	return make([]decide.Result, len(reqs))
}
```
Use an identical `countingFakeDecider` (or reuse this one if `internal/understand`'s tests can
import `cmd/engram`'s — more likely, define a package-local copy in `internal/understand`'s own test
file, since `cmd/engram` and `internal/understand` should not import each other) to prove the
`{enabled:false}` short-circuit path (D-04) never touches the decider.

### Slog-capture test pattern (D-17: "no query text without audit flag, present with it")
```go
// Source: internal/server/decider_test.go:479-484 (read in full this session)
var buf bytes.Buffer
prev := slog.Default()
slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
t.Cleanup(func() { slog.SetDefault(prev) })

// ... exercise the code path ...

out := buf.String()
if strings.Contains(out, "the query text used in this test") {
	t.Errorf("log output contains query text without the audit flag set: %s", out)
}
```
This exact `bytes.Buffer` + `slog.NewJSONHandler` + `t.Cleanup(slog.SetDefault(prev))` shape appears
three times already in this codebase (`decider_test.go:481-484,858-861,1042-1045`,
`rerank_report_test.go:189-192,229-232`) — it is the house pattern for asserting on log output, not
a new technique this phase invents.

### Browser-test scaffolding for D-17's UI proof ("SearchMemories request unchanged until a chip is clicked")
```typescript
// Source: ui/src/routes/search/search.browser.test.ts:1-79 (read in full this session)
const { gotoSpy, pageState, searchMemoriesSpy, /* ... */ understandQuerySpy } = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/search');
  return { /* ... */, understandQuerySpy: vi.fn() };
});

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, searchMemories: searchMemoriesSpy, understandQuery: understandQuerySpy /* , ... */ }
  };
});
```
The existing file already mocks `engram.searchMemories`/`getMemory`/`listScopes`/`listMemories`/
`listTags`/`relatedMemories` this exact way (`search.browser.test.ts:60-71`) — add `understandQuery`
to the same mocked `engram` object rather than a second mock module. The D-17 proof itself: render
the page with a scripted `understandQuerySpy` response carrying one `category` suggestion, assert
`searchMemoriesSpy`'s last call args are unchanged after the suggested row renders, click/Enter the
chip, then assert `gotoSpy` was called with the same URL a manual `FacetStrip` category click would
produce (i.e. reuse whatever existing assertion the file already has for a manual `FacetStrip`
category toggle, applied to the suggested-chip click instead).

## State of the Art

No "old approach" existed for this capability before this phase — it is new. The relevant "state of
the art" is intra-repo: this is the third `internal/decide` consumer, and the pattern has not
changed since `internal/verdict` (Phase 2 of this milestone... actually 2026-08-23.01 era per
`decider.go`'s comments) through `internal/relevance` (this milestone, Phase 1). No deprecations.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `internal/understand` is the right package name/location (CONTEXT.md lists this as an explicit "Claude's Discretion" item, not locked) | Recommended Project Structure | Low — a rename is mechanical; no behavior depends on the package name |
| A2 | A 2000-char (or similar) truncation bound on query text before sending to Jev is warranted, mirroring `relevance.MaxQueryChars` | Common Pitfalls (Pitfall 4) | Low-medium — without it, a pasted long "query" burns timeout budget on `ErrDecisionContextTooLarge` every time; D-05's fallback contract already makes this safe (never fails the search), so the cost is wasted latency/spend, not correctness |
| A3 | `UnderstandQueryRequest` needs no `buf.validate` annotations, matching the D-03 text's explicit citation of `eneecjyzxh` | Recommended Project Structure (proto sketch) | Low — this is a locked decision (D-03), not actually assumed; listed here only because the specific rule id `eneecjyzxh` was not independently re-verified against its own source this session (it is quoted from CONTEXT.md, not re-derived) |

**Risk framing:** every other claim in this document is `[VERIFIED]` against a file read in full
this session (cited inline with `path:line`). The three items above are the only places this
research extrapolated beyond what was directly read.

## Open Questions (RESOLVED)

1. **Does `understandingAudit`'s validation need to be gated at all, or always-unconditional like `RerankAudit`?**
   - What we know: D-16 says Validate "rejects a non-boolean" and the flag "warns when set while
     understanding is off" — both point to unconditional parsing (like `RerankAudit`).
   - What's unclear: whether `Config.Validate` should also require `UnderstandingTimeout` to be a
     valid positive duration only when understanding is *effectively* on (via Pitfall 1's resolver)
     or whenever `Search.Understanding` is the *explicit* string `"jev"` — these differ for a
     deployment with `ENGRAM_DECISIONS_PROVIDER=jev` and `ENGRAM_SEARCH_UNDERSTANDING` unset.
   - Recommendation: validate `UnderstandingTimeout` whenever the *effective* resolved state is on
     (Pitfall 1's three-way switch), not only on the literal string `"jev"` — otherwise a
     deployment that gets understanding "on by default" via D-01 could ship with an invalid timeout
     that Validate never caught, then fail at the `understandDeciderFromConfig` construction site
     instead of at startup config validation. Confirm this reading with the user/planner before
     locking the Validate implementation, since D-01a's own text ("Validate rejects a non-positive
     value **while understanding is on**") is ambiguous between "explicitly on" and "effectively on."
   - RESOLVED (06-04-PLAN.md): validate whenever understanding is *effectively* on — explicit `jev`
     and unset-with-provider-jev.

2. **Where does `internal/understand`'s test suite construct its fixture `caller`/`Subject` values?**
   - What we know: `internal/server/tags.go`/`related.go` both take a `caller` struct
     (`internal/server/identity.go:88-91`) defined in `internal/server`, not `internal/store`.
   - What's unclear: whether `internal/understand` (a new package outside `internal/server`, per
     CONTEXT.md's discretion note "beside internal/relevance") should take a `store.Subject`
     directly (like `internal/relevance`/`internal/verdict` do — neither imports `internal/server`)
     or be handed pre-resolved scope/tag lists by the `internal/server` caller, keeping
     `internal/understand` free of any `internal/server` or `internal/store` dependency at all
     (mirroring `internal/relevance`'s own "performs no I/O of its own beyond the decide.Decider it
     is handed" design).
   - Recommendation: follow `internal/relevance`'s zero-I/O discipline exactly — `internal/understand`
     takes already-fetched scope names/tag names as plain `[]string` arguments (fetched by the
     `internal/server` handler/core via `d.st.ListScopes`/`d.listTags` beforehand), never a
     `Subject` or store reference itself. This keeps the new package testable with zero store
     dependency, matching `internal/relevance`'s and `internal/verdict`'s existing test suites
     (neither constructs a `store.Store` or Qdrant fixture).
   - RESOLVED (06-01/06-02-PLAN.md): zero-I/O — `Input.Scopes []string` / `Input.Tags []string`,
     no store or Subject import.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `ENGRAM_DECISIONS_PROVIDER=jev` + valid Jev/OpenRouter credentials | Live end-to-end verification of D-01's default-on path, D-05's ≥0.9 threshold behavior | Not verifiable in this research session (no live credentials probed) | — | Every server-side unit test uses a fake `Decider` (Pattern/Code Example above) — no live provider is needed for the automated test suite; only a manual/UAT pass needs real credentials, consistent with how `internal/relevance`/`internal/verdict` were originally verified (spike sessions, not CI) |
| Qdrant (for `ListScopes`/`ListTags` live behavior) | D-08/D-09's option-set fetch | Already required by every existing phase; no new requirement | — | — |

**Missing dependencies with no fallback:** none — the phase's automated test suite needs no live
external service (fake Decider + existing `storetest` Qdrant harness already used by
`ListScopes`/`ListTags`'s own tests).
**Missing dependencies with fallback:** live Jev credentials for a manual UAT confirmation of the
default-on disclosure line and a real ≥0.9 suggestion — covered by fake-Decider unit tests in the
interim, per D-17's own list (which is entirely behaviour tests, "no tests of Jev itself").

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (Go) | `go test` (stdlib), no third-party test framework |
| Framework (UI) | `vitest` `5.x` via `vitest-browser-svelte`, config in `ui/vite.config.ts`/`ui/package.json` |
| Config file | `ui/package.json` scripts (`test`, `test:browser`, `test:node`); Go: none (stdlib `go test`) |
| Quick run command | `go test ./internal/config/... ./internal/understand/... ./internal/server/... -run TestUnderstand` (name to be confirmed once tests are authored — see `-run` false-green trap note below) |
| Full suite command | `task test:go` (Go) + `pnpm --dir ui test:browser` (UI) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| NLQ-01 | Unset follows provider; explicit off/jev; Validate rejects bad combos | unit | `go test ./internal/config/... -run TestSearchConfigValidate` (existing file, new subtests) or a new `TestUnderstandingConfigValidate` in a new `understanding_config_test.go` | ❌ Wave 0 — new test file, mirroring `search_config_test.go` (confirmed exists, read in full) |
| NLQ-01 | Docs disclose the new vars | unit (docs gate) | `go test ./internal/config/... -run TestUnderstandingVarsDocumented` | ❌ Wave 0 — new file per Pitfall 2, mirroring `search_docs_test.go` (confirmed exists, read in full) |
| NLQ-01 | Startup discloses when on | unit (slog capture) | `go test ./internal/server/... -run TestUnderstandingEnabledLogLine` | ❌ Wave 0 — mirrors `TestDeciderEnabledLogLine` (confirmed exists via `go test -list`, `internal/server/decider_test.go`) |
| NLQ-02 | Off → fake Decider never called, `{enabled:false}` | unit | `go test ./internal/server/... -run TestUnderstandQueryDisabledNoDeciderCall` (or wherever the Connect handler test lives) | ❌ Wave 0 |
| NLQ-02 | Batches ≤4 Noul + time Choice + scope Choice into ONE Decide call | unit | `go test ./internal/understand/... -run TestNewRequestShape` (mirrors `TestNewRequestShape`, confirmed existing in `internal/relevance` via `go test -list`) | ❌ Wave 0 — new package |
| NLQ-02 | Error/timeout → zero suggestions, never an RPC error | unit | `go test ./internal/understand/... -run TestSuggestFallbackNeverErrors` (mirrors `TestHookFailureLogIsContentFree`/`TestHookNilDecider`, confirmed existing in `internal/relevance` via `go test -list`) | ❌ Wave 0 |
| NLQ-02 | ≥0.9 threshold gate | unit | `go test ./internal/understand/... -run TestFromResponseThreshold` (mirrors `TestFromResultThresholdBoundary`, confirmed existing in `internal/verdict` via `go test -list`) | ❌ Wave 0 |
| NLQ-02 | ≤254-scope gate | unit | `go test ./internal/understand/... -run TestScopeQuestionGate` or `./internal/server/...` depending on where the gate lives | ❌ Wave 0 |
| NLQ-03 | Search results unchanged until chip clicked; accepted chip == manual chip | browser | `pnpm --dir ui test:browser -- search.browser.test.ts -t "understand"` (extends existing `search.browser.test.ts`, confirmed 1328 lines, read scaffolding this session) | ❌ Wave 0 — new `describe` block in existing file |
| NLQ-04 | No query text in logs without audit flag; present with it | unit (slog capture) | `go test ./internal/understand/... -run TestAuditLogGating` (mirrors the `bytes.Buffer`/`slog.NewJSONHandler` pattern confirmed 3x in `decider_test.go`/`rerank_report_test.go`) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** the specific `-run` pattern for that task's new test(s), confirmed against
  `go test -list '.*' ./internal/<pkg>/...` **before** being written into any VALIDATION.md row
  (per the durable `bsbsvn4hbc` false-green trap already recorded in `.planning/STATE.md:488` —
  re-verified this session, still current guidance).
- **Per wave merge:** `task test:go` (full Go suite) + `pnpm --dir ui test:browser` (full browser
  suite).
- **Phase gate:** `task` (lint + test, full repo) green before `/gsd-verify-work`, plus
  `task proto:gen`/`task proto:lint` (buf) re-run and committed since this phase changes
  `engram.proto`.

### Wave 0 Gaps
- [ ] `internal/understand/understand_test.go` — new package, no existing tests
- [ ] `internal/config/understanding_config_test.go` (or extend `search_config_test.go` carefully
      — see Pitfall 2/3 before choosing) — covers NLQ-01
- [ ] `internal/config/understanding_docs_test.go` — new file per Pitfall 2, NOT an extension of
      `search_docs_test.go`
- [ ] `internal/server/understand_test.go` (Connect handler + shared core, mirrors
      `related.go`/`tags.go`'s own test files if present) — covers NLQ-02
- [ ] `ui/src/lib/components/SuggestedRow.svelte` has no component-level unit test file yet
      (whether one is warranted beyond the `search.browser.test.ts` integration coverage is a
      planner decision — the UI-SPEC's own scope is narrow enough that integration coverage alone
      may suffice, matching the existing pattern where `FacetStrip.svelte` also has no dedicated
      unit test, only `search.browser.test.ts` coverage)
- [ ] Framework install: none — `go test`/`vitest` are already fully configured

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Unchanged — `UnderstandQuery` is authenticated exactly like every other Connect read RPC (`callerFromConnectContext`/`subjectFromConnectContext`, `connectapi.go:717` subject interceptor) |
| V3 Session Management | no (new surface) | No new session concept; reuses the existing Connect cookie session |
| V4 Access Control | yes | D-08/D-09's option sets (`ListScopes`/`ListTags`) are already authz-scoped inside `internal/store` (DEC-cgb) — the new handler must not re-derive or widen this (Pitfall 6, `.planning/research/PITFALLS.md`) |
| V5 Input Validation | yes | The shared server-side core validates the request (D-03: no `buf.validate`) — apply the same `field=<name> hint=<code>` envelope convention every other RPC uses; also apply Pitfall 4's query-length bound before it reaches `decide.Request.State` |
| V6 Cryptography | no | Nothing new — reuses the existing Jev client's TLS/transport, no new key material |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Console query text (potentially containing sensitive project/record vocabulary) sent to an external decision provider by default when `ENGRAM_DECISIONS_PROVIDER=jev` was configured for an unrelated feature (consolidate) | Information Disclosure | D-15's mandatory startup disclosure line (not a mitigation that prevents the send — a mitigation that makes the send visible/auditable, matching the design's own stated trade-off in CONTEXT.md D-01: "the mitigation is the startup disclosure, not an opt-in gate") |
| A slow/unavailable decision provider blocking or slowing every search | Denial of Service (self-inflicted) | The dedicated no-retry, short-timeout client (D-01a, Pattern 2) plus D-05's "never fails/delays the search beyond the budget" contract — the RPC runs in parallel with `SearchMemories`, never gates it (D-10) |
| A suggested chip silently changing search results (the whole "advisory" contract failing) | Tampering (of the user's own search intent) | D-11/D-12's UI contract: results never change until an explicit click; proven by the D-17 browser test |
| Query text leaking into logs/traces without the operator's knowledge | Information Disclosure | D-14 (bounded, no-text span attrs) + D-16 (opt-in-only audit log, mirroring `ENGRAM_SEARCH_RERANK_AUDIT`'s already-shipped pattern) |

## Sources

### Primary (HIGH confidence — read in full or targeted-read this session)
- `internal/server/decider.go` (full file, 442 lines) — every resolver/client-construction pattern this phase must mirror three times over
- `internal/decide/decide.go`, `internal/decide/errors.go`, `internal/decide/validate.go` (full files) — question vocabulary, D-12 error classes, `MaxChoices=255`
- `internal/relevance/relevance.go` (full file) — the direct structural template for `internal/understand`
- `internal/verdict/verdict.go` (partial, 1-220) — `Choice`/`Noul` batching precedent, state-truncation helper
- `internal/store/rerank_report.go` (full file) + call sites in `internal/store/rerank.go`/`store.go` — tier-1/tier-2 telemetry split, `trace.SpanFromContext(ctx)` technique
- `internal/config/registry.go`, `internal/config/config.go`, `internal/config/validate.go` (search.*/decisions.* sections) — registry shape, confirms no conditional-default mechanism exists
- `internal/config/search_config_test.go` (full file), `internal/config/search_docs_test.go` (full file) — the exact docs-gate mechanism and its prefix-collision risk (Pitfall 2)
- `internal/config/validate_test.go`, `internal/config/config_test.go`, `internal/config/decisions_config_test.go`, `internal/config/service_auth_test.go` (targeted reads) — every `SearchConfig{...}` literal site (Pitfall 3)
- `internal/server/connectapi.go` (lines 160-230, 640-726) — `ListScopes`/`ListTags`/`RelatedMemories` handler shapes, the Connect-only-RPC precedent, interceptor chain/ordering
- `internal/server/identity.go` (full file) — `caller`/`Subject` resolution for both auth lanes
- `internal/server/tags.go`, `internal/server/related.go` (full files) — the exact shared-core call shape for D-08/D-09
- `internal/server/connectcsrf.go` (lines 1-45) — confirms `UnderstandQuery` needs no CSRF allowlist entry (read RPC)
- `internal/store/store.go` (`ListScopes` signature, line 2081), `internal/store/listtags.go` (`ListTags` signature, line 99)
- `proto/engram/v1/engram.proto` (lines 600-742) — `RelatedEdge` oneof style, message/service additive-numbering convention
- `cmd/engram/spine_review_consolidate_test.go` (lines 760-845) — fake-Decider test patterns
- `internal/server/decider_test.go` (lines 460-514, via `go test -list`) — slog-capture test pattern, confirmed real test names
- `ui/src/lib/search/classify.ts`, `ui/src/lib/search/params.ts` (full files) — D-10's classifier reuse, `SearchParams`/`onchange` shape
- `ui/src/lib/components/FacetStrip.svelte` (lines 1-40) — the exact `onchange` prop signature D-11 must match
- `ui/src/routes/search/+page.svelte` (full file, 781 lines) — the mount point and `navigate`/`onchange` wiring
- `ui/src/routes/search/search.browser.test.ts` (lines 1-120) — browser-test scaffolding (mock shape, `vi.hoisted`)
- `ui/src/lib/client.ts` (full file) — `engram`/`engramWrite` split confirms `UnderstandQuery` uses the read-only client
- `docs-site/src/content/docs/guides/configure.md` (lines 155-284) — the "Search reranking (Jev)" section this phase's new section must mirror
- `.claude/skills/spike-findings-engram/SKILL.md` — spike-003's 0.9-threshold/1.00-accuracy finding, p50 ~270ms decide-call latency (context for the 2s default timeout)
- `.planning/STATE.md` (lines 1-394, `bsbsvn4hbc`/`s780vae1vr` durable gotchas)

### Secondary (MEDIUM confidence)
- `.planning/research/ARCHITECTURE.md` ("Pattern: query-understanding as a server-side, budgeted `internal/decide` consumer") — the design basis CONTEXT.md itself cites; MEDIUM per its own stated confidence ("no direct precedent... extrapolated")
- `.planning/research/PITFALLS.md` (Pitfall 9, "Advisory NL query understanding quietly stops being advisory") — same milestone-research provenance

### Tertiary (LOW confidence)
- None — every claim above traces to a file read this session or a locked CONTEXT.md decision.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, every pattern copied from a live, shipped consumer read in full this session
- Architecture: HIGH — the Connect-only-RPC-with-no-MCP-tool shape, the two/three-client timeout pattern, and the tier-1/tier-2 telemetry split are all already shipped; only the question *content* is new
- Pitfalls: HIGH — Pitfalls 1-3 (config resolution, docs gate, test literals) are demonstrated against actual file contents read this session, not inferred

**Research date:** 2026-09-28
**Valid until:** 30 days (stable, in-repo patterns; the only external-facing dependency, Jev/OpenRouter, is already pinned to a fixed model per `ENGRAM_DECISIONS_MODEL`)
