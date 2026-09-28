# Phase 2: Recall-First Search - Pattern Map

**Mapped:** 2026-09-26
**Files analyzed:** 34 (18 Go, 16 UI/skill)
**Analogs found:** 34 / 34

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `proto/engram/v1/engram.proto` (modify: `RecallGateHiddenCount` msg + field 5 on `SearchMemoriesResponse`, field 8 on `ListMemoriesResponse`) | config (IDL) | request-response | same file, `ListMemoriesResponse`/`SearchMemoriesResponse` (existing `searched_scopes` fields) | exact |
| `internal/server/hiddencount.go` (new: `diffAndBucketByState` + wiring helper) | utility | transform | `internal/server/tools.go` `scopeCoverage`/`searchedScopes`/`recallResultMap` (L2020-2078) | exact (same file's own coverage sub-pattern, extracted to a sibling concept) |
| `internal/server/hiddencount_test.go` (new) | test | transform | `internal/server/crossspinecoverage_test.go` | exact |
| `internal/server/connectapi.go` (modify: `ListMemories`, `SearchMemories` handlers) | controller | request-response | same file, same functions (already the target) | exact |
| `internal/server/tools.go` (modify: `deps.searchMemory`, `deps.listMemory`, `search_memory`/`list_memory` MCP closures, `recallResultMap`) | controller/service | request-response | same file, same functions | exact |
| `internal/store/store.go` — **no diff planned** (Pattern 1: reuse `Store.Search`/`Store.List` unchanged) | service | CRUD | n/a — deliberately zero new store surface | n/a |
| `internal/server/connectapi_parity_test.go` or a new `internal/server/connectapi_read_parity_test.go` (extend/add: read-side hidden-count parity row) | test | request-response | `internal/server/connectapi_write_parity_test.go` (parity pattern) + `internal/server/connectapi_parity_test.go` (field-reflection idiom) | role-match |
| `cmd/engram/client_search.go` (modify: read+print hidden count) | route (CLI command) | request-response | same file, `renderCoverageFooter` call site (L82) | exact |
| `cmd/engram/client_list.go` (modify: same) | route (CLI command) | request-response | same file, `renderCoverageFooter` call site (L94) | exact |
| `cmd/engram/client_common.go` (modify: new `renderHiddenCountFooter` alongside `renderCoverageFooter`) | utility | transform | same file, `renderCoverageFooter` (L316-354) | exact |
| `CLAUDE.md` (modify: memory contract doc — new hidden-count field) | config (docs) | n/a | same file, "Memory contract" section (existing `searched_scopes`/`scopes_truncated`/`scopes_unknown` sentence) | exact |
| `docs-site/.../reference/*.md` (modify: hidden-count field doc) | config (docs) | n/a | existing `searched_scopes` doc entries | role-match |
| `ui/src/lib/search/classify.ts` (new) | utility | transform | `ui/src/lib/memorystate.ts` (pure classifier module, single exported derivation) | role-match |
| `ui/src/lib/search/classify.test.ts` (new) | test | transform | none (`memorystate.ts` has no sibling `*.test.ts`) — see No Analog Found | none |
| `ui/src/lib/search/recall-header.ts` (new) | utility | transform | `ui/src/lib/memorystate.ts` (pure formatter, single exported function + exported order const) | role-match |
| `ui/src/lib/search/recall-header.test.ts` (new) | test | transform | none — see No Analog Found | none |
| `ui/src/lib/errors/connect-error.ts` (new) | utility | transform | `ui/src/lib/errors.ts` (`describeError`, ConnectError narrowing) + `ui/src/lib/client.ts` (`mapAuthError`, `.code`-based narrowing) | exact |
| `ui/src/lib/errors/connect-error.test.ts` (new) | test | transform | none — see No Analog Found | none |
| `ui/src/lib/display.svelte.ts` (new: site-wide text-size store) | store | event-driven | `ui/src/lib/resume.ts` (localStorage/sessionStorage-backed module: safe-storage guard, versioned envelope, best-effort persist) | role-match |
| `ui/src/lib/queries.ts` (modify: extend `ObserveParams`/`listMemoriesKey`-style helpers for the search route's URL state — `k`, chips) | utility | transform | same file, `parseObserveParams`/`observeSearch`/`listMemoriesKey` (existing pattern to extend) | exact |
| `ui/src/lib/components/HeaderSearch.svelte` (new) | component | request-response | `ui/src/lib/components/CommandPalette.svelte` (being split/replaced) + `ui/src/lib/components/ui/command/command.svelte` (restProps passthrough) | exact |
| `ui/src/lib/components/HeaderSearch.browser.test.ts` (new) | test | request-response | `ui/src/lib/components/CommandPalette.browser.test.ts` | exact |
| `ui/src/lib/components/CommandMenu.svelte` (new, replaces `CommandPalette.svelte`'s nav half) | component | event-driven | `ui/src/lib/components/CommandPalette.svelte` | exact |
| `ui/src/lib/components/CommandMenu.browser.test.ts` (new) | test | event-driven | `ui/src/lib/components/CommandPalette.browser.test.ts` | exact |
| `ui/src/lib/components/ResultsList.svelte` (new) | component | streaming (virtualized) | `ui/src/lib/components/MemoryList.svelte` (loading/error/empty branching over a `Memory[]`) | role-match |
| `ui/src/lib/components/ResultsList.browser.test.ts` (new) | test | streaming | `ui/src/lib/components/MemoryList.browser.test.ts` | role-match |
| `ui/src/lib/components/ResultRow.svelte` (new) | component | request-response | `ui/src/lib/components/MemoryRow.svelte` (row layout, state-word badges, dim-iff-past, scope chip, tag overflow) | exact |
| `ui/src/lib/components/ResultRow.browser.test.ts` (new) | test | request-response | `ui/src/lib/components/MemoryRow.browser.test.ts` | exact |
| `ui/src/lib/components/ResultHoverCard.svelte` (new) | component | event-driven | `ui/src/lib/components/ui/hover-card/hover-card.svelte` (controlled `open`, restProps) + `ui/src/lib/components/ScopeChip.svelte` (HoverCard.Root/Trigger/Content usage pattern) | exact |
| `ui/src/lib/components/FacetStrip.svelte` (new) | component | event-driven | `ui/src/routes/observe/+page.svelte`'s `ScopesSidebar` filter callbacks (`onfilter`/`oninclude`) — filter-chip-to-URL-state wiring | role-match |
| `ui/src/lib/components/ScopeCombobox.svelte` (new) | component | request-response | `ui/src/lib/components/ScopeChip.svelte` (scope parsing/rendering) — combobox primitive itself has no existing analog, see No Analog Found for the primitive install | role-match |
| `ui/src/lib/components/ScopeCombobox.browser.test.ts` (new) | test | request-response | `ui/src/lib/components/ScopeChip.browser.test.ts` | role-match |
| `ui/src/lib/components/DetailPane.svelte` (rebuilds `MemoryDetail.svelte`) | component | request-response | `ui/src/lib/components/MemoryDetail.svelte` (entire file — being restructured from tabs to stacked sections, contract preserved) | exact |
| `ui/src/lib/components/DetailPane.browser.test.ts` (rebuilds `MemoryDetail.browser.test.ts`) | test | request-response | `ui/src/lib/components/MemoryDetail.browser.test.ts` | exact |
| `ui/src/routes/search/+page.svelte` (rebuilt) | route | request-response | same file (existing 68-line version — being replaced) | exact |
| `ui/src/routes/+page.svelte` (modified to adopt shared row list) | route | request-response | `ui/src/routes/observe/+page.svelte` (query wiring, `Resizable.PaneGroup` shape) | role-match |
| `ui/src/routes/observe/+page.svelte` (modified: swap `MemoryList`/`MemoryDetail` for `ResultsList`/`DetailPane`) | route | request-response | same file (existing version) | exact |
| `ui/src/routes/+layout.svelte` (modified: mount `HeaderSearch` + `CommandMenu` instead of `CommandPalette`) | route (layout) | event-driven | same file (current `CommandPalette` mount site) | exact |
| `ui/src/app.css` (modified: new tokens) | config | n/a | same file (existing `.dark`/`:root` token blocks) | exact |
| `.claude/skills/engram-console-conventions/SKILL.md` (new) | config (skill doc) | n/a | `.claude/skills/sketch-findings-engram/SKILL.md` | exact |
| `.claude/skills/engram-connect-client/SKILL.md` (new) | config (skill doc) | n/a | `.claude/skills/sketch-findings-engram/SKILL.md` | exact |

## Pattern Assignments

### `proto/engram/v1/engram.proto`

**Analog:** same file — `ListMemoriesResponse`/`SearchMemoriesResponse` existing coverage fields (`proto/engram/v1/engram.proto:116-187`)

**Core pattern** (lines 116-187, read this session):
```protobuf
message ListMemoriesResponse {
  repeated Memory memories = 1;
  uint64 total = 2;
  bool approximate = 3 [deprecated = true];
  string next_page_token = 4;
  repeated string searched_scopes = 5;
  bool scopes_truncated = 6;
  bool scopes_unknown = 7;
  // NEW field 8 this phase:
  // RecallGateHiddenCount hidden = 8;
}

message SearchMemoriesResponse {
  repeated Memory memories = 1;
  repeated string searched_scopes = 2;
  bool scopes_truncated = 3;
  bool scopes_unknown = 4;
  // NEW field 5 this phase:
  // RecallGateHiddenCount hidden = 5;
}
```
Follow the existing doc-comment convention exactly: a field-level comment explaining presence/absence semantics (proto3 zero-value-as-absence), citing the D-number that introduced it, same as `searched_scopes`'/`scopes_unknown`'s comments above. Define the new message once:
```protobuf
// RecallGateHiddenCount is the per-state count of records the recall gate
// hid from this response that would otherwise have appeared (D-01/D-02).
// Reused verbatim on both SearchMemoriesResponse and ListMemoriesResponse —
// one message, two field slots, per their own independent field-number
// sequences (buf breaking: breaking.use: [FILE]).
message RecallGateHiddenCount {
  uint64 archived = 1;
  uint64 superseded = 2;
  uint64 expired = 3;
  uint64 scheduled = 4;
}
```
`buf.yaml`'s `breaking.use: [FILE]` (confirmed in RESEARCH.md) means appending at the next free number is safe; never reuse `approximate`'s number 3 or any other deprecated slot. After editing, regenerate: `task proto:gen` (regenerates `gen/` — CI-checked for drift) and `pnpm --dir ui` regen for `ui/src/lib/gen`.

---

### `internal/server/hiddencount.go` (new)

**Analog:** `internal/server/tools.go` — `scopeCoverage` type + `searchedScopes` + `recallResultMap` (lines 2020-2078, read this session) — the file's own precedent for "compute a cross-cutting recall fact once, share it across MCP result maps and Connect proto fields."

**Core pattern** — second, unmodified `Store.Search`/`Store.List` call, diff by id, bucket by state (RESEARCH.md Pattern 1 / Code Examples, verified against `internal/store/store.go:218-246` Memory field shapes):
```go
// Source: internal/store/store.go:1146-1192 (SearchOptions) and
// internal/server/tools.go:1917-1946 (deps.searchMemory), read this session.
type hiddenBucket struct{ Archived, Superseded, Expired, Scheduled uint64 }

func diffAndBucketByState(gated, ungated []store.Memory) hiddenBucket {
	shown := make(map[string]bool, len(gated))
	for _, m := range gated {
		shown[m.ID] = true
	}
	var b hiddenBucket
	now := time.Now()
	for _, m := range ungated {
		if shown[m.ID] {
			continue
		}
		if m.ArchivedAt != nil {
			b.Archived++
		}
		if m.SupersededBy != nil {
			b.Superseded++
		}
		expired := false
		if m.NotAfter != nil && !m.NotAfter.After(now) {
			b.Expired++
			expired = true
		}
		if !expired && m.NotBefore != nil && m.NotBefore.After(now) {
			b.Scheduled++
		}
	}
	return b
}
```
Precedence (`expired` evaluated first, suppresses `scheduled`) MUST mirror `cmd/engram/memory_state.go:39-67`'s `memoryStateWords` exactly — same comparison operators (`NotAfter` exclusive-Gt-bound, `NotBefore` inclusive-Lte-bound) — but reimplemented against `store.Memory`, not the proto type, since `memoryStateWords` lives in `package main` and is not importable from `internal/server`.

**Anti-pattern to avoid** (RESEARCH.md, explicit): call `d.st.Search` for the ungated comparison, **never** `d.st.SearchReranked` — the latter doubles the Jev rerank cost/log volume for a count that only needs ids and state fields. Call site inside `deps.searchMemory`:
```go
// Existing gated call (searchMemory, tools.go:1934-1944) is UNCHANGED above this.
ungated, uerr := d.st.Search(ctx, scope, c.Subj, vec, req.K, store.SearchOptions{
	Tags: req.Tags, Categories: req.Categories,
	CreatedAfter: req.CreatedAfter, CreatedBefore: req.CreatedBefore,
	IncludeArchived: true, IncludeSuperseded: true, IncludeScheduled: true,
})
```
Both the gated and ungated calls MUST pass `c.Subj` and the SAME `scope` (post-`effectiveSearchScope`) — never a subject-less second call — so the authz predicate (`ownerScopeFilter`/`ownerOrSharedCondition`) is identically applied on both sides (Security Domain, V4). For `ListMemories`, mirror with a second `Store.List` call at the same page window (A2 in RESEARCH.md Assumptions Log — zero new `internal/store` code, the discretion-compliant default).

**Where it's read from:** `deps.searchMemory`/`deps.listMemory` return the hidden bucket alongside the existing `[]store.Memory`/`coreListResult` (extend the return shape, e.g. a second named return or a field on `coreListResult`) so both `internal/server/connectapi.go`'s handlers and the MCP closures in `internal/server/tools.go` read the SAME computed value — exactly how `cov := a.d.searchedScopes(...)` is shared today.

**No `recallEntryPointSeeds`/`recallTransmitters` change needed:** `internal/store/schemaversion_recallgate_test.go`'s three-list AST scan classifies emission *sites* inside `internal/store`, not call *sites* in `internal/server` — since this pattern adds zero new `internal/store` functions (RESEARCH.md Summary, confirmed against the test file's own doc comment), no row is added there. If a future plan instead builds a new `Store.HiddenRecallCounts` method (the Alternatives-Considered option), THAT method would need a `recallTransmitters` row — flag this trade-off explicitly if the plan deviates from the second-call approach.

---

### `internal/server/connectapi.go` (modify `ListMemories`, `SearchMemories`)

**Analog:** same file, same two handlers (lines 244-333, 337-383, read this session)

**Core pattern to extend** (wiring the new field onto the existing response construction, `SearchMemories` shown, `ListMemories` symmetric):
```go
// Existing (connectapi.go:378-383) — cov is already computed and shared:
cov := a.d.searchedScopes(ctx, c, req.Msg.CrossSpine)
return connect.NewResponse(&engramv1.SearchMemoriesResponse{
	Memories:        shapeProtoMemories(ms, req.Msg.Full, a.d.summaryMaxChars),
	SearchedScopes:  cov.Scopes,
	ScopesTruncated: cov.Truncated,
	ScopesUnknown:   cov.Unknown,
	// NEW: Hidden: hiddenToProto(hidden),  — hidden comes back from
	// a.d.searchMemory alongside ms, computed unconditionally (see
	// Anti-Pattern below), never nested inside the crossSpine branch.
}), nil
```
**Anti-pattern (RESEARCH.md, explicit):** do NOT gate the hidden count inside the same conditional that gates `searched_scopes`/`scopes_truncated`/`scopes_unknown` (`recallResultMap`'s `if !crossSpine { return base }`, tools.go:2067-2078). A scope-confined search can still have archived/superseded/expired/scheduled records hidden in it, so the hidden count field is populated unconditionally — always compute and always assign, never `if crossSpine`.

**Error handling pattern** (unchanged): every rejection still routes through `connectError(ctx, err)` (connecterror.go single mapper, D-11) — the hidden-count comparison call's own error (if `Store.Search`'s second call fails) should be treated like `searchedScopes`' own failure mode: log server-side, degrade gracefully (e.g. omit the `Hidden` field or report all-zero with a debug log), never fail the whole RPC for a supplementary count — mirroring `searchedScopes`' `scopeCoverage.Unknown` value-not-error design (tools.go:2023-2036).

---

### `internal/server/tools.go` (modify: `deps.searchMemory`, `deps.listMemory`, `search_memory`/`list_memory` MCP closures)

**Analog:** same file, same functions (lines 1838-2100, 2790-2930, read this session)

**Imports** (already present, no new import needed for the hidden-count wiring itself — `store`, `slog`, `context` already imported at file top).

**Core pattern — MCP result-map wiring** (client_search.go/client_list.go precedent: `recallResultMap` mutates a `map[string]any`):
```go
// Existing (tools.go:2860): cov := d.searchedScopes(ctx, c, a.CrossSpine)
// hits := shapeRecall(ms, a.Full, d.summaryMaxChars)
// result := recallResultMap(map[string]any{"memories": hits}, a.CrossSpine, cov)
//
// NEW: the hidden count is a value returned from d.searchMemory alongside ms
// (extend its signature or wrap in a small result struct), then added
// UNCONDITIONALLY (unlike recallResultMap's crossSpine gate):
result["hidden"] = map[string]uint64{
	"archived": hidden.Archived, "superseded": hidden.Superseded,
	"expired": hidden.Expired, "scheduled": hidden.Scheduled,
}
```
This keeps the MCP JSON shape (`{"archived":0,"superseded":0,"expired":0,"scheduled":0}`) documented as the target shape in RESEARCH.md's Alternatives Considered table, symmetric with the one shared proto message.

**Do NOT** add `include_archived`/`include_superseded`/`include_scheduled` request fields to `searchArgs`/`listArgs` (D-03 — explicitly out of scope; this is an output-only field, the 2026-08-20 decision `fenpnam8ah` still holds).

---

### `cmd/engram/client_common.go` (new `renderHiddenCountFooter`)

**Analog:** same file, `renderCoverageFooter` (lines 316-354, read this session) — the exact sibling-footer pattern to copy.

**Core pattern:**
```go
// Source: cmd/engram/client_common.go:342-354 (renderCoverageFooter, read
// this session) — same shape: gate inside the helper (never at the call
// site), key: value text, always printed (never conditional on cross_spine
// for THIS footer, since a hidden count can occur on a scope-confined call
// too — do not copy renderCoverageFooter's crossSpine-gate here).
func renderHiddenCountFooter(w io.Writer, hidden *engramv1.RecallGateHiddenCount) error {
	if hidden == nil {
		return nil
	}
	total := hidden.GetArchived() + hidden.GetSuperseded() + hidden.GetExpired() + hidden.GetScheduled()
	if total == 0 {
		return nil
	}
	_, err := fmt.Fprintf(w, "hidden_by_recall_gate: %d (archived=%d superseded=%d expired=%d scheduled=%d)\n",
		total, hidden.GetArchived(), hidden.GetSuperseded(), hidden.GetExpired(), hidden.GetScheduled())
	return err
}
```
Call site pattern (mirrors `cmd/engram/client_search.go:82`'s `renderCoverageFooter` call immediately below `renderMemoryTable`):
```go
if err := renderCoverageFooter(cmd.OutOrStdout(), searchCrossSpine, resp.Msg.GetSearchedScopes(), resp.Msg.GetScopesTruncated(), resp.Msg.GetScopesUnknown()); err != nil {
	return err
}
if err := renderHiddenCountFooter(cmd.OutOrStdout(), resp.Msg.GetHidden()); err != nil {
	return err
}
```
Per CLAUDE.md's "CLI is correct-by-reading" (`4aksmneehh`), this new output line needs help-text/docs treatment (`engram search --help` description, docs-site reference), not just a test.

---

### `internal/server/hiddencount_test.go` / read-side parity test (new)

**Analog:** `internal/server/crossspinecoverage_test.go` (failure-injection double pattern, lines 1-40+, read this session) for the Go unit tests; `internal/server/connectapi_write_parity_test.go` + `internal/server/connectapi_parity_test.go` (field-reflection idiom, lines 1-50, read this session) for the MCP↔Connect symmetry test.

**Core pattern:** build a `spyStore`-shaped double (same idiom `failingListScopesStore` uses, embedding `*spyStore` and overriding one method) that returns a scripted archived/superseded/expired/scheduled mix from the ungated call, assert `diffAndBucketByState` buckets correctly, and assert both `a.SearchMemories`/`a.ListMemories` (Connect) and the `search_memory`/`list_memory` MCP closures report byte-identical hidden counts for the same fixture — extending the existing parity-test pattern (which today only proves symmetry for `searched_scopes`/writes) with one new symmetric assertion, per D-03's explicit requirement ("MCP↔Connect parity and cross-spine coverage tests stay symmetric").

---

### `ui/src/lib/search/classify.ts` (new)

**Analog:** `ui/src/lib/memorystate.ts` (whole file, 55 lines, read this session) — pure, dependency-light, single-responsibility derivation module; and RESEARCH.md's own worked example.

**Core pattern** (module shape: exported types + one pure function, no side effects, same style as `memoryStateWords`):
```typescript
// Style precedent: ui/src/lib/memorystate.ts:9,15,35 — exported union type,
// exported canonical-order const, one pure exported function taking the
// input plus an optional `now`/injectable seam for testability.
export type ClassifiedInput =
  | { kind: 'id'; id: string }
  | { kind: 'short_id'; shortId: string }
  | { kind: 'text'; query: string; chips: OperatorChip[] };

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const SHORT_ID_RE = /^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{10}$/; // Crockford base32, case-insensitive

export function classify(input: string): ClassifiedInput { /* ... */ }
```
The `scope:`/`in:`, `#tag`/`tag:`, `is:<category>` operator-token extraction (D-09, CONTEXT.md "Carried forward") lives in the same module. A short_id-shaped miss must say it was searched as text instead — never a silent reinterpretation (CONTEXT.md, carried forward) — surface this as a discriminated result the caller can render, not a thrown error.

---

### `ui/src/lib/search/recall-header.ts` (new)

**Analog:** `ui/src/lib/memorystate.ts` (pure formatter pattern) + the Copywriting Contract in `02-UI-SPEC.md` (verbatim strings, not to be paraphrased).

**Core pattern:** a pure function `(hits, scopes, coverage, hiddenCount, ranked) => string` returning exactly the wording UI-SPEC's Copywriting Contract specifies verbatim, e.g.:
```typescript
// Wording is LOAD-BEARING and copied verbatim from 02-UI-SPEC.md's
// Copywriting Contract — do not paraphrase.
// "12 hits across 3 scopes for query · ranked by cosine · 2 hidden by
//  recall gate · scopes_truncated: 1 readable scope not searched"
export function formatResultsHeader(input: {
  hits: number; scopes: number; query: string; ranked: boolean; reranked: boolean;
  hidden: { archived: number; superseded: number; expired: number; scheduled: number };
  scopesTruncated: boolean; scopesUnknown: boolean;
}): string { /* singular/plural agreement per zero-one-many contract (E1/E8) */ }
```
Never blend `score` and `relevance` into one number (D-05) — the `ranked`/`reranked` inputs are mutually exclusive booleans the caller derives from whether ANY hit carries `relevance`.

---

### `ui/src/lib/errors/connect-error.ts` (new)

**Analog:** `ui/src/lib/errors.ts` (`describeError`, lines 10-14) + `ui/src/lib/client.ts` (`mapAuthError`, lines 30-35) — both narrow on `ConnectError`/`.code` already; this module extends that idiom to `.rawMessage` pattern matching.

**Imports pattern** (mirrors `ui/src/lib/client.ts:1`):
```typescript
import { ConnectError, Code } from '@connectrpc/connect';
```

**Core pattern** (RESEARCH.md Code Examples, verified against `node_modules/@connectrpc/connect/dist/cjs/connect-error.js:40` and `internal/server/connecterror.go:84-85`, `argerror.go:99-101` this session):
```typescript
const ENVELOPE_RE = /^field=([^ ]+) hint=([^:]+): (.+)$/;
const AMBIGUOUS_RE = /^ambiguous short id: (.+)$/;

export function parseConnectError(err: unknown) {
  if (!(err instanceof ConnectError)) return { kind: 'opaque' as const, detail: String(err) };
  if (err.code === Code.NotFound) return { kind: 'not-found' as const };
  const ambiguous = AMBIGUOUS_RE.exec(err.rawMessage);
  if (ambiguous) return { kind: 'ambiguous-short-id' as const, shortId: ambiguous[1] };
  const envelope = ENVELOPE_RE.exec(err.rawMessage);
  if (envelope) return { kind: 'rejected' as const, field: envelope[1], hint: envelope[2], detail: envelope[3] };
  return { kind: 'opaque' as const, detail: err.rawMessage, code: err.code };
}
```
**Anti-pattern (RESEARCH.md Pitfall C, explicit):** never branch on `err.code === Code.FailedPrecondition` alone to distinguish D-04's ambiguous-short_id case from ENTRY-05's rejected-envelope case — both share that code. Never read `.message` (code-prefixed); always `.rawMessage` (raw server text) — this is exactly the same `.code`-not-`.message` discipline `ui/src/lib/client.ts:31`'s `mapAuthError` already follows (`err.code === Code.Unauthenticated`).

---

### `ui/src/lib/display.svelte.ts` (new: site-wide text-size store)

**Analog:** `ui/src/lib/resume.ts` (whole file, 148 lines, read this session) — versioned envelope, `safeSessionStorage`-style guard (here: `localStorage`), best-effort persist that never throws.

**Core pattern** (adapt resume.ts's `safeSessionStorage`/`persistResume`/`peekResume` shape to `localStorage` + Svelte 5 runes):
```typescript
// Style precedent: ui/src/lib/resume.ts:67-76 (safe storage guard, never
// throws) and :81-90 (persist, best-effort, catches quota/serialization
// failures). Adapted here to localStorage + $state (Svelte 5 runes) rather
// than a plain module function, per the display.js contract (foundations.md).
const DISPLAY_KEY = 'engram:display';
const MIN = 12, MAX = 16, DEFAULT = 15;

function safeLocalStorage(): Storage | null {
  try { return typeof localStorage === 'undefined' ? null : localStorage; }
  catch { return null; }
}

let fontSize = $state(DEFAULT);
// Read synchronously before first paint (E9: "applies synchronously from
// local persistence before first paint; there is no loading state" — must
// run in +layout.svelte's <script> module scope or an inline blocking
// script, not onMount, to avoid a flash at the wrong size).
```
E9's error contract ("If persistence is unavailable the preference falls back to 15px for the session without an error message") mirrors `resume.ts`'s own "never throws — degrades to no resume available" philosophy exactly.

---

### `ui/src/lib/queries.ts` (extend)

**Analog:** same file — `ObserveParams`/`parseObserveParams`/`observeSearch`/`listMemoriesKey` (lines 1-52, read this session) — the URL-state and query-key pattern this phase explicitly extends (CONTEXT.md "Reusable Assets").

**Core pattern to extend** (add `k`, chip fields for `/search`'s URL state, following the exact same parse/encode-must-not-drift discipline):
```typescript
// Precedent: INCLUDE_STATES is shared by BOTH parse and encode so they
// cannot drift (queries.ts:9-11). Apply the same discipline to any new
// chip vocabulary (category/scope/tags/window/state) and to `k` (D-08:
// part of URL state, escalates 50->100->250->1000).
export const INCLUDE_STATES: readonly string[] = ['archived', 'superseded', 'scheduled'];
```
`ROW-05`'s filter-chip round-trip test should extend the existing parse/encode test pattern for `parseObserveParams`/`observeSearch` rather than inventing a second URL-state scheme (RESEARCH.md's Recommended Project Structure lists `search/` as siblings, but the URL-codec discipline itself belongs in `queries.ts`).

---

### `ui/src/lib/components/HeaderSearch.svelte` (new)

**Analog:** `ui/src/lib/components/CommandPalette.svelte` (whole file, 22 lines, read this session) being split, and `ui/src/lib/components/ui/command/command.svelte` (restProps passthrough, read this session).

**Imports pattern** (mirrors CommandPalette.svelte:1-3):
```svelte
<script lang="ts">
  import * as Command from '$lib/components/ui/command';
  import * as Popover from '$lib/components/ui/popover'; // new install this phase
  import { base } from '$app/paths';
</script>
```

**Core pattern — Popover + Command, NOT Command.Dialog** (RESEARCH.md Pattern 2, `command.svelte:18-25` confirms `...restProps` passthrough):
```svelte
<!-- Anti-pattern this phase fixes: CommandPalette.svelte:9 uses
     <Command.Dialog bind:open> — the "lying palette" bug (ENTRY-04,
     PITFALLS.md Pitfall 1). HeaderSearch.svelte must NOT copy that line. -->
<Popover.Root bind:open>
  <Popover.Content customAnchor={inputEl}>
    <Command.Root shouldFilter={false}>
      <Command.List>
        <!-- items driven from live SearchMemories/ListScopes/ListTags responses -->
      </Command.List>
    </Command.Root>
  </Popover.Content>
</Popover.Root>
```
**Error handling / empty state:** route `parseConnectError` output into the Copywriting Contract's E2 rows (rejected-envelope fix rows, ambiguous-short_id warning tone, not-found message) — never render a raw `ConnectError.message`.

---

### `ui/src/lib/components/HeaderSearch.browser.test.ts` (new)

**Analog:** `ui/src/lib/components/CommandPalette.browser.test.ts` (existing sibling test — read its structure before writing; same `vitest-browser-svelte` harness, same `render`/`page.getBy...` idiom other `*.browser.test.ts` files use, e.g. `MemoryDetail.browser.test.ts`).

---

### `ui/src/lib/components/CommandMenu.svelte` (new)

**Analog:** `ui/src/lib/components/CommandPalette.svelte` (whole file) — keeps `Command.Dialog`'s default client-side filtering (its items are static navigation, so filtering locally is honest per D-11), but the hand-off row must be exempt from the filter:

**Core pattern:**
```svelte
<Command.Dialog bind:open>
  <Command.Input placeholder="jump to… or search memories" bind:value={q} />
  <Command.List>
    <Command.Group heading="Go to">
      <Command.Item onSelect={() => { onnavigate(`${base}/observe`); open = false; }}>Observe</Command.Item>
      <!-- ...static nav items, default shouldFilter... -->
    </Command.Group>
    <!-- The hand-off row is NEVER itself subject to Command's filter (D-11):
         give it a `value` matching every keystroke, or render outside the
         filtered Command.Group. -->
    <Command.Item value={q} onSelect={() => { onsearch(q); open = false; }}>
      Search memories for "{q}" ↵
    </Command.Item>
  </Command.List>
</Command.Dialog>
```

---

### `ui/src/lib/components/ResultsList.svelte` (new)

**Analog:** `ui/src/lib/components/MemoryList.svelte` (whole file, 25 lines, read this session) for the loading/error/empty/populated branching shape; `@humanspeak/svelte-virtual-list`'s own API (RESEARCH.md, Context7-verified) for the virtualization layer.

**Core pattern** (branching shape to copy, virtualization to add underneath the populated branch):
```svelte
<!-- Precedent: MemoryList.svelte:12-25 — loading (Skeleton), error, two
     distinct empty states (no-scope vs no-results), then populated. Keep
     this exact branch order and copy for ResultsList; ADD the virtualizer
     and role="listbox"/aria-activedescendant wiring only inside the
     populated branch. -->
{#if loading}
  ...Skeleton rows (first load only — E1 "loading": never on re-query, keep
  previous results at ~55% opacity instead, per keepPreviousData)...
{:else if error}
  ...
{:else if hits.length === 0}
  ...honest empty state naming query + coverage (E1 "empty")...
{:else}
  <div role="listbox" tabindex="0" aria-activedescendant={activeId}>
    <!-- @humanspeak/svelte-virtual-list wraps the row rendering; see
         Pitfall A below before finalizing role placement. -->
  </div>
{/if}
```
**Pitfall A (build-and-verify spike, first plan wave):** `@humanspeak/svelte-virtual-list`'s scrollable viewport hardcodes `role="region"` with no prop override (Context7-verified this session). Do not assume `role="listbox"` can be set on the library's own element. Resolve via (1) a post-mount action rewriting the role (fragile) or (2) placing the real `listbox`/`option` roles on an inner wrapper the `renderItem` snippet controls, verified in a real accessibility-tree inspector before locking in the architecture.

**Keyboard model:** `j`/`k`/arrows/`Home`/`End` call the virtualizer's `scroll({ index, align, smoothScroll })` (NOT `scrollToIndex` — confirmed live against the installed `0.5.14` API this session).

---

### `ui/src/lib/components/ResultsList.browser.test.ts` (new)

**Analog:** `ui/src/lib/components/MemoryList.browser.test.ts` — extend with an accessibility-tree assertion for `aria-activedescendant` (write only after the Pitfall A spike resolves, per RESEARCH.md's Phase Requirements → Test Map).

---

### `ui/src/lib/components/ResultRow.svelte` (new)

**Analog:** `ui/src/lib/components/MemoryRow.svelte` (whole file, 105 lines, read this session) — closest possible match: state-word badges, dim-iff-past, scope chip, tag truncation, category color dot are ALL directly reusable.

**Imports pattern** (mirrors MemoryRow.svelte:1-14):
```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import { memoryStateWords, isPastState } from '$lib/memorystate';
  import { Badge } from '$lib/components/ui/badge';
  import ScopeChip from './ScopeChip.svelte';
</script>
```

**Core pattern to copy verbatim** (state words + dim-iff-past, MemoryRow.svelte:52-54, 75):
```svelte
const stateWords = $derived(memoryStateWords(memory));
const dimmed = $derived(isPastState(stateWords));
const dimCls = $derived(dimmed ? 'opacity-60' : '');
...
{#each stateWords as word (word)}<Badge variant="outline" class="text-[10px] uppercase shrink-0">{word}</Badge>{/each}
```
**Divergence from the analog (per D-17, D-04, ROW-04):** the row's ⋯ dropdown menu (MemoryRow.svelte:81-104) is REMOVED — ROW actions move to `DetailPane.svelte`'s inline buttons (D-15); state chips collapse to `first +N` instead of unbounded `{#each}` (this **replaces** the carried-forward wrap rule per CONTEXT.md, since the row is now fixed-height); a `score`/`relevance` column and the row-action `<kbd>` legend are new, with no existing analog (see No Analog Found is NOT needed here — this is additive to the copied shape, not a gap).

---

### `ui/src/lib/components/ResultHoverCard.svelte` (new)

**Analog:** `ui/src/lib/components/ui/hover-card/hover-card.svelte` (7 lines, read this session — bits-ui `LinkPreview` passthrough) + `ui/src/lib/components/ScopeChip.svelte` (HoverCard.Root/Trigger/Content usage idiom, lines 16-54).

**Core pattern** (controlled `open`, `customAnchor`, per RESEARCH.md Pattern 3):
```svelte
<!-- hover-card.svelte:1-7 confirms `open` is bindable and restProps spread
     through to bits-ui LinkPreview.Root -->
<HoverCard.Root bind:open={activeCardOpen}>
  <HoverCard.Content openDelay={250} closeDelay={120} customAnchor={activeRowEl}>
    <!-- content line-clamped to 6 lines, 480px wide, per UI-SPEC E6 -->
  </HoverCard.Content>
</HoverCard.Root>
```
**Critical divergence (Pitfall 4, carried forward):** drive `open` as APP state, never the primitive's own hover detection — pointer hover uses 250ms delay, keyboard focus opens instantly (0ms). Track "hover card open" and "keyboard-active row" as two SEPARATE variables that happen to be set together on a pointer-driven open — never conflate them (this is exactly the bug ScopeChip's own simple `HoverCard.Root` usage does NOT need to avoid, since it has no keyboard-active-row concept; ResultHoverCard does).

---

### `ui/src/lib/components/FacetStrip.svelte` (new)

**Analog:** `ui/src/routes/observe/+page.svelte`'s `ScopesSidebar` wiring (`onfilter`/`oninclude` callbacks, lines 68-69, read this session) — the existing filter-chip-to-URL-state callback shape to copy, applied to a horizontal strip instead of a sidebar.

**Core pattern** (callback-driven URL navigation, mirrors `navigate({...})` calls in observe/+page.svelte:67-69):
```typescript
// Precedent: ui/src/routes/observe/+page.svelte:21-23 — navigate() merges a
// partial param object into the current URL state via observeSearch-style
// encoding, then goto()s with keepFocus/noScroll. FacetStrip should emit the
// same shape of partial-update callbacks (oncategory, onscope, oncrossspine,
// oninclude, ontag) rather than owning navigation itself.
function navigate(next: Partial<SearchParams>) {
  goto(`${base}/search?${encodeSearchParams({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
}
```
Zero-count category chips render at 45% opacity instead of being hidden (E4 overflow); facet counts keep previous values during a re-query (E4 loading, `keepPreviousData`).

---

### `ui/src/lib/components/ScopeCombobox.svelte` (new)

**Analog:** `ui/src/lib/components/ScopeChip.svelte` (scope-parsing/rendering idiom, `parseScope`, category-class map) — the combobox *primitive* itself is a fresh shadcn-svelte install (`popover`/`combobox`, not yet present — see No Analog Found).

**Core pattern** (reuse `parseScope` + category class map from ScopeChip.svelte:2,7-13):
```svelte
import { parseScope } from '$lib/scope';
const catClass = {
  repo: 'text-cat-convention', discovery: 'text-cat-decision',
  project: 'text-cat-preference', '': 'text-muted-foreground',
} as const;
```
Backed by `engram.listScopes({})` (already used in `ui/src/routes/observe/+page.svelte:25`); render each entry's readable-record count with a `~` prefix when `ListScopes` reports approximate (E5 populated).

---

### `ui/src/lib/components/DetailPane.svelte` (rebuilds `MemoryDetail.svelte`)

**Analog:** `ui/src/lib/components/MemoryDetail.svelte` (whole file, 199 lines, read this session) — every field ROW-07 lists (supersession links, schedule window, archive stamp, schema version, citations, both id forms) already exists here; the rebuild changes STRUCTURE (tabs → stacked sections) and ACTIONS (⋯ menu → inline buttons), not data.

**Imports pattern to keep** (MemoryDetail.svelte:1-18, all still needed):
```svelte
import type { Memory } from '$lib/gen/engram_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { ConnectError, Code } from '@connectrpc/connect';
import { relativeTime, fullTimestamp } from '$lib/time';
import { memoryStateWords } from '$lib/memorystate';
import { renderMarkdown } from '$lib/markdown';
```
**Structural divergence (D-14, explicit — supersedes ROW-07's tab wording):** replace `Tabs.Root`/`Tabs.List`/`Tabs.Content` (MemoryDetail.svelte:105-177) with `Separator`-divided stacked sections in this exact order: sticky head → title → actions → State → Content → Tags → Metadata. The State section's field-by-field rendering (MemoryDetail.svelte:143-175 — archived stamp, superseded-by link, supersedes links, expired/scheduled window) copies verbatim into the new "State" section; the Content tab's markdown rendering (MemoryDetail.svelte:128-129) becomes the "Content" section; the Meta tab's scope/actor/source/visibility/schema/tags block (MemoryDetail.svelte:132-142) becomes "Metadata".

**Actions divergence (D-15, D-16, explicit):** replace the `DropdownMenu.Root` ⋯ menu (MemoryDetail.svelte:82-103) with inline `Button`s under the title: Edit, Supersede… (disabled, tooltip "Arrives with curation — Phase 4"), Archive/Restore (disabled, same tooltip), Share/Make private, Delete (danger color). Use `ui/src/lib/components/ui/tooltip` for the disabled-button tooltips (already installed, listed in UI-SPEC's Component Inventory).

**Copy pattern to keep verbatim** (MemoryDetail.svelte:54-64, clipboard copy with toast):
```typescript
async function copy() {
  if (!memory) return;
  try { await navigator.clipboard.writeText(memory.content); toast.success('copied'); }
  catch { toast.error('copy failed'); }
}
```
Extend with a second copy action for the short_id (D-17: `c` copy short_id, `⇧C` copy full UUID — both id forms copyable per ROW-07).

**Resizable pane wiring** (Pattern 4, RESEARCH.md — `ui/src/lib/components/ui/resizable/resizable-pane-group.svelte:1-24`, read this session, already forwards `autoSaveId`/`keyboardResizeBy`): toggle-close via `pane.collapse()`/`pane.expand()` through `bind:this={pane}`. **Pitfall B (explicit):** `keyboardResizeBy` is a PERCENTAGE of the pane group, not pixels — do not wire the sketch's literal `32` expecting 32px steps; either compute a percentage from the live container width or drive `pane.resize(pct)` imperatively from a custom `keydown` handler.

---

### `ui/src/lib/components/DetailPane.browser.test.ts` (rebuilds `MemoryDetail.browser.test.ts`)

**Analog:** `ui/src/lib/components/MemoryDetail.browser.test.ts` — read its full structure before extending; every field-presence assertion it makes today must still pass against the new stacked-section DOM shape (D-14 changes layout, not the field set).

---

### `ui/src/routes/search/+page.svelte` (rebuilt)

**Analog:** same file (current 68-line version, read this session) for the query-wiring shape; `ui/src/routes/observe/+page.svelte` for the richer `Resizable.PaneGroup` + `WriteSurfaces` + re-auth-resume pattern to adopt.

**Core pattern to keep** (URL-driven `createQuery`, `page.url.searchParams` reactive read — search/+page.svelte:13-23):
```svelte
const searchQ = createQuery(() => {
  const query = page.url.searchParams.get('q') ?? '';
  // NEW: read k, chips, cross_spine from queries.ts's extended parse fn
  // instead of raw searchParams.get() calls — the current file's direct
  // reads (lines 16-17) are the OLD, pre-extension idiom; migrate to the
  // parseObserveParams-style single parse call this phase adds.
  return { queryKey: [...], queryFn: () => engram.searchMemories({...}, { signal }), placeholderData: keepPreviousData, enabled: !!query };
});
```
**Divergence:** default `k = 50` (D-08, was `50n` hardcoded already — keep, but make it part of URL state with the 50→100→250→1000 escalation); add `AbortSignal` (`{ signal }` in `queryFn`, per RESEARCH.md's `keepPreviousData` code example) — the CURRENT file has no signal at all, this is the ENTRY-06 stale-response race fix; swap `MemoryList`/`MemoryDetail` for `ResultsList`/`DetailPane`; add `FacetStrip`, honest results header (`recall-header.ts`), and the operator-only-input → `ListMemories`-infinite-scroll branch (D-09).

---

### `ui/src/routes/+page.svelte`, `ui/src/routes/observe/+page.svelte` (modified)

**Analog:** `ui/src/routes/observe/+page.svelte` (whole file, 121 lines, read this session) is both its own analog (for its own edit) and the analog for `+page.svelte`'s restyle onto the shared row list.

**Core pattern to keep unchanged:** `ScopesSidebar` stays in `/observe` this phase (D-12 — reshaped in Phase 4), `Resizable.PaneGroup`/`Pane`/`Handle` layout (lines 71-120), `WriteSurfaces` host wiring (lines 45-58), re-auth resume `onMount` (lines 55-58) — copy this exact `peekResume`/`reopenFromResume`/`onresumeapplied={consumeResume}` triple onto any new route that hosts writes.

**Divergence:** swap `MemoryList`/`MemoryDetail` (lines 14-15, 83-89, 114-118) for `ResultsList`/`DetailPane`; `/` additionally restyles its scope tiles + recent feed onto the new dense row list per D-10.

---

### `ui/src/routes/+layout.svelte` (modified)

**Analog:** same file — locate the current `CommandPalette` mount (bind `open`, `onsearch`, `onnavigate` props per `CommandPalette.svelte:5`'s prop contract) and replace with `HeaderSearch` (always visible, header-anchored) + `CommandMenu` (⌘K-triggered, same `open`/`onnavigate` contract plus the new hand-off-to-search `onsearch` callback).

---

### `ui/src/app.css` (modified: new tokens)

**Analog:** same file — existing `.dark`/`:root` token blocks (not read this session in full, but referenced throughout CONTEXT.md/RESEARCH.md/UI-SPEC.md as the single source of truth for `--color-*`/`--cat-*` tokens).

**Core pattern:** add `--color-surface-2`, `--color-hover`, `--color-selected`, `--color-border-subtle`, `--color-text-faint`, `--color-primary-soft`, `--color-warning`, `--color-success`, `--cat-rule` to BOTH the `.dark` and `:root` blocks (dark/light values enumerated in UI-SPEC.md's Color section) — follow the file's own existing per-variable-per-theme pairing exactly, do not invent a new naming convention. Remove the hardcoded `body { font-size: 13px; }` (D-13) — the `display.js`/`display.svelte.ts` contract's `--ui-font` variable (default `15`) replaces it; `--u: calc(var(--ui-font) / 13)` derives every scaled dimension.

---

## Shared Patterns

### Authorization stays store-layer-only (DEC-cgb)
**Source:** `internal/store/store.go` `Search`/`List` (ownerScopeFilter/ownerOrSharedCondition, unread in full this session but referenced throughout RESEARCH.md's Security Domain)
**Apply to:** `internal/server/hiddencount.go`, `internal/server/connectapi.go`, `internal/server/tools.go`
The hidden-count comparison call MUST go through `Store.Search`/`Store.List` unchanged (their own filter-building path), passing the SAME `c.Subj`/resolved scope as the gated call — never a hand-rolled second filter that could silently drop the owner/scope authz condition. No new authz path, no handler-level check (V4, RESEARCH.md Security Domain).

### Single connectError mapper (D-11)
**Source:** `internal/server/connecterror.go` (lines 1-105, referenced) / `internal/server/connectapi.go` (every handler)
**Apply to:** any new/modified Connect handler code
Every rejection in a Connect handler routes through `connectError(ctx, err)` — never a hand-wrapped `connect.NewError(connect.CodeInvalidArgument, ...)` at a boundary check, which would override the failure CLASS the shared core (`argError`) already computed.

### Cross-cutting coverage computed once, shared by both lanes
**Source:** `internal/server/tools.go` `(*deps).searchedScopes` + `scopeCoverage` (lines 2020-2050)
**Apply to:** `internal/server/hiddencount.go`'s hidden-count computation
Compute the hidden-count value ONCE inside the shared `deps.searchMemory`/`deps.listMemory` core, return it as a VALUE (never discard already-computed hits on its own failure — mirror `scopeCoverage.Unknown`'s degrade-not-abort design), and let both the Connect handler and the MCP closure read the same computed result — never two independent computations that could drift.

### `.rawMessage`/`.code`, never `.message`, for Connect error classification
**Source:** `ui/src/lib/client.ts:30-35` (`mapAuthError`)
**Apply to:** `ui/src/lib/errors/connect-error.ts`, any new component reading a `ConnectError`
`.message` is code-prefixed (confirmed via `node_modules/@connectrpc/connect/dist/cjs/connect-error.js:40` this session); always branch on `.code` (numeric `Code` enum) and `.rawMessage` (raw server text) instead.

### State-word derivation stays single-sourced per surface
**Source:** `ui/src/lib/memorystate.ts` (whole file) / `cmd/engram/memory_state.go` (whole file)
**Apply to:** `ResultRow.svelte`, `DetailPane.svelte`, any new Go hidden-count bucketing code
Every surface (TS console, Go CLI, and now the new `internal/server` hidden-count bucketer) derives state words independently but with IDENTICAL precedence (`expired` evaluated first, suppresses `scheduled`) and IDENTICAL boundary comparisons (`NotAfter` exclusive-Gt, `NotBefore` inclusive-Lte) — never share the derivation across a package boundary that would require an import cycle; keep each surface's own copy pinned by its own test.

### Query-key / URL-state pattern extension discipline
**Source:** `ui/src/lib/queries.ts` (`INCLUDE_STATES`, `parseObserveParams`/`observeSearch`, `listMemoriesKey`)
**Apply to:** `ui/src/lib/search/classify.ts`'s chip vocabulary, `/search`'s new URL params (`k`, chips)
Any new URL-encoded vocabulary array (e.g. a new chip kind) must be declared ONCE and referenced by both the parse and encode functions — never duplicated as two separate literal arrays that could drift out of sync (the exact discipline `INCLUDE_STATES` already demonstrates).

### `keepPreviousData` + per-query `AbortSignal` for every debounced search query
**Source:** RESEARCH.md Code Examples (`@tanstack/svelte-query@6.1.48`, confirmed byte-identical to v5's `query-core@5.102.8`)
**Apply to:** `HeaderSearch.svelte`, `/search`'s `searchQ`, `ScopeCombobox.svelte`
```typescript
const query = createQuery(() => ({
  queryKey: [...],
  queryFn: ({ signal }) => engram.searchMemories({...}, { signal }),
  placeholderData: keepPreviousData,
  enabled: !!text
}));
```
Never omit `{ signal }` on a debounced query — this is the direct fix for PITFALLS.md's Pitfall 2 (stale-response race) and ENTRY-06.

---

## No Analog Found

Files/primitives with no close existing match (planner should use RESEARCH.md's Architecture Patterns / Code Examples instead):

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `ui/src/lib/search/classify.test.ts`, `recall-header.test.ts`, `errors/connect-error.test.ts` | test | transform | No existing `*.test.ts` (node-project) sibling exists for `memorystate.ts` or any other `ui/src/lib/*.ts` pure module today — these would be the FIRST node-project unit tests in `ui/src/lib`. Use `ui/vite.config.ts`'s `node` project config directly (RESEARCH.md: `pnpm --dir ui vitest run --project node <file>`); no component-test scaffolding needed, just a plain Vitest `describe`/`it` file. |
| `ui/src/lib/components/ui/popover/*` (shadcn install) | component (primitive) | event-driven | Not yet installed (`ls ui/src/lib/components/ui/` confirms no `popover/` directory this session) — install via `pnpm dlx shadcn-svelte@latest add popover`, then treat the generated wrapper the same way `hover-card.svelte`/`resizable-pane-group.svelte` already forward restProps (no hand-authored wrapper needed). |
| `ui/src/lib/components/ui/combobox/*` (shadcn install) | component (primitive) | event-driven | Same as popover — not yet installed, install via `pnpm dlx shadcn-svelte@latest add combobox`. |
| `internal/server/*_hiddencount_test.go`'s Go-side fixture data for a mixed archived+superseded+expired record | test fixture | transform | No existing fixture in `internal/server`'s test doubles constructs a record carrying THREE simultaneous states (A4 in RESEARCH.md's Assumptions Log) — the planner should add one, following `failingListScopesStore`'s fixture-construction idiom in `crossspinecoverage_test.go`. |

## Metadata

**Analog search scope:** `internal/server/*.go`, `internal/store/store.go`, `cmd/engram/*.go`, `proto/engram/v1/engram.proto`, `ui/src/lib/**/*.{ts,svelte}`, `ui/src/routes/**/*.svelte`, `.claude/skills/*/SKILL.md`
**Files scanned:** ~30 direct reads (Go: connectapi.go, tools.go, store.go, memory_state.go, client_common.go, client_search.go, crossspinecoverage_test.go, connectapi_parity_test.go, schemaversion_recallgate_test.go, engram.proto; TS/Svelte: client.ts, queries.ts, errors.ts, memorystate.ts, resume.ts, CommandPalette.svelte, MemoryList.svelte, MemoryRow.svelte, MemoryDetail.svelte, ScopeChip.svelte, search/+page.svelte, observe/+page.svelte, command.svelte, hover-card.svelte, resizable-pane-group.svelte; skills: sketch-findings-engram/SKILL.md) plus full 02-CONTEXT.md/02-RESEARCH.md/02-UI-SPEC.md.
**Pattern extraction date:** 2026-09-26
**Tracked-source gate:** every analog path listed above verified via `git ls-files` (sample: `.claude/skills/sketch-findings-engram/SKILL.md`, `cmd/engram/client_common.go`, `internal/server/connectapi.go`, `internal/server/tools.go`, `proto/engram/v1/engram.proto`, `ui/src/lib/components/CommandPalette.svelte`, `ui/src/routes/search/+page.svelte`) — all tracked, none are gitignored mirrors.
