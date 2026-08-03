# Phase 3: Cross-Spine Memory Recall - Context

**Gathered:** 2026-08-01
**Status:** Ready for planning

<domain>
## Phase Boundary

An agent can recall curated memories across every scope it is permitted to see, via an opt-in
`cross_spine` flag on `search_memory` **and** `list_memory`, at MCP↔Connect parity through additive
proto fields. The authorization filter is proven un-widened rather than assumed safe: the closed
gate (`03-AUTHZ-GATE.md`) established the property by reading, and a two-owner isolation test
against real Qdrant keeps it true. Every result stays attributable to its originating scope, and a
cross-spine response reports which scopes were searched so "found nothing here" is distinguishable
from "searched everywhere I can see and found nothing."

**In scope:** `search_memory`, `list_memory`, their Connect siblings, the `ownerScopeFilter` /
`listFilter` scope conditional, searched-scope reporting, and the isolation tests.

**Out of scope:** `list_scheduled` (separate filter, stays scope-confined), scope prefix/wildcard
targeting (deferred `REQ-cross-spine-scope-prefix`), per-scope counts and skipped-with-reason
coverage receipts (deferred `REQ-cross-spine-coverage-receipt`), and any change to who is
authorized to read what.

</domain>

<decisions>
## Implementation Decisions

### Argument contract and lane parity

- **D-01 (cross_spine is a bool, mirroring search_discovery byte for byte):** `searchArgs` and
  `listArgs` each gain `CrossSpine bool` with the same `json:"cross_spine,omitempty"` tag and
  jsonschema wording as `searchDiscoveryArgs` (`internal/server/tools.go:623-629`), and their
  `Scope` fields become `omitempty`. Rejected alternatives — an explicit `scopes []string` list and
  a `scope_prefix` glob — are both already recorded as deferred future requirements in
  REQUIREMENTS.md, and either would need its interaction with the boolean specified.
  — **Reversibility:** costly — a published tool-argument schema on two tools and two RPCs.

- **D-02 (cross_spine=true with a non-empty scope ignores the scope and logs at Info):** Exactly
  `searchDiscovery`'s behavior (`tools.go:1147-1151`), including the existing discipline of not
  echoing the caller-supplied scope value into the log line, so an unbounded or sensitive scope
  string never reaches log aggregation. Chosen over rejecting as `InvalidArgument` because the two
  search surfaces should not disagree about the same argument combination.
  — **Reversibility:** reversible.

- **D-03 (cross_spine=false with an empty scope is REJECTED at the handler):** A new
  `effectiveSearchScope` mirrors `effectiveDiscoveryScope` (`tools.go:1132-1140`) and returns
  `scope is required unless cross_spine is true`. This is the load-bearing guard for D-05: it is
  what keeps an accidental empty scope from reaching a store whose empty-scope semantics are about
  to change from "matches nothing" to "matches everything readable". Applies to `list_memory` on
  the same helper.
  — **Reversibility:** reversible, but removing it re-opens the silent-widening hazard D-05 creates.

- **D-04 (Connect does NOT infer cross-spine from an empty scope — the explicit field is required):**
  `SearchMemories` and `ListMemories` read the new proto field and never map `scope == ""` to
  cross-spine. `SearchDiscoveries` does map it (`connectapi.go:266`), but only to preserve a
  contract that predated the typed core; memories have no such contract. Inferring it would
  silently widen every existing empty-scope Connect call from "returns nothing" to "returns
  everything readable" — a behavior change no caller opted into.
  — **Reversibility:** costly — inferring later is additive, un-inferring later is a break.

### Filter mechanics and the authz invariant

- **D-05 (the scope conditional is added inside ownerScopeFilter and listFilter; the authz entry stays unconditional):**
  Both helpers become `if scope != "" { append the scope match }` with
  `ownerOrSharedCondition(subj)` still appended unconditionally — the shape `SearchDiscovery`
  already uses (`store.go:977-987`). This is safe to do in place because each helper has exactly
  one production call site: `ownerScopeFilter` is called only from `Store.Search`
  (`store.go:888`), and `listFilter` only from `Store.List` (`store.go:1113`).
  `Store.ListScheduled` and `Store.DeleteAll` build their own filters and are untouched.
  — **Reversibility:** reversible — a local edit in two functions.

- **D-06 (the closed authz gate extends to listFilter by identical structure, not by analogy):**
  `listFilter` (`store.go:1054-1058`) opens with the same two-element `Must` slice as
  `ownerScopeFilter` — `qdrant.NewMatch("scope", scope)` at index 0, `ownerOrSharedCondition(subj)`
  at index 1, separate and unconditional, with everything after it appended. The gate's Evidence 1
  and Evidence 2 arguments therefore transfer verbatim, and `03-AUTHZ-GATE.md` should be amended
  with a short note recording that `listFilter` was read and carries the same property. This is a
  reading, not an assumption — the planner must confirm it against the live tree, not against this
  document.
  — **Reversibility:** n/a — a recorded fact.

- **D-07 (no store-level CrossSpine flag; the handler is the sole guard):** `Store.Search` and
  `Store.List` keep the `scope == ""` convention rather than gaining an explicit
  `SearchOptions.CrossSpine` / `ListOptions.CrossSpine`. D-03 is what makes this safe. Rejected
  the defense-in-depth variant because it creates two sources of truth for one decision and
  diverges from `Store.SearchDiscovery`, which the whole design is mirroring.
  **Planner note:** this makes D-03 load-bearing rather than cosmetic. If the handler guard is ever
  removed or bypassed, an accidental empty scope silently spans everything readable. A test that
  pins the rejection is required, not optional.
  — **Reversibility:** reversible — adding the flag later is additive.

- **D-08 (list_memory gets cross-spine too — decided 2026-08-01 by Sean, over the recommendation):**
  `REQ-cross-spine-search` names `search_memory` only, and the proposal was to leave `list_memory`
  scope-confined. Overridden: recall symmetry matters more than literal requirement wording, and an
  agent that can search across spines but not list across them has an arbitrary hole. The cost is
  genuinely low — D-05's edit is the same shape in both helpers, and D-06 shows the authz argument
  transfers. REQUIREMENTS.md should record the widened interpretation against
  `REQ-cross-spine-search` rather than leaving the phase silently over-delivering.
  — **Reversibility:** costly — a published argument on a second tool and RPC.

- **D-09 (cursor paging is unchanged; the cursor format is already scope-agnostic):**
  `listByCursor` keysets on `created_at` plus a boundary seen-id set, never on scope, so a
  cross-spine page resumes correctly with no cursor change. `maxListLimit` (1000) still bounds a
  page. **Planner note:** `Store.List`'s `total` is an exact server-side `Count` over the built
  filter, so under cross-spine it becomes the exact count across all readable scopes — correct, but
  it is a number that will visibly jump for callers, and it deserves a test.
  — **Reversibility:** reversible.

- **D-10 (rerank is untouched):** `SearchReranked` over-fetches `candidateK(k)` from the already
  authorized `Search` and reorders in-process, strictly after the filter runs (`store.go:929`
  documents this ordering). Cross-spine changes what the filter admits, not when ranking happens.
  No per-scope k allocation — that would invent ranking policy this phase does not own.
  — **Reversibility:** reversible.

### Result provenance and searched-scope reporting

- **D-11 (per-result scope attribution needs no new field — pin it, do not add it):**
  `recallView.Scope` (`internal/server/summary.go:46`) is already on the compact MCP view,
  `store.Memory.Scope` is on the full view, and proto `Memory.scope` is field 3. Criterion 5's
  attribution half is satisfied today on both lanes and in both view shapes; the phase's obligation
  is a test that keeps it true under cross-spine, not a redundant `origin_scope` alias.
  — **Reversibility:** n/a.

- **D-12 (searched-scopes comes from Store.ListScopes, because it uses the same authz predicate):**
  `Store.ListScopes` (`store.go:1380`) filters on `ownerOrSharedCondition(subj)` alone, unpinned by
  scope — the exact predicate a cross-spine query runs under. So the scope set it returns *is* the
  set that was searched, rather than a second approximation of it. Report scope names only; no
  per-scope counts, which belong to the deferred `REQ-cross-spine-coverage-receipt`.
  Rejected deriving the set from the hits: it reports the empty set on a zero-hit search, which is
  precisely the case criterion 5 exists to disambiguate.
  — **Reversibility:** reversible.

- **D-13 (call ListScopes only on the cross-spine path, and surface its truncation flag):**
  A scope-confined search adds no scroll. `ListScopes` scrolls up to `scanCap` (1000) points and
  returns a `more` bool meaning "the counts are a bounded sample"; that bool is surfaced as
  `scopes_truncated` so a large store never silently reports a partial scope list as complete.
  — **Reversibility:** reversible.

- **D-14 (response shape is additive on both lanes, flat, with no coverage sub-message):** MCP adds
  `searched_scopes` beside `memories` in the existing `map[string]any` result
  (`tools.go:1474` for search, the `{memories, next_cursor}` map for list) plus `scopes_truncated`.
  Connect adds `repeated string searched_scopes` and `bool scopes_truncated` as new field numbers
  on `SearchMemoriesResponse` and `ListMemoriesResponse`. Flat fields rather than a `coverage`
  sub-message, because a sub-message is the shape the *deferred* coverage-receipt requirement will
  want and pre-building it invites filling it.
  **Planner note:** omit both keys entirely on a non-cross-spine call so existing consumers see a
  byte-identical response.
  — **Reversibility:** costly on the proto side — field numbers are permanent.

### Test strategy for the isolation gate

- **D-15 (RED is observed by mutating the authz clause, never by toggling cross_spine):**
  Per `03-AUTHZ-GATE.md`:115-119, `scope == ""` matches essentially nothing today, so a naive
  two-owner test would pass vacuously before the feature exists — returning zero records because
  the scope filter excluded everything, not because the authz gate held. The required evidence is:
  delete `ownerOrSharedCondition` from the `Must` slice, observe the test fail, restore it. That
  transcript is recorded in the plan's verification notes. A pass observed only against the
  pre-feature tree is not evidence.
  — **Reversibility:** n/a — a verification obligation.

- **D-16 (the fixture uses two owners over OVERLAPPING scope names):** Both owners hold records under
  scopes with the *same* name, each with a private and a shared record. Overlap is what makes a
  dropped owner clause visible — it returns the other owner's records rather than silently
  returning nothing. Distinct per-owner scope names cannot distinguish a working owner clause from
  an absent one.
  — **Reversibility:** n/a.

- **D-17 (primary test in internal/store against real Qdrant; handler-level test added as well — decided 2026-08-01 by Sean):**
  The primary isolation test lives in `internal/store/store_test.go` beside
  `TestBulkFilterZeroBucketFailsClosed`, using testcontainers, because the composition under test
  is Qdrant's filter evaluation and not Go's. Sean additionally requested a handler-level test in
  `internal/server/tools_test.go` pinning the same property through the `search_memory` and
  `list_memory` handlers — it guards the handler wiring (D-03's guard, the args plumbing) that the
  store test cannot see. Both, not either.
  — **Reversibility:** reversible.

- **D-18 (the isolation test lands and passes in its own commit BEFORE any filter edit):**
  Criterion 2 says the test "exists and passes before the feature is implemented", and D-15 is how
  that claim is made non-vacuous. Task ordering: isolation test commit, then filter edit, then
  handler/proto plumbing.
  — **Reversibility:** n/a — an ordering obligation.

### Claude's Discretion

- Exact naming of `effectiveSearchScope` and whether `search_memory` and `list_memory` share one
  helper or take two — behavior is fixed by D-03, the shape is not.
- Proto field numbers for the new request and response fields, subject to `buf` lint and the
  existing additive-only CI gate.
- Whether `scopes_truncated` is omitted or emitted-false on a cross-spine call that did not
  truncate.
- Test function names, fixture ids, and how the D-15 mutation transcript is captured in the plan.
- Whether the `03-AUTHZ-GATE.md` amendment required by D-06 is a new section or an inline note.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `Store.SearchDiscovery` (`internal/store/store.go:977-987`) is the exact target composition
  already shipped: conditional scope, unconditional `ownerOrSharedCondition`, everything else
  appended after.
- `effectiveDiscoveryScope` (`internal/server/tools.go:1132-1140`) is the guard shape D-03 mirrors.
- `deps.searchDiscovery` (`tools.go:1142-1165`) is the handler shape, including the
  ignore-scope-and-log branch D-02 mirrors and its no-value-echo logging discipline.
- `Store.ListScopes` (`store.go:1380-1414`) already enumerates readable scopes under exactly the
  cross-spine authz predicate, and already returns the bounded-sample `more` flag D-13 surfaces.
- `recallView` (`internal/server/summary.go:40-60`) is a hand-written allow-list struct that
  already carries `Scope`; its own comments warn that a field must be added here *and* populated in
  `toRecallView` to surface.
- `TestBulkFilterZeroBucketFailsClosed` and `TestBulkFilterOrderIndependent`
  (`internal/store/store_test.go`) are the existing real-Qdrant filter tests the new isolation test
  sits beside.

### Established Patterns

- Authorization is enforced in `internal/store` via Qdrant read filters, never in handlers
  (standing invariant, ADR `engram-cdr1` refining LOCKED `DEC-cgb`). This phase changes what the
  scope clause admits and touches the authz clause not at all.
- Transport adapters apply their own `k` / `limit` defaults before calling the typed core; the core
  applies none (`tools.go:1449-1455`, `connectapi.go:189-193`). MCP defaults k=8, Connect k=20.
- MCP recall shaping (`shapeRecall`) lives in the tool closure, not the shared core — the core
  returns raw `[]store.Memory`.
- New payload keys and new proto fields are additive only; one Qdrant collection serves every
  memory kind (`DEC-2bv`). This phase adds no payload key at all.

### Integration Points

- `internal/store/store.go` — `ownerScopeFilter:752`, `listFilter:1054`, `Search:888`, `List:1113`.
- `internal/server/tools.go` — `searchArgs:534`, `listArgs:545`, `search_memory` closure at 1443,
  `list_memory` closure at 1477, and a new `effectiveSearchScope` beside `effectiveDiscoveryScope`.
- `internal/server/connectapi.go` — `SearchMemories:194`, `ListMemories`, both of which pass a
  `coreSearchRequest` / `coreListRequest` through.
- `proto/engram/v1/engram.proto` — `SearchMemoriesRequest:76`, `SearchMemoriesResponse:86`, and the
  `ListMemories` pair; regenerate with `task proto:gen`, and the committed `gen/` tree is
  CI-checked for drift.
- `docs-site/src/content/docs/reference/tools.md` documents `cross_spine` for `search_discovery`
  today and needs the memory-side rows.
- `CLAUDE.md`'s memory contract section describes the recall surface and needs a cross-spine
  sentence.

</code_context>

<specifics>
## Specific Ideas

- The authz gate is CLOSED and must not be re-run. `03-AUTHZ-GATE.md` is the written artifact
  satisfying criterion 1; it needs only the D-06 amendment covering `listFilter`.
- The single most important trap in this phase is the vacuous green described in D-15. A
  two-owner test that passes today proves nothing, because the empty-scope filter already excludes
  everything. Nothing in this phase should be accepted as evidence unless it has been observed to
  fail for the right reason.
- Phase 5 touches `internal/server/tools.go` in `summarizerFromConfig` while this phase touches
  `searchMemory` / `list_memory` in the same file. ROADMAP flags low but non-zero merge risk —
  keep this phase's `tools.go` diff tight and self-contained.

</specifics>

<deferred>
## Deferred Ideas

- **Per-scope counts and a skipped-with-reason coverage receipt** — already recorded as
  `REQ-cross-spine-coverage-receipt` in REQUIREMENTS.md "Future Requirements". D-14's flat response
  shape deliberately avoids pre-building the sub-message this would want.
- **Scope prefix or wildcard targeting** (e.g. all `repo:*` spines but no workspace overlays) —
  already recorded as `REQ-cross-spine-scope-prefix`. Needs its interaction with the `cross_spine`
  boolean specified before it can be planned.
- **Cross-spine on `list_scheduled`** — `Store.ListScheduled` builds its own filter and is out of
  scope here. Revisit only if an agent actually needs windowed records across spines.

</deferred>
