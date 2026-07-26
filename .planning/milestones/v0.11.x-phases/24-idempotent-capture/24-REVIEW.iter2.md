---
phase: 24-idempotent-capture
reviewed: 2026-07-18T00:00:00Z
depth: deep
files_reviewed: 7
files_reviewed_list:
  - internal/server/connecterror.go
  - internal/server/idempotency.go
  - internal/server/idempotency_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/store.go
  - internal/store/store_test.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 24: Code Review Report

**Reviewed:** 2026-07-18T00:00:00Z
**Depth:** deep
**Files Reviewed:** 7
**Status:** issues_found

## Summary

The core idempotency design is sound: `idempotencyPointID`'s length-prefixed encoding of
`(owner, scope, key)` is genuinely injective (verified by hand and cross-checked against
`TestIdempotencyPointIDBoundaryShiftInjective`), owner is baked into the hash input rather than
applied as a post-hoc filter (no cross-tenant point-ID poisoning), `checkIdempotentReplay` runs
strictly before `d.em.Embed` on every path in both `storeMemory` and `scheduleMemory`, and a
fingerprint mismatch always rejects via `store.ErrIdempotencyConflict` — no path upserts on
mismatch. `connectError`'s new arm is correctly positioned and correctly typed.

Two real defects were found under the "concurrency" and "injectivity" deep-focus areas the phase
asked for, plus a schedule_memory retry-safety gap and a test-coverage gap for the new error arm:

1. **Critical** — under a genuine concurrent-identical-key race (the exact scenario the SC4 test
   exercises), each racer independently mints its own `short_id` before the deterministic Upsert;
   Qdrant's Upsert fully replaces the payload, so the "losing" racer's returned `short_id` is
   never persisted and will not resolve on a later lookup.
2. **Warning** — `contentFingerprint`'s tag-joining step uses a raw, unescaped separator
   (`\x1f`) rather than the same injective length-prefixed scheme `idempotencyPointID` uses for
   its own components, so two distinct tag sets can be engineered to fingerprint identically,
   defeating SC2 ("same key + different content is rejected") for the tags field specifically.
3. **Warning** — `scheduleMemory` runs `parseWindow`'s "not_after must be in the future" check
   *before* `checkIdempotentReplay`, so a delayed retry of an already-successful scheduled write
   can be rejected with `ErrInvalidArgument` instead of returning the original record, even though
   the window is deliberately excluded from the replay decision.
4. **Warning** — `connecterror_test.go`'s acceptance table (which its own doc comment says covers
   "every mapping arm") was not extended with a case for the new
   `store.ErrIdempotencyConflict` → `CodeAlreadyExists` arm added to `connecterror.go`.

## Critical Issues

### CR-01: Concurrent identical-key writes can return a short_id that never resolves

**File:** `internal/server/tools.go:671-732` (`checkIdempotentReplay`, `storeMemory`,
`scheduleMemory`, `persistAndEnqueue`)

**Issue:** When two (or more) callers race with the *same* `idempotency_key` and identical
content — precisely the scenario `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint` (SC4)
exercises, and the scenario the idempotency feature exists to make safe (a client retrying after
a lost response) — both racers observe `store.ErrNotFound` from `checkIdempotentReplay`'s `Get`
call (neither sees the other's not-yet-written record), and both fall through to
`persistAndEnqueue`:

```go
func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) {
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```

`m.ID` is identical for both racers (the deterministic `pointID`), but `MintShortID` is called
independently by each, producing two *different* short ids (SID_A, SID_B). Both then
`Upsert` to the same point ID. `Store.Upsert` writes a full replacement payload
(`qdrant.NewValueMap(payload(m))`, `internal/store/store.go:589-597`) — it does not merge — so
whichever Upsert lands last wins the entire payload, including `short_id`. The racer whose write
was overwritten still returns its own locally-minted `short_id` from `persistAndEnqueue`
(`return m.ID, m.ShortID, nil`), which was **never actually persisted**. A later
`get_memory`/`ResolvePointID` call using that returned short id will 404
(`store.go:1350-1366`, no record carries that `short_id` in the payload index), even though the
tool call that "created" it reported success.

This directly contradicts the documented contract on `Memory.ShortID` — "minted alongside ID and
usable anywhere an id is accepted" (`internal/store/store.go:138-141`) — and the MCP tool
descriptions' promise that "The result includes the memory's id and short_id" (`tools.go:1204`,
`1214`). `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint` only asserts `r.id == firstID`
across the 20 racers (`tools_test.go:912-921`); it never asserts `r.shortID == firstShortID`, so
this gap is not caught by the existing suite even though it directly exercises the triggering
race.

This shares its root cause with `storeDiscovery`'s pre-existing replace-path race (which also
mints a fresh short id when its `Get` misses), but that path only races when a caller explicitly
supplies the same UUID twice concurrently — a rare operator/tooling scenario. The idempotent
write path makes the trigger condition the *common* case: concurrent retries of the same
`idempotency_key` are exactly what this feature is meant to make safe.

**Fix:** Make the returned `short_id` always reflect what is actually persisted, not what the
local goroutine happened to mint. The simplest fix mirrors `storeDiscovery`'s
`carriedShortID` pattern: after `Upsert` for a *keyed* write, re-`Get` the point and return its
persisted `ShortID` rather than the locally-minted one — or, more robustly, have
`checkIdempotentReplay`'s "absent" branch not commit to creating at all; instead attempt the
Upsert and then immediately re-fetch by `pointID` to discover the winning payload's actual
`short_id`, e.g.:

```go
func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) {
	if m.ShortID, err = d.st.MintShortID(ctx, nil); err != nil {
		return "", "", err
	}
	if err := d.st.Upsert(ctx, m, vec); err != nil {
		return "", "", err
	}
	// Re-read the point after Upsert so a concurrent keyed racer that lost
	// the last-write-wins race returns the SHORT_ID THAT WAS ACTUALLY
	// PERSISTED, not the one it discarded.
	if persisted, gerr := d.st.Get(ctx, m.ID); gerr == nil {
		m.ShortID = persisted.ShortID
	}
	d.summaryQueue.tryEnqueue(m.ID)
	return m.ID, m.ShortID, nil
}
```

(A narrower fix scoped to only the keyed path is also acceptable, but the above keeps the
keyless path's extra round trip negligible-cost and removes the divergence class entirely.)

## Warnings

### WR-01: contentFingerprint's tag-joining is not injective — a crafted tag can fake a content match

**File:** `internal/server/idempotency.go:50-63`

**Issue:** `idempotencyPointID` deliberately uses a length-prefixed encoding so that
`(owner, scope, key)` boundaries can never shift into a collision (correctly verified by
`TestIdempotencyPointIDBoundaryShiftInjective`). `contentFingerprint` does not apply the same
discipline to its `tags` component — it joins tags with a raw separator before treating the
joined string as one length-prefixed field:

```go
tags := slices.Clone(a.Tags)
slices.Sort(tags)

var b strings.Builder
for _, f := range []string{
	a.Content, a.Category, strings.Join(tags, "\x1f"),
	...
```

`strings.Join(tags, "\x1f")` is **not** injective over the tag *slice*: a single tag containing a
literal `0x1F` byte (reachable via JSON `""` on the MCP wire — nothing sanitizes tag
content) collapses to the same joined string as two separate tags split at that byte. Concretely,
`tags=["a","b"]` and `tags=["a\x1fb"]` both join to `"a\x1fb"`, so both fingerprint identically
if every other field matches. A caller could therefore submit a `store_memory` replay with the
*same* `idempotency_key`, identical `content`/`category`/etc., but an objectively different tag
set — engineered to collide via the separator byte — and instead of the documented
"same key + different content is rejected" (SC2), `checkIdempotentReplay` treats it as a matching
replay and silently returns the original record, discarding the caller's actual (different) tags
with no error. This is exactly the class of delimiter-injection bug the phase's own design
avoided in `idempotencyPointID` but did not carry over to `contentFingerprint`.

**Fix:** Apply the same per-field length-prefixed encoding to the tags list instead of a raw
join, e.g.:

```go
var tagsEnc strings.Builder
for _, t := range tags {
	fmt.Fprintf(&tagsEnc, "%d:%s:", len(t), t)
}
// ... use tagsEnc.String() as the "tags" field instead of strings.Join(tags, "\x1f")
```

### WR-02: schedule_memory's future-window validation can reject a legitimate idempotent retry

**File:** `internal/server/tools.go:744-761` (`scheduleMemory`)

**Issue:**

```go
func (d *deps) scheduleMemory(ctx context.Context, c caller, a scheduleArgs) (string, string, error) {
	now := d.clock()
	nb, na, err := parseWindow(a, now)
	if err != nil {
		return "", "", err
	}
	owner := c.Subj.Owner()
	replay, id, shortID, pointID, err := d.checkIdempotentReplay(ctx, owner, a.storeArgs)
	...
```

`parseWindow` (`tools.go:456-484`) rejects `not_after` values that are not strictly in the future
*relative to the current call's `now`* (`!t.After(now)`), and this validation runs before
`checkIdempotentReplay` ever gets a chance to recognize the call as a replay. The window is
deliberately excluded from the replay fingerprint (documented D-07/Open Question 1, pinned by
`TestScheduleMemoryIdempotentIgnoresWindowChange`) precisely so a retry doesn't need to resend a
window that is still valid. But if a client's original `schedule_memory` call succeeded, the
response was lost (the exact scenario idempotency keys exist to make safe), and the client
retries with the *same* `not_after` value after enough wall-clock time has passed that it is no
longer in the future, `parseWindow` now rejects the retry with `ErrInvalidArgument` — before
`checkIdempotentReplay` can recognize this as an already-completed write and return the original
record. The retry fails even though the record was already correctly persisted and a true replay
should be a no-op per SC1.

**Fix:** Run `checkIdempotentReplay` before (or independent of) `parseWindow`'s future-only check
when a key is present, e.g. resolve/attempt the replay match first and only apply the
future-in-the-future validation on the non-replay (create) path:

```go
owner := c.Subj.Owner()
replay, id, shortID, pointID, err := d.checkIdempotentReplay(ctx, owner, a.storeArgs)
if err != nil {
	return "", "", err
}
if replay {
	return id, shortID, nil
}
now := d.clock()
nb, na, err := parseWindow(a, now)
if err != nil {
	return "", "", err
}
```

### WR-03: connecterror_test.go's acceptance table was not extended for the new sentinel arm

**File:** `internal/server/connecterror_test.go:36-52`

**Issue:** `connecterror.go` gained a new mapping arm this phase
(`store.ErrIdempotencyConflict` → `connect.CodeAlreadyExists`, `connecterror.go:60-65`).
`TestConnectError`'s own doc comment states it is "the mapper's own acceptance table: every
mapping arm listed on connectError's doc comment" — but the `cases` table
(`connecterror_test.go:36-52`) was not updated with a corresponding entry, so the new arm has no
direct unit-test coverage in the one place designed to guarantee every arm stays covered. A
future reordering of the `switch` in `connectError` (e.g. accidentally letting
`ErrIdempotencyConflict` fall through to a different arm) would not be caught here.

**Fix:** Add a case mirroring the others:

```go
{"idempotency_conflict", fmt.Errorf("idempotency key %q reused with different content: %w", "k", store.ErrIdempotencyConflict), connect.CodeAlreadyExists},
```

## Info

### IN-01: idempotency_key has no size bound

**File:** `internal/server/tools.go:435-439`

**Issue:** `storeDiscoveryArgs` enforces explicit size caps on client-supplied fields
(`maxDiscoveryContentBytes`, `maxCitationExcerptBytes`, `maxDiscoveryCitations`,
`tools.go:560-564`), but the new `storeArgs.IdempotencyKey` field has no analogous bound. It is
hashed into the point ID and fingerprint input, so an arbitrarily large key value is cheap to
process, but it's an inconsistency with the size-bound discipline already established elsewhere
in this file for other client-supplied strings.

**Fix:** Consider a modest cap (e.g. a few hundred bytes) consistent with the field's purpose as
a short opaque retry token, purely for defense-in-depth against oversized payloads.

### IN-02: IdempotencyFingerprint goes stale after update_memory changes content

**File:** `internal/store/store.go:1501-1541` (`Store.Update`) /
`internal/server/tools.go:671-687` (`checkIdempotentReplay`)

**Issue:** `Store.Update` re-Upserts the fetched `cur` Memory (which carries the original
`IdempotencyFingerprint` from creation time) with new `Content`, but never recomputes
`IdempotencyFingerprint`. This is not incorrect per se — `update_memory` is a distinct,
explicit mutation path outside the idempotent-capture contract — but it means a future
`store_memory` replay using the *original* creation-time content will still fingerprint-match
against a record whose actual current content has since diverged via `update_memory`, silently
returning the (now stale-relative-to-its-own-history) id/short_id. Worth a one-line doc note on
`IdempotencyFingerprint` so a future reader doesn't assume it tracks the record's current state.

**Fix:** Documentation-only; no behavior change required. Add a sentence to the
`IdempotencyFingerprint` field doc comment noting it reflects only the original keyed
create-time payload and is not updated by `update_memory`.

---

_Reviewed: 2026-07-18T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
