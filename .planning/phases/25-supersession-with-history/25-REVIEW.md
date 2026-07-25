---
phase: 25-supersession-with-history
reviewed: 2026-07-19T00:00:00Z
depth: deep
files_reviewed: 5
files_reviewed_list:
  - internal/store/store.go
  - internal/store/locker.go
  - internal/store/store_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 25: Code Review Report (Final Re-review — Auto-fix Iteration 3)

**Reviewed:** 2026-07-19
**Depth:** deep
**Files Reviewed:** 5
**Status:** clean

## Summary

This is the third and final review pass, scoped tightly to the two fixes applied in
iteration 2 (commits `d84378fb` CR-04 and `74e60a1b` WR-04), plus a full deep-mode
re-scan of the rest of the supersession surface (`Store.Supersede`, the locker,
`supersedeMemory`/`updateMemory` handlers, and the recall gates on
Search/List/SearchDiscovery/ListScheduled). No new defects were found. Both fixes are
correct, complete, and hold up under adversarial tracing.

### CR-04 (`Store.Update` vs. concurrent `Store.Supersede`) — verified sound

- **Deadlock:** `Update` (`store.go:1602`, keyed on `cur.ID`) and `Supersede`
  (`store.go:1854`, keyed on `target`) are the *only* two call sites of
  `s.locker.Lock` in the package (confirmed via full-file grep). Neither is nested
  inside the other — `updateMemory` calls only `st.Update`/`st.UpdatePayload`,
  `supersedeMemory` calls only `st.Supersede`; no store method calls `Update` or
  `Supersede` internally. The in-process `sync.Mutex`-backed locker is non-reentrant,
  but since the two lock acquisitions always happen in independent goroutines/call
  stacks (never nested within one), self-deadlock is structurally impossible.
- **Unlock on every path:** `defer unlock()` runs immediately after the lock is
  successfully acquired (`store.go:1606`), before any subsequent return (the
  re-read's error branch, and the final `Upsert` call) — every exit releases the
  lock.
- **Re-read correctness:** `s.Get(ctx, cur.ID)` re-fetches inside the lock and
  overwrites `cur.Supersedes`/`cur.SupersededBy` from the fresh record
  (`store.go:1607-1611`) before the whole-payload `Upsert`. A fetch error that is
  *not* `ErrNotFound` aborts cleanly (lock still released via defer). The
  concurrent-delete edge case (`Get` returns `ErrNotFound`) is left as documented,
  pre-existing, out-of-scope behavior — `Update` never existence-checked `cur`
  before this fix either, so this is not a new regression.
- **Lock key consistency:** `cur.ID` (Update) and `target` (Supersede) are both
  resolved canonical UUID strings — `cur.ID` traces back through
  `FetchForUpdate(ctx, pid, …)` where `pid = ResolvePointID(a.ID)`, and `target`
  traces back through `targetID = ResolvePointID(a.Supersedes)` in
  `supersedeMemory`. Same normalized id space, so the two lockers genuinely contend
  on the same key when operating on the same record.
- **No other dropped field:** the only fields resynced under the lock are
  `Supersedes`/`SupersededBy` — the specific CR-04 concern. Other fields (e.g.
  `AccessCount`, `Tags`) retain the pre-existing, separately-documented
  read-modify-write tradeoff (last-writer-wins on a stale snapshot), unchanged by
  this fix and not a new regression.
- **Test quality:** `TestSupersedeVsUpdateConcurrent` (`store_test.go:2960`) races
  real `Store.Update` and `Store.Supersede` against a live Qdrant instance under
  `-race`, and asserts both the back-stamp (`SupersededBy`) and the content edit
  survive. Traced the interleaving by hand: because `Supersede`'s back-stamp is a
  *targeted* `SetPayload` (touches only `superseded_by`) while `Update` is a
  whole-payload `Upsert`, the fix is correct **regardless of which goroutine wins
  the lock first** — if `Update` wins, it upserts with a still-nil back-stamp and
  `Supersede` later stamps just that key on top (content survives); if `Supersede`
  wins, `Update`'s in-lock re-read picks up the fresh back-stamp before its own
  Upsert. The test is not order-dependent and genuinely exercises the race.

### WR-04 (defensive `IdempotencyKey` clear) — verified sound

- `a.storeArgs.IdempotencyKey = ""` at the top of `supersedeMemory` (`tools.go`)
  operates on a by-value `supersedeArgs` parameter, so the mutation is local and has
  no aliasing side effects.
- `IdempotencyKey` was never read anywhere else in `supersedeMemory`'s call chain —
  `toMemory` builds `store.Memory` from `Content`/`Scope`/`Repo`/etc. and does not
  reference the field, and `checkIdempotentReplay` is never called from this
  handler. The clear is therefore true defense-in-depth with zero effect on the
  happy path, exactly as documented.
- The corrected doc comment (previously claiming the `json:"-"` shadow field
  excluded `idempotency_key` from *both* schema and decode) is now accurate: it
  excludes the key from the advertised schema only, not the wire decode.
- Test coverage is genuinely end-to-end:
  `TestSupersedeArgsDecodePopulatesPromotedIdempotencyKey` pins that a raw
  `idempotency_key` on the wire still lands in `a.storeArgs.IdempotencyKey` after
  JSON decode (proving the shadow alone would NOT have been sufficient — validating
  why the defensive clear exists). `TestSupersedeMemoryIgnoresIdempotencyKey` then
  drives the full handler with a supplied key and confirms: (a) the call succeeds
  as a normal, non-replay supersede; (b) a second call with the *same* key against
  the now-already-superseded target fails with `ErrAlreadySuperseded` rather than
  returning the first call's id — the only way to prove the key was not silently
  honored as a replay token.

### Remainder of the supersession surface — no regressions

- `Store.Supersede`'s own per-target lock, ownership gate, single-hop/cycle
  rejection, and TOCTOU handling are untouched by iteration 2 and remain correct
  (cross-checked against `TestSupersedeStamp`, `TestSupersedeOwnerGate`,
  `TestSupersedeAlreadySuperseded`, `TestSupersedeConcurrent`,
  `TestSupersedeForwardChain`, `TestSupersedeTOCTOU`).
- The `superseded_by IS EMPTY` soft-hide gate is present and correctly composed
  (as a sibling condition, not folded into the time-window helper) on `Search`
  (`store.go:844`), `List` (`store.go:1070`), `SearchDiscovery` (`store.go:935`),
  and `ListScheduled` (`store.go:1300`) — all four recall surfaces gate consistently
  and `get_memory`/`Get` remain deliberately ungated, matching the documented D-08
  contract.
- `internal/server/connecterror.go` and `internal/server/store_iface.go` (read per
  the mandatory required-reading list) are unaffected by iteration 2: the
  `memStore` interface's `Update`/`Supersede` signatures still match
  `*store.Store` exactly (compile-time assertion `var _ memStore = (*store.Store)(nil)`
  holds, confirmed via `go build ./...`), and `connectError`'s
  `ErrAlreadySuperseded → CodeFailedPrecondition` mapping is unchanged and still
  correctly pre-positioned (supersede_memory remains MCP-only this phase).
- `go build ./...` and `go vet ./...` both ran clean in this session, consistent
  with the stated green baseline.

No Critical or Warning findings. The feature is sound; iteration 2's fixes for
CR-04 and WR-04 are both correct and adequately tested, and no new issues were
introduced by them or found elsewhere in the reviewed supersession surface. This
review is genuinely clean — no marginal nits were manufactured to justify further
iteration.

---

_Reviewed: 2026-07-19_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
