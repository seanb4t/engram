# Phase 7: Console & CLI State Surfacing - Pattern Map

**Mapped:** 2026-08-20
**Files analyzed:** 21 (new + modified, per CONTEXT.md/RESEARCH.md's Recommended Change Map)
**Analogs found:** 20 / 21 (one deliberate "no analog" — see below)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `proto/engram/v1/engram.proto` (+6 bools, +1 RPC) | config/schema | request-response | same file, `ListMemoriesRequest`/`SearchMemoriesRequest` fields 1-12/1-9, `cross_spine` field, `EngramService` RPC list | exact (additive edit to an existing message/service) |
| `internal/store/store.go` — `SearchOptions`/`ListOptions` +3 bools, `Store.Search`/`Store.List` gate sites | store/model | CRUD (filtered read) | same file — `tags`/`created_after`/`created_before` threading through `SearchOptions`→`Store.Search` and `ListOptions`→`listFilter`→`Store.List` | exact |
| `internal/server/tools.go` — `coreListRequest`/`coreSearchRequest` +3 bools | service (typed core) | request-response | same file — existing `Tags`/`CreatedAfter`/`CreatedBefore` fields on the same two structs | exact |
| `internal/server/connectapi.go` — `ListMemories`/`SearchMemories` handler edits | controller (Connect RPC handler) | request-response | same file — `req.Msg.Tags`/window-bound parsing already threaded in the same two handlers | exact |
| `internal/server/connectapi.go` — new `MigrateStatus` handler | controller (Connect RPC handler) | request-response | `ListScopes` handler, same file, lines 170-184 | exact |
| `cmd/engram/client_get.go` (new) | CLI verb / controller | request-response | `cmd/engram/client_list.go` (full file) | exact |
| `cmd/engram/client_<migration-verb>.go` (new) | CLI verb / controller | request-response | `cmd/engram/migrate_family.go` — `migrateStatusCmd`/`migrateStatusReportDoc` (operator-tier sibling) + `client_list.go` (client-tier skeleton) | role-match (blend of two analogs — see below) |
| `cmd/engram/client_common.go` — `renderMemoryTable` STATE column | utility (renderer) | transform | same file — `withScore`'s existing conditional-column precedent, lines ~394-422 | exact |
| `cmd/engram/client_common.go` — footer helper reuse | utility (renderer) | transform | same file — `renderCoverageFooter`, lines ~291-320 | exact |
| `cmd/engram/operator_view.go` — D-11 headline sanitization fix | utility (renderer) | transform | same file — `sanitizeViewValue` (already used on field values), `renderOperatorView`'s headline line | exact (one-line change, function to route through already exists in-file) |
| `internal/surfaces/toolclass.go` — `get_memory` row's `CLICommand` + new migration-verb row | config (registry) | — | same file — the `operations` table itself; `get_memory`'s row **already exists** (see note below) | exact |
| `ui/src/lib/queries.ts` — `ObserveParams`/`parseObserveParams`/`observeSearch`/`listMemoriesKey` +3 flags | store/state (URL-synced filter state) | transform | same file — existing `categories`/`visibility`/`offset` round-trip, lines 1-37 (full file) | exact |
| `ui/src/lib/components/ScopesSidebar.svelte` +3 checkboxes | component | event-driven | same file — existing `Checkbox`/`toggleCat` category-filter block, lines 33-38 (full file, 41 lines) | exact |
| `ui/src/lib/components/MemoryRow.svelte` +state marker/dimming | component | transform | same file — `isRule`/`isShared` derived-flag pattern, lines 40-44 (full file, 94 lines) | exact |
| `ui/src/lib/components/MemoryDetail.svelte` +State section, +schema chip | component | transform | same file — the `by`/`src`/`vis` meta-chip row (lines 122-127) for the chip; the `Tabs.Content` block shape (lines 94-132) for the new section | exact |
| `ui/src/lib/components/AppShell.svelte` +migration banner | component | event-driven (mount fetch) | same file (full file, 43 lines) — no fetch precedent in-component today; nearest sibling pattern is `observe/+page.svelte`'s `createQuery` mount-time fetch | role-match (component itself has no fetch precedent; query pattern borrowed from a sibling file) |
| `ui/src/routes/observe/+page.svelte` — pass 3 new params through | route/page (query wiring) | request-response | same file — existing `createQuery(() => ({...}))` options-function idiom | exact |
| `internal/store/store_test.go` — `TestSupersedeRecallGate`/Multi new sub-cases | test | CRUD (behavioral) | same file, `TestSupersedeRecallGate` at line ~3195 | exact |
| Go-side D-13 state-word derivation + test | utility + test | transform | **no analog — see "No Analog Found" below** | none |
| TS-side D-13 state-word derivation + `.browser.test.ts` | component (derived) + test | transform | `MemoryRow.svelte`'s `isRule`/`isShared`/`isAuto` `$derived` pattern (same-surface precedent only; no cross-surface pair — see below) | partial |
| `internal/server/*_test.go` — new `MigrateStatus` RPC test | test | request-response | existing Connect handler tests for `ListScopes`/other read RPCs in the same test file family | role-match |

## Pattern Assignments

### `internal/store/store.go` (store, filtered-read CRUD)

**Analog:** same file — the `tags`/`created_after`/`created_before` filter-threading pattern already applied to `SearchOptions`/`ListOptions`.

**Options-struct extension pattern** (current, before this phase's edit):
```go
// store.go:1049-1059 — SearchOptions (add IncludeArchived/IncludeSuperseded/IncludeScheduled bool here)
type SearchOptions struct {
	Tags []string
	Categories []string
	CreatedAfter, CreatedBefore time.Time
}
```
```go
// store.go:~1210-1231 — ListOptions (same three fields to add)
type ListOptions struct {
	Limit, Offset uint64
	Categories []string
	Visibility string
	Tags []string
	CreatedAfter, CreatedBefore time.Time
	Cursor string
	CursorMode bool
	Ascending bool
}
```

**The four gate sites (exact confirmed line numbers, matches RESEARCH.md's citations exactly):**
```go
// store.go:1086-1097 — Store.Search (IN SCOPE — gate conditionally on the new opts fields)
f := s.ownerScopeFilter(ctx, scope, subj)
f.Must = append(f.Must, activeWindowConditions(s.now())...)   // line 1086 — gate on !opts.IncludeScheduled (BOTH bounds, Pitfall 3)
f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))   // line 1091 — gate on !opts.IncludeSuperseded
f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))     // line 1097 — gate on !opts.IncludeArchived
```
```go
// store.go:1191/1195 — listFilter's SearchDiscovery-shared idiom — OUT OF SCOPE for this phase (D-01
// names only Search/List). Label this site clearly as deliberately untouched, not overlooked.
must = append(must, qdrant.NewIsEmpty("superseded_by")) // :1191
must = append(must, qdrant.NewIsEmpty("archived_at"))   // :1195
```
```go
// store.go:1328/1333 — Store.List (IN SCOPE — identical shape to Search's gate)
f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by")) // :1328
f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))   // :1333
```
```go
// store.go:1563/1568 — ListScheduled's inline filter — OUT OF SCOPE for this phase (same idiom,
// D-01 explicitly does not name it; builds its filter inline, not via listFilter, so it will not
// accidentally inherit ListOptions' new fields unless someone wires it — do not wire it).
qdrant.NewIsEmpty("superseded_by"), // :1563
qdrant.NewIsEmpty("archived_at"),   // :1568
```

`activeWindowConditions` is at `store.go:1006` — a single function returning **two** `Should`-wrapped
conditions (one per window bound). `IncludeScheduled` must gate the entire call as one unit — see
RESEARCH.md Pitfall 3 for the "expired record reachable but unmarked" failure mode if only one bound
is relaxed.

**Error handling:** none needed — these are plain `bool` struct fields with no invalid values.

---

### `internal/server/connectapi.go` — new `MigrateStatus` RPC handler (controller, request-response)

**Analog:** `ListScopes`, same file, lines 170-184 — the simplest existing handler: no request
fields, subject-only auth check, no owner filter passed to the store call (matches D-06's
any-authenticated-caller, whole-collection semantics exactly).

**Full analog, verbatim:**
```go
// connectapi.go:170-184
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
`MigrateStatus` follows this shape exactly, substituting `a.d.st.MigrateStatus(ctx)` (no `subj`
passed — D-06 is whole-collection, no owner filter) for `a.d.st.ListScopes(ctx, subj)`, and mapping
`MigrateStatusResult`'s buckets into the response message. Error path: `connectError(ctx, err)`,
same as every other handler — **not** a hand-rolled `connect.NewError(connect.CodeInternal, ...)`
(RESEARCH.md Pitfall 6).

**`ListMemories`/`SearchMemories` field-threading pattern** (the three new bools' actual wiring
point — same handlers, existing precedent for reading a Connect request bool/field into the typed
core):
```go
// connectapi.go:198+ — ListMemories already reads req.Msg.CreatedAfter/CreatedBefore, parses them,
// and threads req.Msg.CursorMode/Offset through a relational validation before calling deps.listMemory.
// req.Msg.IncludeArchived/IncludeSuperseded/IncludeScheduled thread the same way, straight into
// coreListRequest / store.ListOptions — no parsing needed, they're plain bools.
```

---

### `internal/server/tools.go` — `coreListRequest`/`coreSearchRequest` (typed core, D-03 enforcement point)

**Analog:** same file, the existing `Tags`/`CreatedAfter`/`CreatedBefore` fields on these two
structs (lines ~1394-1445). Add the three new bool fields the same way. The two MCP `mcp.AddTool`
closures (`tools.go:2385`, `:2432`) build these structs as literals that simply never mention the
new fields — D-03 ("Connect+CLI only, MCP unchanged") is satisfied structurally with **zero code
change** at the MCP call sites, since the zero value of an unset bool field reproduces today's
gated behavior.

---

### `cmd/engram/client_get.go` (new — CLI verb, request-response)

**Analog:** `cmd/engram/client_list.go`, full file (imports + flag vars + `RunE` skeleton).

**Imports pattern** (verbatim, `client_list.go:1-16`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/surfaces"
)
```

**Core CLI pattern** (`client_list.go:32-50`, the dial/deadline sequence `engram get` must reuse
exactly):
```go
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List memories on a remote engram server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireScopeUnlessCrossSpine(listScope, listCrossSpine); err != nil {
			return err
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		// ...
```
(`engram get` has no `--scope` precondition — `GetMemory` is ungated by id — so it skips the
`requireScopeUnlessCrossSpine` guard and goes straight to `clientFromFlags`.)

**The D-10 adapter** — this is the one genuinely new piece, and it is one line, confirmed against
two files:
```go
// operator_view.go:45-49 — viewFields(doc any) calls json.Marshal(doc); encoding/json returns a
// json.RawMessage's bytes VERBATIM from MarshalJSON(). Wrapping already-marshaled protojson bytes
// in json.RawMessage before passing to renderOperatorView reuses the whole Phase 6 pipeline with
// no new serialization code and preserves protojson's Timestamp/optional semantics.
b, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(resp.Msg.GetMemory())
if err != nil {
	return err
}
return renderOperatorView(cmd.OutOrStdout(), headline, json.RawMessage(b))
```
The `protojson.MarshalOptions{...}` literal is copied from `renderJSON`'s existing options at
`client_common.go:380-391` — same options, different destination (marshal-to-bytes-then-wrap vs.
marshal-and-write-directly).

**Registration:**
```go
func init() {
	addClientFlags(getCmd) // client_common.go:42-55 — REQUIRED, not addOperatorOutputFlag (Pitfall 2)
	rootCmd.AddCommand(getCmd)
}
```

**Toolclass registry — IMPORTANT, this is not a new row:** `internal/surfaces/toolclass.go:91-94`
**already has** an `Operation` row for `MCPTool: "get_memory", CLICommand: ""`. `engram get` should
**fill in** `CLICommand: "get"` on this existing row, not add a second row for the same
capability — adding a duplicate row for the same `MCPTool` would itself be a new defect class the
catalog gate does not currently guard against (two rows claiming the same tool). Verbatim current
row:
```go
// toolclass.go:91-94
{
	MCPTool: "get_memory", CLICommand: "",
	Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
},
```

---

### `cmd/engram/client_<migration-verb>.go` (new — CLI verb, request-response)

**Two analogs, blended, per RESEARCH.md's own framing:**

1. **Report-shape analog:** `cmd/engram/migrate_family.go`'s `migrateStatusCmd`/
   `migrateStatusReportDoc` (targeted read: lines ~260-360) — this is the operator-tier sibling
   (`engram migrate status`, direct Qdrant access via `StoreFromEnv`) that already renders
   `Store.MigrateStatus`'s histogram as an operator report doc. Its `statusReportDoc`/`statusSummary`
   nil-to-empty-slice normalization discipline (so json/proto never emit `null` for an empty bucket
   list) is the exact discipline the new RPC's response mapping needs too (RESEARCH.md's
   `MigrateStatus` handler skeleton cites this explicitly).
2. **Client-tier skeleton analog:** `client_list.go` (same as `engram get`'s analog) — dial via
   `clientFromFlags`, `addClientFlags` registration, `wrapRPCError` on failure.

**Existing toolclass row for the operator-tier sibling** (do NOT reuse this row — the new verb is a
**second**, client-tier row, distinct `CLICommand` string):
```go
// toolclass.go:262-266 (operator-tier `migrate status` — leave unchanged)
{
	MCPTool: "", CLICommand: "migrate status",
	Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
},
```
The new client-tier verb needs its **own** new row, `Class{ReadOnly: true, Destructive: false,
Idempotent: true, OpenWorld: false}` (identical classification, different `CLICommand` string —
resolve the exact name against the live registry per CONTEXT.md's Claude's-Discretion note).

---

### `cmd/engram/client_common.go` — `renderMemoryTable` STATE column (utility, transform)

**Analog:** same file — `withScore`'s existing conditional-column pattern, the direct precedent for
varying `renderMemoryTable`'s columns (even though D-12 locks an *unconditional* STATE column,
unlike the conditional `withScore`):
```go
// client_common.go:396-422 (current, before this phase's edit)
func renderMemoryTable(w io.Writer, mems []*engramv1.Memory, withScore bool) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	...
	if withScore {
		writeLine("SHORT_ID\tSCOPE\tCATEGORY\tSCORE\tSUMMARY\n")
	} else {
		writeLine("SHORT_ID\tSCOPE\tCATEGORY\tSUMMARY\n")
	}
	for _, m := range mems {
		summary := truncateSummary(m.GetSummary(), 80)
		if withScore {
			writeLine("%s\t%s\t%s\t%.4f\t%s\n", m.GetShortId(), m.GetScope(), m.GetCategory(), m.GetScore(), summary)
		} else {
			writeLine("%s\t%s\t%s\t%s\n", m.GetShortId(), m.GetScope(), m.GetCategory(), summary)
		}
	}
	...
}
```
**Important distinction to carry into the plan:** `renderMemoryTable` uses `text/tabwriter` and
that stays unchanged (D-12 only adds a column here) — the *separate*, unrelated rejection of
`text/tabwriter` documented in RESEARCH.md applies only to `renderOperatorView`'s field-table
renderer (`operator_view.go`), which uses manual `utf8.RuneCountInString` padding instead. Do not
conflate the two renderers' formatting mechanisms.

**Footer helper reuse — `renderCoverageFooter`** (`client_common.go:291-320` region, full function
verbatim):
```go
func renderCoverageFooter(w io.Writer, crossSpine bool, searchedScopes []string, scopesTruncated bool) error {
	if !crossSpine {
		return nil
	}
	if scopesTruncated {
		_, err := fmt.Fprintf(w, "searched_scopes: %d  scopes_truncated: true\n", len(searchedScopes))
		return err
	}
	_, err := fmt.Fprintf(w, "searched_scopes: %d\n", len(searchedScopes))
	return err
}
```
D-08's migration-backlog advisory footer follows this exact `key: value`, two-space-joined shape
("so the two footer lines already precedented in this codebase read as one family" — RESEARCH.md's
own stated design constraint) — copy the `fmt.Fprintf` shape, not just the gating idiom.

---

### `cmd/engram/operator_view.go` — D-11 headline sanitization fix (utility, transform)

**Analog:** same file — `sanitizeViewValue` already exists and is already used on every field
value; the fix routes the headline through the **same** function, not a new one.

```go
// operator_view.go:223-234 — sanitizeViewValue, already in use on field values (verbatim)
func sanitizeViewValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```
```go
// operator_view.go — renderOperatorView's headline line (current):
func renderOperatorView(w io.Writer, headline string, doc any) error {
	if _, err := fmt.Fprintln(w, headline); err != nil { // <- change to sanitizeViewValue(headline)
		return err
	}
	...
```
One-line change: `fmt.Fprintln(w, headline)` → `fmt.Fprintln(w, sanitizeViewValue(headline))`.

---

### `ui/src/lib/queries.ts` — `ObserveParams` +3 include flags (state, transform)

**Analog:** same file, full file (37 lines) — the existing `categories`/`visibility`/`offset`
round-trip is the literal template for `includeArchived`/`includeSuperseded`/`includeScheduled`.

```typescript
// queries.ts:9-36 (current, full file)
export interface ObserveParams {
  scope: string; categories: Category[]; visibility: Visibility; offset: number; selectedId: string;
}

export function parseObserveParams(sp: URLSearchParams): ObserveParams {
  const vis = sp.get('vis') ?? '';
  return {
    scope: sp.get('scope') ?? '',
    categories: sp.getAll('cat').filter((c): c is Category => (CATEGORIES as readonly string[]).includes(c)),
    visibility: (VISIBILITIES as readonly string[]).includes(vis) ? (vis as Visibility) : '',
    offset: Number(sp.get('offset') ?? '0') || 0,
    selectedId: sp.get('sel') ?? ''
  };
}

export function observeSearch(p: ObserveParams): string {
  const sp = new URLSearchParams();
  if (p.scope) sp.set('scope', p.scope);
  for (const c of p.categories) sp.append('cat', c);
  if (p.visibility) sp.set('vis', p.visibility);
  if (p.offset) sp.set('offset', String(p.offset));
  if (p.selectedId) sp.set('sel', p.selectedId);
  return sp.toString();
}

export function listMemoriesKey(scope: string, categories: string[], visibility: string, limit: number, offset: number) {
  return ['listMemories', scope, categories, visibility, limit, offset];
}
```
Each new flag needs: a field on `ObserveParams`, a `sp.get('inc_archived') === '1'`-shaped parse in
`parseObserveParams` (boolean URL params in this codebase are the `vis`/absent-vs-present idiom,
not yet precedented for a bare bool — the plan should pick one short param name per flag, e.g.
`arc`/`sup`/`sch`, mirroring `cat`/`vis`'s brevity), a conditional `sp.set(...)` in `observeSearch`
(only when true — omit-when-false keeps the URL clean, matching every existing field's omit rule),
and an appended positional arg to `listMemoriesKey`.

---

### `ui/src/lib/components/ScopesSidebar.svelte` — 3 include toggles (component, event-driven)

**Analog:** same file, full file (41 lines) — the existing category `Checkbox` block is the direct
template.

```svelte
<!-- ScopesSidebar.svelte:33-38 (current, full block) -->
<div class="mt-3 text-[10px] uppercase text-muted-foreground">Filters</div>
{#each allCats as c (c)}
  <label class="flex items-center gap-2 text-sm" style="color:var(--cat-{c})">
    <Checkbox checked={categories.includes(c)} onCheckedChange={() => toggleCat(c)} aria-label={c} />{c}
  </label>
{/each}
```
The three include toggles are a new sibling block using the same `<Checkbox>`/`aria-label` shape,
wired to new `onfilter`-equivalent props (or an extension of the existing `onfilter` callback's
signature) rather than a new prop-passing mechanism.

---

### `ui/src/lib/components/MemoryRow.svelte` — state marker + dimming (component, transform)

**Analog:** same file, full file (94 lines) — the `isRule`/`isShared`/`isAuto` `$derived`-flag
pattern is the direct template for the four new `isArchived`/`isSuperseded`/`isExpired`/
`isScheduled` derivations.

```svelte
<!-- MemoryRow.svelte:36,42-44 (current, verbatim) -->
const isAuto = $derived(memory.summarySource === 'auto');
...
const isRule = $derived(memory.category === 'rule');
const isShared = $derived(memory.visibility === 'shared');
const showMenu = $derived(!isRule && !!(onedit || ondelete || (onshare && !isShared)));
```
```svelte
<!-- MemoryRow.svelte:61-68 (current meta line — the state marker joins this row) -->
<div class="flex items-center gap-2 text-[11px] text-muted-foreground min-w-0">
  <span class="font-medium shrink-0" style="color:var(--c)">{memory.category}</span>
  <span class="shrink-0">·</span>
  <span class="tabular-nums shrink-0">{when}</span>
  {#if showScope && memory.scope}<span class="shrink-0"><ScopeChip scope={memory.scope} /></span>{/if}
  {#each shownTags as t (t)}<span class="shrink-0 px-1 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
  {#if overflow > 0}<span class="shrink-0">+{overflow}</span>{/if}
</div>
```
The state marker is a new conditionally-rendered `<span>` in this same meta line (an `aria-label`
carrying the word to a screen reader, per D-15's constraint that the marker — not the dimming — is
the accessible signal); dimming is a class applied to the outer `<div>` at line 47-50, layered on
top of the existing `selected ? 'bg-accent' : 'hover:bg-accent'` conditional class string.

---

### `ui/src/lib/components/MemoryDetail.svelte` — State section + schema chip (component, transform)

**Analog:** same file, full file (154 lines).

**Chip-row precedent** for `schema_version`'s always-present 4th chip (`MemoryDetail.svelte:122-127`):
```svelte
<div class="flex gap-1.5 flex-wrap text-[10.5px]">
  <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">by</span> {memory.actor}</span>
  <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">src</span> {memory.source}</span>
  <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">vis</span> {memory.visibility}</span>
</div>
```
A 4th `<span>` (`schema` / `memory.schemaVersion`) joins this row — same border/padding classes,
same `<span class="text-muted-foreground">label</span> value` shape.

**Conditional-section precedent** for the State section (`MemoryDetail.svelte:101-115`, the
`Summary` tab's `{#if hasSummary}...{:else}...{/if}` shape is the closest same-file precedent for
"render a labeled block only when the record has non-default content"):
```svelte
<Tabs.Content value="summary" class="p-3 min-h-0">
  {#if hasSummary}
    <div class="flex items-center justify-between mb-2">
      <span class="text-[9.5px] uppercase tracking-wide text-muted-foreground font-semibold">Summary</span>
      ...
    </div>
    <div class="text-[13.5px] leading-relaxed">{memory.summary}</div>
  {:else}
    <div class="text-[12px] text-muted-foreground">No summary — see Content.</div>
  {/if}
</Tabs.Content>
```
D-14's State section follows the same `{#if <non-default-state derived>}` gate, rendering
`superseded_by`/`supersedes` as links (new — no existing cross-record-link precedent in this
component; a plain `<a href>` to the same route with `?sel=<id>` is the natural fit, matching
`observeSearch`'s own `sel` param), `not_before`/`not_after` via `fullTimestamp`/`relativeTime`
(already imported, line 11), and `archived_at` the same way.

---

### `ui/src/lib/components/AppShell.svelte` — migration banner (component, event-driven mount fetch)

**No in-component fetch precedent** — `AppShell.svelte` (43 lines, full file read) has zero data
fetching today; every fetch in this codebase happens in `observe/+page.svelte` via
`@tanstack/svelte-query`'s `createQuery(() => ({...}))` options-function idiom. The banner's
`MigrateStatus` fetch should follow that idiom (borrowed from a sibling route file, not from
`AppShell.svelte` itself), mounted once in `AppShell`'s `<script>` block since the banner must
render "on every route" (D-07) and `AppShell` is the one component every route shares (confirmed:
`nav` array in this file lists Observe/Search/Discovery, all children of this shell).

**Insertion point** (`AppShell.svelte:21-28`, header region — a conditional `<div>` strip goes
between the `<header>` and the `<div class="flex flex-1 min-h-0">` row, matching D-07's "banner on
every route" placement):
```svelte
<div class="h-dvh flex flex-col overflow-hidden bg-background text-foreground">
  <header class="flex items-center gap-3 px-3 py-2 border-b border-border">
    ...
  </header>
  <!-- D-07 banner strip inserts here, conditional on the fetched histogram having >0 pending or future-version records -->
  <div class="flex flex-1 min-h-0">
    ...
```

---

### D-13 state-word derivation, Go side (utility, transform) — NEW, no existing analog

**No analog found in the codebase for a Go-side field-presence→label derivation function.** The
nearest same-*kind* precedent is structural, not textual: `store.go`'s own `IsEmpty` gate
conditions are presence checks over the same three fields, but they answer a filter question, not
a rendering question, and live in a different package. This is genuinely new code:

```go
// Confirmed field types: gen/go/engram/v1/engram.pb.go — Memory.SupersededBy *string,
// Memory.NotBefore/NotAfter/ArchivedAt *timestamppb.Timestamp (pointer/message-typed, nilable).
m.GetArchivedAt() != nil       // -> archived
m.GetSupersededBy() != ""      // -> superseded (pointer-check preferred per RESEARCH.md Pitfall 5)
m.GetNotAfter() != nil && m.GetNotAfter().AsTime().Before(now)  // -> expired
m.GetNotBefore() != nil && m.GetNotBefore().AsTime().After(now) // -> scheduled
```
Per D-13, this must be the **only** place the Go side computes the word (no shared constant with
the TS side), and needs its own new unit test asserting the derivation (RESEARCH.md's Wave-0 gap
list names this explicitly: no existing test covers it).

### D-13 state-word derivation, TS side (component, transform) — partial analog only

**Partial analog:** `MemoryRow.svelte`'s `isRule`/`isShared`/`isAuto` `$derived` pattern (cited
above) is the *shape* to follow — a same-surface precedent for "derive a boolean/label from a
Memory field in a component's `<script>` block" — but there is **no existing Go+TS pair that
implements one rule twice and tests it on both sides** (the requirement D-13 states). Searched:
`isRule`/`isShared` have no server-side or CLI-side Go counterpart at all (category/visibility
checks in Go are authz/store logic, not a rendering-label derivation). **State this plainly rather
than inventing a false analog, per this phase's own instruction.** The TS derivation is new code,
following the `$derived` shape shown above (RESEARCH.md's Code Examples section already has the
confirmed field-typed version for `ui/src/lib/gen/engram/v1/engram_pb.ts:153-190`).

---

## Shared Patterns

### Options-struct extension (store layer)
**Source:** `internal/store/store.go` — `SearchOptions`/`ListOptions`, the `Tags`/`CreatedAfter`/
`CreatedBefore` precedent.
**Apply to:** `internal/store/store.go` (the three new bool fields + four gate-site edits).
Zero-value = today's behavior; no positional-parameter break; no call-site change required for any
existing caller.

### Connect-only exposure via the typed-core seam (D-03)
**Source:** `internal/server/tools.go` — `coreListRequest`/`coreSearchRequest`.
**Apply to:** `internal/server/connectapi.go` (populates the new fields from `req.Msg.IncludeX`),
`internal/server/tools.go` (MCP call sites at `:2385`/`:2432` — leave unmodified, they simply never
set the new fields).

### `connectError` as the single error mapper
**Source:** `internal/server/connectapi.go` — every existing handler, `ListScopes` at lines 170-184
as the simplest example.
**Apply to:** the new `MigrateStatus` handler — no bespoke `connect.NewError` calls (RESEARCH.md
Pitfall 6).

### `addClientFlags` for client-tier CLI commands
**Source:** `cmd/engram/client_common.go:42-55`.
**Apply to:** `cmd/engram/client_get.go`, `cmd/engram/client_<migration-verb>.go` — both MUST call
this, not `addOperatorOutputFlag`, or `operatorCommands()`'s structural `"server"`-flag filter
(`cmdwalk.go:107-109`) misclassifies them (RESEARCH.md Pitfall 2).

### `internal/surfaces/toolclass.go` registry row requirement
**Source:** same file, `operations` slice — every CLI command needs exactly one row or
`buildCatalog` (`cmd/engram/catalog.go:98-107`) panics at test time (`TestCatalogBlastRadiusMatchesToolClasses`).
**Apply to:** `engram get` (fill the **existing** `get_memory` row's `CLICommand` field — do not
duplicate), the new migration-status verb (genuinely new row, distinct from the existing
`migrate status` operator-tier row).

### `json.RawMessage` as the D-10 protobuf→operator-view adapter
**Source:** `cmd/engram/operator_view.go:45-49` (`viewFields`), `cmd/engram/client_common.go:380-391`
(`renderJSON`'s `protojson.MarshalOptions`).
**Apply to:** `cmd/engram/client_get.go` only — the one place this phase renders a single protobuf
record through the field-table view.

### `sanitizeViewValue` for all record-derived text reaching an operator report
**Source:** `cmd/engram/operator_view.go:223-234`.
**Apply to:** `renderOperatorView`'s headline (D-11 fix) — the same function already used for field
values, now also routed through for the headline line.

### URL-synced filter state
**Source:** `ui/src/lib/queries.ts` — `ObserveParams`/`parseObserveParams`/`observeSearch`/
`listMemoriesKey`.
**Apply to:** `ui/src/lib/components/ScopesSidebar.svelte` (3 new checkboxes), `ui/src/routes/observe/+page.svelte`
(passes the 3 new params into the `listMemories`/`searchMemories` query calls and into
`listMemoriesKey`).

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| Go-side D-13 state-word derivation function + its unit test | utility + test | transform | No existing Go code derives a display label from `Memory` field presence for rendering purposes (store-layer `IsEmpty` conditions are filter logic, a different concern, in a different package). Genuinely new; RESEARCH.md's own Wave-0 gap list confirms no test exists today. |
| A cross-surface (Go+TS) precedent for "one rule implemented twice, tested on both sides" (the property D-13 requires) | — | — | Searched `isRule`/`isShared`/category-derived logic on both the Go and TS sides; no paired implementation exists anywhere in the codebase today. State this plainly per the phase-context instruction rather than inventing a false analog — the planner should treat both derivations as new code following each surface's own idiomatic shape (Go: plain functions/methods; TS: `$derived` in the consuming component(s)), with two new tests, not one shared mechanism (D-13 explicitly rejects a shared string table). |

## Metadata

**Analog search scope:** `internal/store/`, `internal/server/`, `cmd/engram/`, `internal/surfaces/`,
`proto/engram/v1/`, `ui/src/lib/components/`, `ui/src/lib/`, `ui/src/routes/observe/`.
**Files scanned:** ~20 read in full or targeted detail this session (store.go, connectapi.go,
tools.go targeted, client_common.go, client_list.go, operator_view.go, toolclass.go, cmdwalk.go,
migrate_family.go targeted, queries.ts, MemoryRow.svelte, MemoryDetail.svelte, ScopesSidebar.svelte,
AppShell.svelte, MemoryRow.browser.test.ts, plus line-number verification greps across store.go and
connectapi.go).
**Pattern extraction date:** 2026-08-20
