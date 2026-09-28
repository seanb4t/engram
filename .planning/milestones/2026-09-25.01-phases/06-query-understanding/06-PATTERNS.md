# Phase 6: Query Understanding - Pattern Map

**Mapped:** 2026-09-28
**Files analyzed:** 15
**Analogs found:** 15 / 15

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/understand/understand.go` | service | request-response | `internal/relevance/relevance.go` | exact |
| `internal/understand/understand_test.go` | test | request-response | `internal/relevance/relevance_test.go` (mirror), `cmd/engram/spine_review_consolidate_test.go` (fake decider) | exact |
| `proto/engram/v1/engram.proto` (+FilterSuggestion/UnderstandQuery*) | config/schema | request-response | existing `RelatedEdge`/`ListTags*` messages, same file | exact |
| `internal/server/decider.go` (+understandingEnabled/Timeout/Audit resolvers, +understandDeciderFromConfig) | service/config | request-response | same file's `searchDeciderFromConfig`/`searchRerankTimeout`/`searchRerankAudit`/`logSearchRerankAuditEnabled` | exact |
| `internal/server/understand.go` (NEW: deps.understandQuery core) | controller/service | request-response | `internal/server/related.go` (`relatedMemories`), `internal/server/tags.go` (`listTags`) | exact |
| `internal/server/connectapi.go` (+UnderstandQuery handler) | controller | request-response | same file's `ListScopes` handler | exact (documented Connect-only precedent) |
| `internal/config/config.go` (+SearchConfig fields) | config | CRUD | same file's existing `SearchConfig` struct | exact |
| `internal/config/registry.go` (+3 rows) | config | CRUD | same file's `search.rerank_timeout`/`search.rerank_audit` rows | exact |
| `internal/config/validate.go` (+D-01/D-01a/D-16 block) | config | CRUD | same file's `search.ranker`/`RerankAudit` validation block (~line 380/409) | exact |
| `internal/config/search_config_test.go` (+3 fields, +cases) | test | CRUD | same file's existing `searchFields`/`TestSearchConfigValidate` | exact |
| `internal/config/understanding_docs_test.go` (NEW) | test | request-response | `internal/config/search_docs_test.go` (structural copy, new prefix/heading) | exact |
| `ui/src/lib/components/SuggestedRow.svelte` (NEW) | component | request-response | `ui/src/lib/components/FacetStrip.svelte` | exact |
| `ui/src/routes/search/+page.svelte` (+query, +mount) | component | request-response | same file's existing `createQuery`/`FacetStrip` mount + `navigate()` | exact |
| `ui/src/routes/search/search.browser.test.ts` (+assertions) | test | request-response | same file's existing spy/hoisted scaffolding | exact |
| `docs-site/.../guides/configure.md` (+"Query understanding" section) | config/docs | request-response | same file's "## Search reranking (Jev)" section | exact |

## Pattern Assignments

### `internal/understand/understand.go` (service, request-response)

**Analog:** `internal/relevance/relevance.go`

**Package doc + imports** (lines 1-23):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package relevance is the search-path relevance question set shared by the
// server's Jev rank hook and the retrieval eval: spike 004's noul-per-
// candidate request, the D-08 budgeted candidate state, and mapping a
// decide.Response to a per-id probability map. It performs no I/O of its
// own beyond the decide.Decider it is handed, and it never filters, drops
// or acts on a probability — callers do that.
package relevance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/verdict"
)
```
Model `internal/understand`'s doc comment on this shape: "performs no I/O of its own beyond the decide.Decider it is handed." Import the same `internal/decide`, plus `internal/store` (for `store.Subject`/`ListScopes`/`TagCount` types) — never `internal/relevance` itself (siblings, not a dependency).

**Core pattern — build-one-request, decide-once, map-back** (RESEARCH.md Pattern 1, sourced from `internal/relevance/relevance.go:127-175,186-201`):
```go
func buildRequest(query string, applied appliedFilters) decide.Request {
	state := decide.State{StateQuery: query} // never record content (D-05)
	questions := map[string]decide.Question{}
	for _, cat := range consoleCategories {
		if !applied.hasCategory(cat) {
			questions["category_"+cat] = decide.Noul(instructionsFor(cat), whenTrueFor(cat), whenFalseFor(cat))
		}
	}
	if !applied.hasWindow() {
		questions["time_window"] = decide.Choice(windowInstructions, timeWindowOptions)
	}
	if scopeEligible {
		questions["scope"] = decide.Choice(scopeInstructions, scopeOptionsFromListScopes)
	}
	return decide.Request{State: state, Questions: questions}
}
```
`FromResponse` mirrors `relevance.FromResponse`'s strict-validation shape: missing/mistyped/out-of-range answers become `decide.ErrDecisionMalformedResponse`, never silently skipped. Call `dec.Decide` exactly once — never `DecideMany` (one State: the query text, per D-05).

**Query truncation (Pitfall 4):** reuse `relevance.MaxQueryChars` (2000) or `internal/verdict`'s rune-safe truncation (`internal/verdict/verdict.go:84-101`) before writing the query into `decide.Request.State` — `decide.Request.Validate()` does no size checking itself.

**Telemetry (ambient span, Pattern 5)** — source `internal/store/rerank_report.go:1-43,68-112` and call site `internal/store/store.go:1370-1375`:
```go
const (
	AttrRerankOutcome       = "engram.rerank.outcome"
	AttrRerankFallbackClass = "engram.rerank.fallback_class"
	// ...
)

ranked, rep := rankWithReport(ctx, query, hits, int(k), opts.RankHook)
rep.stamp(trace.SpanFromContext(ctx))
if opts.RankAudit {
	rep.audit(ctx, "search_memory", query, ownerOf(subj))
}
```
For `internal/understand`, the handler/core calls `trace.SpanFromContext(ctx).SetAttributes(...)` with `engram.understand.outcome`/`fallback_class`/`suggestion_count`/`questions_asked` — no manual span creation (the ambient RPC span from `otelconnect.NewInterceptor()` already covers it).

**Error classification:** use `decide.Status(err)` (the shared D-12 sentinel classifier) — never a hand-rolled switch on error text.

---

### `internal/server/decider.go` (+3 resolvers, +1 client builder)

**Analog:** same file, existing `searchDeciderFromConfig`/`searchRerankTimeout`/`searchRerankAudit`/`logSearchRerankAuditEnabled`

**Two-client no-retry pattern** (verbatim, `internal/server/decider.go:198-224`):
```go
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
`understandDeciderFromConfig` is a structural copy substituting `understandingTimeout(cfg)` for `searchRerankTimeout(cfg)`. Gate it on `cfg.Decisions.Provider`, never on `cfg.Search.Understanding` alone (mirrors the existing function's own comment).

**D-01 config-resolution pitfall (no registry precedent for a cross-field default):**
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
`internal/config.Validate` needs its own independent copy of this same three-way switch (config cannot import server).

**Audit-flag pattern (verbatim, `internal/server/decider.go:168-196`):**
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
`understandingAudit(cfg)` / `logUnderstandingAuditEnabled(enabled bool)` are structural copies, including the "warns when set while off" branch (D-16).

**D-15 disclosure** mirrors `logSearchRankerEnabled` (`decider.go:298-317`): one `slog.Info` naming the endpoint host only, plus whether "jev" came from default or explicit (the `source` string above).

---

### `internal/server/understand.go` (NEW core) + `connectapi.go` (+handler)

**Analog:** `internal/server/related.go` (shared-core shape), `internal/server/tags.go` (authz passthrough), `internal/server/connectapi.go`'s `ListScopes` (Connect-only precedent)

**Authz passthrough — never re-filter** (verbatim, `internal/server/tags.go:39-44`):
```go
func (d *deps) listTags(ctx context.Context, c caller, a listTagsArgs) ([]store.TagCount, bool, error) {
	if err := rejectOverMaximumCount("limit", a.Limit); err != nil {
		return nil, false, err
	}
	return d.st.ListTags(ctx, c.Subj, a.Scope, a.Limit)
}
```
The understanding core calls `d.st.ListScopes(ctx, c.Subj)` (D-08, gate on `len(scopes) <= 254` per Pitfall 5 — not the `more`/`approx` flag) and `d.listTags(ctx, c, listTagsArgs{Scope: ...})` (D-09, reuse the existing shared core, never a second `d.st.ListTags` call path).

**Connect-only handler, no MCP counterpart** (verbatim, `internal/server/connectapi.go:176-193`):
```go
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
`UnderstandQuery`'s handler uses `callerFromConnectContext` (not `subjectFromConnectContext`) because `d.listTags` needs `caller.Actor`-shaped context. State the "no MCP tool" comment citing D-02 instead of "research OQ2." When understanding is off, short-circuit before any `ListScopes`/`ListTags` call (D-04) — return `{enabled:false}` immediately.

---

### `internal/config/*` (registry, validate, config, docs test)

**Analog:** same-package existing `search.rerank_timeout`/`search.rerank_audit`/`search.ranker` rows and their validation/docs-test siblings.

**Anti-pattern (do not do this):** `{Key: "search.understanding", Env: "ENGRAM_SEARCH_UNDERSTANDING", Default: "jev"}` — there is no field-to-field conditional default in `field{}` (`internal/config/registry.go:12-19`). Set `Default: ""` and resolve unset in the hand-written `understandingEnabled` function in `decider.go` (see above); `Config.Validate` needs its own independent copy of the same three-way switch.

**Docs-gate pitfall:** do NOT extend `internal/config/search_docs_test.go` — its `searchRegistryEnvNames()` matches every `ENGRAM_SEARCH_` prefixed `Env` and hardcodes `len(envs) != 3` as a positive control (`search_docs_test.go:74`). Instead author a new `internal/config/understanding_docs_test.go`, a structural copy filtering on the `ENGRAM_SEARCH_UNDERSTANDING` prefix and scoped to a new `## Query understanding (Jev)` heading in `configure.md` (mirroring the existing `## Search reranking (Jev)` section, `configure.md:220-260`).

**Test-literal pitfall (Pitfall 3):** every existing `SearchConfig{RerankAudit: "false"}` literal (`internal/config/validate_test.go:22`, `internal/config/config_test.go:253`, `internal/config/service_auth_test.go:288`) must also set `UnderstandingAudit: "false"` once that field is validated unconditionally like `RerankAudit` already is — re-run `rg -n 'SearchConfig\{' internal/config` at implementation time; this list is a snapshot.

---

### `ui/src/lib/components/SuggestedRow.svelte` (NEW component)

**Analog:** `ui/src/lib/components/FacetStrip.svelte`

**`onchange` prop contract** (verbatim, `FacetStrip.svelte` lines ~14-45):
```svelte
// `onchange` and the page (+page.svelte) is what turns that into a URL.
...
onchange,
...
onchange: (next: Partial<SearchParams>) => void;
...
function toggleCategory(c: string) {
  const has = params.categories.includes(c);
  onchange({ categories: has ? params.categories.filter((x) => x !== c) : [...params.categories, c] });
}
function removeTag(t: string) {
  onchange({ tags: params.tags.filter((x) => x !== t) });
}
```
`SuggestedRow` must take the identical `onchange: (next: Partial<SearchParams>) => void` prop and call it with the exact per-kind partial shape UI-SPEC's "Per-kind accept mapping" table specifies (category/time_window/scope/tag), so the resulting `navigate()` call and `SearchMemoriesRequest` are indistinguishable from a manual `FacetStrip` chip.

**Chip anatomy / ScrollArea edge-fade:** reuse `FacetStrip.svelte`'s `.facet-chip`/`.facet-chip-removable` CSS classes, `ScrollArea.Root orientation="horizontal"`, and `measureFade`/`data-fade-start`/`data-fade-end` mechanism verbatim — do not reimplement (UI-SPEC Component Inventory).

**Dismiss button, non-nested-button DOM shape:** `FacetStrip.svelte`'s tag-chip `removeTag` button pattern — a non-interactive wrapper holding two sibling `<button>`s (accept control in the roving set, dismiss control `tabindex="-1"`), never a button nested inside a button (UI-SPEC Interaction Notes, roving-tabindex section).

---

### `ui/src/routes/search/+page.svelte` (+query, +mount)

**Analog:** same file's existing `FacetStrip` mount and `navigate()` function.

**`onchange` wiring to reuse verbatim** (`+page.svelte` line 551):
```svelte
onchange={(partial) => navigate({ ...partial, sel: '' })}
```
Mount `<SuggestedRow onchange={(partial) => navigate({ ...partial, sel: '' })} ... />` directly under `<FacetStrip>` using this exact same callback — this is what makes an accepted suggestion "exactly a manually added chip" (D-11).

**`navigate()` function** (`+page.svelte` line 77) — the single URL/request codec; the query key for the new `createQuery` must be `['understandQuery', q]` (bare `q`, not the merged `effective` object) per UI-SPEC's "Query-key pitfall" — building it from `effective` would silently refetch on every facet-chip toggle, unwiring D-10's firing rule.

**Trigger/eligibility source:** `ui/src/lib/search/classify.ts`'s `classifyInput()` — reuse verbatim for D-10's "not UUID/short_id-shaped, no operator chips, 2+ words" gate. `ui/src/lib/search/params.ts` is the one URL/request codec an accepted chip's resulting `Partial<SearchParams>` must round-trip through.

---

### `ui/src/routes/search/search.browser.test.ts` (+assertions)

**Analog:** same file's existing hoisted spy scaffolding (lines 1-79), which already includes an `understandQuerySpy` placeholder per RESEARCH.md.

**Fake-Decider pattern for server-side D-17 proof** (verbatim, `cmd/engram/spine_review_consolidate_test.go:791-806`):
```go
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
Define a package-local copy in `internal/understand`'s own test file (do not cross-import `cmd/engram`) to prove the `{enabled:false}` short-circuit never touches the decider.

**Slog-capture pattern for D-17's "no query text without audit flag"** (verbatim, `internal/server/decider_test.go:479-484`):
```go
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
This exact shape appears three times already (`decider_test.go:481-484,858-861,1042-1045`, `rerank_report_test.go:189-192,229-232`) — the house pattern for log-output assertions.

---

## Shared Patterns

### Advisory-decision, never-acted-on contract
**Source:** `internal/relevance` (rank hook), `internal/verdict` (consolidate)
**Apply to:** `internal/understand`, the Connect handler, and `SuggestedRow.svelte`
A decision failure/timeout degrades to "no decision" (zero suggestions), never an error surfaced to the caller or search failure. Applies identically server-side (D-12/D-17) and client-side (row is simply absent, no error UI).

### Two-client, dedicated no-retry timeout
**Source:** `internal/server/decider.go` (`searchDeciderFromConfig`/`searchRerankTimeout`, and consolidate's `deciderFromConfig`)
**Apply to:** the new `understandDeciderFromConfig`/`understandingTimeout` pair — this is the THIRD such dedicated client; never share the sweep or search-rerank client.

### Tier-1/Tier-2 telemetry split
**Source:** `internal/store/rerank_report.go` (`rankReport.stamp`/`rankReport.audit`)
**Apply to:** `internal/understand`'s telemetry — always-on bounded span attrs (`engram.understand.*`) stamped on the ambient RPC span via `trace.SpanFromContext(ctx)`, plus a single opt-in `slog.InfoContext` audit line gated on `understandingAudit(cfg)`, never a new child span.

### Authz passthrough, never re-filter
**Source:** `internal/server/tags.go` (`listTags`), `internal/server/related.go` (`relatedMemories`)
**Apply to:** the understanding core's `ListScopes`/`ListTags` calls — pass `c.Subj` straight into the store call; never re-derive or re-filter the returned set (locked invariant DEC-cgb / Pitfall 6).

### URL/request codec as single source of truth
**Source:** `ui/src/lib/search/params.ts`, `+page.svelte`'s `navigate()`
**Apply to:** `SuggestedRow`'s accept path — every accepted chip must produce a `Partial<SearchParams>` that round-trips through the existing codec, identical to a manually toggled `FacetStrip` control.

## No Analog Found

None — every file in scope has a direct, exact-match analog already shipped in this codebase (this phase is explicitly the "third consumer" of an already-twice-proven pattern set per RESEARCH.md).

## Metadata

**Analog search scope:** `internal/relevance`, `internal/verdict`, `internal/server` (`decider.go`, `connectapi.go`, `tags.go`, `related.go`, `decider_test.go`), `internal/store/rerank_report.go`, `internal/config` (`registry.go`, `validate.go`, `search_docs_test.go`, `search_config_test.go`), `cmd/engram/spine_review_consolidate_test.go`, `ui/src/lib/components/FacetStrip.svelte`, `ui/src/routes/search/+page.svelte`, `ui/src/routes/search/search.browser.test.ts`, `ui/src/lib/search/classify.ts`, `ui/src/lib/search/params.ts`.
**Files scanned:** 15 target files against ~14 analog files, all git-tracked (verified via `git ls-files`).
**Pattern extraction date:** 2026-09-28
