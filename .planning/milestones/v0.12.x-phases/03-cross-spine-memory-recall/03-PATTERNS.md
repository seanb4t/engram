# Phase 3: Cross-Spine Memory Recall - Pattern Map

**Mapped:** 2026-08-01
**Files analyzed:** 8 (all modified, zero new files)
**Analogs found:** 8 / 8 — this phase is unusually analog-rich; every mechanism has an
already-shipped sibling (`search_discovery`'s cross-spine implementation).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/store.go` (`ownerScopeFilter`, L752-757) | service (filter builder) | CRUD (read filter) | `Store.SearchDiscovery`'s inline `must` build, L977-987 | exact — same function family, same file |
| `internal/store/store.go` (`listFilter`, L1054-1077) | service (filter builder) | CRUD (read filter) | `Store.SearchDiscovery`'s inline `must` build, L977-987 | exact |
| `internal/server/tools.go` (`searchArgs`, L534-543; `listArgs`, L545-554) | request-schema struct | request-response | `searchDiscoveryArgs`, L623-629 | exact — same struct family in same file |
| `internal/server/tools.go` (new `effectiveSearchScope`) | utility (guard) | request-response | `effectiveDiscoveryScope`, L1129-1140 | exact |
| `internal/server/tools.go` (`coreSearchRequest` L1048-1056, `coreListRequest` L1018-1029) | service (transport-neutral core request) | request-response | same file, no direct discovery analog (discovery search has no typed-core struct) — closest is the struct's own existing shape plus `searchDiscoveryArgs` field-naming convention | role-match |
| `internal/server/tools.go` (`search_memory`/`list_memory` MCP closures, L1443-1515) | controller (MCP tool handler) | request-response | `deps.searchDiscovery`, L1142-1165 | exact |
| `internal/server/connectapi.go` (`SearchMemories` L194-221, `ListMemories` L142-187) | controller (Connect RPC handler) | request-response | `SearchDiscoveries`, L253-272 | exact (but see D-04 divergence below — do NOT copy the empty-scope inference) |
| `proto/engram/v1/engram.proto` (`SearchMemoriesRequest/Response`, `ListMemoriesRequest/Response`) | config/schema | request-response | `SearchDiscoveriesRequest` (no `cross_spine` field — inferred) is the anti-pattern; the additive-field-numbering convention itself is the analog | role-match |
| `internal/store/store_test.go` (new `TestCrossSpineAuthzIsolation`) | test | CRUD (integration) | `TestBulkFilterZeroBucketFailsClosed`/`TestBulkFilterOrderIndependent`, L4314/4355; two-owner seeding from `TestSearchDiscoveryOwnerIsolation` L371-415 / `TestSearchListOwnerIsolation` L675-713 | exact (technique) / partial (fixture — overlap requirement is new, see D-16) |
| `internal/server/tools_test.go` (new handler-level isolation test) | test | request-response | `TestSearchMemoryCategoriesArg`/`TestListMemoryCategoriesArg`, L2096-2165 | exact |

## Pattern Assignments

### `internal/store/store.go` — `ownerScopeFilter` (L752-757) and `listFilter` (L1054-1077)

**Analog:** `Store.SearchDiscovery`'s `must` construction, `internal/store/store.go:977-987`

**Current (pre-edit) code — both target functions:**
```go
// ownerScopeFilter, store.go:752-757
func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		s.ownerOrSharedCondition(subj),
	}}
}

// listFilter, store.go:1054-1058 (opening lines; opts-driven appends follow through L1077)
func (s *Store) listFilter(scope string, subj Subject, opts ListOptions) *qdrant.Filter {
	must := []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		s.ownerOrSharedCondition(subj),
	}
	// ... categoryMatchCondition, tagMatchConditions, visibility switch appended after, unchanged
```

**The exact target shape to mirror (already shipped, byte-for-byte pattern), `store.go:977-987`:**
```go
must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
if scope != "" {
	must = append(must, qdrant.NewMatch("scope", scope))
}
if kind != "" {
	must = append(must, qdrant.NewMatch("kind", kind))
}
must = append(must, s.ownerOrSharedCondition(subj))
must = append(must, qdrant.NewIsEmpty("superseded_by"))
```

**Apply as (D-05):**
```go
func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
	must := []*qdrant.Condition{}
	if scope != "" {
		must = append(must, qdrant.NewMatch("scope", scope))
	}
	must = append(must, s.ownerOrSharedCondition(subj))
	return &qdrant.Filter{Must: must}
}
```
and the identical `if scope != "" { ... }` wrap as the first two lines of `listFilter`, leaving every
append after it (categories, tags, visibility switch) untouched.

**Already-live proof this exact composition (authz-only, no scope element at all) is valid in
production** — `Store.ListScopes`, `store.go:1396-1401`:
```go
Filter: &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}},
```

**Single production call sites (safe-to-edit-in-place justification, D-05):** `ownerScopeFilter` is
called only from `Store.Search` (`store.go:888`); `listFilter` only from `Store.List` (`store.go:1113`).

---

### `internal/server/tools.go` — `searchArgs` (L534-543) / `listArgs` (L545-554)

**Analog:** `searchDiscoveryArgs`, `tools.go:623-629`

```go
type searchDiscoveryArgs struct {
	Query      string `json:"query"`
	Scope      string `json:"scope,omitempty" jsonschema:"required unless cross_spine"`
	Kind       string `json:"kind,omitempty" jsonschema:"map|fact filter"`
	K          uint64 `json:"k,omitempty"`
	CrossSpine bool   `json:"cross_spine,omitempty" jsonschema:"span all discovery scopes (ignores scope)"`
}
```

Apply to `searchArgs`/`listArgs`: change `Scope string \`json:"scope"\`` → `Scope string
\`json:"scope,omitempty" jsonschema:"required unless cross_spine"\``, and add `CrossSpine bool
\`json:"cross_spine,omitempty" jsonschema:"span all readable scopes (ignores scope)"\`` — reuse the
exact jsonschema wording style, tailored from "discovery scopes" to "readable scopes" per CONTEXT.md's
own wording note in the Docs Surfaces table.

---

### `internal/server/tools.go` — new `effectiveSearchScope`

**Analog:** `effectiveDiscoveryScope`, `tools.go:1129-1140`

```go
// effectiveDiscoveryScope resolves the scope filter for a discovery search:
// "" means span all discovery scopes (cross_spine). cross_spine ignores any
// supplied scope; otherwise a scope is mandatory.
func effectiveDiscoveryScope(a searchDiscoveryArgs) (string, error) {
	if a.CrossSpine {
		return "", nil
	}
	if a.Scope == "" {
		return "", fmt.Errorf("scope is required unless cross_spine is true")
	}
	return a.Scope, nil
}
```

Mirror byte-for-byte for `searchArgs`/`listArgs` (whether one shared helper taking an interface/two
fields, or two near-identical functions, is Claude's Discretion per CONTEXT.md).

---

### `internal/server/tools.go` — `deps.searchDiscovery` handler shape (L1142-1165)

**Analog for the MCP `search_memory`/`list_memory` closures' new guard + ignore-scope-and-log branch:**

```go
func (d *deps) searchDiscovery(ctx context.Context, c caller, a searchDiscoveryArgs) ([]store.Memory, error) {
	scope, err := effectiveDiscoveryScope(a)
	if err != nil {
		return nil, err
	}
	if a.CrossSpine && a.Scope != "" {
		// Don't echo the caller-supplied scope value into logs (avoids
		// unbounded/sensitive scope strings reaching log aggregation).
		slog.InfoContext(ctx, "search_discovery: cross_spine=true; ignoring supplied scope")
	}
	if a.K == 0 {
		a.K = 8
	}
	vec, err := d.em.EmbedQuery(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	return d.st.SearchDiscovery(ctx, scope, a.Kind, c.Subj, vec, a.K)
}
```

**Key detail to copy exactly:** the log call names the tool and never interpolates `a.Scope` — copy
this no-value-echo discipline verbatim into the new `search_memory`/`list_memory` ignore branches (e.g.
`slog.InfoContext(ctx, "search_memory: cross_spine=true; ignoring supplied scope")`).

**Current `search_memory`/`list_memory` closures to extend** (`tools.go:1443-1515`) — both build a
`coreSearchRequest`/`coreListRequest` directly from `a.Scope` today with no guard; the new closures
insert `scope, err := effectiveSearchScope(a)` (returning early on error, same pattern as every other
validation in these closures — see the existing `parseRFC3339` error-return style at L1456-1463) before
constructing the core request, and pass `scope` (not `a.Scope`) into it.

---

### `internal/server/connectapi.go` — `SearchMemories` (L194-221) / `ListMemories` (L142-187)

**Analog:** `SearchDiscoveries`, `connectapi.go:253-272` — **but do NOT copy the empty-scope inference.**

```go
// SearchDiscoveries, connectapi.go:262-267 — THE ANTI-PATTERN FOR THIS PHASE
ms, err := a.d.searchDiscovery(ctx, c, searchDiscoveryArgs{
	Query:      req.Msg.Query,
	Scope:      req.Msg.Scope,
	K:          k,
	CrossSpine: req.Msg.Scope == "",   // <-- D-04: memories must NOT do this
})
```

Per D-04, `SearchMemories`/`ListMemories` must read a genuinely new `req.Msg.CrossSpine` proto field
explicitly:
```go
ms, err := a.d.searchMemory(ctx, c, coreSearchRequest{
	Scope: req.Msg.Scope, Query: req.Msg.Query, K: k, Tags: req.Msg.Tags,
	CreatedAfter: after, CreatedBefore: before, Categories: req.Msg.Categories,
	CrossSpine: req.Msg.CrossSpine, // new — never inferred from Scope==""
})
```
(exact field name/whether `coreSearchRequest` carries `CrossSpine` at all vs. resolving to `Scope=""`
before the call is Claude's Discretion — Research Question 6 / Assumption A1 recommends an explicit
field so the `ListScopes`-trigger and `searched_scopes` assembly have an unambiguous signal.)

Connect currently has **no** empty-scope-without-cross-spine guard at all (unlike the MCP lane, which
will gain `effectiveSearchScope`) — this phase must add one at the Connect handler boundary too, styled
like the existing `CodeInvalidArgument` fail-fast guards already in `ListMemories` (L159-161):
```go
if req.Msg.CursorMode && req.Msg.Offset > 0 {
	return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cursor_mode is mutually exclusive with offset"))
}
```
Copy this same `connect.NewError(connect.CodeInvalidArgument, ...)` shape for `scope is required unless
cross_spine is true`.

---

### `proto/engram/v1/engram.proto` — additive field growth

**Analog:** the `categories` field previously added to `SearchMemoriesRequest` as field 8 — precedent
for pure-additive growth under the `buf breaking --against main` FILE-mode gate.

**Exact next-free field numbers** [VERIFIED live, `proto/engram/v1/engram.proto:55-96`]:

| Message | Next field # | New field(s) |
|---|---|---|
| `SearchMemoriesRequest` | 9 | `bool cross_spine = 9;` |
| `SearchMemoriesResponse` | 2 | `repeated string searched_scopes = 2;` then `bool scopes_truncated = 3;` |
| `ListMemoriesRequest` | 12 | `bool cross_spine = 12;` |
| `ListMemoriesResponse` | 5 | `repeated string searched_scopes = 5;` then `bool scopes_truncated = 6;` |

Note: `ListMemoriesResponse.approximate = 3 [deprecated = true]` is still declared — field 3 remains
reserved; 5 is correctly the next free number, not 3. Regenerate via `task proto:gen` in the same
commit as the proto edit (CI's `buf` job checks `gen/` and `ui/src/lib/gen/` drift).

---

### Isolation test — `internal/store/store_test.go`

**Analog for technique:** `TestBulkFilterZeroBucketFailsClosed` / `TestBulkFilterOrderIndependent`
(`store_test.go:4314`, `4355`) — controlled-mutation authz-composition tests against real Qdrant.

**Analog for two-owner seeding shape:** `TestSearchDiscoveryOwnerIsolation` (`store_test.go:371-415`),
`TestSearchListOwnerIsolation` (`store_test.go:675-713`) — both use an `mk(id, owner, vis)` closure.
**Deviation required (D-16):** both existing analogs use DISTINCT scope names per owner; the new test
`TestCrossSpineAuthzIsolation` must use the SAME scope name for both owners (overlap is what makes a
dropped authz clause visible as leaked records rather than as an empty result).

**Do not drive the test through `Store.Search`/`Store.List` with `scope=""`** — that is the vacuous-
green trap (RESEARCH.md Research Question 1). Instead build the raw filter directly and run it via
`s.client.Scroll`, reusing the exact shape `Store.ListScopes` already runs in production
(`store.go:1396-1401`):
```go
Filter: &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}},
```
Full test body is worked out in RESEARCH.md Research Question 1 (ready to adapt, not just a sketch) —
seed two owners under one overlapping scope name (private + shared each), scroll with a `Must` slice
containing only `s.ownerOrSharedCondition(Authenticated("sub-A"))`, assert owner B's private record
never appears and owner A's + B's shared record do.

**RED-by-mutation (D-15):** delete `s.ownerOrSharedCondition(subj)` from the `Must` slice (or
temporarily neuter the production method), observe the test fail, restore. Record the transcript in the
plan's verification notes — this is the primary non-vacuous gate and must land BEFORE the `ownerScopeFilter`/`listFilter` edit (D-18).

---

### Handler-level isolation test — `internal/server/tools_test.go`

**Analog:** `TestSearchMemoryCategoriesArg` / `TestListMemoryCategoriesArg`, `tools_test.go:2096-2165`
— both call `d.searchMemory(ctx, c, coreSearchRequest{...})` / `d.listMemory(ctx, c, coreListRequest
{...})` directly, bypassing MCP tool registration but exercising the `deps` method (the handler-wiring
layer D-17 targets).

Fixture helpers to reuse (all in `tools_test.go`):
- `testDeps(t) *deps` (L294-298) / `testDepsWithStore(t)` (L304-326)
- `authedContext(t, sub) context.Context` (L405-424) — round-trips through `mcpauth.RequireBearerToken`
  with a stub verifier
- `callerFor(ctx, t) caller` (L436+)
- `cleanupErr(t, what, err)` (L392-397)

Two-caller pattern: `callerFor(authedContext(t, "sub-A"), t)` and `callerFor(authedContext(t, "sub-B"),
t)`, seed overlapping-scope records (same D-16 overlap requirement as the store-level test), then call
`d.searchMemory`/`d.listMemory` with `CrossSpine: true` on the request and assert no cross-owner leak.
This test necessarily lands AFTER the args/proto plumbing exists (D-18 wave ordering) — it is
defense-in-depth for Go-level wiring, not the primary gate.

## Shared Patterns

### Conditional-scope / unconditional-authz filter composition
**Source:** `internal/store/store.go:977-987` (`Store.SearchDiscovery`)
**Apply to:** `ownerScopeFilter` (L752-757), `listFilter` (L1054-1058)
```go
must := []*qdrant.Condition{ /* other unconditional elements */ }
if scope != "" {
	must = append(must, qdrant.NewMatch("scope", scope))
}
must = append(must, s.ownerOrSharedCondition(subj)) // ALWAYS appended, never inside the if
```

### Handler cross-spine guard ("scope required unless cross_spine")
**Source:** `internal/server/tools.go:1129-1140` (`effectiveDiscoveryScope`)
**Apply to:** new `effectiveSearchScope`, used by both `search_memory` and `list_memory` MCP closures
and the new Connect-side guard in `SearchMemories`/`ListMemories`
```go
func effectiveDiscoveryScope(a searchDiscoveryArgs) (string, error) {
	if a.CrossSpine {
		return "", nil
	}
	if a.Scope == "" {
		return "", fmt.Errorf("scope is required unless cross_spine is true")
	}
	return a.Scope, nil
}
```

### No-value-echo logging discipline on the ignore-scope-and-log branch
**Source:** `internal/server/tools.go:1147-1151`
**Apply to:** new `search_memory`/`list_memory` MCP closures' cross-spine branch
```go
if a.CrossSpine && a.Scope != "" {
	slog.InfoContext(ctx, "search_discovery: cross_spine=true; ignoring supplied scope")
}
```
Never interpolate the caller-supplied scope string into the log line.

### Connect fail-fast `CodeInvalidArgument` guard shape
**Source:** `internal/server/connectapi.go:159-161` (`ListMemories`'s existing `cursor_mode`/`offset`
mutual-exclusion guard)
**Apply to:** new Connect-side empty-scope-without-cross-spine rejection in `SearchMemories`/
`ListMemories` (Connect currently has NO such guard — must be added, unlike the MCP lane which gets
`effectiveSearchScope`)
```go
if req.Msg.CursorMode && req.Msg.Offset > 0 {
	return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cursor_mode is mutually exclusive with offset"))
}
```

## Test Fixture Inventory — Every Site Constructing `searchArgs{`/`listArgs{`/`coreSearchRequest{`/`coreListRequest{`

Per engram gotcha `3q4cx33cta`: `go build ./...` does NOT compile `_test.go` files, so these sites are
invisible to a build-only check after a field addition. **Verified live via `rg -n
'searchArgs\{|listArgs\{|coreSearchRequest\{|coreListRequest\{' internal/server/*_test.go`.**

**Convention confirmed: every site below is a KEYED struct literal (field: value).** No positional/
unkeyed literals of any of the four types exist — Assumption A3 in RESEARCH.md is CONFIRMED, not just
assumed. Adding `CrossSpine bool` to any of the four structs is therefore safe against silent field-
shift; the only required action is `go vet ./...` (not `go build ./...`) as the compile gate, since a
keyed literal simply defaults the new field to its zero value (`false`) and compiles clean either way.

| File:Line | Struct | Literal style |
|---|---|---|
| `connectapi_test.go:424` | `coreSearchRequest{` | keyed |
| `connectapi_test.go:451,462,473,485` | `searchArgs{` | keyed |
| `connectapi_test.go:506` | `coreSearchRequest{` | keyed |
| `connectapi_test.go:1241` | `coreSearchRequest{` | keyed |
| `connectapi_test.go:1272` | `coreListRequest{` | keyed |
| `embed_wiring_test.go:48` | `coreSearchRequest{` | keyed |
| `tools_test.go:530,546` | `coreListRequest{` | keyed |
| `tools_test.go:904` | `coreSearchRequest{` | keyed |
| `tools_test.go:913` | `coreListRequest{` | keyed |
| `tools_test.go:1744` | `coreSearchRequest{` | keyed |
| `tools_test.go:1765` | `coreListRequest{` | keyed |
| `tools_test.go:2025,2041,2050` | `coreSearchRequest{` | keyed |
| `tools_test.go:2032,2057` | `coreListRequest{` | keyed |
| `tools_test.go:2111,2120` | `coreSearchRequest{` | keyed (this is `TestSearchMemoryCategoriesArg`'s own body — the D-17 test template site) |
| `tools_test.go:2149,2162` | `coreListRequest{` | keyed (`TestListMemoryCategoriesArg`) |
| `tools_test.go:2186` | `coreSearchRequest{` | keyed |
| `tools_test.go:2197` | `coreListRequest{` | keyed |
| `tools_test.go:2427` | `coreListRequest{` | keyed |
| `tools_test.go:2438` | `coreSearchRequest{` | keyed |
| `tools_test.go:3129` | `coreSearchRequest{` | keyed |
| `tools_test.go:3136` | `coreListRequest{` | keyed |
| `tools_test.go:3379,3391` | `coreListRequest{` | keyed |
| `tools_test.go:3445` | `coreListRequest{` | keyed |
| `tools_test.go:3478,3488` | `coreListRequest{` | keyed |
| `tools_test.go:3503` | `coreListRequest{` | keyed |
| `tools_test.go:3759` | `coreSearchRequest{` | keyed |
| `tools_test.go:3762` | `coreListRequest{` | keyed |

None of these sites require edits for the field addition itself (keyed literals tolerate new fields
silently); they matter only as candidates for a NEW test asserting the D-15/D-16 isolation property, and
as the template pool for the D-17 handler-level test (best templates: `tools_test.go:2111` and `:2149`,
`TestSearchMemoryCategoriesArg`/`TestListMemoryCategoriesArg`, since they already show the two-call,
same-scope, tag/category-filter pattern closest to what the isolation test needs).

## No Analog Found

None — every file in this phase's scope has a live, already-shipped analog (the phase's stated
character: reuse `search_discovery`'s already-proven cross-spine mechanics, byte-for-byte, for
`search_memory`/`list_memory`).

## Metadata

**Analog search scope:** `internal/store/store.go`, `internal/store/store_test.go`,
`internal/server/tools.go`, `internal/server/tools_test.go`, `internal/server/connectapi.go`,
`internal/server/connectapi_test.go`, `internal/server/embed_wiring_test.go`,
`proto/engram/v1/engram.proto`
**Files scanned:** 8 source/test files read live this session + RESEARCH.md's prior verified reads
**Pattern extraction date:** 2026-08-01
