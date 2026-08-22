---
phase: 07-console-cli-state-surfacing
plan: 03
subsystem: api
tags: [protobuf, connect-rpc, qdrant, cobra, go, recall-gate]

# Dependency graph
requires:
  - phase: 07-console-cli-state-surfacing
    provides: "07-01's List-lane opt-in shape (ListOptions.IncludeArchived/IncludeSuperseded/IncludeScheduled, the conditional Store.List gate, and recallEntryPointSeeds) — this plan mirrors it onto Store.Search rather than inventing a new shape"
provides:
  - "SearchMemoriesRequest.include_archived/include_superseded/include_scheduled (fields 10-12), the Search-lane half of the D-01/D-02 opt-in wire contract"
  - "store.SearchOptions.IncludeArchived/IncludeSuperseded/IncludeScheduled and the conditional Store.Search gate that consumes them, mirroring Store.List's shape exactly"
  - "engram search --include-archived/--include-superseded/--include-scheduled CLI flags, help text identical to engram list's"
  - "A test proven (via a live, reverted RED experiment) to fail if ListScheduled ever inherits the excluded opt-in — the 2-of-4 scope is enforced, not merely stated"
  - "TestSearchAndListAuthorizationOrthogonalToState — a cross-owner proof that the three flags never widen authorization, only state visibility (D-04)"
  - "backlogFilter's reachability doc comment re-grounded on claims that survive conditional recall gates, naming this phase as the change that forced the re-derivation"
affects: [07-04, 07-05, 07-06, 07-07]

# Actuals (#2632)
actuals:
  tokens: 19859
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Orthogonal recall-gate opt-in bools mirrored across a second gate site (Store.Search) using the exact same per-field `if !opts.IncludeX { ... }` idiom 07-01 established for Store.List — no shared helper, no combined branch"
    - "A deliberately-excluded gate site (ListScheduled) gets a NEGATIVE assertion under all-flags-true, with the assertion's realism proven once via a live temporary code mutation (wire the excluded gate to the flag, watch the test fail, revert) rather than trusted on description alone"

key-files:
  created: []
  modified:
    - proto/engram/v1/engram.proto
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/store/migratebacklog.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/connectdescriptor_test.go
    - cmd/engram/client_search.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-03-red-1-toplevel.patch
    - .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-03-red-2-nested.patch
    - .planning/phases/07-console-cli-state-surfacing/deferred-items.md

key-decisions:
  - "The plan's Task 1 verify command shifted two unrelated phase-02 red-evidence patches' context lines out from under them (Store.Search's body moved when the conditional gate was added). Repaired both patches in place (Rule 1 — a bug my own change introduced), regenerated with git diff against the actual new source rather than hand-edited, and re-verified TestRedEvidencePatchesAreLive goes green with both patches still proving TestSchemaVersionNeverGatesRecall RED. See Deviations."
  - "The 2-of-4 negative-assertion test (Task 2) is backed by a genuine, reverted RED experiment: ListScheduled's archived_at condition was temporarily wired to opts.IncludeArchived, the extended TestArchiveRecallGateListScheduled failed exactly as the acceptance criterion demands, then the experiment was reverted (confirmed via a clean git diff on store.go) before committing."
  - "backlogFilter's re-grounded doc comment names recallEntryPointSeeds (the actual derivation) and phase 07 plan 03's three include bools (the actual forcing change) rather than restating the disproven claim in softened language — the acceptance criteria's negative-grep on the old clause and the two positive greps for the new grounding both pass mechanically, not by inspection alone."

requirements-completed: [REQ-cli-record-state, REQ-console-record-state]

coverage:
  - id: D1
    description: "engram search --include-archived/--include-superseded/--include-scheduled reach archived/superseded/windowed-inactive records through a published wire-level opt-in, with help text identical to engram list's, end-to-end from proto field to CLI flag"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchIncludeArchived"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchIncludeSupersededAndScheduled"
        status: pass
      - kind: unit
        ref: "cmd/engram/testdata/help.golden (search --include-* flags, identical wording to list)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.SearchReranked forwards SearchOptions unchanged to Store.Search and adds no second gate — reranking never widens or narrows the state-relaxed result set"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchRerankedMatchesSearchMembership"
        status: pass
    human_judgment: false
  - id: D3
    description: "The deliberate 2-of-4 gate-site scope (Store.Search/Store.List in; Store.SearchDiscovery/Store.ListScheduled excluded) is enforced by a test proven, via a live reverted RED experiment, to fail if ListScheduled ever inherits the opt-in"
    requirement: "REQ-console-record-state"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveRecallGateListScheduled"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestListScheduledSupersededHidden"
        status: pass
    human_judgment: false
  - id: D4
    description: "Authorization stays orthogonal to state (D-04): setting all three include bools never reveals another owner's private record on either Search or List, and a shared record that is archived or superseded IS revealed to the non-owning caller once the flag is set"
    requirement: "REQ-console-record-state"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchAndListAuthorizationOrthogonalToState"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestSupersedeRecallGate"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestSupersedeMultiRecallGate"
        status: pass
    human_judgment: false
  - id: D5
    description: "The schemaversion_recallgate_test.go AST-reachability derivation was re-run against the conditionally-gated Store.Search/Store.List and still holds; backlogFilter's reachability doc comment is re-grounded on claims that survive conditional gating"
    requirement: "REQ-console-record-state"
    verification:
      - kind: integration
        ref: "internal/store/schemaversion_recallgate_test.go#TestSchemaVersionNeverGatesRecall"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-21
status: complete
---

# Phase 07 Plan 03: Search-Lane Recall-Gate Opt-In and Reachability Re-derivation Summary

**`engram search --include-archived/--include-superseded/--include-scheduled` mirrors the List-lane opt-in end-to-end (proto field 10-12 through `Store.Search`'s conditional gate to identical CLI help text), with the deliberate 2-of-4 exclusion and D-04 authorization orthogonality both proven by tests rather than merely stated, and both reachability claims the relaxation invalidated re-derived against what actually shipped.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-08-21
- **Tasks:** 3/3 completed
- **Files modified:** 16 (0 created, 16 modified)

## Accomplishments

- `SearchMemoriesRequest.include_archived` (10), `.include_superseded` (11), and `.include_scheduled` (12) are published, additive wire fields, each mapping 1:1 onto exactly one `Store.Search` `Must` condition via its own `if !opts.IncludeX` guard — proven by a method-scoped `rg` count of exactly 3 guards, with `Store.List`'s own three guards re-checked unchanged in the same pass.
- `Store.SearchReranked` forwards `SearchOptions` unchanged and adds no second gate: under `IncludeArchived: true, IncludeScheduled: true`, `SearchReranked` and a direct `Store.Search` call return the identical record-ID set (`TestSearchRerankedMatchesSearchMembership`).
- The deliberate 2-of-4 scope (`Store.Search`/`Store.List` in; `Store.SearchDiscovery`/`Store.ListScheduled` excluded, per D-01) is enforced by a test proven live: `ListScheduled`'s `archived_at` condition was temporarily wired to `opts.IncludeArchived`, the extended `TestArchiveRecallGateListScheduled` failed exactly as expected, and the experiment was reverted (confirmed by a clean `git diff` on `store.go`) before the commit landed.
- `TestSearchAndListAuthorizationOrthogonalToState` proves D-04 directly on both gate sites: a cross-owner caller setting all three flags sees zero of the other owner's `private` records but DOES see the other owner's `shared` records that are archived or superseded — sharing is unchanged by state.
- `TestSupersedeRecallGate` and `TestSupersedeMultiRecallGate` gained positive-relaxation sub-cases: the record(s) each test already proves hidden by default are now also proven revealed under `IncludeSuperseded`, on both `Search` and `List`.
- `go test ./internal/store/... -run TestSchemaVersionNeverGatesRecall -count=1 -v` was re-run after Task 1 landed: **PASS**, 18 filters walked across all six `recallEntryPointSeeds` members (`Count:4, Query:6, Scroll:8`), zero containing `schema_version`. No seed-list edit was needed — Task 1 added no new store method and renamed nothing, so the AST-reachability derivation's callee-name graph is unchanged. This result is recorded here per the plan's `must_haves.truths` requirement.
- `backlogFilter`'s doc comment is re-grounded: its original claim ("Search/List/SearchDiscovery/ListScheduled all apply activeWindowConditions... never this filter") is gone (`rg` count 0), replaced with grounds that survive conditional gating — `backlogFilter` is not referenced by any of the six `recallEntryPointSeeds` functions, and a recall entry point relaxing its own gate removes conditions from its own filter rather than adopting a different one — naming both `recallEntryPointSeeds` and this phase's three include bools as the forcing change.

## Task Commits

Each task was committed atomically:

1. **Task 1: Mirror the three-flag opt-in onto the Search lane** - `77786bb0` (feat)
2. **Task 2: Prove the 2-of-4 scope and the authorization orthogonality** - `bdfe25de` (test)
3. **Task 3: Re-derive the two reachability claims the relaxation invalidates** - `3ec3c5ed` (docs)

_Note: this plan carried `tdd="true"` on Tasks 1-2; tests and implementation landed together per task (matching 07-01's precedent) rather than as separate RED/GREEN commits — see Deviations._

An additional deviation-tracking commit — `3ea1f318` (docs) — logs the pre-existing SA1019 lint finding as still out of scope for this plan.

## Files Created/Modified

- `proto/engram/v1/engram.proto` - Adds `SearchMemoriesRequest` fields 10-12 (`include_archived`, `include_superseded`, `include_scheduled`)
- `internal/store/store.go` - `SearchOptions.IncludeArchived/IncludeSuperseded/IncludeScheduled` and the conditional gate block in `Store.Search`
- `internal/store/store_test.go` - `TestSearchIncludeArchived`, `TestSearchIncludeSupersededAndScheduled`, `TestSearchRerankedMatchesSearchMembership`, `TestSearchAndListAuthorizationOrthogonalToState`, plus extensions to `TestArchiveRecallGateListScheduled`, `TestListScheduledSupersededHidden`, `TestArchiveRecallGateSearchDiscovery`, `TestSupersedeRecallGate`, `TestSupersedeMultiRecallGate`
- `internal/store/migratebacklog.go` - `backlogFilter`'s doc comment re-grounded on conditional-gating-surviving reasoning
- `internal/server/tools.go` - `coreSearchRequest`'s three new fields, threaded into `deps.searchMemory`'s `store.SearchOptions{}` literal
- `internal/server/connectapi.go` - `SearchMemories` handler reads the three request bools into `coreSearchRequest`
- `internal/server/connectdescriptor_test.go` - Pinned `SearchMemoriesRequest` field count bumped 9 → 12
- `cmd/engram/client_search.go` - `--include-archived`/`--include-superseded`/`--include-scheduled` flags
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - Regenerated via `task surfaces:gen`
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` - Regenerated via `task proto:gen`
- `.planning/phases/02-record-schema-versioning-foundation/red-evidence/02-03-red-1-toplevel.patch`, `.../02-03-red-2-nested.patch` - Repaired stale context lines (see Deviations)
- `.planning/phases/07-console-cli-state-surfacing/deferred-items.md` - Logged the still-present, still-unrelated SA1019 finding

## Decisions Made

- **Two stale phase-02 red-evidence patches repaired, not left broken.** Task 1's conditional-gate restructuring of `Store.Search` shifted the exact line `f := s.ownerScopeFilter(...)` / `f.Must = append(..., activeWindowConditions(...))` context that `02-03-red-1-toplevel.patch` and `02-03-red-2-nested.patch` depend on. `TestRedEvidencePatchesAreLive` caught this immediately (`git apply --check` failed — patch stale). Regenerated both patches by applying the identical semantic injection (a `schema_version` condition into `Store.Search`'s filter) against the NEW source via a throwaway Python edit + `git diff`, then reverting the throwaway edit. Verified `TestRedEvidencePatchesAreLive` green afterward, still correctly proving `TestSchemaVersionNeverGatesRecall` goes RED with either injection applied.
- **`backlogFilter`'s doc-comment repair did not restate the disproven clause even negated.** The acceptance criteria negative-greps the exact banned string, so the new text states the new grounds directly (`recallEntryPointSeeds` non-membership + "relaxing removes conditions, never adopts a different filter") instead of describing what changed about the old claim.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Repaired two phase-02 red-evidence patches broken by Task 1's own restructuring**
- **Found during:** Task 1's `<verify>` step, discovered while running the plan's own required `go test ./internal/store/... -count=1` after Task 1
- **Issue:** `02-03-red-1-toplevel.patch` and `02-03-red-2-nested.patch` (both phase-02 artifacts, pre-existing) inject a `schema_version` condition immediately after `f := s.ownerScopeFilter(...)` in `Store.Search`. Task 1's conditional-gate change inserted a comment block and an `if !opts.IncludeScheduled {` guard at exactly that point, so the patches' context no longer matched — `git apply --check` failed with "patch does not apply."
- **Fix:** Regenerated both patches against the new source (same semantic injection, new line context) via a throwaway edit + `git diff`, confirmed the throwaway edit fully reverted (clean `git diff` on `store.go`), then committed only the two `.patch` files.
- **Files modified:** `.planning/phases/02-record-schema-versioning-foundation/red-evidence/02-03-red-1-toplevel.patch`, `.../02-03-red-2-nested.patch`
- **Verification:** `go test ./internal/store/... -run TestRedEvidencePatchesAreLive -count=1 -v` green, both patches confirmed still proving `TestSchemaVersionNeverGatesRecall` RED.
- **Committed in:** `bdfe25de` (Task 2 commit — the patches were fixed alongside Task 2's test work, ahead of Task 2's own git-status-clean requirement for `TestRedEvidencePatchesAreLive` to run at all)

**2. [Rule 3 - Blocking issue] Pinned `SearchMemoriesRequest` field-count assertion updated for the new additive fields**
- **Found during:** Task 1
- **Issue:** `internal/server/connectdescriptor_test.go`'s `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` pins `SearchMemoriesRequest`'s exact field count (a semantic-reflection regression test) — adding the three fields broke it (9 → wanted 12), the exact same maintenance 07-01 required for `ListMemoriesRequest`.
- **Fix:** Bumped the pinned count with a comment naming the phase-07-plan-03 fields as the cause.
- **Files modified:** `internal/server/connectdescriptor_test.go`
- **Verification:** `go test ./internal/server/... -count=1` green after the fix.
- **Committed in:** `77786bb0` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug repair on unrelated pre-existing artifacts, 1 blocking pinned-count fix).
**Impact on plan:** Both auto-fixes necessary to keep `go test ./internal/store/...` and `go test ./internal/server/...` green after Task 1's change. No scope creep — neither fix touches code outside what Task 1's own restructuring disturbed.

## Issues Encountered

- **`task lint` has one pre-existing, unrelated `staticcheck SA1019` finding** at `internal/server/connectapi.go:271` (`Approximate: false,` inside `ListMemories`, a field already marked `deprecated = true` in the proto before this phase — same finding 07-01 logged, shifted by three lines from this plan's own unrelated edits earlier in the file). Confirmed this plan's `connectapi.go` edits are confined to `SearchMemories`, not `ListMemories`. Out of scope per SCOPE BOUNDARY; logged in `deferred-items.md` (commit `3ea1f318`). `task lint:go` fails on this single finding; `go test ./... -count=1` is fully green across all 17 packages.
- **`task fmt`'s dprint stage reformatted four files outside this plan's scope** (`.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, `ui/tsconfig.json`) while verifying the plan's own formatting. Reverted all four with `git checkout -- <file>` (sanctioned single-file revert) before committing — none of this plan's own edits needed dprint's touch, `gofmt -l` reported them clean already.

Neither issue affects this plan's shipped code; both are documented above and in `deferred-items.md` for future cleanup.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The Search-lane store options and wire fields this plan lands complete the D-01/D-02 opt-in across both `Store.Search` and `Store.List` — the full 2-of-4 scope is now implemented AND tested at both boundaries (in-scope reveal, out-of-scope non-reveal).
- `backlogFilter`'s doc comment now records the correct reachability grounds for any future plan that further conditions a recall entry point's gate — the pattern (re-derive on `recallEntryPointSeeds` non-membership, not on a since-falsified unconditional-gate premise) is reusable.
- One pre-existing, unrelated issue (`task lint` SA1019) remains open in `deferred-items.md` — not a blocker for 07-04/07-05/07-06/07-07.

## Self-Check: PASSED

- FOUND: `internal/store/store.go` (SearchOptions + conditional gate)
- FOUND: `internal/store/store_test.go` (new + extended tests)
- FOUND: `internal/store/migratebacklog.go` (re-grounded doc comment)
- FOUND: `.planning/phases/07-console-cli-state-surfacing/07-03-SUMMARY.md`
- FOUND: `.planning/phases/07-console-cli-state-surfacing/deferred-items.md`
- FOUND commit `77786bb0` (Task 1)
- FOUND commit `bdfe25de` (Task 2)
- FOUND commit `3ec3c5ed` (Task 3)
- FOUND commit `3ea1f318` (deviation doc)

---
*Phase: 07-console-cli-state-surfacing*
*Completed: 2026-08-21*
