# Phase 3: Cross-Spine Memory Recall - Research

**Researched:** 2026-08-01
**Domain:** Go/Qdrant authorization-filter composition, additive protobuf/Connect API design, real-Qdrant integration testing
**Confidence:** HIGH

## Summary

Criterion 1 (the authz gate) is CLOSED — `03-AUTHZ-GATE.md` is the written artifact and this research
does not re-derive it. This research does two things beyond that: (1) confirms the D-06 amendment by
reading `listFilter` live (`internal/store/store.go:1054-1058`) — it carries the identical two-element
`Must` shape the closed gate found in `ownerScopeFilter`; and (2) works out the concrete, non-vacuous
shape of the criterion-2 isolation test, which is the single highest-value output of this research.

The vacuous-green trap is real and mechanically simple to walk into: today `scope==""` passed to
`Store.Search`/`Store.List` means "match the literal empty-string scope payload," which matches
essentially nothing. A test that calls `Store.Search(ctx, "", ...)` before the D-05 filter edit lands
would report "owner B's private records never appeared" for the wrong reason — there were no results
at all. The concrete resolution (Research Question 1 below): do NOT drive the isolation test through
`Store.Search`/`Store.List` with an empty scope. Instead, construct the exact `*qdrant.Filter` shape
the post-edit cross-spine path will use — `Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}}`,
with no scope element at all — using the real, unexported, production `ownerOrSharedCondition` method
(the test file is `package store`), and execute it directly against real Qdrant via `s.client.Scroll`.
This is not a hypothetical composition — `Store.ListScopes` (`store.go:1396-1401`) already builds and
executes exactly this filter shape in production today, which is independent, already-tested proof
that the shape is valid and running. The new test seeds two owners under an OVERLAPPING scope name
(D-16) and asserts the scroll never returns owner B's private record — a test that is meaningful and
non-vacuous **today**, stays valid unchanged after the D-05 edit lands, and goes RED under the D-15
mutation (temporarily deleting `s.ownerOrSharedCondition(subj)` from the test's own `Must` slice, or
temporarily neutering the production method, either of which an empty/always-true filter would leak
owner B's private record through).

Research also surfaced one correction to a CONTEXT.md claim: D-12 asserts `Store.ListScopes` "uses the
same authz predicate" a cross-spine query runs under, and infers the returned scope set "*is* the set
that was searched." Reading `ListScopes` end to end shows this is true for the AUTHZ predicate only —
`ListScopes`'s filter is `ownerOrSharedCondition(subj)` alone, with NO `activeWindowConditions`, NO
`superseded_by` soft-hide, NO tags, NO categories. A scope whose only records are superseded, outside
their recall window, or excluded by a tag/category filter will still appear in `ListScopes`'s output
(counted, non-zero), even though a simultaneous cross-spine `Search`/`List` returns zero hits from that
scope. Criterion 5's framing — "found nothing here" vs. "searched everywhere and found nothing" — is
therefore slightly imprecise if read as "these are the scopes with recallable content": `searched_scopes`
reports the scopes you are AUTHORIZED to read, not the scopes with content the current query's other
filters would actually surface. This is a real product-facing caveat, not an authz hole (D-12's
authz-predicate match is exactly correct), and should be either accepted explicitly or worded in
generated docs as "the scopes searched under your authorization," not "the scopes with results."

**Primary recommendation:** Land the store-level isolation test (raw-filter construction against real
Qdrant, no scope conditional required) in its own commit first, run and record the D-15 mutation
transcript against it, then make the two-line D-05 filter edit, then the handler/proto plumbing —
exactly the D-18 ordering, now with the test's concrete implementation resolved.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Scope-conditional / authz filter composition | Database/Storage (Qdrant filter built in `internal/store`) | — | Authorization is enforced exclusively in `internal/store` via Qdrant read filters (standing invariant, ADR `engram-cdr1`); this phase edits `ownerScopeFilter`/`listFilter` only |
| Cross-spine opt-in guard (reject empty scope without the flag) | API/Backend (`internal/server` handler closures) | — | D-03/D-07: the handler is the SOLE guard: no store-level `CrossSpine` flag exists or is added |
| Searched-scope enumeration | Database/Storage (`Store.ListScopes`) | API/Backend (assembling the response field) | `ListScopes` already runs the authz-predicate-only scan (D-12); the transport layer only reads its result and maps it into the response |
| Result scope attribution | API/Backend (MCP `recallView`/proto `Memory.scope`) | — | Already-shipped field (D-11) — no store or wire change, only a test obligation |
| MCP↔Connect parity (additive proto fields) | API/Backend | — | `proto/engram/v1/engram.proto` + generated `gen/` tree; buf lint/breaking/drift are CI gates, not runtime concerns |
| Two-owner isolation proof | Database/Storage (real Qdrant via testcontainers) | API/Backend (handler-level D-17 test) | D-17: primary test exercises Qdrant's filter evaluation directly; the handler test only pins the Go-level wiring (D-03's guard, args plumbing) |

## Package Legitimacy Audit

Not applicable — this phase adds **zero new external packages**. It extends existing seams only:
`internal/store` (Qdrant Go client, already a dependency), `proto/`/`gen/` (buf + connect-go, already
generated), and `internal/server` (stdlib + existing MCP/Connect SDKs). Consistent with the milestone's
standing zero-new-dependency constraint (`ROADMAP.md`, `REQUIREMENTS.md` Out of Scope: "New Go
dependencies").

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-cross-spine-search | `cross_spine=true` on `search_memory` (and, per D-08, `list_memory`) spans every readable scope, MCP↔Connect parity via additive proto field | Research Questions 3, 6 — proto field numbers, call-site migration table |
| REQ-cross-spine-authz-verified | Cross-spine never widens the authz filter; pinned by a two-owner isolation test against real Qdrant, non-vacuous | Research Question 1 (primary output) — concrete test shape; D-06 amendment confirmed live |
| REQ-cross-spine-result-provenance | Every result attributable to its scope; response reports which scopes were searched | D-11 (already shipped, needs only a test) confirmed live; Research Question 4 — `ListScopes` divergence caveat |
</phase_requirements>

## D-06 Amendment — `listFilter` Read Live (confirms CONTEXT.md D-06)

`internal/store/store.go:1054-1058`:

```go
func (s *Store) listFilter(scope string, subj Subject, opts ListOptions) *qdrant.Filter {
	must := []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		s.ownerOrSharedCondition(subj),
	}
```

This is the identical two-element `Must` shape `03-AUTHZ-GATE.md` verified in `ownerScopeFilter`
(`store.go:752-757`): index 0 is the scope match, index 1 is `ownerOrSharedCondition(subj)`, separate
and unconditional. Everything else `listFilter` appends (`categoryMatchCondition`,
`tagMatchConditions`, the visibility switch at `store.go:1063-1075`) is appended AFTER these two
elements and never touches either. **[VERIFIED: internal/store/store.go:1054-1058]** — read this
session, quoted verbatim above. The gate's Evidence 1 and Evidence 2 arguments transfer to `listFilter`
without qualification. `03-AUTHZ-GATE.md` should record this confirmation (Claude's Discretion: new
section vs. inline note — either is fine; the content above is what to add).

## Research Question 1 — The Vacuous-Green Trap: Concrete Test Shape

### The trap, restated precisely

`03-AUTHZ-GATE.md:115-119` (already closed, quoted for context): a naive test calling
`Store.Search(ctx, "", ownerA, ...)` today returns zero hits because `ownerScopeFilter` unconditionally
emits `qdrant.NewMatch("scope", "")` **[VERIFIED: internal/store/store.go:752-757]**:

```go
func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		s.ownerOrSharedCondition(subj),
	}}
}
```

— with `scope=""`, element 0 requires the record's `scope` payload to literally equal the empty
string, which no real record has. Zero results is a certainty regardless of whether the authz clause
(element 1) works at all. A test asserting "owner B's private record never appeared" against this call
would pass for a reason unrelated to authorization.

### The resolution: build the filter directly, bypass `Store.Search`/`Store.List` entirely

The test does **not** call `Store.Search` or `Store.List` with an empty scope. It constructs, in
`internal/store/store_test.go` (`package store`, so the unexported method is reachable), the exact
`*qdrant.Filter` shape the D-05 edit will produce for a cross-spine query — a `Must` slice containing
**only** `s.ownerOrSharedCondition(subj)`, no scope element — and runs it directly against real Qdrant:

```go
// TestCrossSpineAuthzIsolation proves the authz composition Store.Search/List will
// rely on for cross_spine=true: a Must filter containing ONLY ownerOrSharedCondition,
// no scope element, run directly against real Qdrant. This is deliberately NOT
// Store.Search(ctx, "", ...) — today scope=="" means "literal empty-string scope
// payload" (matches nothing), so driving this through Store.Search would pass
// vacuously regardless of whether the authz clause works. This test instead
// exercises the exact filter Store.ListScopes (store.go:1396-1401) already builds
// and runs in production today, proving the shape is valid before Store.Search's
// D-05 conditional exists.
func TestCrossSpineAuthzIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:cross-spine-overlap" // SAME scope name for both owners (D-16)
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner, vis string) {
		m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	mk(idA, "sub-A", "")       // A private
	mk(idB, "sub-B", "")       // B private — must NEVER appear for A
	mk(idBShared, "sub-B", "shared")

	f := &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(Authenticated("sub-A"))}}
	pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection, Filter: f,
		Limit: qdrant.PtrOf(uint32(100)), WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("scroll: %v", err)
	}
	got := map[string]bool{}
	for _, p := range pts {
		got[p.Id.GetUuid()] = true
	}
	if got[idB] {
		t.Fatalf("cross-spine-shaped filter leaked owner B's private record: %v", got)
	}
	if !got[idA] || !got[idBShared] {
		t.Fatalf("cross-spine-shaped filter missing expected records: %v", got)
	}
}
```

Why this satisfies criterion 2's "exists and passes before the feature is implemented" without being
vacuous:

1. **It passes today, for the right reason.** It never touches `Store.Search`/`Store.List`'s
   `scope==""` semantics at all, so the D-05 edit is not a precondition. It exercises the REAL
   production `ownerOrSharedCondition` method against real Qdrant — the exact filter primitive the
   cross-spine feature will reuse unchanged (D-05 leaves `ownerOrSharedCondition` itself untouched;
   only `ownerScopeFilter`/`listFilter`'s surrounding `if scope != ""` wrapping is new).
2. **It is not a reimplementation.** `Store.ListScopes` (`store.go:1396-1401`)
   **[VERIFIED: internal/store/store.go:1396-1401]** already builds and runs this identical shape in
   production:
   ```go
   Filter: &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}},
   ```
   So the test is not inventing a hypothetical composition — it is pinning a composition that is
   already live and already load-bearing for `list_scopes`.
3. **It stays valid, unchanged, after the D-05 edit lands.** Once `Store.Search(ctx, "", subj, opts)`
   gains the conditional, its Must slice becomes `[ownerOrSharedCondition(subj), <window/superseded/
   tags/categories appends>]` — a strict superset of what this test already pins. No update to this
   test is required when D-05 lands; criterion 3's separate end-to-end test (`Store.Search(ctx, "",
   ...)` returns hits from multiple scopes) is the wiring proof that composition, not the authz proof.
4. **RED-by-mutation (D-15) is straightforward and does not touch feature code that doesn't exist yet.**
   Two equivalent mutation options, either is sufficient evidence, transcript recorded in the plan's
   verification notes:
   - Mutate the TEST: temporarily change `Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}}`
     to `Must: []*qdrant.Condition{}` (an empty `Must` matches every record in Qdrant) — confirm the
     test now fails (owner B's private record appears), then restore.
   - Mutate production `ownerOrSharedCondition` (`store.go:680-698`): temporarily short-circuit it to
     return an always-true condition (e.g. `return qdrant.NewIsEmpty("__nonexistent__")` negated, or
     simply comment out the `ownAllowed`/`sharedAllowed` gating and always `should = append(should,
     qdrant.NewMatch("owner", owner))` for every owner) — confirm the test fails, then restore.
   Either demonstrates the test is sensitive to the authz clause being dropped or defeated, satisfying
   "the required evidence is: delete `ownerOrSharedCondition` from the `Must` slice, observe the test
   fail, restore it" (`03-AUTHZ-GATE.md:118`).

This test belongs beside `TestBulkFilterZeroBucketFailsClosed`/`TestBulkFilterOrderIndependent`
(`internal/store/store_test.go:4314`, `4355`) per D-17 — both already use the same pattern of
constructing/injecting a controlled authz condition (`decideBucketHook`) and running the real query
against real Qdrant, which is precedent for testing the authz composition at this level of directness.

**A second, distinct test is criterion 3's job**, and must come AFTER the D-05 edit lands: an
end-to-end `Store.Search(ctx, "", ownerA, ...)` call proving `cross_spine=true` (empty scope) returns
hits from multiple scopes and an equivalent scoped call returns only the named scope. This is the
wiring proof; `TestCrossSpineAuthzIsolation` above is the authz proof. Both are needed; they are not
redundant.

## Research Question 2 — Fixture and Helper Inventory

### `internal/store/store_test.go`

| Helper | Location | Purpose |
|--------|----------|---------|
| `TestMain` | `store_test.go:50-74` | Provisions real Qdrant: prefers `ENGRAM_QDRANT_TEST_ADDR`, else boots an ephemeral testcontainer (`tcqdrant.Run`, `qdrantImageTag = "qdrant/qdrant:v1.18.2"`, `store_test.go:30`) |
| `dialTestClient(t)` | `store_test.go:88-106` | Dials the provisioned test Qdrant; `t.Skip`s if none available |
| `testStore(t) *Store` | `store_test.go:108-115` | `store.New(dialTestClient(t), "mem_eval_test")` + `EnsureCollection` — the standard fixture every store test uses |
| `cleanupErr(t, what, err)` | `store_test.go:120-125` | Surfaces a deferred-cleanup failure; tolerates `ErrNotFound` |
| Two-owner seeding pattern | `TestSearchDiscoveryOwnerIsolation` (`store_test.go:371-415`), `TestSearchListOwnerIsolation` (`store_test.go:675-713`) | Existing precedent for `mk(id, owner, vis)` closures seeding multiple owners; the new test's `mk` mirrors these exactly, but MUST use one shared scope string for both owners (D-16), unlike `TestSearchDiscoveryOwnerIsolation` which intentionally used two distinct scope names because it was pinning an already-correct implementation, not testing for a dropped-clause leak |
| `s.decideBucketHook` injection | `TestBulkFilterZeroBucketFailsClosed` (`store_test.go:4314-4348`), `TestBulkFilterOrderIndependent` (`store_test.go:4355-4412`) | Precedent for controlled-mutation testing of the authz composition against real Qdrant — same technique class as the D-15 mutation transcript |
| `Authenticated(subj string)` | used pervasively, e.g. `store_test.go:395` | Constructs a `Subject` for an authenticated caller |

### `internal/server/tools_test.go`

| Helper | Location | Purpose |
|--------|----------|---------|
| `testDeps(t) *deps` | `tools_test.go:294-298` | Qdrant-backed `deps` + fake embedder; delegates to `testDepsWithStore` |
| `testDepsWithStore(t) (*deps, *store.Store)` | `tools_test.go:304-326` | Same, plus the concrete `*store.Store` for call sites needing it |
| `cleanupErr(t, what, err)` | `tools_test.go:392-397` | Same pattern as the store package's version |
| `authedContext(t, sub) context.Context` | `tools_test.go:405-424` | Builds an authenticated context by round-tripping through `mcpauth.RequireBearerToken` with a stub verifier — the only way to inject `TokenInfo` since the go-sdk stores it under an unexported context key |
| `callerFor(ctx, t) caller` | `tools_test.go:436-...` | Resolves `caller` from a context exactly as `callerFromContext` does, failing the test on error |
| `TestSearchMemoryCategoriesArg` (`tools_test.go:2096-2127`), `TestListMemoryCategoriesArg` (`tools_test.go:2133-...`) | — | **Direct template for the D-17 handler-level test.** Both call `d.searchMemory(ctx, c, coreSearchRequest{...})` / `d.listMemory(ctx, c, coreListRequest{...})` directly — bypassing the MCP tool-registration closure but exercising the `deps` method (the "handler wiring" layer D-17 targets) |

The handler-level isolation test (D-17) should follow the `TestSearchMemoryCategoriesArg` template:
`d := testDeps(t)`, two callers via `callerFor(authedContext(t, "sub-A"), t)` and
`callerFor(authedContext(t, "sub-B"), t)`, seed overlapping-scope records, then call
`d.searchMemory`/`d.listMemory` with `coreSearchRequest{CrossSpine: true, ...}` (or however the plan
names the field per Claude's Discretion) and assert no cross-owner leak. This test necessarily lands
AFTER the args/proto plumbing exists (wave 3, per D-18's ordering) — it is a defense-in-depth
confirmation of the Go-level wiring, not the primary non-vacuous gate; `TestCrossSpineAuthzIsolation`
(Research Question 1) is the primary gate and is what must exist and pass first.

## Research Question 3 — Proto + Codegen Mechanics

### buf configuration [VERIFIED: buf.yaml, ci.yaml:126-153]

`buf.yaml`: `breaking: use: [FILE]`. `.github/workflows/ci.yaml:126-153` runs, in the `buf` CI job:

```yaml
- name: buf lint
  run: go tool buf lint
- name: buf breaking (vs main)
  run: go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'
- name: generated-code drift
  run: |
    go tool buf generate
    git diff --exit-code -- gen/ || (echo "gen/ is stale; run 'task proto:gen'"; exit 1)
- name: vendored console gen client drift
  run: |
    rm -rf ui/src/lib/gen/engram ui/src/lib/gen/buf
    cp -R gen/ts/. ui/src/lib/gen/
    git diff --exit-code -- ui/src/lib/gen/ || (echo "ui/src/lib/gen/ is stale; run 'task proto:gen'"; exit 1)
- name: idempotency ban (no side-effect-free RPC)
  run: |
    if grep -rEn 'idempotency_level[[:space:]]*=[[:space:]]*NO_SIDE_EFFECTS' proto/; then ...
```

Local equivalents: `task proto:lint` (`Taskfile.yaml:196-204`, `go tool buf lint` + the same
`idempotency_level` grep ban) and `task proto:gen` (`Taskfile.yaml:205-210`, `go tool buf generate` +
re-vendoring `gen/ts` into `ui/src/lib/gen`). This phase adds no new RPC and sets no `idempotency_level`
annotation, so the idempotency-ban gate is a no-op for this change, but `task proto:lint`/`task
proto:gen` must both be run and their output committed.

**Breaking-change detection is FILE mode** (whole-file comparison against `main`), which flags field
removal, field renumbering, and type changes — NOT purely additive new fields on an existing message.
Adding `bool cross_spine`, `repeated string searched_scopes`, and `bool scopes_truncated` at new,
never-before-used field numbers is additive and will NOT trip `buf breaking`. The risk is entirely
self-inflicted: reusing a field number that collided with a deprecated-but-still-declared field (e.g.
`ListMemoriesResponse.approximate = 3 [deprecated = true]`) would trip it, so new numbers must be
strictly higher than every existing number in the message, deprecated or not.

### Exact next-free field numbers [VERIFIED: proto/engram/v1/engram.proto:55-96]

```protobuf
message ListMemoriesRequest {
  string scope = 1; uint64 limit = 2; uint64 offset = 3; repeated string categories = 4;
  string visibility = 5; repeated string tags = 6; bool full = 7; string created_after = 8;
  string created_before = 9; string page_token = 10; bool cursor_mode = 11;
}
message ListMemoriesResponse {
  repeated Memory memories = 1; uint64 total = 2; bool approximate = 3 [deprecated = true];
  string next_page_token = 4;
}
message SearchMemoriesRequest {
  string query = 1; string scope = 2; uint64 k = 3; repeated string tags = 4; bool full = 5;
  string created_after = 6; string created_before = 7; repeated string categories = 8;
}
message SearchMemoriesResponse { repeated Memory memories = 1; }
```

| Message | Next field number | New field(s) to add |
|---------|-------------------|----------------------|
| `SearchMemoriesRequest` | 9 | `bool cross_spine = 9;` |
| `SearchMemoriesResponse` | 2 | `repeated string searched_scopes = 2;` then `bool scopes_truncated = 3;` |
| `ListMemoriesRequest` | 12 | `bool cross_spine = 12;` |
| `ListMemoriesResponse` | 5 | `repeated string searched_scopes = 5;` then `bool scopes_truncated = 6;` |

Note `ListMemoriesResponse.approximate = 3` is `[deprecated = true]` but still declared — field 3
remains reserved by declaration; the next free number is correctly 5, not 3.

### D-04 precedent already in the repo [VERIFIED: internal/server/connectapi.go:253-272]

`SearchDiscoveriesRequest` (`proto/engram/v1/engram.proto:91-95`) carries **no** `cross_spine` field at
all — Connect infers it from an empty scope:

```go
// connectapi.go:262-267
ms, err := a.d.searchDiscovery(ctx, c, searchDiscoveryArgs{
    Query: req.Msg.Query, Scope: req.Msg.Scope, K: k,
    CrossSpine: req.Msg.Scope == "",
})
```

CONTEXT.md D-04 explicitly does NOT want this pattern repeated for memories — the new
`SearchMemoriesRequest`/`ListMemoriesRequest.cross_spine` fields must be read explicitly and never
inferred from `scope == ""`, so `SearchMemories`/`ListMemories` need a genuinely new field, unlike
`SearchDiscoveries`.

### Generated tree

`gen/go/engram/v1/engram.pb.go` and `gen/go/engram/v1/engramv1connect/` (Go stubs), `gen/ts/engram/`
and `gen/ts/buf/` (TS, vendored into `ui/src/lib/gen/` by `task proto:gen`) all regenerate from
`buf.gen.yaml`'s pinned remote plugins (`protocolbuffers/go:v1.36.11`, `connectrpc/go:v1.20.0`,
`bufbuild/es:v2.12.1`) — regeneration must happen in the SAME commit as the proto edit or the `buf` CI
job's drift checks fail.

## Research Question 4 — `Store.ListScopes` Cost, Semantics, and Divergence

### Cost [VERIFIED: internal/store/store.go:1380-1414]

```go
const scanCap = 1000
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
    CollectionName: s.collection,
    Filter:         &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}},
    Limit:          qdrant.PtrOf(uint32(scanCap)),
    WithPayload:    qdrant.NewWithPayload(true),
})
```

One bounded `Scroll` of up to 1000 points with full payload, aggregated in-process via a Go map, then
sorted. This is a SECOND round-trip to Qdrant on the cross-spine path, in addition to the `Query`/
`Scroll` the search/list itself issues — D-13's design (call it only on the cross-spine path) is the
right call; a scope-confined query adds no cost. For a store with a bounded live working set (the
scanCap is 1000, matching `maxListLimit`), the added latency is one more network round-trip of similar
shape to the primary query — not free, but bounded and not quadratic.

### `more` bool semantics [VERIFIED: internal/store/store.go:1414, TestListScopes at store_test.go:1556-1589]

```go
return out, len(pts) == scanCap, nil
```

`more` (surfaced by D-13 as `scopes_truncated`) is true exactly when the scroll returned a FULL page
(1000 points) — i.e., when the caller's readable set is large enough that scanCap might have cut it
off before all scopes were represented. `TestListScopes` confirms `approximate=false` for a small
seeded set (`store_test.go:1579-1581`). This is a legitimate "bounded sample" signal, not an
approximation of individual counts — the aggregation itself is exact FOR the scanned points; only
completeness of the scope SET is at risk when truncated. D-13's `scopes_truncated` framing is accurate.

### Divergence from what a cross-spine Search/List actually finds — CONFIRMED

`ListScopes`'s filter (`store.go:1398`) is `Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}`
— nothing else. Compare to `Store.Search` (`store.go:888-900`), which appends, on top of the same
`ownerScopeFilter`: `activeWindowConditions(s.now())` (recall-window gate), `qdrant.NewIsEmpty
("superseded_by")` (soft-hide), `tagMatchConditions(opts.Tags)`, and `categoryMatchCondition
(opts.Categories)`. `Store.List` (via `listFilter` + `store.go:1114-1118`) appends the same window and
superseded-hide conditions.

**Consequence:** a scope containing ONLY superseded records, or ONLY records outside their active
window, or ONLY records that would be excluded by a search's `tags`/`categories` filter, still appears
in `ListScopes`'s output with a non-zero count — because `ListScopes` never applies those additional
gates. A simultaneous cross-spine `Search`/`List` call would return ZERO hits from that scope. So
`searched_scopes` (D-12/D-14) reports **the set of scopes you are authorized to read from**, not **the
set of scopes with content this specific query's other filters would surface**. This is exactly correct
for the authz half of criterion 5 (a scope you cannot read never appears), but the "found nothing here
vs. searched everywhere" framing is subtly imprecise: a caller could see a scope listed in
`searched_scopes` and still get zero hits from it for reasons entirely unrelated to whether the search
"found nothing" — the scope's only records may be structurally unreachable by this endpoint's other
gates. Recommend the plan word the field's documentation/description as "scopes searched under your
authorization" (not "scopes with matching content") to avoid over-promising, and confirm with the user
whether this caveat needs an explicit callout in the tool description (`tools.go`/proto comment) or is
accepted as-is per D-12's rejected-alternative reasoning (deriving from hits was explicitly rejected
for the OPPOSITE reason — it reports empty on a zero-hit search — so `ListScopes` remains the right
choice; this is a documentation-precision issue, not a design flaw).

## Research Question 5 — `Store.List` Total Semantics Under Cross-Spine

`Store.List` (`store.go:1123-1129`) **[VERIFIED: internal/store/store.go:1123-1129]**:

```go
// Exact total over the filtered set (replaces the scanCap approximation).
total, err = s.client.Count(ctx, &qdrant.CountPoints{
    CollectionName: s.collection, Filter: f, Exact: qdrant.PtrOf(true),
})
```

`f` is the SAME filter (`listFilter` + window + superseded-hide + optional created-range) that the
subsequent page fetch uses. Under cross-spine (`scope==""` post-D-05), `f`'s scope element is simply
omitted, so `Count` runs over every readable, currently-recallable record across every scope — an
EXACT count (`Exact: true`), not an approximation, confirming D-09's planner note. This is correct
behavior, but it is a number that will visibly jump upward the first time a caller flips
`cross_spine=true` on an existing scope-confined workflow — worth a dedicated test (as D-09 already
flags) asserting `total` equals the sum of per-owner-readable records across all seeded scopes in the
isolation fixture, not just that no leak occurs.

### `maxListLimit`/pagination interaction

`maxListLimit = 1000` (`store.go:1175`) bounds a single cursor page and a decoded cursor's `Seen` set
(`store.go:1185-1187`, `1196-1198`) — this cap is entirely independent of scope. `listByCursor`
(`store.go:1180-1236`) keysets on `created_at` plus the boundary `Seen` id set from the SAME filter `f`
passed in; nothing in the cursor encoding or resume logic references scope, confirming D-09's "cursor
format is already scope-agnostic" claim **[VERIFIED: internal/store/store.go:1180-1236]**. A
cross-spine list resumes correctly with no cursor-format change required.

## Research Question 6 — Call-Site Migration Table

No `store.SearchOptions`/`ListOptions` field is added (D-07: no store-level `CrossSpine` flag) — `Store
.Search`/`Store.List`'s signatures do not change at all. The entire store-layer diff is the two-line
`if scope != "" { ... }` wrap inside `ownerScopeFilter` (`store.go:752-757`) and `listFilter`
(`store.go:1054-1058`).

| File | Site | Change |
|------|------|--------|
| `internal/store/store.go` | `ownerScopeFilter` (752-757) | Wrap scope match in `if scope != ""` |
| `internal/store/store.go` | `listFilter` (1054-1058) | Same wrap |
| `internal/store/store_test.go` | new, beside `TestBulkFilterZeroBucketFailsClosed`/`OrderIndependent` (4314/4355) | `TestCrossSpineAuthzIsolation` — lands FIRST, before the filter edit (D-18) |
| `internal/server/tools.go` | `searchArgs` (534-543) | Add `CrossSpine bool \`json:"cross_spine,omitempty"\`` field; `Scope` tag gains `,omitempty` |
| `internal/server/tools.go` | `listArgs` (545-554) | Same two changes |
| `internal/server/tools.go` | new, beside `effectiveDiscoveryScope` (1129-1140) | `effectiveSearchScope` (naming/sharing is Claude's Discretion) mirroring the D-03 reject-empty-unless-cross-spine guard |
| `internal/server/tools.go` | `coreSearchRequest` (1048-1056) | Add `CrossSpine bool` (or infer purely from `Scope==""` post-guard — Claude's Discretion, but the ListScopes-trigger decision downstream needs the signal explicitly available) |
| `internal/server/tools.go` | `coreListRequest` (1018-1029) | Same |
| `internal/server/tools.go` | `deps.searchMemory` (1116-1127) | No signature change if `CrossSpine` stays out of the core struct; if added, threaded through unchanged (store call itself needs no new param) |
| `internal/server/tools.go` | `deps.listMemory` (1066-1082) | Same |
| `internal/server/tools.go` | `search_memory` MCP closure (1443-1475) | Call `effectiveSearchScope`, log-and-ignore per D-02, call `Store.ListScopes` when cross-spine, add `searched_scopes`/`scopes_truncated` to the `map[string]any` result (1474) |
| `internal/server/tools.go` | `list_memory` MCP closure (1477-1515) | Same shape, result map at 1514 |
| `internal/server/connectapi.go` | `SearchMemories` (194-221) | Read `req.Msg.CrossSpine` explicitly (D-04: never infer from empty scope, UNLIKE `SearchDiscoveries` at line 266); add the handler-level empty-scope-without-cross-spine guard Connect currently lacks; assemble `SearchMemoriesResponse.searched_scopes`/`scopes_truncated` |
| `internal/server/connectapi.go` | `ListMemories` (142-187) | Same |
| `proto/engram/v1/engram.proto` | `SearchMemoriesRequest` (76-85), `Response` (86) | New fields per Research Question 3 table |
| `proto/engram/v1/engram.proto` | `ListMemoriesRequest` (55-67), `Response` (69-74) | New fields per Research Question 3 table |
| `gen/go/engram/v1/*`, `gen/ts/*`, `ui/src/lib/gen/*` | — | Regenerate via `task proto:gen`, commit in the same change |
| `internal/server/tools_test.go` | new, using `TestSearchMemoryCategoriesArg`/`TestListMemoryCategoriesArg` as template (2096, 2133) | D-17 handler-level isolation test — lands AFTER the args/proto plumbing exists |
| `internal/server/tools_test.go` | any exhaustive/positional `searchArgs{...}`/`listArgs{...}`/`coreSearchRequest{...}`/`coreListRequest{...}` literal | Enumerate at plan time via `rg -n 'searchArgs{|listArgs{|coreSearchRequest{|coreListRequest{' internal/server/*_test.go`; a positional (non-keyed) struct literal would silently shift fields — confirm none exist (Go convention in this codebase is keyed literals throughout, per every example read this session, so risk is low but must be checked, not assumed) |
| `cmd/engram/client_search.go`, `client_list.go` | `--scope` flag (already optional, default `""`) | OUT OF SCOPE per CONTEXT.md (CLI not named in-scope) but flagged: post-phase, a CLI caller has no `--cross-spine` flag and cannot reach the new capability at all. Worth an Open Question / follow-up issue, not a phase task. |

**Compile-gate discipline (engram gotcha `3q4cx33cta`):** run `go vet ./...`, not `go build ./...`, after
struct field additions — `go build` does not compile `_test.go` files, so a fixture-breaking field
change would pass `go build` while failing every test file that constructs these structs, exactly the
documented failure class from v0.12.x Phase 1.

## Research Question 7 — Docs Surfaces That Must Move With the Code

Per engram convention `yaj7dqz9qq` (a new MCP tool argument with no agent-facing guidance is an
incomplete feature):

| Surface | Location | Required change |
|---------|----------|------------------|
| `docs-site/src/content/docs/reference/tools.md` | `search_memory` args table (88-108) | Add `cross_spine` row (type bool, required: no, description mirroring `search_discovery`'s row at line 305: "Span all readable scopes; ignores `scope` when true"); make `scope`'s Required column "conditional" (mirroring line 302) |
| same file | `list_memory` args table (113-131) | Same addition (D-08 extends cross-spine to `list_memory` too) |
| same file | Both sections' return-value prose | Mention `searched_scopes`/`scopes_truncated` appear only on a cross-spine call, per D-14's "omit both keys entirely on a non-cross-spine call" |
| `CLAUDE.md` (repo root) | "Memory contract (stable)" section — near the existing `tags`/`created_after`/`created_before`/`cursor` sentence | Add a sentence: `search_memory`/`list_memory` accept `cross_spine` (bool) to span every scope the caller can read, with the response reporting `searched_scopes`/`scopes_truncated` |
| `skill/engram/skills/curating-memory/SKILL.md` | "Tagging" section (already documents `tags`/`created_after`/`created_before`/`cursor` recall dimensions) | Add a short paragraph on `cross_spine`, when to reach for it (recall across projects/repos vs. staying scope-confined) — mirrors the existing tagging-discipline prose style |

`docs-site/src/content/docs/reference/tools.md` ALREADY documents `cross_spine` for `search_discovery`
(lines 294-307) — that section is the style template for the new rows, already using the phrase
"required unless cross_spine is true" the new rows should reuse verbatim for consistency.

## Standard Stack

No new libraries. This phase is a pure extension of already-adopted seams:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/qdrant/go-client/qdrant` | already pinned in `go.mod` | Filter/Scroll/Query/Count primitives | Already the store's sole Qdrant client |
| `connectrpc.com/connect` + generated stubs | already pinned | Connect RPC surface | Already the Connect transport |
| `github.com/testcontainers/testcontainers-go/modules/qdrant` | already a test dependency | Real-Qdrant integration tests | Already used by every `internal/store` integration test |

No installation step — this phase modifies existing files only.

## Architecture Patterns

### System Architecture Diagram

```
MCP client / Connect client
        │
        ▼
 search_memory / list_memory  (MCP closures, tools.go:1443/1477)
 SearchMemories / ListMemories (Connect handlers, connectapi.go:194/142)
        │
        ▼
 effectiveSearchScope(a)  ── CrossSpine=false & scope=="" ──▶ REJECT (D-03)
        │  CrossSpine=true ──▶ scope="" (D-02: log, ignore any supplied scope)
        │  CrossSpine=false & scope!="" ──▶ scope=a.Scope
        ▼
 coreSearchRequest / coreListRequest  (transport-neutral core, tools.go:1048/1018)
        │
        ▼
 deps.searchMemory / deps.listMemory  (tools.go:1116/1066)
        │
        ▼
 Store.SearchReranked → Store.Search / Store.List  (store.go:940/866/1085)
        │
        ▼
 ownerScopeFilter / listFilter  (store.go:752/1054)
   Must: [ if scope != "" { scope match }, ownerOrSharedCondition(subj) (UNCONDITIONAL), ...other appends ]
        │
        ▼
      Qdrant  ── real filter evaluation, the composition under test ──▶ scored/scrolled points
        │
        ▼ (cross-spine path only, D-13)
 Store.ListScopes  (store.go:1380)  ── Must: [ ownerOrSharedCondition(subj) ] only, no window/superseded/tag/category gates
        │
        ▼
 response assembly: memories (scope-attributed, D-11) + searched_scopes + scopes_truncated (D-14, omitted on non-cross-spine calls)
```

### Recommended Project Structure

No new files or directories — every change lands in existing files (`internal/store/store.go`,
`internal/store/store_test.go`, `internal/server/tools.go`, `internal/server/tools_test.go`,
`internal/server/connectapi.go`, `proto/engram/v1/engram.proto`, the generated `gen/` tree, and the
three docs surfaces above).

### Pattern: Conditional-scope / unconditional-authz filter composition
**What:** `if scope != "" { must = append(must, scopeCondition) }` followed by an ALWAYS-appended
`ownerOrSharedCondition(subj)`.
**When to use:** Any bulk read filter that needs an opt-in "span everything I can read" mode without
ever weakening the authz gate.
**Example (already shipped, the pattern to mirror byte-for-byte):**
```go
// Source: internal/store/store.go:977-987 (Store.SearchDiscovery)
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

### Anti-Patterns to Avoid
- **Driving the isolation test through `Store.Search`/`Store.List` with `scope=""` before the D-05 edit
  lands:** this is the vacuous-green trap itself (Research Question 1) — zero results proves nothing.
- **Inferring `cross_spine` from an empty Connect scope field for memories:** explicitly rejected by
  D-04; `SearchDiscoveries` does this only to preserve a pre-existing contract, memories have none.
- **Adding a store-level `SearchOptions.CrossSpine`/`ListOptions.CrossSpine` field:** explicitly
  rejected by D-07 — the handler guard (D-03) is the sole source of truth; a second flag creates two
  sources of truth for one decision.
- **Treating `ListScopes`'s output as "scopes with results":** it is "scopes you may read," which can
  diverge from what a cross-spine `Search`/`List` actually surfaces (Research Question 4).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-spine authz filter composition | A new bespoke conditional-scope helper | The `if scope != ""`-wrap-plus-unconditional-authz pattern already shipped in `SearchDiscovery` (`store.go:977-987`) | Byte-for-byte reuse means the already-verified authz property (the closed gate) transfers by structural identity, not by a fresh audit |
| Searched-scope enumeration | A per-query scope-tracking accumulator, or deriving from hit results | `Store.ListScopes` (already shipped, already authz-correct) | D-12 explicitly rejected deriving from hits (reports empty on a zero-hit search — the exact case criterion 5 needs distinguished) |

**Key insight:** every mechanism this phase needs already exists in the codebase in a sibling form
(`SearchDiscovery`'s conditional scope, `ListScopes`'s authz-only scan, `recallView.Scope`'s
attribution). The phase's actual work is disciplined reuse plus the isolation-test rigor to prove the
reuse didn't silently change the authz property along the way.

## Common Pitfalls

### Pitfall 1: The vacuous-green isolation test
**What goes wrong:** A two-owner test calling `Store.Search`/`Store.List` with `scope=""` before the
D-05 edit lands "passes" because empty scope currently matches nothing — not because authz held.
**Why it happens:** `scope==""` looks semantically like "no scope filter" but today means "match the
literal empty string," an easy conflation.
**How to avoid:** Use the raw-filter-construction technique in Research Question 1 — never assert
non-leakage via a call path whose CURRENT behavior returns zero results for an unrelated reason.
**Warning signs:** A "RED" observation obtained by TOGGLING `cross_spine` rather than by mutating the
authz clause (explicitly the wrong technique per D-15).

### Pitfall 2: Treating `searched_scopes` as "scopes with content"
**What goes wrong:** A caller reads a non-empty `searched_scopes` list and assumes every listed scope
has recallable content matching the query's other filters.
**Why it happens:** `ListScopes` applies only the authz predicate, not the window/superseded/tag/
category gates the search/list itself applies (Research Question 4).
**How to avoid:** Word the field's description as "scopes searched under your authorization," and
confirm with product/user whether the divergence needs an explicit callout.
**Warning signs:** A user report of "cross_spine said it searched scope X but scope X has records I
know exist" — expected when X's records are superseded/windowed-out/filtered, not a bug.

### Pitfall 3: `go build ./...` false confidence after struct field additions
**What goes wrong:** Adding `CrossSpine` to `searchArgs`/`listArgs`/`coreSearchRequest`/
`coreListRequest` compiles clean under `go build ./...` (which never compiles `_test.go`), while a
positional (unkeyed) struct literal in a test file silently shifts field assignments.
**Why it happens:** Documented recurring failure class in this repo (engram gotcha `3q4cx33cta`,
`STATE.md`), previously bit v0.12.x Phase 1's resolver arity change.
**How to avoid:** Run `go vet ./...` as the compile gate, not `go build ./...`; grep for any
unkeyed literal of the four struct types before editing.
**Warning signs:** A test passing with obviously wrong field values, or a compile error only surfacing
in CI's test step, never in a local `go build`.

## Code Examples

### Verified pattern to mirror for the filter edit
```go
// Source: internal/store/store.go:977-987 (Store.SearchDiscovery — already shipped)
must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
if scope != "" {
	must = append(must, qdrant.NewMatch("scope", scope))
}
must = append(must, s.ownerOrSharedCondition(subj))
```

### Verified pattern to mirror for the handler guard
```go
// Source: internal/server/tools.go:1129-1140 (effectiveDiscoveryScope — already shipped)
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

### Verified pattern already exercising the exact cross-spine filter shape
```go
// Source: internal/store/store.go:1396-1401 (Store.ListScopes — already shipped, already in production)
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
	CollectionName: s.collection,
	Filter:         &qdrant.Filter{Must: []*qdrant.Condition{s.ownerOrSharedCondition(subj)}},
	Limit:          qdrant.PtrOf(uint32(scanCap)),
	WithPayload:    qdrant.NewWithPayload(true),
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `search_memory`/`list_memory` always scope-confined | Opt-in `cross_spine` spans every readable scope, mirroring `search_discovery` | This phase | New capability, opt-in only (no default flip — explicitly out of scope per `REQUIREMENTS.md`) |

Nothing in this domain is deprecated by this phase; it is additive throughout.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `coreSearchRequest`/`coreListRequest` should carry an explicit `CrossSpine bool` field rather than relying purely on `Scope==""` post-guard, so the ListScopes-trigger decision and searched-scopes assembly have an unambiguous signal | Research Question 6 | Low — this is flagged as Claude's Discretion in CONTEXT.md; either shape works, this is a recommendation not a locked fact |
| A2 | The two mutation options offered for D-15's RED transcript (mutate the test's own `Must` slice, or mutate production `ownerOrSharedCondition`) are both acceptable evidence | Research Question 1 | Low — `03-AUTHZ-GATE.md`'s own wording ("delete `ownerOrSharedCondition` from the `Must` slice") is closer to the second option; the plan should pick one explicitly and record which |
| A3 | No unkeyed/positional struct literals of `searchArgs`/`listArgs`/`coreSearchRequest`/`coreListRequest` exist in test files today | Research Question 6 | Medium if wrong — a silent fixture field-shift would need `go vet`/manual grep to catch; this session did not exhaustively grep every call site, only confirmed the codebase's general keyed-literal convention from examples read |

## Open Questions

1. **Should the CLI (`engram search`/`list`) gain a `--cross-spine` flag in this phase or a follow-up?**
   - What we know: CONTEXT.md's in-scope list names only `search_memory`, `list_memory`, "their Connect
     siblings" — the CLI is not named. `cmd/engram/client_search.go`'s `--scope` flag is already
     optional (`store_test.go` pattern; `client_search.go:73`).
   - What's unclear: whether a headless CLI caller having no way to reach cross-spine recall is an
     acceptable gap for this milestone or should be filed as a follow-up issue.
   - Recommendation: treat as out of scope per CONTEXT.md's explicit boundary; file a follow-up issue
     if the planner or user wants CLI parity tracked.

2. **Does `searched_scopes`'s divergence from "scopes with recallable content" (Research Question 4)
   need an explicit user-facing callout, or is D-12's already-recorded rejection of the hit-derived
   alternative sufficient justification to ship as-is?**
   - What we know: the divergence is real and mechanically confirmed by reading `ListScopes` vs.
     `Search`/`List`'s filter composition.
   - What's unclear: whether this rises to a product decision needing explicit sign-off, or is minor
     enough to just word carefully in the tool description.
   - Recommendation: word the proto/tool-description text to say "authorized to read," not "with
     results," and let the planner decide if a CONTEXT.md-level confirmation is warranted.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `testcontainers-go/modules/qdrant` for real-Qdrant integration |
| Config file | none — `internal/store/store_test.go:TestMain` provisions Qdrant programmatically (`ENGRAM_QDRANT_TEST_ADDR` env override or ephemeral testcontainer) |
| Quick run command | `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v` |
| Full suite command | `task` (lint + `go test ./...`, per `CLAUDE.md`'s Taskfile convention) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-cross-spine-authz-verified | Two-owner overlapping-scope isolation holds under the cross-spine-shaped filter, real Qdrant | integration | `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v` | ❌ Wave 0 — new test, lands FIRST per D-18 |
| REQ-cross-spine-authz-verified | D-06 amendment: `listFilter` carries the same two-element shape | reading (recorded in writing) | n/a — satisfied by this RESEARCH.md's confirmation and the planned `03-AUTHZ-GATE.md` amendment | ✅ confirmed this session |
| REQ-cross-spine-search | `cross_spine=true` returns hits from multiple scopes; omitted returns only the named scope | integration | `go test ./internal/store/... -run TestSearchCrossSpine -v` (new, after D-05 edit) | ❌ Wave 0 |
| REQ-cross-spine-search | Handler-level: `search_memory`/`list_memory` wiring (D-03 guard, args plumbing) preserves isolation | integration | `go test ./internal/server/... -run TestSearchMemoryCrossSpineIsolation -v` (new, D-17) | ❌ Wave 0 — after proto/args plumbing |
| REQ-cross-spine-search | MCP↔Connect parity | integration | `go test ./internal/server/... -run TestSearchMemoriesConnectCrossSpine -v` (new) | ❌ Wave 0 |
| REQ-cross-spine-result-provenance | Every result carries its originating scope | unit/integration | existing `recallView.Scope`/proto `Memory.scope` — add a cross-spine-specific assertion to the new search test | ❌ extend Wave 0 test |
| REQ-cross-spine-result-provenance | Response reports `searched_scopes`/`scopes_truncated`, omitted on non-cross-spine calls | integration | new assertion in the handler-level test | ❌ Wave 0 |
| REQ-cross-spine-result-provenance | `Store.List`'s exact `Count` under cross-spine sums correctly | integration | `go test ./internal/store/... -run TestListCrossSpineTotal -v` (new, per D-09's planner note) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/store/... -run TestCrossSpine -v` (or the relevant package) plus `go vet ./...`
- **Per wave merge:** `task` (full lint + test suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`; `task proto:lint`/`task proto:gen` clean-diff before any proto-touching wave is considered done

### Wave 0 Gaps
- [ ] `TestCrossSpineAuthzIsolation` in `internal/store/store_test.go` — covers REQ-cross-spine-authz-verified (primary gate, lands before the filter edit per D-18)
- [ ] `TestSearchCrossSpine`/`TestListCrossSpine` in `internal/store/store_test.go` — covers REQ-cross-spine-search's wiring proof (after the filter edit)
- [ ] Handler-level isolation + parity tests in `internal/server/tools_test.go` and a Connect-side equivalent — covers REQ-cross-spine-search parity and REQ-cross-spine-authz-verified's D-17 defense-in-depth
- [ ] `TestListCrossSpineTotal` — covers the D-09 planner note under REQ-cross-spine-result-provenance
- No new framework install required — `testcontainers-go/modules/qdrant` is already a test dependency and `TestMain` already provisions Qdrant for the whole `internal/store` package.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | Unchanged this phase — bearer/cookie verification lives in `internal/auth`, untouched |
| V3 Session Management | no | Unchanged |
| V4 Access Control | yes | Qdrant `Must`-clause authz composition in `internal/store` (`ownerOrSharedCondition`), enforced exclusively at the store layer per standing invariant ADR `engram-cdr1`; this phase's entire risk surface is here |
| V5 Input Validation | yes | `effectiveSearchScope`/`effectiveListScope` (D-03) reject an empty scope without `cross_spine=true`, mirroring the already-shipped `effectiveDiscoveryScope` pattern |
| V6 Cryptography | no | Not touched |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Scope-filter widening silently drops the authz clause | Elevation of Privilege | The unconditional-authz / conditional-scope composition (D-05), proven non-vacuously by `TestCrossSpineAuthzIsolation` (Research Question 1) before the widening edit lands |
| Handler bypasses the empty-scope guard (accidental `scope=""` reaches the store pre-cross-spine-intent) | Elevation of Privilege | D-03's `effectiveSearchScope` guard is the SOLE chokepoint (D-07: no store-level defense-in-depth flag) — a test pinning the rejection is required, not optional, per CONTEXT.md's own planner note on D-07 |
| `searched_scopes` over-discloses scope names the caller cannot otherwise enumerate | Information Disclosure | `ListScopes` already runs under `ownerOrSharedCondition` — a caller only ever sees scope names for records they are independently authorized to read (own + shared); no new disclosure surface is created, confirmed by reading `store.go:1396-1414` |

## Sources

### Primary (HIGH confidence — read live this session)
- `internal/store/store.go` — `ownerOrSharedCondition` (680-698), `ownerScopeFilter` (752-757),
  `Search` (866-909), `SearchReranked` (940-949), `SearchDiscovery` (958-997), `listFilter`
  (1054-1077), `List` (1085-1169), `listByCursor` (1180-1236), `maxListLimit` (1175), `ListScopes`
  (1380-1414)
- `internal/store/store_test.go` — `TestMain` (50-74), `testStore` (108-115),
  `TestSearchDiscoveryOwnerIsolation` (371-415), `TestSearchListOwnerIsolation` (675-713),
  `TestListScopes` (1556-1589), `TestBulkFilterZeroBucketFailsClosed` (4314-4348),
  `TestBulkFilterOrderIndependent` (4355-4412)
- `internal/server/tools.go` — `searchArgs`/`listArgs` (534-554), `searchDiscoveryArgs` (623-629),
  `effectiveDiscoveryScope`/`searchDiscovery` (1129-1165), `coreListRequest`/`coreSearchRequest`
  (1018-1056), `listMemory`/`searchMemory` (1066-1127), MCP tool closures (1443-1588)
- `internal/server/tools_test.go` — `testDeps`/`testDepsWithStore` (294-326), `authedContext`
  (405-424), `callerFor` (436+), `TestSearchMemoryCategoriesArg`/`TestListMemoryCategoriesArg`
  (2096-2165)
- `internal/server/connectapi.go` — `ListMemories` (142-187), `SearchMemories` (194-221),
  `SearchDiscoveries` (253-272)
- `internal/server/summary.go` — `recallView` (40-60)
- `proto/engram/v1/engram.proto` — full message set (1-96)
- `buf.yaml`, `buf.gen.yaml`, `Taskfile.yaml:196-211`, `.github/workflows/ci.yaml:126-153`
- `.planning/phases/03-cross-spine-memory-recall/03-CONTEXT.md`, `03-AUTHZ-GATE.md`
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`
- `docs-site/src/content/docs/reference/tools.md` (1-320)
- `skill/engram/skills/curating-memory/SKILL.md` (1-80)
- `cmd/engram/client_search.go` (flag definitions)

No external/web sources were needed — this is a purely in-repo composition-verification task, per the
phase's own nature (a reading-and-testing gate, not a new-technology adoption).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, entirely existing seams
- Architecture: HIGH — every pattern is read live and already shipped in `SearchDiscovery`/
  `effectiveDiscoveryScope`/`ListScopes`
- Pitfalls: HIGH — the vacuous-green trap is independently confirmed by reading the current
  `ownerScopeFilter` behavior, not inferred from CONTEXT.md's description alone

**Research date:** 2026-08-01
**Valid until:** No expiry driver — this is an in-repo composition study, not a fast-moving external
dependency; valid as long as `internal/store/store.go`'s filter functions are unchanged from the lines
cited above.
