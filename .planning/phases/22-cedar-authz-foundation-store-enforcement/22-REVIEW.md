---
phase: 22-cedar-authz-foundation-store-enforcement
reviewed: 2026-07-18T00:08:04Z
depth: deep
files_reviewed: 13
files_reviewed_list:
  - docs/adr/engram-cdr1-cedar-pdp-decides-predicate-store-enforces-qdrant-filter.md
  - internal/authz/authz.go
  - internal/authz/authz_test.go
  - internal/authz/entities.go
  - internal/authz/policies.go
  - internal/authz/policies/defense_empty_owner.cedar
  - internal/authz/policies/own_records.cedar
  - internal/authz/policies/shared_read.cedar
  - internal/authz/policies/tenant_isolate.cedar
  - internal/authz/policy_corpus_test.go
  - internal/authz/schema.json
  - internal/store/store.go
  - internal/store/store_test.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-07-18T00:08:04Z
**Depth:** deep
**Files Reviewed:** 13
**Status:** issues_found

## Summary

The Cedar PDP foundation (`internal/authz`) and its wiring into `internal/store` are
well-designed and the policy corpus is thoroughly regression-tested against the real
embedded `.cedar` bytes (not mocks). I traced the full import graph
(`rg -l "internal/authz"`) and confirmed the ADR's central invariant holds: **only**
`internal/store` imports `internal/authz` — no `internal/server` handler, Connect
handler, or `cmd/engram` command touches the PDP directly, and every owner-based
Qdrant filter condition (`qdrant.NewMatch("owner", ...)`) lives in `store.go` alone.
Both the MCP tool lane (`tools.go`/`rules.go`) and the Connect lane
(`connectapi.go` → `deps.searchMemory`/`d.st.*`) route through the same shared
`*store.Store`, so there is no lane divergence. `DecideBucket` is called with
`authz.ActionRead` only, exactly matching the ADR's stated bulk-recall scope. The
Cedar policy logic itself (forbid-wins, scoped empty-owner defense, shared-read-only,
anonymous-bucket reachability) was traced by hand against all four `.cedar` files and
matches the corpus tests in every case checked, including several edge combinations
not directly asserted by an existing test (own+anonymous, cross-owner+shared,
anonymous+shared-probe).

Three issues carry forward from the standard-depth pass (tracked in GH#394); each was
re-verified against the current code and either confirmed or refined below. One new
cross-file finding emerged from tracing the `authz.Action` verb vocabulary end to end:
`getWritable` — the single gate behind `Delete`, `SetVisibility`, and
`FetchForUpdate`/`Update` — hardcodes `authz.ActionWrite` for what are, at the
handler level, three distinct verbs (write, delete, share). This has zero behavioral
effect under the current four-policy corpus (which is action-blind except for
`shared_read`'s `action == Action::"read"` guard), but it defeats the ADR's stated
purpose for shipping the full `Action` vocabulary "so later ABAC phases add policies,
never actions" (D-05) — a future Phase 23 policy authored against `Action::"delete"`
or `Action::"share"` will silently never fire for `Delete`/`SetVisibility` unless
`store.go`'s call sites are also updated to pass the matching verb.

## Warnings

### WR-01: `Store.DeleteAll` bypasses the PDP entirely (carried from GH#394, reconfirmed)

**File:** `internal/store/store.go:1737-1770`
**Issue:** `DeleteAll` hand-rolls its own `Subject` type switch to derive `owner`
and builds the delete filter directly (`qdrant.NewMatch("owner", owner)`), never
calling `s.decideRecord`/`s.decideBucket` or consulting `internal/authz` at all. I
confirmed via `rg -n 'qdrant.NewMatch\("owner"'` that every other owner-scoped
condition in the package (`ownerOrSharedCondition`, `ownerOnlyCondition`,
`matchNothing`) routes through the PDP, and `DeleteAll` is the sole caller-facing
exception (the remaining raw `NewMatch("owner", ...)` sites — `CountAnonymousBucket`,
`RemapOwner`, `MigrateSetOwner` — are documented operator sweeps with explicitly
no-subject semantics, a different and accepted category, verified against their doc
comments and `cmd/engram/migrate.go`/`prune.go` callers). This currently produces
correct behavior because `own_records.cedar` permits every action unconditionally
when `resource.owner == principal.owner`, so the duplicated type-switch and the PDP
agree today. But it violates the ADR's stated invariant ("internal/store is the sole
enforcement chokepoint: it asks authz for a decision and translates that decision")
and means a future per-category or per-tenant delete restriction (Phase 23) would
silently not apply to bulk `delete_all`, since this path never asks Cedar anything.
**Fix:**
```go
func (s *Store) DeleteAll(ctx context.Context, scope string, subj Subject) (err error) {
	...
	owner, kind, ok := principalParams(subj)
	if !ok {
		return fmt.Errorf("%w: nil subject", ErrNotFound)
	}
	if !s.decideBucket(owner, kind, authz.ActionDelete, authz.BucketOwn).Allow {
		return nil // or an explicit deny error, per product decision
	}
	filter := &qdrant.Filter{Must: []*qdrant.Condition{
		qdrant.NewMatch("scope", scope),
		qdrant.NewMatch("owner", owner),
	}}
	...
}
```

### WR-02: Deny→ErrNotFound message-uniformity is unverified for `getWritable`/`OwnedOrAbsent` (carried from GH#394, refined)

**File:** `internal/store/store.go:1382-1430`, `internal/store/store_test.go:3426-3482`
**Issue:** Re-examined at depth: the *functional* Deny→`ErrNotFound` mapping for
`getWritable`/`OwnedOrAbsent` IS exercised today — via real cross-owner scenarios in
`TestDeleteOwnerGate`, `TestSetVisibilityOwnerGate`, `TestUpdateOwnerGateAndSharedFlag`,
and `TestOwnedOrAbsent` — so the standard-pass framing ("untested") was broader than
the actual gap. What is genuinely missing, and is the more precise restatement: none
of those tests assert the exact error *string*, the way
`TestGetReadableDenyMapsToNotFound` does (`err.Error() != want` against the plain
`fmt.Errorf("%w: %s", ErrNotFound, id)` form) — they only check
`errors.Is(err, ErrNotFound)`. `TestGetReadableDenyMapsToNotFound` is the sole
regression guard for DEC-xa6's "Diagnostic never leaks into the caller-facing error"
invariant (D-10), and it covers only the read gate. A future change that
accidentally threaded `diag` into `getWritable`'s or `OwnedOrAbsent`'s error (e.g.
for debug logging) would pass every existing write-path test while violating D-10,
and nothing in the suite would catch it. There is also no `decideRecordHook`-injected
all-deny test for `getWritable`/`OwnedOrAbsent` on an *owned* record (only the
real-PDP cross-owner path and the absent-id short-circuit, `TestIdAddressedAbsentShortCircuit`,
are covered) — under the current policy corpus this branch is unreachable in
production (`own_records` permits every action unconditionally for the owner), but an
injected-hook test would still catch a future write-path Diagnostic leak the same way
`TestGetReadableDenyMapsToNotFound` catches it for reads.
**Fix:** Add a `getWritable`/`OwnedOrAbsent` analogue of `TestGetReadableDenyMapsToNotFound`
using `decideRecordHook` to force Deny on an owned/existing record, asserting the
exact `err.Error()` equals the plain missing-id form.

### WR-03 (new): `getWritable` conflates write/delete/share into a single `authz.ActionWrite` call — a Phase 23 landmine

**File:** `internal/store/store.go:1382-1395` (definition), called from
`store.go:1455` (`FetchForUpdate`), `store.go:1659` (`SetVisibility`), and
`store.go:1724` (`Delete`)
**Issue:** Traced every caller of `getWritable`: `FetchForUpdate` (backs `Update`,
correctly a write), `SetVisibility` (a share/unshare toggle), and `Delete`. All
three call `s.decideRecord(owner, kind, authz.ActionWrite, ...)` — `getWritable`
hardcodes `authz.ActionWrite` regardless of which of these three verbs the caller
actually represents. Confirmed by `rg` that `authz.ActionDelete` and
`authz.ActionShare` are declared in `authz.go`'s five-verb vocabulary and exercised
in `policy_corpus_test.go`, but are **never once constructed or passed** by
`internal/store` in production code — `authz.ActionSchedule` is likewise dead (new
scheduled records go through the same `OwnedOrAbsent`/write path as `store_memory`,
never a distinct schedule verb; confirmed by tracing `scheduleMemory` in
`internal/server/tools.go` back to the shared `storeMemory` write helper). Under the
current four-policy corpus this is behaviorally invisible: `own_records.cedar`
permits any action unconditionally for the owner and none of the four policies
discriminate on `write` vs `delete` vs `share` vs `schedule`. But the ADR is explicit
that D-05 ships "the full verb list ... so later ABAC phases add policies, never
actions" — i.e. the intent is that a future Cedar policy authored against
`action == Action::"delete"` (e.g. "only an admin role may hard-delete a shared
record") should take effect by adding a `.cedar` file alone. As written today, that
policy would silently never match for `Store.Delete`, `Store.SetVisibility`, or
`Store.OwnedOrAbsent`'s cross-owner-write guard, because they all present themselves
to Cedar as `Action::"write"`. This is exactly the kind of "kind/tenant placeholder
assumption that could bite Phase 23" the review was asked to hunt for: the
vocabulary is declared, but the store-side wiring that is supposed to make it
load-bearing is incomplete for three of the five verbs.
**Fix:** Either (a) thread the actual verb through `getWritable(ctx, id, subj, action)`
and pass `authz.ActionDelete`/`authz.ActionShare`/`authz.ActionWrite` from each
caller respectively, or (b) if collapsing write/delete/share onto one verb is an
intentional simplification for this phase, document that decision explicitly next to
the `Action` constants in `authz.go` (and in the ADR) so Phase 23 doesn't assume the
verb is already correctly threaded when it starts authoring per-action policies.

## Info

### IN-01: `Decision.diag` is captured but never read anywhere (carried from GH#394, reconfirmed)

**File:** `internal/authz/authz.go:44-51`, `internal/authz/authz.go:69-71`
**Issue:** `rg -n '\.diag\b'` across both `internal/authz` and `internal/store`
returns zero matches outside the struct field declaration itself — `Decision.diag`
is populated on every `DecideRecord`/`DecideBucket` call and then discarded. The doc
comment says it "exists solely for future debug-level logging / OTel span
attachment by internal/store," which is a legitimate forward-compat placeholder, but
as of this phase it is fully dead weight: no span attribute, no log line, no test
reads it. This is lower risk than a typical unused-field finding because its
un-exported status already prevents any accidental caller-facing leak (the property
D-10 depends on), but it is worth flagging as effort spent with zero current payoff
on a path this phase itself calls out as sensitive (two `DecideBucket` calls per
`Search`/`List`/etc.).
**Fix:** No action required this phase; if it remains unread by the time
service-principal tenancy work (Phase 23) lands, either wire it into a `store.*` OTel
span attribute on Deny (as originally intended) or remove the field until it has a
consumer.

### IN-02 (new): `store.WithAuthz` is exported but has zero callers anywhere in the repo

**File:** `internal/store/store.go:289-294`
**Issue:** `rg -n 'WithAuthz'` across the whole repo shows `WithAuthz` defined and
referenced only in a doc comment (`store_test.go:3425`, "not a custom *authz.PDP
built through WithAuthz") — no test or production code actually calls it. All
authz-injection tests use the `decideBucketHook`/`decideRecordHook` function-var
seams instead, which the code comments say exist specifically because `*authz.PDP`
"has no exported constructor besides MustDefault." That's a slightly confusing
combination: `WithAuthz` is a public `Option` that requires a `*authz.PDP` callers
can currently only obtain via `MustDefault()` (i.e. it can only be used today to
install another copy of the *same* default policy corpus), while the actually-useful
test-injection mechanism (all-deny / call-counting probes) is the unexported hook
fields. This isn't a bug, but it's unused public API surface with a narrower purpose
than its name suggests ("override the PDP" reads as "inject a custom policy," which
isn't possible without an exported policy-set constructor).
**Fix:** Either exercise `WithAuthz` from a test (e.g. constructing a `Store` with a
`MustDefault()`-backed PDP explicitly, to prove the `Option` wiring itself works) or
note in its doc comment that, absent an exported way to build a non-default
`*authz.PDP`, its only current use is re-installing the default corpus — the hook
fields are the actual test-injection surface.

---

_Reviewed: 2026-07-18T00:08:04Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
