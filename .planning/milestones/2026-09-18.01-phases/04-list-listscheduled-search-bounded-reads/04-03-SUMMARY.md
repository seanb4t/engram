---
phase: 04-list-listscheduled-search-bounded-reads
plan: 03
subsystem: database
tags: [qdrant, bounded-reads, pagination, offset, scheduled, red-evidence]

# Dependency graph
requires:
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-02: Store.collectOrderedPages (the D-05 assembly loop), Store.scrollOrderedPage reclassified into recallTransmitters, store.MaxRecallLimit — this plan composes both directly"
provides:
  - "keysView() — the D-07 keys-only readView (id + created_at only, fixed keysRecordCeiling) for cheaply walking a skipped offset prefix"
  - "Store.walkOffsetPrefix — skips opts.Offset via keysView() before collectOrderedPages fetches the caller's page, replacing the former all[opts.Offset:] client-side slice"
  - "Store.List's offset mode: no offset ceiling — a deep offset costs O(offset) tiny keys-only reads, never an unbounded or oversized fetch"
  - "Store.ListScheduled assembled from collectOrderedPages/scrollOrderedPage — no longer issues its own Scroll, reclassified out of recallTransmitters"
  - "boundedread.go's file doc comment states D-05's correction: pageByteBudget bounds cursor-mode responses only"
  - "collectOrderedPages simplified to (items, err) — the resume cursor and exhaustion flag were dead in both real callers"
affects: [04-04, 04-05, 04-06, 04-07, 04-08]

# Actuals (#2632)
actuals:
  tokens: 7516
  tasks: 2
  commits: 2
  plan_head_before: 5331bc01b3315decafc1324478afb9db503116ea

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Deep-offset skip via a keys-only readView (D-07): walk the skipped prefix with scrollOrderedPage over a projection excluding every capped field (id + created_at only), then resume the caller-view fetch from the SAME listCursor shape — no translation between the two views' resume tokens."
    - "A method with no callers needing its full return shape gets that shape trimmed (collectOrderedPages 4-tuple -> 2-tuple) rather than carrying dead named returns forward — golangci-lint's unparam is the mechanical trigger, caught at `task` time, not left for a later cleanup pass."
    - "TDD RED reconstructed after the fact by round-tripping the target file to its pre-rewrite committed HEAD (git checkout -- <file>, run the new test, restore the GREEN version from a saved copy) when authoring order was GREEN-then-test rather than test-then-GREEN — never git stash."

key-files:
  created:
    - internal/store/listscheduled_oversized_test.go
  modified:
    - internal/store/boundedread.go
    - internal/store/store.go
    - internal/store/export_test.go
    - internal/store/schemaversion_recallgate_test.go
    - internal/store/listbounded_oversized_test.go

key-decisions:
  - "D-07 executed exactly as locked: no offset ceiling. walkOffsetPrefix loops scrollOrderedPage over keysView() (selector qdrant.NewWithPayloadInclude(\"created_at\"), fixed keysRecordCeiling=256) until the offset boundary is reached or the primitive reports Exhausted, then collectOrderedPages resumes the caller's fullView() fetch from that SAME listCursor — no translation between the two views' resume tokens, exactly as D-07 specifies."
  - "D-05 executed for ListScheduled: it now composes the identical collectOrderedPages/scrollOrderedPage loop Store.List's offset mode uses, always in the full-payload view (04-RESEARCH Pitfall 6 — no full/summary knob invented, since no surface exposes one). The filter is built ONCE, before the loop, and passed unchanged to every assembled RPC, so the owner-only/state-gated/created_at-windowed envelope holds on every RPC, not just the first."
  - "boundedread.go's file doc comment revised in place (not restated elsewhere) to record D-05's correction: pageByteBudget bounds cursor-mode responses only; offset-mode Store.List and Store.ListScheduled are bounded by count alone."
  - "Store.scrollOrderedPage's existing recallTransmitters justification was EXTENDED (not duplicated) to note it now also serves the ListScheduled seed, rather than adding a redundant second classification entry for the same enclosing function."
  - "collectOrderedPages' signature was narrowed from (items, next, exhausted, err) to (items, err) mid-Task-2, discovered via `task lint`'s unparam check once a second real caller (ListScheduled) existed alongside Store.List's offset mode — both callers had always discarded next/exhausted, so unparam correctly flagged first `next` then `exhausted` once each was dropped in turn. Fixed as Rule 1 (blocking lint failure), not deferred."

patterns-established:
  - "Pattern D-07 (Task 1): walkOffsetPrefix — a package-level-view-driven prefix skip composed from the SAME scrollOrderedPage primitive as the page fetch, differing only in the payload selector, never a second paging scheme."
  - "Pattern D-05 (Task 2): ListScheduled joins Store.List's offset mode as collectOrderedPages' second caller, proving the assembly loop generalizes across call sites with no per-caller special-casing."

requirements-completed: [REQ-list-bounded, REQ-list-scheduled-bounded]

coverage:
  - id: D1
    description: "A deep offset in Store.List never overflows and never needs an offset ceiling: the skipped prefix costs ids+timestamps via a keys-only view, the page costs one keyset-resumed bounded fetch in the caller's view, and the result matches a plain zero-offset walk at the same positions"
    requirement: "REQ-list-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestStoreListDeepOffsetBounded (few-large, many-small)"
        status: pass
      - kind: integration
        ref: "internal/store#TestStoreListOffsetBounded, TestStoreListCursorBounded, TestStoreListContractInvariant, TestListAllLimitEmptyScope (regression bundle)"
        status: pass
      - kind: unit
        ref: "internal/store#TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.ListScheduled assembles its page from bounded ordered pages with its owner-only, state-gated, created_at-windowed filter intact on every RPC; visible behavior (default 20, full payloads, hidden states) is unchanged on both oversized fixture shapes"
    requirement: "REQ-list-scheduled-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestListScheduledBounded (few-large, many-small)"
        status: pass
      - kind: unit
        ref: "internal/store#TestRecallEmissionSetIsCompleteAndClassified, TestSchemaVersionNeverGatesRecall"
        status: pass
      - kind: other
        ref: "task (lint + full test suite, including ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1)"
        status: pass
    human_judgment: false

# Metrics
duration: ~40min
completed: 2026-09-20
status: complete
---

# Phase 4 Plan 3: Deep-Offset Prefix Walk and Bounded ListScheduled Summary

**A deep `Store.List` offset now walks its skipped prefix as cheap keys-only reads before resuming the page in the caller's view, and `Store.ListScheduled` is assembled from the same bounded ordered-page loop `Store.List`'s offset mode uses — closing every `List`-shaped read site in Phase 4's inventory except the two search sites.**

## Performance

- **Duration:** ~40 min
- **Started:** ~2026-09-20T03:00Z (approx)
- **Completed:** 2026-09-20T07:43Z
- **Tasks:** 2
- **Files modified:** 6 (1 created, 5 modified)

## Accomplishments

- Added `keysView()` (D-07): a fixed-allowance, id+created_at-only `readView` (`keysRecordCeiling = 256`), and `Store.walkOffsetPrefix`, which loops `scrollOrderedPage` over it to skip a large `Offset` cheaply — never widening the caller's filter, never fetching the skipped prefix in the caller's full view.
- Rewired `Store.List`'s offset branch onto the prefix walk: the caller-view fetch now requests the effective limit alone (never `offset+limit`), an offset that exhausts the matched set before reaching the boundary returns an empty page with the real `total`, and the former `all[opts.Offset:]` client-side slice is gone.
- Revised `boundedread.go`'s file doc comment to record D-05's correction in the same place the budget itself is defined: `pageByteBudget` bounds cursor-mode responses only.
- Proved the deep-offset walk over both oversized fixture shapes (`TestStoreListDeepOffsetBounded`): identical ids to a zero-offset baseline walk at the same positions, every RPC within its own view's `PerRPCLimit`, the keys-only selector on every prefix RPC and the full selector on every page RPC (in that order), and offset-at/beyond-total returning an empty page with the real total.
- Rewrote `Store.ListScheduled` onto `collectOrderedPages`/`scrollOrderedPage` (D-05): the filter is built once and passed unchanged to every assembled RPC, visible behavior is byte-for-byte unchanged (default limit 20, full payloads, hidden states), and it deliberately gained no full/summary knob.
- Reclassified `Store.ListScheduled` out of `recallTransmitters` (it no longer calls `s.client.Scroll` directly) and extended `Store.scrollOrderedPage`'s existing classification entry to note it now also serves the `ListScheduled` seed.
- Followed the TDD gate for Task 2 (`tdd="true"`): authored `TestListScheduledBounded` and reconstructed genuine RED against the pre-rewrite method by round-tripping `store.go` to its committed pre-Task-2 HEAD (never `git stash`), observed and recorded both shapes failing with `qdrant response exceeded the client's receive limit`, then restored the GREEN implementation and confirmed pass.
- Discovered and fixed a `task lint` `unparam` failure mid-Task-2: `collectOrderedPages`' `next`/`exhausted` named returns were dead in both real callers once `ListScheduled` became a second caller — narrowed its signature to `(items, err)`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Deep-offset prefix walk over a keys-only budgeted view** - `c36cfd43` (feat)
2. **Task 2: ListScheduled assembled from bounded ordered pages** - `0f7940cb` (feat)

_No plan-metadata commit yet; STATE.md/ROADMAP.md updates follow this summary._

## RED Evidence (Task 2, TDD)

Observed against the pre-rewrite `Store.ListScheduled` (single unbounded `Scroll` at the full requested limit), reconstructed by temporarily reverting `internal/store/store.go` to its committed Task-1 HEAD (via `git checkout -- internal/store/store.go`, restored afterward from a saved copy — never `git stash`):

```
--- FAIL: TestListScheduledBounded/few-large
    ListScheduled(max): Scroll() failed: ...: qdrant response exceeded the client's receive limit: /qdrant.Points/Scroll: grpc: received message after decompression larger than max 4194304
--- FAIL: TestListScheduledBounded/many-small
    ListScheduled(max): Scroll() failed: ...: qdrant response exceeded the client's receive limit: /qdrant.Points/Scroll: grpc: received message after decompression larger than max 4194304
--- FAIL: TestListScheduledBounded (1.00s)
```

Both failures are genuine RED (an assertion for the planned behavior failing on an oversized single-Scroll fetch) — not `INVALID_RED`. After restoring the GREEN implementation (`collectOrderedPages` wired in), both subtests pass.

## Files Created/Modified

- `internal/store/boundedread.go` - `keysRecordCeiling` const, `keysView()`; file doc comment revised for D-05's cursor-mode-only scope
- `internal/store/store.go` - `Store.walkOffsetPrefix` (new); `Store.List`'s offset branch rewritten onto it; `Store.ListScheduled` rewritten onto `collectOrderedPages`; `collectOrderedPages` narrowed to `(items, err)`
- `internal/store/export_test.go` - `KeysView()` shim
- `internal/store/schemaversion_recallgate_test.go` - `Store.ListScheduled` classification entry removed; `Store.scrollOrderedPage`'s justification extended
- `internal/store/listbounded_oversized_test.go` - `TestStoreListDeepOffsetBounded` (new); a local `selectorRecorder`/`selectorRecordedCall` interceptor distinguishing keys-only vs full-view RPCs
- `internal/store/listscheduled_oversized_test.go` (new) - `TestListScheduledBounded`

## Decisions Made

See key-decisions in frontmatter. Notably: `collectOrderedPages`' return shape was narrowed reactively (Rule 1, lint-driven), and the `Store.scrollOrderedPage` classification entry was extended rather than duplicated for `ListScheduled`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `collectOrderedPages`' dead `next`/`exhausted` named returns, surfaced by `task lint`'s unparam check**
- **Found during:** Task 2's own `<verify>` (`task`)
- **Issue:** Once `Store.ListScheduled` became `collectOrderedPages`' second real caller, `golangci-lint`'s `unparam` linter correctly flagged first the `next` result (never read by either of the two real callers) and, after removing it, the `exhausted` result (also never read).
- **Fix:** Narrowed `collectOrderedPages`' signature from `(items []Memory, next listCursor, exhausted bool, err error)` to `(items []Memory, err error)`, updating both call sites (`Store.List`'s offset branch, `Store.ListScheduled`) and the function's own doc comment to state that neither caller needs the resume cursor or the exhaustion flag scrollOrderedPage's own page contract already tracks internally.
- **Files modified:** `internal/store/store.go`
- **Verification:** `task lint` clean; `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` green; full `task` green.
- **Committed in:** `0f7940cb` (part of Task 2's commit)

---

**Total deviations:** 1 auto-fixed (1 bug — a lint-only signature narrowing, no behavior change)
**Impact on plan:** No scope creep. The fix is entirely internal to `collectOrderedPages`' own return shape, required for the plan's own `<verify>` (`task`) to pass; neither caller's observable behavior changed.

## Issues Encountered

- **Literal-command mismatch in Task 2's own acceptance criteria (same pattern documented in 04-02-SUMMARY.md's Issues Encountered):**
  1. `rg -v '^\s*//' internal/store/store.go | rg -c -e 's[.]client[.]Scroll[(]'` is written to expect `0` ("no `List`-shaped read in `store.go` issues its own Scroll any more"), but the true whole-file count is `2` — `Store.ListScopes` and `Store.ResolvePointID` both call `s.client.Scroll` directly and are outside this plan's (and this phase's inventory's) scope; neither is a `List`-shaped recall read. Verified instead against the criterion's own stated intent, scoped to the two functions this plan actually touches: `awk '/^func \(s \*Store\) List\(/,/^}/' internal/store/store.go | rg -c -e 's[.]client[.]Scroll[(]'` and the same for `ListScheduled` both print `0`.
  2. `rg -n -e 'Store[.]ListScheduled' internal/store/schemaversion_recallgate_test.go | wc -l` is written to expect `1` ("the remaining mention is its `recallEntryPointSeeds` membership, not a classification entry"), but the true count after removing the classification entry is `3` — the seed-list membership (1) plus two `entryPoint: "Store.ListScheduled"` fields in `recallInvocationRows` (2), which the file's own `TestSchemaVersionNeverGatesRecall`'s classification-coverage linkage subtest (line ~796) REQUIRES to remain in lockstep with `recallEntryPointSeeds` — removing or renaming them would break that linkage, which this plan's own instructions forbid touching ("do NOT edit any recallInvocationRows expected count or method multiset"). Verified instead via a criterion scoped to the actual classification list: `awk '/^var recallTransmitters = /,/^}/' internal/store/schemaversion_recallgate_test.go | rg -c -e 'enclosingFunc: "Store.ListScheduled"'` prints `0`.
- No blockers encountered executing either task; Docker/Qdrant was reachable throughout, and no auth gates were hit.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every `List`-shaped recall path in 03-INVENTORY's Phase 4 rows except the two search sites (`Store.Search`/`SearchReranked`, `Store.SearchDiscovery`) now composes Phase 3's ordered-page primitive — 04-04 (Search `k`) is the next and last read-site migration.
- `Store.collectOrderedPages` now has two real callers (`Store.List` offset mode, `Store.ListScheduled`) proving the assembly loop generalizes with no per-caller special-casing; its narrowed `(items, err)` signature is the stable shape any future caller should match.
- `keysView()` and `Store.walkOffsetPrefix` are available if a future plan needs a cheap keys-only skip elsewhere, though no other Phase 4 site currently needs one.
- The 04-02-SUMMARY.md-flagged open question (whether offset-mode should hard-reject a `Limit` above `MaxRecallLimit`) is untouched by this plan — still 04-05/04-06's scope, per that summary's own flag.
- No blockers for 04-04.

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

Both files verified present on disk: `internal/store/listscheduled_oversized_test.go` and this SUMMARY. All three commits (`c36cfd43`, `0f7940cb`, `4f2109f7`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing at final HEAD, including the two scoped literal-vs-intent checks documented under Issues Encountered. The plan-level `<verification>` block passes in full: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1` ok (0 `FAIL` lines across the whole package); `task` (lint + full test suite) exits 0; `task license:check` clean; `go test ./internal/keylinks/ -count=1` ok; `git diff --exit-code HEAD -- go.mod go.sum` exits 0 (no diff).
