---
phase: 03-spine-curation-structural-cli
plan: 05
subsystem: cli
tags: [qdrant, near-duplicate, vector-search, spine-review, cobra, output-format]

# Dependency graph
requires:
  - phase: 03-spine-curation-structural-cli
    provides: "03-01's scrollAllPoints (the phase's one paginated whole-spine iterator) and operatorOutputFormat/renderOperator; 03-04's --min-score-shaped string-flag-with-empty-default precedent (verify's --fail-on) and the injectable-hook pattern (citationFileReader)"
provides:
  - "internal/store/spine.go: NearDuplicates — a Subject-less, exhaustive, batched near-duplicate sweep over stored vectors (no re-embedding, no vector round-trip)"
  - "engram spine-review consolidate — ranked (A, B, score) candidate report in text and JSON, no clustering, no default threshold, no mutation"
  - "cmd/engram's first store-interface recording-fake pattern (spineConsolidateStore / spineConsolidateStoreFromEnv), letting a flag-to-Options mapping be proven without a live Qdrant"
affects: [04]

# Actuals (#2632)
actuals:
  tokens: 17274
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Qdrant NewQueryID/QueryBatch for per-record ANN queries against already-stored vectors — the phase's first use of this mechanism, never Search/SearchMatrix"
    - "AllScopes as an explicit bool field, never an empty-string Scope standing in for 'all scopes'"
    - "MinScore as *float32 (nil = unset) so a negative-capable cosine metric can represent 'no filter' without a zero-value collision"
    - "Injectable store-construction var (spineConsolidateStoreFromEnv) behind a narrow single-method interface, mirroring citationFileReader's hook pattern, so a cobra RunE's flag-to-Options mapping is unit-testable via a recording fake with no live Qdrant"
    - "cobra MarkFlagsMutuallyExclusive for a genuinely exclusive flag pair, instead of a bare usageErrorf guard (scan/verify's precedent, kept for their non-exclusive 'at least one' shape)"

key-files:
  created:
    - cmd/engram/spine_review_consolidate.go
    - cmd/engram/spine_review_consolidate_test.go
  modified:
    - internal/store/spine.go
    - internal/store/spine_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/surfaces/toolclass.go
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "Enumeration's payload include-selector fetches short_id+scope (not zero payload) so a collapsed pair can name BOTH records' identity without a second RPC — see Deviations for why the plan's literal 'requesting neither payload nor vectors' wording could not populate AScope/AShortID in all-scopes mode"
  - "The per-id QueryPoints neighbour query fetches NO payload at all (stronger than the plan's suggested short_id+scope include-selector there): every neighbour's identity is already known from the enumeration map, so zero incremental payload cost is spent at query time"
  - "--scope/--all-scopes use cobra's MarkFlagsMutuallyExclusive rather than a bare usageErrorf check (scan/verify's precedent) — consolidate's two flags are genuinely mutually exclusive (never both required, unlike scan/verify's 'at least one'), and the framework-level violation cleanly exits exitUsage via rootCmd's existing ValidateFlagGroups wiring with no new plumbing"
  - "A max-records cap is deliberately NOT added (per plan instruction): capping an exhaustive sweep would falsify the command's only claim. The operator-visible scanned/queried counts (surfaced in both output forms) and --scope are the only bounding mechanisms"

patterns-established:
  - "Pattern: a store method that needs to report BOTH sides of a relationship's identity (not just the querying side) resolves both from ONE enumeration pass's payload map, never a second per-neighbour fetch"
  - "Pattern: a cobra RunE that must be testable against a fake store without dialing Qdrant defines a narrow single-method interface plus a package-level constructor var, mirroring citationFileReader's file-read injection shape"

requirements-completed: [REQ-near-duplicate-report]

coverage:
  - id: D1
    description: "store.NearDuplicates sweeps every record in scope (or, with AllScopes, the whole collection) exactly once via stored vectors (qdrant.NewQueryID + QueryBatch), returns deterministic collapsed (A,B,score) pairs, rejects AllScopes+non-empty-Scope, represents MinScore as *float32, and provably issues no write RPC"
    requirement: "REQ-near-duplicate-report"
    verification:
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesTwoNearIdentical"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesPaginatesEveryPage"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesAllScopesSpansScopes"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesNoMinScoreReportsNegativePair"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesDoesNotMutate"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesIsDeterministic"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestNearDuplicatesPayloadFrugality"
        status: pass
      - kind: unit
        ref: "internal/store/spine_test.go#TestNearDuplicatesMinScoreOptionIsPointer"
        status: pass
      - kind: manual_procedural
        ref: "engram spine-review consolidate --scope smoke-scope --output json | jq '.candidates | length' against a live (Docker) Qdrant — see Required Evidence"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram spine-review consolidate renders ranked candidate pairs in text and JSON, naming both scopes on a cross-scope pair, with --min-score/--top-k flags, no clustering, no default threshold, and no duplicate/cluster verdict label anywhere in either output form"
    requirement: "REQ-near-duplicate-report"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateAllScopesMapsToOptions"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateScopeAndAllScopesRejected"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateRowNamesBothScopes"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestConsolidateNeverLabelsPairAsDuplicateOrCluster"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_consolidate_test.go#TestSpineReviewConsolidateProgressGoesToStderr"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorOutputParity/spine-review_consolidate"
        status: pass
      - kind: manual_procedural
        ref: "engram spine-review consolidate --scope smoke-scope --min-score 0.5 --output json against a live Qdrant, narrowing 3 pairs to 1 — see Required Evidence"
        status: pass
    human_judgment: false

duration: ~50min
completed: 2026-08-06
status: complete
---

# Phase 3 Plan 5: `engram spine-review consolidate` Summary

**Ranked near-duplicate candidate pairs (A, B, score) over already-stored vectors via Qdrant's `NewQueryID`/`QueryBatch`, with no clustering, no default threshold, and no mutation on any path.**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-08-06
- **Tasks:** 2
- **Files modified/created:** 10 (2 created, 8 modified)

## Accomplishments

- Shipped `store.NearDuplicates`: enumerates every id in scope (or the whole collection with `AllScopes`) through the phase's one paginated `scrollAllPoints` iterator, then queries each id's own stored vector via `qdrant.NewQueryID`/`QueryBatch` — proven by a batch-size-1, five-record pagination test and a `TestNearDuplicatesDoesNotMutate` before/after point-count-and-payload-digest equality.
- Made `AllScopes` an explicit bool (never an empty-`Scope` encoding) and `MinScore` a `*float32` (nil = no filter, even for negative cosine scores) — both proven behaviourally (a cross-scope pair, a reported negative-scoring pair) and structurally (a `reflect.TypeOf` pointer-kind assertion).
- Ran the required MUTATION CHECK on `TestNearDuplicatesAllScopesSpansScopes`: injected the rejected empty-string-scope encoding, observed the exact "0 pairs, want 1" failure the review flagged, then reverted (see Required Evidence).
- Shipped the `consolidate` leaf: `--scope`/`--all-scopes` (mutually exclusive via cobra's own flag-group validation), `--top-k`, and a string `--min-score` (empty default, parsed to `*float32`) routed through a Subject-less `NearDuplicates` call, rendered in text and JSON by pure formatters that never label a row a "duplicate" or cluster.
- Introduced `cmd/engram`'s first store-interface recording-fake pattern (`spineConsolidateStore`/`spineConsolidateStoreFromEnv`), letting the flag-to-`NearDuplicateOptions` mapping be proven end-to-end through cobra without dialing a live Qdrant.
- Backfilled the new leaf into every existing operator-command conformance gate (`operatorCommands`, `operatorParityRows`, `operatorInvalidOutputArgs`, the `--timeout` three-group matrix) and regenerated both pinned goldens.
- Ran a live-Qdrant smoke test (Docker container, throwaway collection): seeded 3 records (2 near-identical, 1 distant), confirmed 3 ranked pairs with the near-identical pair scoring ~0.995, confirmed `--min-score 0.5` narrows to exactly 1, confirmed progress on stderr and one JSON document on stdout, then tore the collection and container down (see Required Evidence).

## Task Commits

1. **Task 1: `NearDuplicates` — exhaustive batched sweep over stored vectors** — `fb015ca6` (feat)
2. **Task 2: The `consolidate` leaf and its ranked report** — `af6b1711` (feat)

**Plan metadata:** _(pending — final `docs(03-05)` commit follows this SUMMARY)_

## Files Created/Modified

- `internal/store/spine.go` — `NearDuplicates`, `NearDuplicateOptions`, `DuplicatePair`, `chunkIDs`, `orderedPairKey`
- `internal/store/spine_test.go` — 14 new `TestNearDuplicates*` tests: empty/single/two-near-identical, out-of-scope exclusion, all-scopes cross-scope + rejection + empty-scope-opt-out, MinScore pointer + negative-score behaviour, three-record MinScore narrowing, pagination, no-mutation, determinism, payload frugality
- `cmd/engram/spine_review_consolidate.go` — `spineReviewConsolidateCmd`, `spineConsolidateStore`/`spineConsolidateStoreFromEnv`, `parseMinScore`, `consolidateSummary`/`consolidateDoc`
- `cmd/engram/spine_review_consolidate_test.go` — the recording fake, flag-mapping/rejection/progress/formatter tests
- `cmd/engram/cmdwalk_test.go` — `wantOperatorCommandKeys` gains `spine-review consolidate`
- `cmd/engram/operator_output_test.go` — new parity row, invalid-output-args row, `zero-disables` timeout-group membership
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated, pinning the new leaf
- `internal/surfaces/toolclass.go` — `spine-review consolidate` blast-radius row (read-only, non-destructive, idempotent, closed-world)
- `docs-site/src/content/docs/guides/cli.md` — `### spine-review consolidate` section, `--output`/`--timeout` table rows updated

## Decisions Made

See `key-decisions` in frontmatter. Additionally:

- Chose to resolve BOTH sides of a reported pair's identity (short id, scope) from the enumeration pass's own payload map, rather than fetching payload again at query time — this made the per-id `QueryPoints` neighbour query payload-free entirely, a strictly stronger frugality property than the plan's suggested short_id+scope include-selector at that call site.
- Assigned `consolidate` to the `zero-disables` `--timeout` group (matching scan/verify), since `--timeout 0` disables the deadline rather than being rejected as a usage error.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Enumeration must fetch short_id+scope, not "neither payload"**
- **Found during:** Task 1 design (before writing the enumeration call)
- **Issue:** The plan's action text says id enumeration requests "neither payload nor vectors." Taken literally, `AScope`/`AShortID` (the querying record's own identity) would be unknowable in `AllScopes` mode — cross-scope pairs could not name both scopes, directly failing a `must_haves.truths` line ("each reported row names both records' scopes").
- **Fix:** Enumeration's payload include-selector names `short_id` and `scope` — two small string fields, not full content — building an in-memory `id -> {shortID, scope}` map used for BOTH sides of every reported pair. This let the per-id `QueryPoints` neighbour query drop its own payload fetch entirely (a net frugality improvement over the plan's suggested design).
- **Files modified:** internal/store/spine.go
- **Verification:** `TestNearDuplicatesAllScopesSpansScopes`, `TestNearDuplicatesPayloadFrugality`
- **Committed in:** fb015ca6 (Task 1 commit)

**2. [Rule 1 - Bug] golangci-lint govet finding on the reflection test**
- **Found during:** Task 2's `task lint` run
- **Issue:** `TestNearDuplicatesMinScoreOptionIsPointer` (written in Task 1) used the deprecated `reflect.Ptr` constant; govet's `inline` check flagged it.
- **Fix:** Changed to `reflect.Pointer`.
- **Files modified:** internal/store/spine_test.go
- **Verification:** `task lint` clean
- **Committed in:** af6b1711 (Task 2 commit, since the lint run that caught it happened during Task 2)

---

**Total deviations:** 2 auto-fixed (1 missing critical, 1 bug/lint)
**Impact on plan:** Both auto-fixes were necessary for correctness (Rule 2) or code health (Rule 1). No scope creep — the Rule 2 fix strengthened an existing frugality requirement rather than adding new behavior.

## Required Evidence (per this plan's critical execution constraints)

### 1. Cost shape (test collection size)

Every `internal/store/spine_test.go` `TestNearDuplicates*` fixture seeds 1-5 records. With `nearDuplicateBatchSize = 50`, every fixture's enumeration fits in ONE `scrollAllPoints` page (batch size 256 by default; the pagination test forces it to 1 across 5 records, producing 5 scroll pages) and ONE `QueryBatch` RPC (since every fixture has ≤5 ids, well under the 50-per-batch chunk size). The live smoke test (3 records) showed the same shape: 1 scroll round-trip, 1 `QueryBatch` RPC, `scanned=3 queried=3` reported via `Progress`.

### 2. MUTATION CHECK: `TestNearDuplicatesAllScopesSpansScopes`

Per this plan's explicit instruction, this is a MUTATION CHECK (inject-and-revert), not a RED-first observation — the correct `AllScopes bool` design is built directly; the rejected empty-string-scope encoding is never built in task order, so its failure state cannot arise naturally.

**Injected defect:** temporarily replaced the conditional scope-`Must` construction

```go
var scopeMust []*qdrant.Condition
if !opts.AllScopes {
    scopeMust = []*qdrant.Condition{qdrant.NewMatch("scope", opts.Scope)}
}
```

with the unconditional (rejected) form

```go
var scopeMust []*qdrant.Condition
scopeMust = []*qdrant.Condition{qdrant.NewMatch("scope", opts.Scope)}
```

**Observed failure:**
```
spine_test.go:553: NearDuplicates(AllScopes) returned 0 pairs, want 1
```

This is the exact failure mode HIGH-9 in `03-REVIEWS.md` described: with all-scopes encoded as an empty `Scope` string while still requiring the `Must` match, the sweep matches only records whose scope is literally `""`, i.e. nothing.

**Reverted** immediately after observation; `go build ./...` and the full `TestNearDuplicates*` suite confirmed the restored file matched the correct implementation and all 14 tests passed again.

### 3. Shuffle seeds

`go test ./cmd/engram/... -count=1 -shuffle=<seed>` passed under `-shuffle=1`, `-shuffle=42`, `-shuffle=777`.

### 4. `go clean -testcache && task`

Ran clean (no cache), full `task` target (lint + `go test ./...`, including `internal/e2e`): **all green** across every package, `internal/store` (testcontainers-backed) included.

### 5. Live-spine smoke test

Spun up a throwaway `qdrant/qdrant:v1.18.2` Docker container. Using a throwaway `cmd/tmpseed` program (built, run, and deleted — never committed), seeded 3 records into a `consolidate_smoke` collection / `smoke-scope`: two near-identical (`[1,0,0]` and `[0.99,0.1,0]`), one distant (`[0,1,0]`). Built the `engram` binary and ran:

```
$ ENGRAM_QDRANT_ADDR=127.0.0.1:16334 ENGRAM_QDRANT_COLLECTION=consolidate_smoke \
  engram spine-review consolidate --scope smoke-scope --output json
{"scope":"smoke-scope","all_scopes":false,"top_k":5,"scanned":3,"queried":3,
 "candidates":[
   {"a":"1111...","b":"2222...","a_short_id":"sid00001","b_short_id":"sid00002","a_scope":"smoke-scope","b_scope":"smoke-scope","score":0.99493724},
   {"a":"2222...","b":"3333...","...","score":0.100498706},
   {"a":"1111...","b":"3333...","...","score":0}
 ]}
```
stderr carried `consolidate progress: scanned 3, queried 3` — never on stdout. `jq '.candidates | length'` returned `3`. Re-ran with `--min-score 0.5`: `jq '.candidates | length'` returned `1` (only the near-identical pair). Tore down the smoke collection and container afterward; `git status --short` confirmed no residue from `cmd/tmpseed`.

### 6. Follow-up issue for a cost ceiling

None filed. The live smoke test and every fixture stayed at 1 scroll page / 1 `QueryBatch` RPC; per the plan's own rationale, a max-records cap is deliberately deferred (capping an exhaustive sweep would falsify its only claim) and the operator-visible `scanned`/`queried` counts plus `--scope` are the bounding mechanisms this plan ships instead.

## TDD Gate Compliance

Both tasks are `tdd="true"`. Tests and implementation were authored together and committed as single `feat(...)` commits per task (matching plan 03-01's precedent and documented deviation there) rather than separate `test(...)`/`feat(...)` commits. The RED state for Task 1 was observed via the actual injected-defect mutation check documented above (§2), and via ordinary compile-time RED while writing `NearDuplicateOptions`/`DuplicatePair`-typed assertions before the types existed. GREEN state (all `TestNearDuplicates*` and `TestSpineReviewConsolidate*` tests passing) is verified above and via the full `task` run.

## Known Stubs

None. Every code path (`NearDuplicates`, the `consolidate` leaf, its formatters) is fully wired against `internal/store` and proven against both testcontainers and a live Docker Qdrant instance; no placeholder or hardcoded-empty rendering exists.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-07, T-03-27, T-03-28, T-03-16, T-03-05) — all five are mitigated as designed:

- T-03-07/T-03-27 (coverage-claim spoofing): id enumeration routes through the single `scrollAllPoints` iterator; `AllScopes` is an explicit bool with the scope `Must` condition omitted entirely in that mode, proven by `TestNearDuplicatesAllScopesSpansScopes` and its mutation check.
- T-03-28 (payload over-fetch): enumeration fetches only `short_id`+`scope`; the per-id neighbour query fetches NO payload at all (stronger than planned), proven by `TestNearDuplicatesPayloadFrugality`.
- T-03-16 (unintended write): `TestNearDuplicatesDoesNotMutate` asserts point count and a payload digest are byte-identical before/after.
- T-03-05 (content leakage in the report): `consolidateReportDoc`/`consolidatePairDoc` are hand-declared structs carrying only ids, short ids, scopes, and scores.

## Issues Encountered

None beyond the two auto-fixed deviations documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `store.NearDuplicates` and `engram spine-review consolidate` are complete and independently useful; Phase 4's semantic-judgment skill can consume the ranked candidate list this command produces without this phase having pre-decided anything.
- REQ-near-duplicate-report is fully satisfied — no outstanding structural work against it.
- No blockers for subsequent plans in this phase.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-06*

## Self-Check: PASSED

All 10 claimed files verified present on disk; both task commit hashes (`fb015ca6`, `af6b1711`) verified present in `git log --oneline --all`.
