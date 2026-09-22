---
phase: 04-list-listscheduled-search-bounded-reads
plan: 06
subsystem: api
tags: [connect, mcp, error-envelope, hint-code, projection, rules, red-evidence]

# Dependency graph
requires:
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-05: ListOptions.Full/recallView/rejectOverMaximum at the store, plus a Rule 2 fix that already wired coreListRequest.Full through Connect ListMemories and MCP list_memory"
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-01: HintOutOfRange (out_of_range), classMalformed by D-10, and the twelve-code errors.md transcription this plan wires the first real callers onto"
provides:
  - "internal/server/rules.go: deps.listRules threads a.Full into its direct Store.List call (the one list caller outside the typed core) — the last unwired projection-threading gap 04-RESEARCH.md Pattern 6 step 4 named"
  - "internal/server/tools.go: rejectOverMaximumCount(field, count) — the published D-10 wire-boundary rejection, called as the FIRST count validation in all four shared core methods (deps.listMemory, deps.listScheduled, deps.searchMemory, deps.searchDiscovery), ahead of scope resolution and the embed call"
  - "internal/server/rules.go + tools.go: D-03's rule-listing ceiling restated in the doc comment, the store-call comment, and the tool description (composed from store.MaxRecallLimit, not a hardcoded literal)"
  - "internal/server/recallfullthreading_test.go, outofrange_test.go: real-entry-point (MCP CallTool + Connect handler) proof for the projection threading, the seven-surface over-maximum rejection, the before-any-backend cost proof, and the zero-count default pins"
affects: [04-07, 04-08]

# Actuals (#2632)
actuals:
  tokens: 12711
  tasks: 3
  commits: 3
  plan_head_before: e5e9611e78a8bc1937990d812ac9d411317172bf

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "rejectOverMaximumCount(field, count) — a shared server-boundary rejection called as the literal first count validation in every typed-core method, published on the wire (field=<f> hint=out_of_range, classMalformed), distinct from and layered in front of the store's own rejectOverMaximum backstop (04-05)."
    - "Real-entry-point test proof: an in-memory MCP server/client session (mirroring responsetoolarge_test.go) plus a directly-constructed engramAPI, both driven through a shared *deps, so a wiring test proves the actual transport plumbing rather than the typed core alone."
    - "grpcCallCounter/failEmbedder (outofrange_test.go): a generic any-method gRPC call counter and an embedder stub using t.Errorf (never t.Fatal, which the MCP in-memory transport's own server goroutine would violate) — the pattern for proving a rejection costs nothing downstream."

key-files:
  created:
    - internal/server/recallfullthreading_test.go
    - internal/server/outofrange_test.go
  modified:
    - internal/server/rules.go
    - internal/server/rules_test.go
    - internal/server/tools.go
    - internal/server/argattribution_test.go

key-decisions:
  - "internal/server/connectapi.go was NOT touched, despite being in the plan's own files_modified list: 04-05's own Rule 2 fix already wired coreListRequest.Full through Connect ListMemories and the MCP list_memory closure (verified by reading both call sites before starting, per the plan's own cross-reference note). This plan's Task 1 share was exactly and only the rule-listing's own threading — confirmed by TestFullSelectsFetchView passing unmodified on first run, before any of this plan's code changes."
  - "rejectOverMaximumCount is a 2-parameter (field, count) helper with store.MaxRecallLimit named once via a local const (maxCount, not max — golangci-lint's revive redefines-builtin-id correctly flagged the builtin shadow) inside the function body, exactly matching the acceptance criterion expecting store.MaxRecallLimit to appear exactly once in tools.go."
  - "TestZeroCountKeepsSurfaceDefaults' Connect ListMemories row asserts store.MaxRecallLimit-scale behavior (every seeded record, not a small default) rather than the plan's own action text (\"twenty on the Connect list\") — 04-CONTEXT.md's D-01 is a LOCKED, one-way, already-implemented decision that a Connect ListMemories zero limit resolves to store.MaxRecallLimit, not a per-surface default; D-08 (the source of the '20' family of defaults) governs SEARCH k, never LIST limit. Asserting 20 here would pin a regression against D-01. See Issues Encountered."
  - "TDD ordering for Task 2: the helper and its four call sites were authored before outofrange_test.go's two new tests were written (a genuine process deviation from the plan's literal 'author both tests first' instruction). Corrected before committing by reconstructing true RED: saving the working tools.go, reverting to the pre-Task-2 committed state (git checkout HEAD --, never git stash), re-running both new tests plus the two new argattribution_test.go rows to confirm genuine assertion-level RED (no panics — see RED Evidence below), then restoring the implementation from the saved copy and confirming GREEN. The two new argattribution_test.go rows were switched from a zero-value *deps to testDeps(t) specifically because a zero-value deps' nil store PANICS once the check is absent, which is INVALID_RED (a crash, not the intended assertion failure) rather than genuine RED."

patterns-established:
  - "Pattern (Task 1): a two-transport + one-bypass threading proof — drive the MCP tool, the Connect RPC, and the one caller that skips the typed core, all against a single real store with a payload-selector recorder, rather than trusting the typed core's own field alone."
  - "Pattern (Task 2/3): rejectOverMaximumCount as the sole D-10 boundary — one helper, four call sites, one seven(+one)-surface table test driving real entry points, so a future surface bypassing the shared core methods fails the table rather than shipping unbounded."

requirements-completed: []
# REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded, and
# REQ-list-limit-contract-decided are all declared by sibling plans in this
# phase that have not yet finished (requirements.ready-ids returned 0/4
# ready: 04-07/04-08 still outstanding) — none is marked complete here per
# the shared-ID gate (#2388).

coverage:
  - id: D1
    description: "A caller's full=true/false projection choice selects what the STORE fetches on every list lane, including list_rules (the one list caller outside the typed core) — not just each transport's response shaping; every response shape (compact vs full, including the no-summary truncation fallback) is unchanged from before this phase"
    requirement: "REQ-list-bounded"
    verification:
      - kind: integration
        ref: "internal/server#TestFullSelectsFetchView (MCP list_memory + Connect ListMemories, real entry points, Scroll-selector recorder)"
        status: pass
      - kind: integration
        ref: "internal/server#TestListRulesFullThreaded"
        status: pass
      - kind: unit
        ref: "internal/server#TestListRulesHandler (strengthened full-shape content assertion)"
        status: pass
      - kind: integration
        ref: "internal/server (full suite)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every recall count knob (Connect ListMemories limit, Connect SearchMemories/SearchDiscoveries k, and the four MCP tools' limit/k) refuses a count above store.MaxRecallLimit with field=<f> hint=out_of_range, classified malformed, before any embed call or Qdrant RPC; the maximum itself is accepted on every surface; the rejected value is never echoed"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: integration
        ref: "internal/server#TestOutOfRangeRejectedOnEveryRecallSurface (21 subtests: 7 surfaces x at-maximum/one-above, plus parent rollups)"
        status: pass
      - kind: integration
        ref: "internal/server#TestOutOfRangeRejectedBeforeAnyBackend (zero embed calls, zero gRPC calls on rejection)"
        status: pass
      - kind: unit
        ref: "internal/server#TestValidationErrorAttributionMatrix, TestHintNeverEchoesValue (new rows)"
        status: pass
      - kind: other
        ref: "task (lint + full test suite, including ENGRAM_REQUIRE_QDRANT=1 go test ./... and TestRedEvidencePatchesAreLive, on a clean committed tree)"
        status: pass
    human_judgment: false
  - id: D3
    description: "list_rules' contract is restated with the same ceiling (tool description composed from store.MaxRecallLimit, plus the code comment above deps.listRules and its store call); every surface's zero-count default (20 on Connect list*/both Connect searches per D-01/D-08, 20 on MCP list/list_scheduled, 8 on both MCP search tools) is pinned by a test seeded above every default, so a changed default is visible as a changed result count"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: integration
        ref: "internal/server#TestZeroCountKeepsSurfaceDefaults (8 subtests: 7 count knobs + list_rules)"
        status: pass
    human_judgment: false

# Metrics
duration: ~35min
completed: 2026-09-20
status: complete
---

# Phase 4 Plan 6: Recall Projection Threading, Over-Maximum Rejection, and the Rule-Listing Ceiling Summary

**`list_rules` now threads its own `full` flag into the store fetch (closing 04-RESEARCH's one named gap), every recall count knob rejects a value above `store.MaxRecallLimit` by name — `field=<limit|k> hint=out_of_range`, malformed, before any embed call or Qdrant RPC — and `list_rules`' contract and every surface's zero-count default are pinned by real-entry-point tests.**

## Performance

- **Duration:** ~35 min
- **Started:** ~2026-09-20T09:36Z (approx)
- **Completed:** 2026-09-20T10:07:49Z
- **Tasks:** 3
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- Threaded `listRulesArgs.Full` into `deps.listRules`' direct `store.ListOptions{}` literal — the one list caller outside the typed core that 04-05's own Rule 2 fix could not reach (04-RESEARCH.md Pattern 6, step 4). Confirmed the fix with genuine RED first: `TestListRulesFullThreaded` failed with a "lost its content" assertion against the pre-fix code, then passed after the one-line addition.
- Strengthened `TestListRulesHandler`'s pre-existing full-shape block to assert `Content`, not just the Go type — pinning the fix against a future silent regression. This assertion is (correctly) green both before and after the fix, since the rule listing already fetched full payloads pre-04-05; it is the guard that makes the fix falsifiable going forward.
- Authored `recallfullthreading_test.go`'s `TestFullSelectsFetchView`, driving the real MCP `list_memory` tool (in-memory transport, `registerTools`) and the real Connect `ListMemories` RPC (`engramAPI`) against a store dialed with a Scroll-selector recorder — proving 04-05's own list/Connect wiring end to end through the actual entry points, not the typed core.
- Added `rejectOverMaximumCount(field, count)`: the published D-10 boundary rejection, called as the literal first count validation in all four shared core methods (`deps.listMemory`, `deps.listScheduled`, `deps.searchMemory`, `deps.searchDiscovery`) — ahead of scope resolution and the embed call, so a rejected request costs nothing downstream. Classified `classMalformed` per D-10's explicit lock, never `classOutOfRange`.
- Authored `outofrange_test.go`'s `TestOutOfRangeRejectedOnEveryRecallSurface` (one table over all seven recall count knobs, each driven through its real MCP/Connect entry point, at-maximum and one-above) and `TestOutOfRangeRejectedBeforeAnyBackend` (a failing embedder plus a generic gRPC call counter prove zero embed calls and zero gRPC calls on a rejected search request).
- Extended `TestValidationErrorAttributionMatrix` and `TestHintNeverEchoesValue` with the new rejection's rows.
- Restated D-03's rule-listing contract in `deps.listRules`' doc comment and its store-call comment (naming `store.MaxRecallLimit`, never a bare number), and composed the `list_rules` tool description's ceiling from the same constant via `fmt.Sprintf` rather than a hardcoded literal.
- Authored `TestZeroCountKeepsSurfaceDefaults`: a table over all seven count knobs plus the rule listing, seeded with more records than any documented default, proving a zero count is never treated as over-maximum and yields each surface's own default unchanged.
- **Rule 1 fixes (self-caught during `task`):** two `golangci-lint`/`revive` issues in this plan's own new code — `redefines-builtin-id` (a local `const max` shadowing the Go 1.21+ builtin) and `context-as-argument` (a test helper's `ctx` parameter not first) — both fixed before committing.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — the caller's projection choice reaches the fetch on every list lane, including the one that bypasses the typed core** - `7a28b522` (feat)
2. **Task 2: An over-maximum count is refused by name on all seven recall surfaces, before any backend call** - `0998db50` (feat)
3. **Task 3: The rule listing's contract restated with the same number, and every zero-count default pinned** - `416286f8` (docs)

_No plan-metadata commit yet; STATE.md/ROADMAP.md updates follow this summary._

## RED Evidence

**Task 1** (`TestListRulesFullThreaded`, genuine RED before `rules.go`'s `Full: a.Full` addition):

```
listRules full=true: recorded Scroll selector(s) excluded content/citations — want the full view
full rule shape lost its content: Content = "", want "recallfull rule content, must reach the full shape only when full=true" (the projection flag must have reached the store)
--- FAIL: TestListRulesFullThreaded
```

`TestFullSelectsFetchView` (the same authoring pass's other new test) PASSED on first run, before any Task 1 code change — confirming 04-05's own MCP/Connect wiring was already complete and this plan's real gap was exactly the rule listing.

**Task 2** (reconstructed by temporarily reverting `tools.go` to its pre-Task-2 committed state via `git checkout HEAD --`, restored afterward from a saved copy — never `git stash`; two `argattribution_test.go` rows switched from a zero-value `*deps` to `testDeps(t)` first, since a zero-value deps' nil store panics once the check is absent — an `INVALID_RED` crash, not the intended assertion failure):

```
argattribution_test.go:258: argFieldsOf(err) = [], want [limit] (err: limit exceeds the maximum of 1000: invalid argument)
argattribution_test.go:258: argHintOf(err) = , want out_of_range (err: limit exceeds the maximum of 1000: invalid argument)
--- FAIL: TestValidationErrorAttributionMatrix/list_memory_limit_over_maximum
--- FAIL: TestValidationErrorAttributionMatrix/search_memory_k_over_maximum

outofrange_test.go:136: CallTool(list_memory): Text = "limit exceeds the maximum of 1000: invalid argument", want prefix "field=limit hint=out_of_range"
outofrange_test.go:152: CallTool(search_memory) one above the maximum: IsError = false, want true
outofrange_test.go:171: ListMemories one above the maximum: err.Error() = "invalid_argument: limit exceeds the maximum of 1000: invalid argument", want substring "field=limit hint=out_of_range"
--- FAIL: TestOutOfRangeRejectedOnEveryRecallSurface (7/7 surfaces)

outofrange_test.go:295: search_memory: embed calls = 1, want 0
outofrange_test.go:306: search_discovery: embed calls = 2, want 0
outofrange_test.go:314: SearchMemories: embed calls = 3, want 0
outofrange_test.go:322: SearchDiscoveries: embed calls = 4, want 0
--- FAIL: TestOutOfRangeRejectedBeforeAnyBackend (4/4 search surfaces)
```

Every failure above is genuine RED — a clean assertion mismatch or an explicit `t.Errorf` from the failing embedder, with the store's own pre-existing 04-05 backstop (`rejectOverMaximum`) visibly firing in the error TEXT ("limit exceeds the maximum of 1000: invalid argument") but lacking the server-boundary `field=... hint=out_of_range` envelope this task adds — never a panic, a zero-test discovery, or an unrelated failure. After restoring the implementation, all listed tests pass (see Self-Check).

## Files Created/Modified

- `internal/server/rules.go` - `deps.listRules` threads `a.Full` into its direct `store.ListOptions` literal; its doc comment and store-call comment restate D-03's ceiling by name
- `internal/server/rules_test.go` - `TestListRulesHandler`'s full-shape block now asserts `Content`, not just the Go type
- `internal/server/tools.go` - `rejectOverMaximumCount(field, count)` (new helper); called as the first count validation in `listMemory`/`listScheduled`/`searchMemory`/`searchDiscovery`; `list_rules` tool description composes its ceiling from `store.MaxRecallLimit`
- `internal/server/argattribution_test.go` - two new attribution-matrix rows (list + search) and one new never-echoes-value row for the D-10 rejection
- `internal/server/recallfullthreading_test.go` (new) - `TestFullSelectsFetchView`, `TestListRulesFullThreaded`, plus the shared `scrollSelectorRecorder`/`testDepsWithRecorder` helpers
- `internal/server/outofrange_test.go` (new) - `TestOutOfRangeRejectedOnEveryRecallSurface`, `TestOutOfRangeRejectedBeforeAnyBackend`, `TestZeroCountKeepsSurfaceDefaults`, plus the shared `grpcCallCounter`/`failEmbedder`/`mcpResultCount`/`newOutOfRangeMCPSession` helpers

## Decisions Made

See key-decisions in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two `golangci-lint`/`revive` issues in this plan's own new code**
- **Found during:** Task 3's own plan-level `<verify>` (`ENGRAM_REQUIRE_QDRANT=1 task`)
- **Issue:** (a) `rejectOverMaximumCount`'s local `const max = store.MaxRecallLimit` shadowed the Go 1.21+ builtin `max` (revive: `redefines-builtin-id`). (b) `outofrange_test.go`'s `mcpResultCount` helper took `ctx context.Context` as its third parameter instead of its first (revive: `context-as-argument`).
- **Fix:** (a) Renamed the local constant to `maxCount`. (b) Reordered `mcpResultCount`'s parameters to `(ctx, t, cs, ...)` and updated all four call sites.
- **Files modified:** `internal/server/tools.go`, `internal/server/outofrange_test.go`
- **Verification:** `ENGRAM_REQUIRE_QDRANT=1 task` (lint + full test suite) exits 0.
- **Committed in:** `416286f8` (Task 3's commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 — a lint bug introduced and caught within this plan's own execution, never shipped in an intermediate commit)
**Impact on plan:** No scope creep. Both fixes are entirely inside this plan's own new code and were required for the plan's own `<verify>` (`task`) to pass.

## Issues Encountered

- **Plan-text vs. locked-decision conflict in Task 3's own action text (resolved in favor of the locked decision, per this plan's own "do not re-open a locked decision" discipline):** Task 3's action text describes `TestZeroCountKeepsSurfaceDefaults`'s expected zero-count defaults as "twenty on the Connect list and both Connect searches" — but 04-CONTEXT.md's **D-01** (marked one-way/LOCKED, already implemented by an earlier phase-4 plan, and independently confirmed against the live `internal/server/connectapi.go` source before writing the test) states that Connect `ListMemories`' `limit: 0` resolves to `store.MaxRecallLimit` (1000), never a small per-surface default — that is a DIFFERENT decision (D-01) than the one governing the "20" family (D-08, which is scoped to search `k`, not list `limit`). Running the test against a literal "20" expectation for the Connect list row failed immediately with "got 25 memories, want 20" (all 25 seeded records legitimately came back, since 25 < 1000). Implemented and documented per D-01's own text and the live, already-correct code, not per Task 3's own imprecise summary of it; the test's inline comment records this explicitly so a future reader is not misled by the plan text alone.
- **TDD ordering (Task 2, corrected before commit):** the `rejectOverMaximumCount` helper and its four call sites were implemented before `outofrange_test.go`'s two new tests and `argattribution_test.go`'s two new rows were authored — a genuine deviation from the plan's literal "author both tests first" instruction. Caught and corrected before committing by reconstructing true RED (see RED Evidence above): saved the working `tools.go`, reverted to the pre-Task-2 committed state, re-ran the affected tests to confirm genuine (non-panicking) RED, then restored the implementation and confirmed GREEN. Recorded here in the interest of an accurate account of the actual authoring order, not just the reconstructed evidence.
- **Literal-command mismatches in the plan's own acceptance criteria (same pattern documented in every prior plan's SUMMARY this phase — 04-01 through 04-05):**
  1. Task 2's acceptance criterion `rg -v '^\s*//' internal/server/tools.go | rg -c -e 'store[.]MaxRecallLimit'` expects `1`; the naive helper body (using `store.MaxRecallLimit` twice — once in the comparison, once in the error message) would have produced `2`. Resolved by naming it once via a local `const maxCount`, bringing the true count to exactly `1` — no mismatch needed to be tolerated here, unlike prior plans' occurrences of this pattern.
  2. Task 2's/the plan-level `<verification>`'s `rg -o -e '1000' internal/server/tools.go internal/server/connectapi.go | wc -l` expects `0`, but the true count is `1`: a PRE-EXISTING (confirmed via `git show <pre-Task-2-commit>:internal/server/tools.go`) code-comment line-number citation, `"tools.go:1000-1003"`, in `connectapi.go`, unrelated to any hardcoded maximum literal — verified this is the sole match and that it predates this plan's own changes.
  3. Task 2's acceptance criterion `rg -v '^\s*//' internal/server/tools.go internal/server/connectapi.go | rg -c -e 'classOutOfRange'` expects `0`, but the true count is `12` — all twelve are PRE-EXISTING (confirmed via `git show`), unrelated uses of `classOutOfRange` for other validations (too-long fields, too-many-tags, etc.) that predate this plan entirely. The correct, verified intent — that THIS plan's new rejection does not use `classOutOfRange` — holds: `rejectOverMaximumCount` uses `classMalformed` exclusively.
- No blockers encountered executing any task; Docker/Qdrant was reachable throughout, and no auth gates were hit.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every recall surface named in D-02 (Connect `ListMemories`/`SearchMemories`/`SearchDiscoveries`, and the four MCP tools) now refuses an over-maximum count by name, before any downstream cost, and the caller's projection choice reaches the store's own fetch on every list lane including `list_rules`.
- `rejectOverMaximumCount` and the `scrollSelectorRecorder`/`grpcCallCounter`/`failEmbedder` test helpers are available for any future recall surface or wiring-threading proof.
- REQ-list-bounded, REQ-list-scheduled-bounded, REQ-search-k-bounded, and REQ-list-limit-contract-decided all stay **blocked** (not marked complete) pending 04-07/04-08 (`requirements.ready-ids` returned 0/4 ready) — this plan does not run `requirements.mark-complete` for any of them.
- 04-07 (proto comments, `gen/` trees, CLI help + goldens, docs-site pages, CLAUDE.md, PROJECT.md) has zero `files_modified` overlap with this plan, confirmed both by the plan's own cross-plan note and by this plan's actual final diff (`internal/server/{rules.go,rules_test.go,tools.go,argattribution_test.go}` plus two new test files — no `proto/`, `gen/`, `cmd/engram/`, `docs-site/`, `CLAUDE.md`, or `.planning/PROJECT.md` touched).
- No blockers for 04-07/04-08.

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

Both created files verified present on disk: `internal/server/recallfullthreading_test.go`, `internal/server/outofrange_test.go`. All three commits (`7a28b522`, `0998db50`, `416286f8`) verified present in `git log --oneline --all`. Every acceptance criterion for all three tasks re-run and confirmed passing at final HEAD, including the two documented pre-existing literal-vs-intent mismatches above (both verified against their own stated intent via `git show` on the pre-Task-2 commit). The plan-level `<verification>` block passes in full: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/store/... -count=1` ok (including `TestRedEvidencePatchesAreLive` on the clean committed tree); `task` (lint + full test suite) exits 0; `task license:check` clean; `go test ./internal/keylinks/ -count=1` ok; `git diff --exit-code HEAD -- go.mod go.sum` exits 0 (no diff); the `1000`-literal sweep over `tools.go`/`connectapi.go` returns exactly 1 match, confirmed pre-existing and unrelated (a comment line-number citation).
