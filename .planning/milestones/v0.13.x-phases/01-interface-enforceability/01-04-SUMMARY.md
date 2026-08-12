---
phase: 01-interface-enforceability
plan: 04
subsystem: cli
tags: [cobra, pflag, exit-codes, flag-groups, migrate-remap-owner]

# Dependency graph
requires:
  - phase: 01-02
    provides: "The tracer spine: a declared cobra flag group validated centrally by
      rootCmd.PersistentPreRunE calling cmd.ValidateFlagGroups(), wrapped by usageErrorf
      into a *cliError{exitUsage} — proven end-to-end against a zero-accept listener."
provides:
  - "The second and third D-07 exclusivity claim sites (searchCmd/listCmd's
    --scope/--cross-spine; migrateRemapOwnerCmd's --from/--from-missing/--from-anon)
    converted to cobra's declarative API, reusing the plan 01-02 tracer mechanism
    unchanged — no second interception path."
  - "requireScopeUnlessCrossSpine: the renamed, halved validateScopeCrossSpine, carrying
    only the asymmetric rule cobra's flag groups cannot express."
  - "buildRemapSource simplified: no longer counts selected sources (cobra's
    MarkFlagsOneRequired + MarkFlagsMutuallyExclusive guarantee exactly one), still calls
    store.ValidateOwnerRemap, still pure (no I/O), and now rejects the residual
    '--from empty-string' case cobra's flag groups cannot express."
  - "TestEveryDeclaredExclusivityHasAFlagGroup: a standing conformance gate over the
    live command tree — a flag whose Usage claims mutual exclusivity with a peer must
    have a real MarkFlagsMutuallyExclusive group covering the pair, or the test fails
    and names the exact command/flag pair."
affects: [01-05, 01-06, 01-07, 01-08, 01-09]

# Actuals (#2632)
actuals:
  tokens: 8776
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MarkFlagsMutuallyExclusive is a per-Command method: the same shared Go guard's
      symmetric half had to be declared TWICE (searchCmd and listCmd), one call each —
      there is no way to declare a cross-command group once."
    - "MarkFlagsOneRequired + MarkFlagsMutuallyExclusive together express exactly-one-of;
      MarkFlagsMutuallyExclusive alone permits zero, which migrate-remap-owner cannot
      accept (a source is mandatory)."
    - "Cobra's flag groups track pflag.Flag.Changed (a flag being SUPPLIED), never the
      flag's value — so a supplied '--from \"\"' satisfies MarkFlagsOneRequired but still
      yields an unusable empty-string source, a gap only the pure validator can close."
    - "Reading cobra's own group annotation (cobra_annotation_mutually_exclusive) directly
      off *pflag.Flag.Annotations is the conformance-test-safe way to inspect declared
      groups from another package — the annotation key itself is unexported from cobra,
      but the pflag.Flag field carrying it is public."

key-files:
  created: []
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/client_common_test.go
    - cmd/engram/client_list.go
    - cmd/engram/client_search.go
    - cmd/engram/client_search_test.go
    - cmd/engram/migrate.go
    - cmd/engram/migrate_test.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/flaggroup_test.go

key-decisions:
  - "CONTEXT.md's discretion item resolved: the old validateScopeCrossSpine shared guard
    is NOT retained as a defense-in-depth backstop for the symmetric rule after cobra
    takes it over. A backstop that can never fire (cobra always rejects first) reads as
    an active rule to a future reader and is itself the 'fourth hand-rolled guard' SC1
    exists to eliminate. The function is renamed to requireScopeUnlessCrossSpine and
    keeps only the asymmetric half."
  - "client_common.go:236 was never a separate guard from validateScopeCrossSpine — it
    WAS validateScopeCrossSpine's body (client_common.go, not a client_search.go call
    site as CONTEXT.md's line numbers suggested by phase-start line drift). Same
    resolution: renamed and halved, no second guard introduced."
  - "buildRemapSource's residual gap (a supplied '--from \"\"' satisfying
    MarkFlagsOneRequired but not being a usable source) is real and was closed with an
    explicit rejection, per the plan's flagged assumption — confirmed by reading
    store.ValidateOwnerRemap, which does NOT reject an empty-string RemapFrom source on
    its own (only nil source, empty --to, and from==to)."
  - "store.ValidateOwnerRemap's three bare errors are now typed via
    usageErrorf(\"%w\", err) — %w preserves the store's exact message text (the plan's
    'do not re-word the store's messages' constraint), only the carrier changes."

requirements-completed: [REQ-flag-exclusivity-enforced, REQ-exit-code-unified]

coverage:
  - id: D1
    description: "search and list both reject --scope with --cross-spine (including the
      D-08 widened blast radius of --cross-spine=false) before any dial, via a declared
      cobra flag group on each command"
    requirement: "REQ-flag-exclusivity-enforced"
    verification:
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestFlagGroupScopeCrossSpine"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestValidateScopeCrossSpineParity"
        status: pass
    human_judgment: false
  - id: D2
    description: "migrate-remap-owner accepts exactly one of --from/--from-missing/--from-anon,
      declaratively; buildRemapSource stays pure and its rejections (including the
      residual empty --from case) exit 2"
    requirement: "REQ-flag-exclusivity-enforced"
    verification:
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestFlagGroupMigrateSourceExactlyOne"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_test.go#TestRemapOwnerFlagValidation"
        status: pass
    human_judgment: false
  - id: D3
    description: "A conformance test proves every prose exclusivity claim on the live
      command tree has a real declared cobra flag group behind it, observed RED before
      being restored to green"
    requirement: "REQ-flag-exclusivity-enforced"
    verification:
      - kind: unit
        ref: "cmd/engram/flaggroup_test.go#TestEveryDeclaredExclusivityHasAFlagGroup"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-09 baseline rows for all five behaviors this plan changes flip from
      before to after, staying green"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-03
status: complete
---

# Phase 01 Plan 04: Expand Flag Groups to Search/List/Migrate + Conformance Gate Summary

**Extends plan 01-02's tracer mechanism to the two remaining D-07 exclusivity sites
(search/list's `--scope`/`--cross-spine`, migrate-remap-owner's exactly-one-of source
selection), simplifies `buildRemapSource` while closing a gap cobra's flag groups
cannot express, and adds a standing conformance test that fails the moment a future
command re-states an exclusivity rule in prose without declaring it.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 3 (all auto, TDD on Tasks 1-2)
- **Files modified:** 9

## Accomplishments

**Task 1** — `searchCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")` and the same
call on `listCmd` (a per-`Command` method, so it could not be declared once for both).
`validateScopeCrossSpine` renamed to `requireScopeUnlessCrossSpine`, losing its symmetric
rejection branch entirely — cobra now owns it exclusively, with no defense-in-depth
backstop retained (CONTEXT.md's discretion item, resolved against retention: a guard that
can never fire reads as an active rule to a future reader, and is itself the "fourth
hand-rolled guard" SC1 exists to eliminate). `TestFlagGroupScopeCrossSpine` covers all six
behaviors from the plan's `<behavior>` block, including the D-08 widened blast radius
(`--cross-spine=false` is still a *supplied* member of the group and gets rejected too).
`TestValidateScopeCrossSpineParity` rewritten: the D-04 divergence row (scope set together
with cross-spine — the client is stricter than the server here, deliberately) is now
asserted end-to-end on both `search` and `list`, since the rejection moved one layer
earlier than the Go function it used to live in. Flipped `search/scope+cross-spine-false`'s
baseline row to `landed: true`.

**Task 2** — `migrateRemapOwnerCmd.MarkFlagsMutuallyExclusive("from", "from-missing",
"from-anon")` paired with `MarkFlagsOneRequired` over the same trio (mutual exclusivity
alone permits zero sources, which this command cannot accept — a source is mandatory).
`buildRemapSource` no longer counts selected sources; cobra guarantees exactly one is
supplied before `RunE` — and therefore this function — ever runs. Confirmed by reading
`store.ValidateOwnerRemap` that the residual case flagged in the plan's assumptions is
real: a supplied `--from ""` satisfies `MarkFlagsOneRequired` (the flag was `Changed`) but
`ValidateOwnerRemap` does not itself reject an empty-string `RemapFrom` source — only nil
source, empty `--to`, and `from==to`. Added an explicit rejection for it.
`store.ValidateOwnerRemap`'s three bare errors are now typed via `usageErrorf("%w", err)`,
preserving the store's exact message text per the plan's "do not re-word" constraint.
`TestFlagGroupMigrateSourceExactlyOne` covers the two cobra-owned rejections (no source,
two sources, in both pairings) end-to-end, proving zero Qdrant dials via an
`ENGRAM_QDRANT_ADDR`-pointed accept-counting listener — structurally guaranteed anyway
since `buildRemapSource` runs before `server.StoreFromEnv()` in `RunE` and returns early on
error. `migrate_test.go`'s `TestRemapOwnerFlagValidation` now keeps only the rows
`buildRemapSource` itself still validates (empty `--from`, empty `--to`, `from==to`); the
selected-count rows moved to the end-to-end test. Flipped `migrate-remap/no-source`,
`/two-sources`, and `/identical-from-to`'s baseline rows to `landed: true`.

**Task 3** — `TestEveryDeclaredExclusivityHasAFlagGroup`: walks `rootCmd.Commands()`, and
for every flag whose `Usage` text contains the phrase "mutually exclusive" (parsed
conservatively — split into sentences on `;`/`.`, keep only the sentence naming the
phrase, extract `--flag-name` tokens from it, so `--scope`'s Usage naming `--cross-spine`
twice in two different clauses only counts the exclusivity clause), asserts a real
`MarkFlagsMutuallyExclusive` group covers the pair. Reads the group membership directly off
`*pflag.Flag.Annotations["cobra_annotation_mutually_exclusive"]` — the exact string
confirmed by reading the vendored `cobra@v1.10.2/flag_groups.go`'s unexported
`mutuallyExclusiveAnnotation` constant, rather than reaching for unexported
`*cobra.Command` state. A flag naming a peer the command doesn't declare is itself a
failure (the help text is lying). Covers 8 claimed pairs across `search`/`list`/
`migrate-remap-owner`, well above the plan's 4-pair floor.

## Task Commits

Each task was committed atomically:

1. **Task 1: scope/cross-spine flag group** — `4ffac6df` (feat), including the deviation
   fix below.
2. **Task 2: migrate-remap-owner exactly-one-of** — `36af57ba` (feat).
3. **Task 3: conformance invariant** — `2bda8014` (test).

**Plan metadata:** committed alongside this summary.

## RED Proof for TestEveryDeclaredExclusivityHasAFlagGroup

Per the plan's Task 3 action, `searchCmd.MarkFlagsMutuallyExclusive("scope",
"cross-spine")` was temporarily commented out and the test re-run against that state:

```
=== RUN   TestEveryDeclaredExclusivityHasAFlagGroup/search/--cross-spine
    flaggroup_test.go:442: search --cross-spine Usage claims mutual exclusivity with --scope, but no declared cobra flag group (MarkFlagsMutuallyExclusive) covers both
=== RUN   TestEveryDeclaredExclusivityHasAFlagGroup/search/--scope
    flaggroup_test.go:442: search --scope Usage claims mutual exclusivity with --cross-spine, but no declared cobra flag group (MarkFlagsMutuallyExclusive) covers both
--- FAIL: TestEveryDeclaredExclusivityHasAFlagGroup (0.00s)
```

The failure names the exact command (`search`) and flag pair (`--cross-spine`/`--scope`)
that lost its enforcement. The declaration was restored immediately after (`git diff`
confirmed clean against the committed state) and the test re-verified green.

## Files Created/Modified

- `cmd/engram/client_search.go` — added `MarkFlagsMutuallyExclusive("scope",
  "cross-spine")`; renamed the call to `requireScopeUnlessCrossSpine`.
- `cmd/engram/client_list.go` — added `MarkFlagsMutuallyExclusive("scope",
  "cross-spine")` alongside the existing paging-trio declaration; renamed the call.
- `cmd/engram/client_common.go` — `validateScopeCrossSpine` renamed to
  `requireScopeUnlessCrossSpine`, symmetric branch deleted, doc comment rewritten to
  record where the other half moved and why it has no declarative equivalent.
- `cmd/engram/client_common_test.go` — `TestValidateScopeCrossSpineParity` rewritten: 3
  rows against `requireScopeUnlessCrossSpine` directly, plus an end-to-end subtest for the
  D-04 divergence row on both `search` and `list`.
- `cmd/engram/client_search_test.go` — deviation fix (see below): added
  `resetCommandFlagState(t, searchCmd)` to all 17 tests in the file.
- `cmd/engram/migrate.go` — `buildRemapSource` simplified and typed; both flag groups
  declared on `migrateRemapOwnerCmd`.
- `cmd/engram/migrate_test.go` — `TestRemapOwnerFlagValidation` trimmed to the rows
  `buildRemapSource` still owns.
- `cmd/engram/exitcode_baseline_test.go` — flipped four rows' `landed` to `true`
  (`search/scope+cross-spine-false`, `migrate-remap/no-source`, `/two-sources`,
  `/identical-from-to`).
- `cmd/engram/flaggroup_test.go` — new tests: `TestFlagGroupScopeCrossSpine`,
  `TestFlagGroupMigrateSourceExactlyOne`, `TestEveryDeclaredExclusivityHasAFlagGroup`
  (plus its supporting `flagsClaimedMutuallyExclusive`/`declaredGroupCoversPair`
  helpers).

## Decisions Made

See `key-decisions` in frontmatter. Summary: no defense-in-depth backstop retained for
the symmetric scope/cross-spine rule after cobra took it over; the residual empty-`--from`
gap was confirmed real (not already covered by `store.ValidateOwnerRemap`) and closed
explicitly; store errors typed via `%w` to preserve exact wording.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `pflag.Flag.Changed` leak across `client_search_test.go`'s own
tests, exposed by Task 1's new scope/cross-spine flag group**
- **Found during:** Task 1, running the full `cmd/engram` package suite after Task 1's
  own new tests passed in isolation.
- **Issue:** Every one of the 17 pre-existing tests in `client_search_test.go` called
  `resetClientFlags(t)` only, never `resetCommandFlagState(t, searchCmd)`. This is the
  exact same class of defect plan 01-02 already fixed for `client_list_test.go`
  (documented there as deviation #1), now surfacing on `searchCmd` once its own flags
  entered `ValidateFlagGroups()` scope: `TestClientSearchCrossSpineEndToEnd` and
  `TestClientSearchNoFooterWithoutCrossSpine` — which each supply only ONE of
  `--scope`/`--cross-spine` — spuriously tripped the new mutual-exclusivity group because
  an earlier test in the same run had left the other flag's `Changed` latch set.
- **Fix:** Added `resetCommandFlagState(t, searchCmd)` immediately after
  `resetClientFlags(t)` in all 17 tests in the file.
- **Files modified:** `cmd/engram/client_search_test.go`
- **Verification:** `go test ./cmd/engram/...` green; full package green under
  `-shuffle=on -count=2`.
- **Committed in:** `4ffac6df` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug, confined to test infrastructure — no
production `.go` file outside the plan's declared scope was touched).
**Impact on plan:** Necessary for Task 1's flag group to be provably correct across the
whole package, not just in isolation — the same lesson plan 01-02 already recorded for
`listCmd`, now applied to `searchCmd`.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All three D-07 exclusivity claim sites (`client_list.go`'s paging trio from 01-02,
  `client_search.go`/`client_list.go`'s scope/cross-spine from this plan, and
  `migrate.go`'s exactly-one-of source from this plan) now carry declared cobra flag
  groups. No hand-rolled symmetric guard remains anywhere in the binary
  (`rg -n "validateScopeCrossSpine" cmd/engram/` returns nothing;
  `rg -n "selected :=|selected\+\+" cmd/engram/migrate.go` returns nothing).
- `TestEveryDeclaredExclusivityHasAFlagGroup` is a standing gate: any future command
  that states an exclusivity claim in `Usage` prose without a matching
  `MarkFlagsMutuallyExclusive` call will fail this test by name, closing the
  generalization plan 01-02 flagged as an assumption.
- The D-09 baseline table now has 8 of its rows `landed: true`
  (4 from plan 01-02, 4 from this plan); the remaining operator-command rows are
  later plans' (01-05 through 01-08) to flip.
- `go test ./cmd/engram/... -v -shuffle=on` green; `task test` and `task lint` clean.
- No blockers.

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-03*

## Self-Check: PASSED

All created/modified files and all three task commit hashes (4ffac6df, 36af57ba,
2bda8014) verified present.
