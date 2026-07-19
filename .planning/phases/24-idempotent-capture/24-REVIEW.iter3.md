---
phase: 24-idempotent-capture
reviewed: 2026-07-18T16:55:00Z
depth: deep
files_reviewed: 8
files_reviewed_list:
  - internal/server/connecterror.go
  - internal/server/connecterror_test.go
  - internal/server/idempotency.go
  - internal/server/idempotency_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/store.go
  - internal/store/store_test.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 24: Code Review Report (Re-review, Iteration 2)

**Reviewed:** 2026-07-18T16:55:00Z
**Depth:** deep
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Re-reviewed the idempotent-capture code after the prior fixer pass's 6 changes
(CR-01 short_id read-back, WR-01 injective tag encoding, WR-02 replay-before-
window-validation ordering, IN-01 key-size cap, plus a new `connecterror_test.go`
acceptance-table case and a `store.go` doc-only comment). This is an
independent re-review — none of the prior findings or fixes were assumed
correct; each was re-traced from scratch and the previous CRITICAL/WARNING
findings from iteration 1 were re-verified as actually resolved, not just
claimed resolved.

**All 4 prior code fixes verified correct on independent trace + test run
(including `-race`):**

- `idempotencyPointID` and `contentFingerprint` now use the *same*
  length-prefixed (netstring-style) encoding discipline throughout, including
  the per-tag encoding fix. Decoding such an encoding is unambiguous
  regardless of field/tag content (length is read first, then exactly that
  many bytes are consumed) — this is genuinely injective, closing the
  `\x1f`-separator collision from iteration 1's WR-01.
  `TestContentFingerprintTagsBoundaryShiftInjective` passes.
- `checkIdempotentReplay` in `scheduleMemory` now runs strictly before
  `parseWindow`, so a delayed retry whose `not_after` has since lapsed is
  still recognized as a replay instead of being wrongly rejected
  (`TestScheduleMemoryIdempotentRetryAfterWindowLapses` passes). Traced both
  `storeMemory` and `scheduleMemory` line by line: no path reaches
  `d.em.Embed` before `checkIdempotentReplay` has resolved (replay, create,
  or conflict) on either handler, on any code path.
- `maxIdempotencyKeyBytes` (512) is enforced as the very first check inside
  `checkIdempotentReplay`, before `idempotencyPointID` is computed and before
  any store call, and wraps `store.ErrInvalidArgument` (→ `CodeInvalidArgument`).
  Confirmed by `TestStoreMemoryIdempotencyKeyTooLarge`, which uses a bare
  `&deps{}` (nil `st`/`em`) — it would nil-pointer-panic if the bound check
  ran any later than it does.
- The new `store.go` `IdempotencyFingerprint` doc comment ("frozen at create
  time") is accurate: traced `Store.Update` and `Store.UpdatePayload` — both
  correctly preserve `cur.IdempotencyFingerprint` verbatim (`Update`
  re-Upserts `cur`'s existing field unmodified; `UpdatePayload`'s targeted
  `SetPayload` writes only its own key set and never touches
  `idempotency_fingerprint`).
- `TestConnectError`'s new `idempotency_conflict` case is correctly wired and
  passes.

Owner-scoping (no cross-tenant point-ID poisoning), reject-on-mismatch
(never upsert-overwrite on a fingerprint mismatch), and check-before-embed
ordering all hold on every path traced, on both handlers.

**One quality regression was introduced by the CR-01 fix** (scope creep: the
fix now runs on every write, not just the keyed race it targets — see WR-01
below). Two informational notes round out the findings. No new correctness
or security defects, and no previously-fixed issue has regressed.

## Warnings

### WR-01: CR-01's short_id read-back now runs unconditionally on every write, not just the keyed race it targets

**File:** `internal/server/tools.go:731-752` (`persistAndEnqueue`)

**Issue:** The CR-01 fix (iteration 1 → iteration 2) added a re-`Get` after
`Upsert` so a concurrent *keyed* racer returns the short_id that was actually
persisted rather than the one it locally minted and discarded. The fix's own
doc comment frames this entirely in terms of "a concurrent keyed racer" —
but the code has no gate on `pointID != ""` (i.e., whether this write used
an idempotency key at all):

```go
if err := d.st.Upsert(ctx, m, vec); err != nil {
    return "", "", err
}
// Re-read the point after Upsert so a concurrent keyed racer that lost the
// last-write-wins race (same deterministic pointID, independently minted
// short_id) returns the short_id that was ACTUALLY PERSISTED, not the one
// it discarded (CR-01). ...
if persisted, gerr := d.st.Get(ctx, m.ID); gerr == nil {
    m.ShortID = persisted.ShortID
}
```

`persistAndEnqueue` is the shared tail for *every* `store_memory` and
`schedule_memory` call, keyed or not (both call it identically). For a
keyless write, `m.ID` is a fresh `uuid.NewString()` that no concurrent
request can ever target — the race this `Get` exists to resolve is
structurally impossible on that path — so the extra synchronous Qdrant round
trip after every `Upsert` is pure added latency on the overwhelmingly common
case (every keyless write). No test in `tools_test.go` asserts on `Get` call
counts, so this scope creep from "fix the keyed race" to "always re-Get" is
not caught by the existing suite (verified: no `spyStore.callLog()`
assertion anywhere in `tools_test.go` filters on `Method == "Get"`).

**Fix:** Gate the read-back on the keyed path only, matching the doc
comment's actual stated intent:

```go
if err := d.st.Upsert(ctx, m, vec); err != nil {
    return "", "", err
}
if m.IdempotencyFingerprint != "" { // only keyed writes stamp this
    if persisted, gerr := d.st.Get(ctx, m.ID); gerr == nil {
        m.ShortID = persisted.ShortID
    }
}
```

`m.IdempotencyFingerprint` is already only ever non-empty on the keyed
create path (set in `storeMemory`/`scheduleMemory` right before calling
`persistAndEnqueue`), so this requires no new parameter — just a condition
using state `persistAndEnqueue` already receives.

## Info

### IN-01: Idempotency key namespace is shared across store_memory and schedule_memory, untested for the cross-tool case

**File:** `internal/server/tools.go:678-697` (`checkIdempotentReplay`), `:764-804` (`scheduleMemory`)

**Issue:** `IdempotencyKey` is promoted onto `scheduleArgs` via embedding
`storeArgs`, and `checkIdempotentReplay(ctx, owner, a.storeArgs)` derives the
point ID from `(owner, scope, key)` alone, with no signal for which *tool*
originally created the record. A `store_memory` call with
`idempotency_key=K` followed later by a `schedule_memory` call using the
same `scope`+`K`+identical content will hit the fingerprint match and return
`replay=true`, silently returning the original (unscheduled) record's
id/short_id with **no window ever applied** — the caller gets a success
response indistinguishable from a genuinely scheduled write. The existing
tests (`TestScheduleMemoryIdempotentIgnoresWindowChange`,
`TestScheduleMemoryIdempotentRetryAfterWindowLapses`) only exercise the
same-tool (`schedule_memory` → `schedule_memory`) case; there is no test for
`store_memory` → `schedule_memory` (or the reverse) sharing a key. This may
be intentional — the design already treats the schedule window as excluded
from the replay fingerprint by conscious choice (the "Open Question 1"
comment) — but the cross-tool variant is a materially more surprising
outcome than a same-tool window-only replay, and neither tool's MCP
description calls it out.

**Fix:** Either (a) add a test pinning the cross-tool behavior as
intentional, so a future change can't silently alter it unnoticed, or (b)
fold a tool-identity discriminator into the point-ID hash input so the two
tools' key namespaces are structurally disjoint, if the shared-replay-across-
tools behavior is not actually desired.

### IN-02: Anonymous callers share a single idempotency-key bucket

**File:** `internal/server/idempotency.go:24-37` (`idempotencyPointID`)

**Issue:** `checkIdempotentReplay` derives `owner := c.Subj.Owner()`, which
is `""` for every anonymous caller (no OIDC issuer configured). Since
`idempotencyPointID` hashes `owner` directly into the point ID, *all*
anonymous callers using the same `scope`+`idempotency_key` collide onto the
same deterministic point — one anonymous caller's write with a given key
could trigger `ErrIdempotencyConflict` against a completely unrelated
anonymous caller's earlier write under that same key, or (if content happens
to match) silently return another caller's record id/short_id. This is
consistent with the project's existing documented single-anonymous-bucket
invariant (CLAUDE.md: "No issuer → single anonymous bucket (`owner==""`)"),
so it is not a new defect introduced by this phase, but the
`idempotencyPointID`/`checkIdempotentReplay` doc comments' claim that "owner
is part of the hash input... so cross-owner collision is structurally
impossible (D-09)" doesn't call out that this guarantee collapses to a
single shared bucket in the anonymous/auth-disabled case.

**Fix:** Optional — add a one-line cross-reference in `idempotencyPointID`'s
doc comment to the anonymous-bucket caveat, so a future reader doesn't
assume owner-scoping holds unconditionally.

---

_Reviewed: 2026-07-18T16:55:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
