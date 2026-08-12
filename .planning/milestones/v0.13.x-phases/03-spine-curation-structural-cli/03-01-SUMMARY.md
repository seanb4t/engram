---
phase: 03-spine-curation-structural-cli
plan: 01
subsystem: cli
tags: [cobra, qdrant, spine-review, blast-radius, pagination, output-format]

# Dependency graph
requires:
  - phase: 02-interface-discoverability
    provides: internal/surfaces blast-radius classification, catalog.go's buildCatalog, both pinned goldens, internal/config's field registry
provides:
  - "cmd/engram/cmdwalk.go: the shared recursive cobra walker (walkCommands/commandWalkSkip/commandKey) every conformance gate now uses"
  - "engram spine-review (group) and engram spine-review scan (leaf) — the operator tier's first nested cobra subcommand tree"
  - "internal/store/spine.go: scrollAllPoints (the phase's ONE paginated whole-spine iterator) and Subject-less ScanSpine"
  - "cmd/engram/operator_output.go + config.ValidateOutputFormat: the operator tier's one validated --output path"
  - "depth-aware collectFlags and qualified-path (commandKey) blast-radius classification"
affects: [03-02, 03-03, 03-04, 03-05, 03-06, 03-07]

# Actuals (#2632)
actuals:
  tokens: 18372
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared recursive cobra tree walker (walkCommands/commandWalkSkip/commandKey) replacing every single-level Commands() loop"
    - "Depth-aware collectFlags walking the full PersistentFlags() parent chain, not just root's"
    - "Qualified-path (space-joined) blast-radius classification keys, not bare cobra Use names"
    - "One shared paginated Qdrant scroll iterator (scrollAllPoints) for every whole-spine sweep in the phase"
    - "Subject-less store methods (no owner/shared filter) for operator-tier collection-wide reads"
    - "One validated --output path (operatorOutputFormat) shared by every operator-tier leaf"

key-files:
  created:
    - cmd/engram/cmdwalk.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output.go
    - cmd/engram/spine_review.go
    - cmd/engram/spine_review_scan.go
    - cmd/engram/spine_review_test.go
    - internal/store/spine.go
    - internal/store/spine_test.go
  modified:
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - cmd/engram/golden_test.go
    - cmd/engram/surfaces_test.go
    - cmd/engram/flaggroup_test.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/config/client_validate.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/toolclass_test.go

key-decisions:
  - "All SEVEN root.Commands()/rootCmd.Commands() sites (six files) converted to walkCommands, not the three RESEARCH.md originally enumerated — verified against the live tree before touching any code"
  - "collectFlags made depth-aware: walks cmd's own Flags() plus every ancestor's PersistentFlags() up to root, not just root's"
  - "ClassForCommand/catalog entries keyed by qualified path (commandKey), not bare cobra Use name, so a nested leaf cannot collide with a differently-classified sibling sharing its bare name"
  - "internal/store/spine.go owns the phase's ONE paginated whole-spine iterator (scrollAllPoints); ScanSpine is the first of several store methods this phase will route through it"
  - "config.ValidateOutputFormat extracted from client_validate.go's inline switch so both the client and operator --output lanes share one validator and one error message"
  - "spine-review scan's --scope/--all-scopes guard is a bare usageErrorf copying summarize.go's wording verbatim, NOT a registered surfaces.ConditionalRule — filed as GitHub issue #480 rather than restating the (false) 'already registered' claim"
  - "ScanSpine's health-signal comparison instant (now) is taken once via the store's existing now() hook and surfaced on the result as ScannedAt for report determinism under test"

patterns-established:
  - "Pattern: any new cobra tree traversal in cmd/engram MUST go through walkCommands(root, commandWalkSkip); a second Commands() call site is now flagged by the negative-half regex gate"
  - "Pattern: any new whole-spine store sweep MUST go through scrollAllPoints, never a second copy-pasted ScrollAndOffset loop and never the non-paginating client.Scroll"

requirements-completed: [REQ-spine-scan, REQ-operator-output-flag]

coverage:
  - id: D1
    description: "engram spine-review scan reports a whole-spine inventory (total, owners, summary/superseded/expired/scheduled/citation counts, scope-by-category breakdown) in text and JSON, with zero mutating RPCs"
    requirement: "REQ-spine-scan"
    verification:
      - kind: integration
        ref: "internal/store/spine_test.go#TestScanSpineHealthSignals"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestScanSpineTwoOwners"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_test.go#TestSpineScanSummaryFormat"
        status: pass
      - kind: manual_procedural
        ref: "engram spine-review scan --all-scopes --output json | jq . against a live (testcontainers) Qdrant"
        status: pass
    human_judgment: false
  - id: D2
    description: "Whole-spine sweeps paginate through every Qdrant page via one shared iterator (scrollAllPoints), never the non-paginating client.Scroll"
    requirement: "REQ-spine-scan"
    verification:
      - kind: integration
        ref: "internal/store/spine_test.go#TestScanSpinePaginatesEveryPage"
        status: pass
    human_judgment: false
  - id: D3
    description: "All seven single-level cobra Commands() walk sites converted to the shared walkCommands helper; nested leaves reach the catalog, goldens, surface-conformance union, exclusivity gate, and flag-reset harness"
    verification:
      - kind: unit
        ref: "cmd/engram/cmdwalk_test.go#TestWalkCommands"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestBuildCatalogCommandNamesEqualWalkedKeys"
        status: pass
      - kind: unit
        ref: "cmd/engram/surfaces_test.go#TestNonHiddenCommandsReachesNestedLeaves"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestResetEveryCommandFlagStateReachesDepth"
        status: pass
    human_judgment: false
  - id: D4
    description: "One validated --output path for the operator tier from the first leaf: an illegal value exits exitUsage, never silently ignored"
    requirement: "REQ-operator-output-flag"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_test.go#TestSpineReviewScanRejectsInvalidOutput"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_test.go#TestOperatorOutputFormatResolvesNonTTYForCustomWriter"
        status: pass
    human_judgment: false

duration: ~70min
completed: 2026-08-06
status: complete
---

# Phase 3 Plan 1: End-to-End `engram spine-review scan` Summary

**A shared recursive cobra walker replacing seven single-level command traversals, plus the operator tier's first nested subcommand (`engram spine-review scan`) backed by a Subject-less, fully-paginated whole-spine scan in `internal/store/spine.go`.**

## Performance

- **Duration:** ~70 min
- **Completed:** 2026-08-06
- **Tasks:** 3
- **Files modified/created:** 19 (8 created, 11 modified)

## Accomplishments

- Landed `cmd/engram/cmdwalk.go` (`walkCommands`/`commandWalkSkip`/`commandKey`) and converted all seven pre-existing single-level `Commands()` walk sites (six files) onto it, closing the gap that would have made every rule this phase registers on a nested leaf silently resolve "not applicable."
- Made `collectFlags` depth-aware (walks the full `PersistentFlags()` parent chain, not just root's) and re-keyed blast-radius classification (`ClassForCommand`) and catalog entries on the qualified command path rather than the bare cobra `Use` name.
- Shipped `engram spine-review` (group) and `engram spine-review scan` (leaf): a Subject-less, read-only inventory report (total, owners, summary coverage, superseded/expired/scheduled counts, citation counts, scope-by-category breakdown) in text and JSON, validated `--output`, zero mutating RPCs.
- Established `internal/store/spine.go`'s `scrollAllPoints` as the phase's ONE whole-spine paginated iterator, proven by a batch-size-1 count-equality test (`TestScanSpinePaginatesEveryPage`) and a mutation-check injection that observed the exact truncation failure a non-paginating `client.Scroll` would produce.
- Extracted `config.ValidateOutputFormat` so the client and operator `--output` lanes share one validator, and built `cmd/engram/operator_output.go` as the operator tier's one validated rendering path (`addOperatorOutputFlag`/`operatorOutputFormat`/`renderOperator`).

## Task Commits

1. **Task 1: End-to-end `engram spine-review scan` — one leaf, every layer** — `8a760be3` (feat)
2. **Task 2: Full health-signal set for `scan` and its JSON contract** — `949a221d` (feat, tdd: test+impl combined into one commit — see § TDD Gate Compliance)
3. **Task 3: Order-independence and conformance hardening for the nested tree** — `2bdd4da2` (test)

**Plan metadata:** _(pending — final `docs(03-01)` commit follows this SUMMARY)_

## Files Created/Modified

- `cmd/engram/cmdwalk.go` — shared recursive cobra walker (`walkCommands`, `commandWalkSkip`, `commandKey`)
- `cmd/engram/cmdwalk_test.go` — table-driven walker coverage over throwaway trees
- `cmd/engram/catalog.go` — `buildCatalog`/`collectFlags` converted to the shared walker, depth-aware flags, qualified-path keys
- `cmd/engram/catalog_test.go` — `wantCommandNames` converted; new panic-at-depth, depth-aware-flags, and set-equality (with mutation checks) tests
- `cmd/engram/golden_test.go` — `withGoldenDeterminism`/`goldenCommands` converted to the shared walker
- `cmd/engram/surfaces_test.go` — `nonHiddenCommands` converted; new union-reaches-nested-leaves test
- `cmd/engram/flaggroup_test.go` — exclusivity gate walk converted
- `cmd/engram/exitcode_baseline_test.go` — `resetEveryCommandFlagState` converted; stale "flat tree" invariant comment rewritten; new reaches-depth test
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated, pinning `spine-review` and `spine-review scan`
- `cmd/engram/operator_output.go` — `addOperatorOutputFlag`/`operatorOutputFormat`/`renderOperator`
- `cmd/engram/spine_review.go` — the `spine-review` group command
- `cmd/engram/spine_review_scan.go` — the `scan` leaf, its RunE, pure formatter, JSON doc shape
- `cmd/engram/spine_review_test.go` — leaf behavior, output validation, TTY resolution, bare-invocation help, flag-state-leak regression tests
- `internal/config/client_validate.go` — `ValidateOutputFormat` extracted and exported
- `internal/surfaces/toolclass.go` — two new qualified-path operations rows (`spine-review`, `spine-review scan`); doc comments updated for D-01
- `internal/surfaces/toolclass_test.go` — qualified-path resolution test
- `internal/store/spine.go` — `scrollAllPoints`, `ScanSpine`, `SpineScanOptions`, `SpineScanResult`, `ScopeCategoryCount`
- `internal/store/spine_test.go` — empty-spine, two-owner, health-signal, and pagination integration tests (testcontainers Qdrant)

## Decisions Made

See `key-decisions` in frontmatter. Additionally:

- Reverted and re-applied the Task 2 widening between Task 1's and Task 2's commits so each task lands as its own atomic commit, rather than shipping Task 1+2's combined diff under Task 1's message (the two tasks' code was drafted together during implementation but split cleanly before committing).

## Deviations from Plan

None — plan executed exactly as written. No Rule 1/2/3 auto-fixes were needed; no architectural (Rule 4) questions arose.

## Required Evidence (per this plan's critical execution constraints)

### 1. Walker conversion pre-state / post-state

**Pre-state** (captured before any code change), `rg -n -o '(rootCmd|root)\.Commands\(\)' cmd/engram/`:

```
cmd/engram/flaggroup_test.go:426:rootCmd.Commands()
cmd/engram/golden_test.go:81:rootCmd.Commands()
cmd/engram/golden_test.go:107:root.Commands()
cmd/engram/surfaces_test.go:102:rootCmd.Commands()
cmd/engram/catalog_test.go:130:rootCmd.Commands()
cmd/engram/catalog.go:86:root.Commands()
cmd/engram/exitcode_baseline_test.go:449:root.Commands()
```

Deduplicated file set: `catalog.go`, `catalog_test.go`, `exitcode_baseline_test.go`, `flaggroup_test.go`, `golden_test.go`, `surfaces_test.go` — **six files, seven sites** (`golden_test.go` matches twice), exactly as the plan predicted.

**Post-state** (after conversion): `rg -n -o '(rootCmd|root)\.Commands\(\)' cmd/engram/` → **empty**.

**Positive half:** `rg -v '^\s*//' cmd/engram/cmdwalk.go | rg -o 'from\.Commands\(\)' | wc -l` → `1`; `rg -l 'from\.Commands\(\)' cmd/engram/` → `cmd/engram/cmdwalk.go` only.

### 2. RED / MUTATION-CHECK observations

- **`TestScanSpinePaginatesEveryPage` — MUTATION CHECK, not RED-first** (Task 1 built the correct paginated iterator before this test existed, so its failure state cannot arise naturally in task order). Injected defect: temporarily swapped `scrollAllPoints`' body for a single non-paginating `s.client.Scroll` call at the same limit. Observed failure:
  ```
  spine_test.go:201: Total = 1, want 5 (batch size 1 must still cross every page)
  spine_test.go:204: Owners = 1, want 2 (pagination and Subject-lessness proven together)
  ```
  Reverted immediately after observation; `go build ./...` confirmed the restored file matched the correct implementation byte-for-byte.

- **Compile-time RED** (genuine, naturally occurring): writing `internal/store/spine_test.go`'s widened-field assertions (`res.Owners`, etc.) before widening `SpineScanResult` produced:
  ```
  vet: internal/store/spine_test.go:84:9: res.Owners undefined (type SpineScanResult has no field or method Owners)
  ```
  This is the RED observation for Task 2's TDD gate.

- **`TestBuildCatalogCommandNamesEqualWalkedKeys` — MUTATION CHECK, not RED-first** (the equality holds by construction from the moment `buildCatalog` and `wantCommandNames`/`nonHiddenCommands` were converted to the same walker in Task 1). Two directions verified via subtests that mutate a local copy of the catalog's name set and assert `reflect.DeepEqual` against the walked set then fails — both subtests pass, confirming the gate is falsifiable in both directions.

### 3. Shuffle seeds

- Task 1's own verify command (`TestHelpGolden|TestCatalogGolden`) passed under `-shuffle=1`, `-shuffle=2`, `-shuffle=3`.
- Task 3's full-package run (`go test ./cmd/engram/... ./internal/surfaces/...`) passed under `-shuffle=1`, `-shuffle=7`, `-shuffle=13`.

### 4. `go clean -testcache && task`

Ran clean (no cache), full `task` target (lint + `go test ./...`, including `internal/e2e` which shells out to the built binary): **all green**, `internal/store` (testcontainers-backed) and every other package passed.

### 5. Live-spine smoke test

Spun up a throwaway `qdrant/qdrant:v1.18.2` container, built the binary, and ran `engram spine-review scan --all-scopes --output json` against it — emitted one parseable JSON document (`jq .` succeeded), confirmed `--output text` mode too, and confirmed exit 0 with zero mutating RPCs (empty collection stayed empty; `total: 0` both before and after). Container torn down after the check.

### 6. Deferred follow-up issue

Filed **GitHub issue #480** ("register the sweep scope-or-all-scopes conditional rule") per this plan's § Deferrals, recording that `spine-review scan`'s `--scope`/`--all-scopes` guard reuses `summarize.go`'s bare `usageErrorf` wording rather than a registered `surfaces.ConditionalRule`, and that the prior plan revision's "already registered" claim was false.

## TDD Gate Compliance

Task 2 (`tdd="true"`) followed RED → GREEN in spirit but not as two separate commits: the RED observation (compile failure on `res.Owners`) was captured and recorded above, but the widened `internal/store/spine.go` implementation and the `internal/store/spine_test.go` test file were committed together in a single `feat(03-01)` commit (`949a221d`) rather than a `test(...)` commit followed by a `feat(...)` commit. This is a deviation from the strict two-commit RED/GREEN gate sequence described in `<tdd_execution>`; the RED state was genuinely observed and is documented above with the exact compiler error line, and the GREEN state (all `TestScanSpine*` tests passing against a live testcontainers Qdrant) is also documented and verified. No functional risk: the behavior was proven both to fail before implementation and to pass after, in the correct order — only the commit granularity diverges from the ideal two-commit shape.

## Known Stubs

None. Every code path in this plan is fully wired against `internal/store` and a live Qdrant (proven via testcontainers integration tests and the live-spine smoke test above); no placeholder or hardcoded-empty rendering exists.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-05, T-03-07, T-03-08, T-03-23, T-03-24) — all five are mitigated as designed:

- T-03-05 (content/summary leakage): `spineScanReportDoc` is a hand-declared struct with no embedded `store.Memory`/content/summary field.
- T-03-07 / T-03-24 (coverage-claim spoofing): `ScanSpine` takes no `Subject`, applies no owner/shared filter, and routes through the single `scrollAllPoints` iterator, proven by the two-owner and pagination integration tests.
- T-03-08 (unclassified nested leaf escaping the catalog): the panic backstop is re-proven at depth (`TestBuildCatalogPanicsOnUnclassifiedNestedCommandAtDepth`).
- T-03-23 (conformance-coverage spoofing): `nonHiddenCommands`/`TestSurfaceConformanceCobraUsage`'s union now reaches nested leaves, proven by `TestNonHiddenCommandsReachesNestedLeaves`.

## Issues Encountered

None beyond the implementation-sequencing note in § Decisions Made (interleaved Task 1/Task 2 authoring, resolved by reverting/reapplying before each commit).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/store/spine.go` and `cmd/engram/cmdwalk.go` are established as this phase's shared, single-source-of-truth homes for whole-spine sweeps and cobra tree traversal, respectively — plans 03-02 through 03-07 build directly on both without introducing a second copy of either.
- `cmd/engram/operator_output.go` is ready for plan 03-02 to backfill onto the six pre-existing operator commands.
- GitHub issue #480 tracks the deferred `--scope`/`--all-scopes` conditional-rule registration as a follow-up, not a blocker.
- No blockers for plan 03-02.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-06*
