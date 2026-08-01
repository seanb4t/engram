# v0.12.x Phase 3 — Authorization Gate (REQ-cross-spine-authz-verified)

**Status:** ✅ CLOSED — the property holds. Read end to end 2026-08-01 against the live tree.
**Gate:** ROADMAP criterion 1. This must be satisfied *before* implementation begins. It is a gate,
not a task.

## The question

> `Store.Search`'s filter construction has been read end to end and it is recorded **in writing**
> that the owner/authz `Must` clause is composed as a separate, unconditional entry from the scope
> clause — never a combined condition where omitting scope could drop part of the authz gate.

Research raised this and deliberately did **not** resolve it: architecture traced it as "a one-line
conditional in `ownerScopeFilter`"; pitfalls warned that memories lack the `discovery:*` namespace
convention discoveries rely on, and that the single-scope path may have leaned on `scope` being
non-empty as an unaudited narrowing signal. The requirement says: resolved by verification, not
analogy.

## Verdict

**The authz clause is a genuinely separate, unconditional entry. Omitting or conditioning the scope
clause cannot drop any part of the authorization gate.**

### Evidence 1 — the filter is a two-element `Must` slice

`internal/store/store.go:752-757`:

```go
func (s *Store) ownerScopeFilter(scope string, subj Subject) *qdrant.Filter {
	return &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),   // element 0 — the scope clause
		s.ownerOrSharedCondition(subj),    // element 1 — the authz clause
	}}
}
```

These are two independent elements of one `Must` slice. There is no nesting, no shared
conditional, and no boolean combining them. Making element 0 conditional is a purely local edit
that cannot affect element 1. **This is the property the gate asks about, and it holds.**

`Store.Search` (`store.go:888-900`) consumes it and only ever *appends* to `f.Must` — the recall
window, the `superseded_by` soft-hide, tags, categories, and the created-at range. Nothing
re-reads, rewrites, or removes element 1.

### Evidence 2 — the authz condition never reads `scope`

`internal/store/store.go:680-698`. `ownerOrSharedCondition` takes only `subj`. `scope` is not in
scope for it — there is no path by which a scope value, empty or otherwise, can influence the
authorization decision. It is also fail-closed in three separate ways:

- nil/unknown Subject → `matchNothing()`, **without consulting the PDP** (`principalParams` returns
  `ok=false`)
- zero allowed buckets (e.g. an all-deny PDP) → `matchNothing()` — never an unfiltered query
- authenticated → `Should{owner==sub, visibility=="shared"}`; anonymous → `owner==""` only, so the
  anonymous bucket is not a back door to shared records

Both fail-closed paths are already pinned by tests: `TestBulkFilterZeroBucketFailsClosed` and
`TestBulkFilterOrderIndependent` (`internal/store/store_test.go`).

## The correction — architecture research was wrong about the mechanism

There is **no `if scope != ""` conditional in `ownerScopeFilter` today.** It emits
`qdrant.NewMatch("scope", scope)` unconditionally. Passing `scope=""` to `Store.Search` right now
does **not** mean "all scopes" — it means "records whose scope payload is the empty string", which
matches essentially nothing.

So the cross-spine change is not "flip an existing conditional". The conditional must be **added**,
and adding it is the entire mechanical change on the filter side. Pitfalls research was closer to
right: the single-scope path does lean on `scope` being non-empty, though as a *matching* signal
rather than an authz-narrowing one.

Consequence for planning: a test asserting "cross_spine=true returns hits from multiple scopes"
would fail today for a reason unrelated to authz, and a test asserting "scope='' returns nothing"
passes today for the wrong reason. Neither is evidence about the authz gate.

## The shape to mirror is already in this repo — and it is now verified, not assumed

`Store.SearchDiscovery` (`store.go:977-987`) already implements exactly the target composition:

```go
must := []*qdrant.Condition{qdrant.NewMatch("category", "discovery")}
if scope != "" {                                        // conditional scope
	must = append(must, qdrant.NewMatch("scope", scope))
}
if kind != "" {
	must = append(must, qdrant.NewMatch("kind", kind))
}
must = append(must, s.ownerOrSharedCondition(subj))      // unconditional authz
must = append(must, qdrant.NewIsEmpty("superseded_by"))
```

Conditional scope, unconditional authz, same `ownerOrSharedCondition` helper. The requirement's
instruction to mirror `search_discovery`'s `CrossSpine` semantics is therefore safe on the
filter-composition axis — but note this is now a conclusion from *reading both functions*, not the
analogy the pitfalls research warned against. The namespace caveat pitfalls raised is real and
separate: discoveries live in a `discovery:*` scope namespace and are additionally pinned by
`category=="discovery"`; plain memories have no such namespace, so a cross-spine memory search has
a genuinely wider reach than a cross-spine discovery search. That is a *product* consideration
(criterion 5's provenance requirement exists for it), not an authz hole.

## What still must be proven by test, not by reading

Reading establishes the property today. It does not keep it true. Criterion 2 stands unchanged and
is the durable guard:

> A two-owner isolation test against **real Qdrant** (testcontainers, not a mock) proves owner A's
> `cross_spine=true` search over overlapping scope names never returns owner B's private records —
> and it exists and passes **before** the feature is implemented.

"Overlapping scope names" is the load-bearing detail: both owners must have records under scopes
with the *same name*, so a filter that accidentally dropped the owner clause would visibly return
the other owner's records rather than silently returning nothing. A mock cannot establish this —
the composition being verified is Qdrant's, not Go's.

Note the ordering trap: that test must pass *before* the feature exists. Since `scope=""` currently
matches nothing, a naive version of it would pass vacuously today — it would return zero records
because the scope filter excluded everything, not because the authz gate held. The test must
therefore be written so that it fails if the authz clause is removed, and that RED must be observed
by mutating the authz clause, not by toggling the feature.
