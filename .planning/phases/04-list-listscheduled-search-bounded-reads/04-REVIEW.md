---
phase: 04-list-listscheduled-search-bounded-reads
reviewed: 2026-09-20T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - internal/store/store.go
  - internal/store/searchfetch.go
  - internal/store/orderedpage.go
  - internal/store/boundedread.go
  - internal/store/storetest/seed.go
  - internal/server/tools.go
  - internal/server/connectapi.go
  - internal/server/rules.go
  - internal/server/argerror.go
  - internal/server/connecterror.go
  - internal/server/responsetoolarge.go
  - cmd/engram/client_list.go
  - cmd/engram/client_search.go
  - proto/engram/v1/engram.proto
findings:
  critical: 1
  warning: 2
  info: 1
  total: 4
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-09-20T00:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

The phase's core two-phase-search and bounded-ordered-page machinery is sound:
`includeIDs`/`excludeSeen` are correctly narrow-only, the vector-query filter and
the payload-fetch filter are always the identical `*qdrant.Filter` value (no
authz widening between phases), `rejectOverMaximum`/`rejectOverMaximumCount`
run before any Qdrant/embed call at every recall entry point, and the `Full`
projection flag is threaded consistently from wire request through
`ListOptions`/`SearchOptions` into the store's fetch view on every surface that
exposes one.

However, comparing this phase's diff against the pre-phase behavior surfaced a
genuine pagination-correctness regression: **D-01's "zero limit resolves to
1000, never 'all'" change, combined with the pre-existing cursor-mode
selection guard, causes a Connect (or any non-MCP) `cursor_mode=true,
limit=0` list request to silently drop every record past the 1000th while
reporting an empty (exhausted) `next_cursor`.** Before this phase, the same
input combination fell through to the same code path but that path fetched
literally `total` records ("limit 0 = all"), so the empty cursor was
correct; the D-01 change caps that same fallback at 1000 without also
routing it into cursor-mode pagination, breaking the "next_cursor empty only
when genuinely exhausted" contract. See CR-01.

Two lower-severity issues are also worth fixing: `fetchPayloadsByID`
(search's two-phase fetch) omits the batch-of-1 legacy-oversized-record
fallback that `scrollOrderedPage` and `scrollAllPoints` both implement for
the identical class of pre-cap record, so a single such record among a
search hit set fails the whole containing batch rather than being isolated
(WR-01); and a doc comment in `connectapi.go` still asserts the pre-phase
"limit=0 means all" semantics this very phase replaced (WR-02).

## Critical Issues

### CR-01: `cursor_mode=true, limit=0` silently truncates a list past 1000 records and reports it as the last page

**Fix status:** Fixed — commit `45e8885a` (`fix(04): CR-01 route cursor_mode=true, limit=0 into listByCursor`).
Dropped the redundant `opts.Limit > 0` clause from the mode-selection guard
so `CursorMode` alone (with `Offset == 0`, already enforced above) selects
`listByCursor`, which resolves its own zero limit to 20 (unchanged,
pre-existing default) rather than falling through to offset mode's
`MaxRecallLimit`-capped, cursor-less fetch. Added
`TestListCursorModeZeroLimitPagesCorrectly` (`internal/store/list_cursormode_zerolimit_test.go`),
seeding 23 records to isolate the mode-selection bug from the 1000-record
ceiling (seeding past `MaxRecallLimit` was impractical): confirmed RED
against the pre-fix code (first page returned all 23 items with an empty
`next_cursor`) and GREEN after the fix (20 items, non-empty cursor, full
traversal with no duplicates).

**File:** `internal/store/store.go:1697` (mode-selection guard), interacting with the offset-mode fallback at `internal/store/store.go:1708-1740`

**Issue:**

`Store.List`'s paging-mode selector is:

```go
if opts.Cursor != "" || (opts.Offset == 0 && opts.Limit > 0 && opts.CursorMode) {
    items, nextCursor, err = s.listByCursor(ctx, f, opts)
    return items, total, nextCursor, err
}
```

When a caller sets `CursorMode: true` with `Cursor: ""`, `Offset: 0`, and
`Limit: 0` (the natural "start cursor paging, use the default page size"
request), `opts.Limit > 0` is false, so this guard does **not** select cursor
mode. Execution falls through into the offset-mode branch below it:

```go
effectiveLimit := opts.Limit
if effectiveLimit == 0 {
    effectiveLimit = MaxRecallLimit   // 1000, per this phase's D-01
}
...
items, err = s.collectOrderedPages(ctx, f, view, dir, from, effectiveLimit)
...
return items, total, "", nil   // nextCursor is ALWAYS "" in offset mode
```

`total` is the real, exact matched count (from the earlier `Count` call), but
the returned `items` slice is capped at 1000 and `nextCursor` is
unconditionally `""`. A caller reading `next_cursor == ""` as "no more pages"
(exactly what `internal/store/orderedpage.go`'s own page contract and this
phase's stated goal — "a page cut short ... must never be reported as the
last page" — require) receives an incomplete result set with no signal that
anything was withheld, while `total` correctly reports a larger number. This
is a genuine, silent data-loss-from-the-caller's-perspective bug, not merely
a documentation gap.

This is directly reachable, with no guard in between:
* **Connect `ListMemories`**: `internal/server/connectapi.go:273-296` passes
  `Limit: req.Msg.Limit` through unchanged (0 is a legal wire value) and sets
  `CursorMode: req.Msg.CursorMode || req.Msg.PageToken != ""`. A client that
  sends `cursor_mode=true` with `limit` unset (0) and no `page_token` hits
  this exact branch.
* **CLI**: `cmd/engram/client_list.go` defaults `--limit` to `0` and
  `--cursor-mode` to `false`, but the two flags are independently settable —
  `engram list --scope X --cursor-mode` (no `--limit`) sends precisely
  `{Limit: 0, CursorMode: true}` to the Connect RPC above.
* The MCP `list_memory` closure (`internal/server/tools.go:2660-2663`)
  happens to default `limit` to 20 before calling `deps.listMemory`, so the
  MCP lane cannot trigger this — but nothing in `store.List` or
  `deps.listMemory` enforces that discipline; it is incidental to one caller.

Confirmed via `git diff 5afa9749..HEAD`: before this phase, the identical
`Cursor=="" && CursorMode==true && Limit==0` input fell through to the same
branch, but that branch's old body was `fetch := opts.Offset + opts.Limit; if
opts.Limit == 0 { fetch = total }` — i.e. it genuinely fetched every matching
record in one unbounded `Scroll`, so `nextCursor == ""` was accurate (there
was nothing left to withhold). This phase's D-01 change (capping the
zero-limit fallback at `MaxRecallLimit` instead of `total`) is what turns a
previously-correct-but-unbounded read into a bounded-but-incorrect one. The
old behavior was itself an unbounded-read hazard (motivating this whole
milestone) — the fix must bound the fetch *and* preserve the pagination
contract, not trade one defect for the other.

**Fix:** Route `CursorMode==true` (with `Offset==0`) into `listByCursor`
regardless of `Limit`, and let `listByCursor`'s existing `if limit == 0 {
limit = 20 }` default apply there (mirroring the default every other
zero-limit-with-a-default surface already uses):

```go
if opts.Cursor != "" || (opts.Offset == 0 && opts.CursorMode) {
    items, nextCursor, err = s.listByCursor(ctx, f, opts)
    return items, total, nextCursor, err
}
```

This makes `cursor_mode=true, limit=0` return a bounded (20-record) first
page with a correct, non-empty `next_cursor` when more records remain —
closing both the unbounded-read hazard and the silent-truncation regression
in one change. Add a regression test asserting that `List` with
`{CursorMode: true, Limit: 0}` over a fixture with `> 1000` matching records
returns a non-empty `next_cursor` (the existing
`recallmax_oversized_test.go:171-175` `List/zero_default_cursor` subtest only
asserts "no error", not pagination correctness, and would not have caught
this).

## Warnings

### WR-01: `fetchPayloadsByID` lacks the legacy-oversized-record fallback its sibling primitives implement

**Fix status:** Fixed — commit `533fefff` (`fix(04): WR-01 give fetchPayloadsByID a batch-of-1 fallback`).
Extracted the per-batch `Scroll` into `fetchPayloadBatch`, which retries every
id in a batch individually at `Limit: 1` on `ErrResponseTooLarge` when the
batch held more than one id — mirroring `scrollOrderedPage`'s fallback
shape. Added `TestFetchPayloadsByIDBatchOfOneFallback`
(`internal/store/searchfetch_batchfallback_test.go`), mirroring
`TestScrollOrderedPageBatchOfOneFallback`'s two subtests and exact
content-size formulas: `legacy-window` confirmed RED against the pre-fix
code (whole-batch failure) and GREEN after the fix (all ids fetched);
`single-oversized` passed both before and after (a record still over the
limit at `Limit: 1` correctly surfaces `ErrResponseTooLarge` either way).

**File:** `internal/store/searchfetch.go:110-118`

**Issue:** `scrollOrderedPage` (`internal/store/orderedpage.go:151-190`) and
`scrollAllPoints` (per its own doc comments in `boundedread.go`) both
implement a "D-07" fallback: when a Scroll RPC with `Limit > 1` fails with
`ErrResponseTooLarge` (a pre-cap legacy record whose actual bytes exceed the
view's assumed per-record ceiling), they retry the *same* keyset position at
`Limit: 1` for exactly that many records, isolating the one oversized record
so the rest of the batch still succeeds. `fetchPayloadsByID`'s per-batch
`Scroll` call has no equivalent:

```go
pts, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
    CollectionName: s.collection,
    Filter:         includeIDs(f, batch),
    Limit:          qdrant.PtrOf(uint32(len(batch))),
    WithPayload:    view.selector,
})
if err != nil {
    return nil, err   // no batch-of-1 retry
}
```

Since a search's phase-two id batch can contain up to `perRPCLimit(...)`
ids (dozens for the summary view), a single legacy-oversized record anywhere
in one batch fails that entire batch's fetch — and thus the whole
`Store.Search`/`Store.SearchDiscovery`/`backfillNoSummaryContent` call —
even though every other id in the batch, and every other batch, would have
succeeded. The failure still surfaces as the named `ErrResponseTooLarge` →
`CodeResourceExhausted` (never an opaque `internal`), so it does not violate
the phase's headline "never opaque 500" goal, but it is a real robustness
regression relative to the pattern this same phase's `orderedpage.go`
establishes, and it means a single legacy record can make an otherwise
healthy top-k search fail outright.

**Fix:** Give `fetchPayloadsByID` the same batch-of-1 fallback
`scrollOrderedPage` uses: on `ErrResponseTooLarge` with `len(batch) > 1`,
retry the batch one id at a time (or bisect), continuing to accumulate
successes into `out`, and only propagate the error if a single id's own
Scroll still overflows at `Limit: 1`.

### WR-02: Stale doc comment claims Connect `limit=0` still means "all"

**Fix status:** Fixed — commit `04a7c6a0` (`docs(04): WR-02 fix stale ListMemories limit=0 doc comment`).
Updated the `ListMemories` doc comment to state "0 resolves to the maximum,
1000" (matching the proto's own comment) and dropped the stale
`store.go:873-874` citation; also updated the adjacent inline comment on the
`Limit:` field assignment a few lines below (`// 0 = "all"`), the same class
of staleness in the same function, for consistency. Comment-only change,
verified via `go build` and `TestConnectListMemoriesLimitZeroReturnsAll`/
`TestConnectListMemoriesResponseTooLarge` (unaffected, as expected).

**File:** `internal/server/connectapi.go:224-230`

**Issue:** The doc comment on `ListMemories` reads:

> `Limit is passed through UNCHANGED — limit=0 means "all" (store.go:873-874),
> NOT silently capped to 20 ...`

This describes the pre-phase behavior this same phase's D-01 change replaced.
`store.go:873-874` no longer exists at that content (the file has grown and
`Store.List` now resolves a zero `Limit` to `MaxRecallLimit` (1000), not
"all" — see `store.go:1708-1711` and the proto's own updated comment at
`proto/engram/v1/engram.proto:73`, `// 0 resolves to the maximum, 1000; a
larger value is rejected`). A maintainer reading this comment without
checking the proto or `store.go` directly would reasonably (and incorrectly)
conclude that a Connect `limit=0` list can still return an unboundedly large
response — which is precisely the class of assumption CR-01 above shows is
unsafe to make about this code path.

**Fix:** Update the comment to match the proto's own documented semantics
("0 resolves to the maximum, 1000") and drop the stale line-number citation.

## Info

### IN-01: Store-layer `rejectOverMaximum` backstop errors bypass the field/hint envelope

**Fix status:** Skipped — out of the fix pass's scope (critical + warning
only; this review's own Fix section says "No action required for this
phase"). The finding itself documents it as unreachable in current behavior
(every server entry point calls the identical `rejectOverMaximumCount` guard
before the store is ever invoked), so this is not a defect to remediate now.

**File:** `internal/store/store.go:1542-1547`

**Issue:** The server-layer guard (`rejectOverMaximumCount`,
`internal/server/tools.go:1643-1652`) constructs a classified `*argError`
(rendered as `field=<name> hint=out_of_range: ...` per the project's
single-envelope error convention). The store-layer backstop of the same name
(`rejectOverMaximum`, `store.go`) instead returns a bare
`fmt.Errorf("%s exceeds the maximum of %d: %w", field, MaxRecallLimit,
ErrInvalidArgument)`. `connectError`'s `errors.As(err, &ae)` case only
matches `*argError`; a bare `store.ErrInvalidArgument`-wrapping error falls
to the later `case errors.Is(err, store.ErrInvalidArgument)` arm and reaches
the client as plain text with no `field=`/`hint=` structure. Today this is
unreachable in practice — every server entry point calls the identical
`rejectOverMaximumCount` guard before the store is ever invoked, so the
store's copy is documented as "a backstop... for any future recall entry
point that bypasses this one" — but if a future caller ever does hit this
path, the resulting error would violate the project's own single-envelope
convention. Not a defect in current behavior; worth a one-line note if the
backstop is ever exercised directly (e.g., a future store consumer outside
`internal/server`).

**Fix:** No action required for this phase; if a future direct-store caller
is added, consider having it translate `store.ErrInvalidArgument` through
the same `argErrf`/`renderHintEnvelope` machinery rather than surfacing the
raw store message.

---

_Reviewed: 2026-09-20T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
