---
phase: 25-supersession-with-history
reviewed: 2026-07-19T00:00:00Z
depth: deep
files_reviewed: 8
files_reviewed_list:
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/server/tools.go
  - internal/server/connecterror.go
  - internal/server/store_iface.go
  - internal/server/fakestore_test.go
  - internal/server/tools_test.go
  - internal/server/connecterror_test.go
findings:
  critical: 3
  warning: 3
  info: 2
  total: 8
status: issues_found
---

# Phase 25: Code Review Report

**Reviewed:** 2026-07-19T00:00:00Z
**Depth:** deep
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Reviewed the new `Store.Supersede` primitive (`internal/store/store.go`) and the
`supersede_memory` MCP handler (`internal/server/tools.go`) plus their error
mapping, interface surface, fakes, and tests, at DEEP depth (cross-file call
chain from `deps.supersedeMemory` → `store.Supersede` → `getWritable`/`Upsert`/
`SetPayload`).

The single-caller happy path, the owner-gate rejection, the already-superseded
rejection, the error-code mapping (`ErrAlreadySuperseded` → `CodeFailedPrecondition`),
and the `Supersedes`/`SupersededBy` payload codec (absent key → nil, never a
stray empty string) are all correctly implemented and reasonably well tested
(`TestSupersedeStamp`, `TestSupersedeOwnerGate`, `TestSupersedeAlreadySuperseded`,
`TestSupersedeForwardChain`, `TestSupersedeRecallGate`, `TestSupersedeMemory`,
`TestConnectError`).

However, three BLOCKER-class defects survive: (1) the check-then-act
already-superseded guard is not atomic and a genuine concurrent-writer race
(distinct from the deletion race `TestSupersedeTOCTOU` actually covers) can
silently create two untracked "corrections" for one target, violating the
tool's own documented "single live head per chain" guarantee with no error to
either caller; (2) `supersede_memory` has no rule-immutability guard, unlike
every other mutation path (`update_memory`, `set_visibility`), so superseding
a `rule`-category record silently drops it out of `list_rules`' "complete rule
set" index without going through the required delete flow; and (3)
`supersede_memory` performs the billable `Embed` call and the Qdrant-hitting
`MintShortID` call *before* the target ownership gate runs, reintroducing
exactly the cost-amplification class the codebase explicitly hardened
`update_memory` against (`TestUpdateMemoryEmbedNotCalledForNonOwner`).

Additional warnings: two other recall paths (`SearchDiscovery`, `ListScheduled`)
were not updated with the new `superseded_by IS EMPTY` gate that `Search` and
`List` received, and `supersede_memory` silently no-ops the `idempotency_key`
field it inherits (via Go embedding) from `storeArgs`, despite the field's
wire-schema description promising replay safety.

## Narrative Findings (AI reviewer)

### Critical Issues

#### CR-01: Concurrent Supersede calls on the same target race past the "already superseded" guard, silently forking the correction chain

**File:** `internal/store/store.go:1780-1818` (`Store.Supersede`)

**Issue:** The already-superseded check is classic check-then-act with no
compare-and-swap and no per-record locking anywhere in the `Store` type (no
`sync.Mutex` exists in the package):

```go
// 1. Owner-only write gate on the TARGET
targetRec, err := s.getWritable(ctx, target, subj, authz.ActionWrite)
...
// 2. Single-hop / cycle rejection
if targetRec.SupersededBy != nil && *targetRec.SupersededBy != "" {
    return fmt.Errorf("%w: %s", ErrAlreadySuperseded, target)
}
// 3. Store the new record
if err := s.Upsert(ctx, newMem, vec); err != nil { return err }
// 4. Back-stamp the target
_, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
    Payload: qdrant.NewValueMap(map[string]any{"superseded_by": newMem.ID}),
    ...
})
```

Two concurrent `Supersede(target)` calls (e.g. two agent sessions racing to
correct the same fact) can both read `target.SupersededBy == nil` at step 2
before either reaches step 4. Both then succeed at step 3 (independent new
records N1, N2) and both succeed at step 4 (`SetPayload` on the *same* target
key — last write wins, and Qdrant's point-ID-selector `SetPayload` has no
optimistic-concurrency/version guard). Result: the target ends up with
`superseded_by == N2` (say), N1 persists as a completely normal, fully
visible, un-linked "correction" of the same original claim — the tool's own
docstring promise ("Rejects if the target is already superseded (single live
head per chain)") is silently violated, and **neither caller receives an
error**. This is a data-integrity bug, not a documented tradeoff: the doc
comment on `Store.Supersede` (store.go:1766-1779) only discusses the
*partial-failure* case (step 4 erroring after step 3 succeeds) — it never
addresses two *successful* concurrent callers.

The existing `TestSupersedeTOCTOU` (store_test.go:2800) does **not** cover
this: it only exercises the target being *deleted* between the gate and the
back-stamp, never two concurrent *writers* both passing the guard. There is no
test anywhere in this diff that spins up concurrent `Supersede` calls on one
target.

**Fix:** Make the back-stamp a compare-and-swap, e.g. add a Qdrant
`ConditionalPayload`/version check on `superseded_by` being absent at
write-time (or re-`Get` the target immediately before the `SetPayload` inside
a retry loop and fail with `ErrAlreadySuperseded` if it changed), and add a
concurrency test that fires two goroutines at the same target and asserts
exactly one succeeds and the other returns `ErrAlreadySuperseded` (or an
explicit conflict error) rather than both succeeding.

---

#### CR-02: `supersede_memory` has no rule-immutability guard — superseding a rule silently removes it from `list_rules`

**File:** `internal/server/tools.go:1256-1285` (`deps.supersedeMemory`)

**Issue:** Every other mutation path that can touch a `rule`-category record
explicitly rejects it:

```go
// updateMemory, tools.go:1116
if cur.Category == "rule" { ... return errRuleImmutable ... }
// setVisibility, tools.go:1233
if rec.Category == "rule" { return mutationResult{}, fmt.Errorf("%w — delete the rule instead of changing its visibility", errRuleImmutable) }
```

`supersedeMemory` has no equivalent check on the resolved target's category.
Since `Store.Supersede`'s `getWritable` call already fetches the full target
record (`targetRec`), a caller who owns a rule can call `supersede_memory`
with `supersedes` pointing at the rule's id/short_id and it will happily
back-stamp `superseded_by` onto the rule.

The consequence is worse than a simple gap: `list_rules` (rules.go:184-217)
is implemented as `d.st.List(..., Categories: []string{"rule"}, ...)`, and
`Store.List` unconditionally applies `qdrant.NewIsEmpty("superseded_by")`
(store.go:1049). So the instant a rule is superseded, it silently vanishes
from `list_rules`' "COMPLETE rule set" index (per the tool description and
the project's memory contract: "`list_rules` returns the complete set...
Rules surface at session start as a progressive-disclosure index") — with no
delete having occurred, no `errRuleImmutable` rejection, and no test coverage.
An agent restarting a session will simply never see that normative rule
again. `get_memory` on the rule's id would still show it (ungated), but
nothing prompts an operator to look there.

**Fix:** Add the same guard `supersedeMemory` is missing, gated on the
resolved target's category (available from `Store.Supersede`'s internal
`getWritable` result, or by having the handler fetch/inspect it before
calling `Store.Supersede`):

```go
if targetRec.Category == "rule" {
    return "", "", fmt.Errorf("%w — delete the rule instead of superseding it", errRuleImmutable)
}
```

and add a test mirroring `TestUpdateMemoryPreservesSharingHandler`'s rule
guard that asserts superseding a rule is rejected and the rule stays present
in `list_rules`.

---

#### CR-03: `supersede_memory` embeds and mints a short id *before* the target ownership gate — cost-amplification regression

**File:** `internal/server/tools.go:1256-1272` (`deps.supersedeMemory`)

**Issue:** The call order is:

```go
targetID, err := d.st.ResolvePointID(ctx, a.Supersedes)   // owner-agnostic
...
m := a.toMemory(owner, c.Actor, d.clock())
m.Supersedes = &targetID
vec, err := d.em.Embed(ctx, store.EmbedText(m.Content, m.Tags))       // BILLABLE, unconditional
...
if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil { ... }    // Qdrant Count round-trip(s)
if err := d.st.Supersede(ctx, m, vec, targetID, c.Subj); err != nil { // owner-gate runs HERE
```

`ResolvePointID` is owner-agnostic by design, and the actual write-ownership
check (`getWritable(target, ActionWrite)`) only runs *inside*
`Store.Supersede`, i.e. after the embed and the short-id mint have already
happened. This exactly reproduces the vulnerability class the codebase
explicitly hardened `update_memory` against — see
`TestUpdateMemoryEmbedNotCalledForNonOwner` (tools_test.go:2037) and its
comment: "verifies that updateMemory does NOT invoke the embedder when the
caller does not own the record (cost-amplification hardening for
eu8.4/eu8.2)" — and the same pattern `storeDiscovery`'s replace path already
follows (`OwnedOrAbsent` runs at tools.go:862, strictly before `d.em.Embed` at
tools.go:878).

Any authenticated caller can supply another owner's memory id (a UUID or a
known/guessed short_id) as `supersedes` and force the server to perform a
billable `Embed` HTTP call plus at least one Qdrant `Count` RPC
(`MintShortID`) before being rejected with `ErrNotFound` — a cost/DoS vector
against the operator's embedding-API spend, with zero test coverage analogous
to `TestUpdateMemoryEmbedNotCalledForNonOwner`.

**Fix:** Resolve the target and run the ownership gate (e.g. a
`FetchForUpdate`/`getWritable`-style call) before calling `d.em.Embed`,
mirroring `updateMemory`'s and `storeDiscovery`'s ordering; add a
`TestSupersedeMemoryEmbedNotCalledForNonOwner` test using a counting embedder.

### Warnings

#### WR-01: `SearchDiscovery` does not apply the `superseded_by` soft-hide gate

**File:** `internal/store/store.go:888-924`

**Issue:** `Search` (store.go:826) and `List` (store.go:1049) both append
`qdrant.NewIsEmpty("superseded_by")` to soft-hide superseded records.
`SearchDiscovery`'s filter (store.go:907-914) does not. Since nothing in
`supersede_memory` prevents a discovery-category target from being superseded
(see CR-02's sibling issue — no category restriction at all), a superseded
discovery remains fully visible via `search_discovery`, contradicting the
documented soft-hide guarantee for the feature.

**Fix:** Add `qdrant.NewIsEmpty("superseded_by")` to `SearchDiscovery`'s
`must` conditions (store.go:907-914), and add a test analogous to
`TestSupersedeRecallGate` scoped to `SearchDiscovery`.

#### WR-02: `ListScheduled` does not apply the `superseded_by` soft-hide gate

**File:** `internal/store/store.go:1247-1294`

**Issue:** `Store.Supersede`'s ownership gate (`getWritable`→`Get`) is
time-window-agnostic — it can supersede a not-yet-active or already-expired
scheduled memory (`Get`/`getWritable` deliberately bypass
`activeWindowConditions`, matching the "`get_memory` is not recall-gated"
design intent). `ListScheduled`'s filter (store.go:1272-1276) only applies
`ownerOnlyCondition` + `scheduledStateCondition`, never the `superseded_by`
gate `Search`/`List` carry. A scheduled-but-already-superseded record
therefore still surfaces in `list_scheduled`'s management view as if it were
still a live pending/expired candidate — a caller managing their scheduled
memories has no signal that a listed record has already been corrected away.

**Fix:** Add the same `qdrant.NewIsEmpty("superseded_by")` condition to
`ListScheduled`'s filter, or explicitly document/test why a scheduled record
that has been superseded should still surface there.

#### WR-03: `supersede_memory` silently no-ops the inherited `idempotency_key` field

**File:** `internal/server/tools.go:492-495` (`supersedeArgs`), `1256-1285`
(`deps.supersedeMemory`)

**Issue:** `supersedeArgs` embeds `storeArgs` (tools.go:492-495), and the
doc comment explicitly frames this as intentional field-set parity: "so
`supersede_memory` inherits the full `store_memory` field set
(content/scope/source/category/tags/repo/workspace/worktree/base_dir/
summary/**idempotency_key**)". Because of Go field promotion, the MCP tool's
reflected JSON schema for `supersede_memory` therefore advertises
`idempotency_key` with its `store_memory`-inherited description ("a repeat
call with the same key and identical content returns the original record
unchanged... omit for a fresh record every time").

However `deps.supersedeMemory` never calls `checkIdempotentReplay`, never
sets `pointID`, and never stamps `IdempotencyFingerprint` — the field is
read off the wire and then completely ignored. A client that (reasonably,
given the schema and its own documented behavior on the sibling tools) relies
on `idempotency_key` for safe supersede retries will get a **new**
`Upsert`+back-stamp attempt on every retry; the first retry after a
transient timeout will fail with `ErrAlreadySuperseded` (since the original
call's back-stamp already landed) rather than returning the original id/
short_id as store_memory/schedule_memory do — a silent behavioral
divergence from the documented contract, not a no-op the client can safely
assume.

**Fix:** Either wire `checkIdempotentReplay`/`IdempotencyFingerprint` through
`supersedeMemory` for parity with `storeMemory`/`scheduleMemory`, or drop
`IdempotencyKey` from `supersedeArgs`'s effective schema (e.g. a
`json:"-"`-shadowing field, or exclude it explicitly from the reflected
schema) and update the doc comment — do not leave a schema-advertised field
silently inert.

### Info

#### IN-01: No test verifies the target's stored *vector* survives `Supersede`

**File:** `internal/store/store_test.go` (`TestSupersedeStamp`,
`TestSupersedeRecallGate`)

**Issue:** `TestSupersedeStamp` asserts `Content`/`Tags`/`Visibility` survive
the back-stamp (proving `SetPayload` merges rather than replaces), but no
test asserts the target's *vector* is unchanged. `Store.Get`
(store.go:1357-1359) requests `WithPayload: true` only — it never fetches
vectors — so `store.Memory` has no vector field to assert against via the
existing helpers. The vector-preservation claim in `Store.Supersede`'s doc
comment ("single-key `SetPayload`, vector-preserving") is currently backed
only by trust in Qdrant's documented `SetPayload` semantics, not by a direct
assertion in this test suite.

**Fix (optional, low priority):** Add a targeted test using the raw
`s.client.Get(..., WithVectors: qdrant.NewWithVectors(true))` before/after
`Supersede` to directly assert the target's stored vector bytes are
byte-identical, closing the gap between the documented guarantee and what is
actually tested.

#### IN-02: `supersede_memory` handler tests do not exercise rule/discovery targets or the non-owner cost path

**File:** `internal/server/tools_test.go` (`TestSupersedeMemory`,
line 1865)

**Issue:** `TestSupersedeMemory` is the only handler-level test for
`supersede_memory`. It covers the happy path and the cross-owner
`ErrNotFound` re-wrap, but — consistent with CR-02/CR-03 above — never
exercises: superseding a `rule`-category or `discovery`-category target,
`idempotency_key` behavior (WR-03), or the non-owner cost-amplification path
(`TestUpdateMemoryEmbedNotCalledForNonOwner`'s pattern, which would have
caught CR-03 directly). These are exactly the edge cases this new feature
needed coverage for, and their absence is why CR-02/CR-03/WR-03 shipped
undetected.

**Fix:** Add the four missing test cases once CR-02/CR-03/WR-03 are
addressed (or to document the current behavior precisely, if any of them are
deliberately deferred).

---

_Reviewed: 2026-07-19T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
