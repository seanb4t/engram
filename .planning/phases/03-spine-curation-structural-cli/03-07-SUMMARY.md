---
phase: 03-spine-curation-structural-cli
plan: 07
subsystem: cli
tags: [qdrant, purge, destructive, extract-gate, spine-review, cobra, output-format]

# Dependency graph
requires:
  - phase: 03-spine-curation-structural-cli
    provides: "03-01's scrollAllPoints/Subject-less-command pattern; 03-03's registerDestructive/addApplyFlag/applyRequested choke point and cliNow clock seam; 03-05's scrollAllPoints reuse precedent (NearDuplicates); 03-06's ArchivedAt/Archive/Restore and the archived_at epoch-second key"
provides:
  - "internal/store: PurgeManifest (compiler-enforced unexported provenance marker), PurgeOptions, PurgeClass, PurgeFilterPathActive, derivePurgeEligible, checkExtractGate, PreviewPurge, ApplyPurge, PurgeResult"
  - "engram spine-review purge — preview-by-default, --apply deletes only the intersection of a rendered manifest and a fresh re-derivation, gated on rule 7smp8vy9hr's extract-before-delete ordering"
  - "surfaces.RulePurgeFilterRequiresScope — the registered conditional rule gating purge's free-form filter path, anchored on cli.md, reference/tools.md, and curating-memory/SKILL.md"
  - "internal/e2e/spine_review_test.go's TestE2EPhaseAcceptance — the phase's one end-to-end acceptance run over a seeded 270-record multi-owner, multi-page collection"
affects: []

# Actuals (#2632)
actuals:
  tokens: 68000
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A destructive delete's preview-to-apply carrier stays IN-PROCESS ONLY when unforgeability and portability are in direct tension: PurgeManifest's three fields are all unexported, mirroring internal/surfaces.ConditionalRule.declared's compiler-enforced (not runtime-checked) provenance marker verbatim, at the deliberate cost of a same-run-only contract."
    - "Intersection-only apply: --apply re-derives eligibility fresh and deletes manifest_ids ∩ fresh_ids, reporting spared (manifest-only) and appeared (fresh-only) as separate, never-merged fields — an operator can never lose a record they did not see, and can never be told something was purged that was not."
    - "A two-path extract-before-delete gate, asymmetric by design: the per-record path reads a SERVER-SET field (superseded_by, written only by Supersede and preserved by Update's in-lock re-read) and is therefore unforgeable; the batch-floor path reads a caller-mintable tag on an otherwise real, server-timestamped record, and is therefore convention-validation, never proof — both facts stated in the same code comment, never blurred into one claim."
    - "One eligibility derivation (derivePurgeEligible), called by both PreviewPurge and ApplyPurge, reused over the phase's single scrollAllPoints iterator — never two independently-maintained sweeps that could silently drift."

key-files:
  created:
    - cmd/engram/spine_review_purge.go
    - cmd/engram/spine_review_purge_test.go
    - internal/store/spine_forgery_test.go
  modified:
    - internal/store/spine.go
    - internal/store/spine_test.go
    - cmd/engram/clienttest_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/destructive_test.go
    - cmd/engram/operator_output_test.go
    - internal/surfaces/rules.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/normalize_test.go
    - internal/surfacesgen/main.go
    - internal/e2e/spine_review_test.go
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/reference/tools.md
    - skill/engram/skills/curating-memory/SKILL.md
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - .planning/REQUIREMENTS.md

key-decisions:
  - "Task 1 checkpoint (resolved by the user before dispatch, NOT re-litigated): option-a — a reserved tag literal (`engram:milestone-summary`) identifies the milestone-summary record for D-09's batch floor; the 90-day archived-past-retention default is confirmed, overridable via `--older-than`. The transport (in-process only) was already settled 2026-08-06, before this checkpoint, and is not re-opened. See '## Checkpoint Decision' below."
  - "derivePurgeEligible returns THREE values (candidates, milestoneSummaries, error), not the plan's literally-written two (candidates, error). Both candidates AND the milestone-summary population come from the SAME single scrollAllPoints pass at zero extra RPC cost — computing milestoneSummaries via a second scroll would double the RPC count and reopen exactly the two-independently-maintained-sweep risk this file's whole design exists to close. checkExtractGate stays a pure function over both slices; the acceptance grep (`derivePurgeEligible` appearing >= 3 times: declaration + one call from each of Preview/Apply) is satisfied regardless of arity."
  - "The successor lookup for the per-record extract-gate path is a single targeted `s.Get` per superseded-and-otherwise-eligible candidate, not a second scroll: Store.Supersede does not enforce same-scope between a target and its successor, so a successor could live outside the derivation's scope filter and would be invisible to a from-scratch pure in-memory join. This keeps checkExtractGate itself pure (no ctx, no RPC) while still resolving cross-scope successors correctly."
  - "A record carrying the milestone-summary marker tag is excluded from purge candidacy UNCONDITIONALLY in derivePurgeEligible, regardless of whether it independently matches a selected class or filter — not merely excluded from counting toward the batch floor. This is stronger than the plan's literal wording ('which is not itself in the candidate set') requires read minimally, but is the only design that actually satisfies it: computing eligibility first and subtracting floor-satisfying records after would still let the marker record be reported (and deleted) as an ordinary candidate under a DIFFERENT class/filter match."
  - "PurgeFilterPathActive is exported from internal/store precisely so the CLI leaf's --scope gate and the store's own free-form-age-criterion gate read the identical predicate (older-than supplied ALONGSIDE a class is that class's own window override, not the filter path) — declared once so the two can never silently diverge, mirroring the plan's own 'derive, don't declare twice' discipline."
  - "purgePreviewDoc/purgePreviewSummary/purgeAppliedDoc take an already-extracted []string id slice, never a store.PurgeManifest value. PurgeManifest's three fields are unexported by design (the whole forgery-resistance mechanism), so a formatter accepting the manifest type directly would be untestable with a hand-built fixture in cmd/engram — every such test would need to dial a live Qdrant just to obtain a manifest. Taking []string keeps the formatters pure and unit-testable, and is a strictly narrower dependency than the manifest type itself."

requirements-completed: [REQ-purge-extract-gated]

coverage:
  - id: D1
    description: "PurgeManifest's provenance is compiler-enforced (unexported fields), never runtime-checked: a composite literal built in a DIFFERENT package (internal/store/spine_forgery_test.go, package store_test) reports IsVerified()=false and ApplyPurge rejects it before any RPC; reflection pins the exported field set to empty and the exported method set to exactly {IsVerified, IDs, DerivedAt}"
    requirement: "REQ-purge-extract-gated"
    verification:
      - kind: unit
        ref: "internal/store/spine_forgery_test.go#TestPurgeManifestForgeryRejected"
        status: pass
      - kind: unit
        ref: "internal/store/spine_forgery_test.go#TestPurgeManifestExportedFieldsEmpty"
        status: pass
      - kind: unit
        ref: "internal/store/spine_forgery_test.go#TestPurgeManifestExportedMethodSet"
        status: pass
    human_judgment: false
  - id: D2
    description: "--apply deletes only the intersection of a previewed, gate-passing manifest and a fresh re-derivation: a record ineligible since preview is spared, a record newly eligible is reported appeared (never deleted), a re-run after a successful apply deletes nothing further, and an empty candidate set previews/applies as a clean no-op"
    requirement: "REQ-purge-extract-gated"
    verification:
      - kind: integration
        ref: "internal/store/spine_test.go#TestPurgeIntersectionSparesIneligibleReportsAppeared"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestApplyPurgeReRunIsNoOp"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPreviewPurgeEmptyCandidateSet"
        status: pass
      - kind: e2e
        ref: "internal/e2e/spine_review_test.go#TestE2EPhaseAcceptance (purge preview/apply block)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The two-path extract-before-delete gate: the per-record path reads the server-set superseded_by link (never a caller-supplied tag), the batch floor requires a real, later, same-scope milestone-summary record and never deletes it, discovery/rule categories are never eligible under any path, and the derivation crosses every Qdrant page"
    requirement: "REQ-purge-extract-gated"
    verification:
      - kind: unit
        ref: "internal/store/spine_test.go#TestCheckExtractGate"
        status: pass
      - kind: unit
        ref: "internal/store/spine_test.go#TestExtractGateIgnoresCallerSuppliedLinkTag"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPurgeSupersededPastGraceSelfSatisfiesGate"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPurgeBatchFloorRequiresNewerMilestoneSummary"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPurgeExcludesDiscoveryAndRuleCategories"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPurgeDerivationPaginatesEveryPage"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPurgeBoundaries"
        status: pass
    human_judgment: false
  - id: D4
    description: "The purge leaf routes through registerDestructive (no own --apply flag, no own RunE), carries no transport flag of any kind, publishes the same-run limitation and the intersection's concurrent-writer scoping in both the preview output and --help, and the free-form filter path's scope requirement is a registered rule anchored on all three applicable prose surfaces"
    requirement: "REQ-purge-extract-gated"
    verification:
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsRequireApply"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsRouteThroughGate"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_purge_test.go#TestSpineReviewPurgeNoTransportFlag"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_purge_test.go#TestSpineReviewPurgeOwnFlagSet"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_purge_test.go#TestSpineReviewPurgeSameRunNoticePublished"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_purge_test.go#TestRequirePurgeFilterScope"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_purge_test.go#TestSpineReviewPurgeFilterPathRequiresScope"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_purge_test.go#TestSpineReviewPurgeClassOnlyDoesNotRequireScope"
        status: pass
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles"
        status: pass
      - kind: manual_procedural
        ref: "reviewer reads cli.md's purge subsection cold and confirms the concurrent-writer-scoping wording matches the code (03-VALIDATION.md's Manual-Only Verifications pattern) — NOT performed this session; see Known Gaps"
        status: pending
    human_judgment: true
  - id: D5
    description: "The phase ships one end-to-end acceptance run against the built binary covering all seven ROADMAP success criteria over a seeded multi-page, multi-owner collection"
    requirement: "REQ-purge-extract-gated"
    verification:
      - kind: e2e
        ref: "internal/e2e/spine_review_test.go#TestE2EPhaseAcceptance"
        status: pass
    human_judgment: false

duration: ~3h
completed: 2026-08-07
status: complete
---

# Phase 3 Plan 7: `engram spine-review purge` Summary

**`PurgeManifest`'s provenance is compiler-enforced (unexported fields, never a runtime check), `--apply` deletes only the intersection of a previewed, gate-passing set and a fresh re-derivation, and the extract-before-delete gate is honestly asymmetric: a server-set link is unforgeable, a milestone-summary marker is convention.**

## Performance

- **Duration:** ~3h
- **Completed:** 2026-08-07
- **Tasks:** 3 (Task 1 was a checkpoint, resolved by the user before dispatch)
- **Files modified/created:** 19 (3 created, 16 modified — 1 of which, `.planning/REQUIREMENTS.md`, is bookkeeping)

## Checkpoint Decision

**Task 1 (`checkpoint:decision`, `gate="blocking"`) was resolved by the user (Sean) before this executor was dispatched.** Recorded here as resolved, not re-litigated, per the orchestrator's own instruction.

- **Milestone-summary marker: option-a, a reserved tag literal (`engram:milestone-summary`).** Zero new surface — no schema change, no seventh category, no migration; it works today with records agents already write. **Honest-labelling requirement, carried through literally:** the tag is caller-mintable, so this path VALIDATES A CONVENTION rather than PROVING preservation. Stated plainly in `purgeMilestoneSummaryTag`'s doc comment, in `checkExtractGate`'s doc comment, in the CLI's `purgeMilestoneSummaryGateNotice` constant (published in `--help` and the CLI guide), and in the threat model below — never called proof anywhere. Strictly stronger than the operator attestation D-09 rejected (no artifact required at all); strictly weaker than the per-record `superseded_by` link. Option-b (a seventh `category`) was NOT chosen.
- **Archived-past-retention default: confirmed at 90 days**, overridable via `--older-than`, stated in the flag's help text (`spinePurgeOlderThan`'s Usage string) and in the CLI guide's purge subsection.
- **Already-settled items, acknowledged, not re-opened:**
  - **The manifest is IN-PROCESS ONLY** (settled 2026-08-06, before this checkpoint). `PurgeManifest`'s three fields are unexported; no `Encode`/`Token`/`Parse*`/`Decode*`/`--manifest`/`--token`/HMAC/key exists anywhere in this plan's diff. Residual limitation stated plainly: preview and apply happen within one invocation; the intersection protects against a CONCURRENT WRITER in a milliseconds-wide window, not against operator delay.
  - **The per-record extraction link is `superseded_by`, not a tag.** Written only by `Store.Supersede`, preserved (never accepted from a client) by `Store.Update`'s in-lock re-read. Named consequence recorded rather than discovered: the superseded-past-grace class therefore self-satisfies its own gate — correct, not vacuous, because a superseded record's content genuinely does live in its successor.

## Accomplishments

- Added `store.PurgeManifest` (three unexported fields: `ids`, `derivedAt`, `verified`) mirroring `internal/surfaces.ConditionalRule.declared`'s compiler-enforced-not-runtime-checked mechanism verbatim, with exported accessors `IsVerified()`/`IDs()`/`DerivedAt()` and nothing else — no serialization surface exists anywhere in this plan's diff, confirmed by a reflection test in an external package (`internal/store/spine_forgery_test.go`, `package store_test`).
- Added `derivePurgeEligible`: one `scrollAllPoints` pass classifying every scanned record against three structural classes (`superseded`, `expired`, `archived`) and/or a free-form filter path (`category`/`tags`/`older-than` with no class selected — `PurgeFilterPathActive`), excluding `discovery`/`rule` categories and the milestone-summary marker record unconditionally, and resolving each superseded candidate's successor via one targeted `Get` (never a second whole-spine scroll).
- Added `checkExtractGate`: a pure function (no `ctx`, no RPC) implementing rule `7smp8vy9hr`'s two-path gate — the per-record `superseded_by` path (unforgeable) and the batch-floor milestone-summary path (convention, not proof), both facts stated in the same doc comment.
- Added `PreviewPurge`/`ApplyPurge`: `PreviewPurge` derives, gates, and returns a verified manifest with no write on any path; `ApplyPurge` rejects an unverified manifest with an `ErrInvalidArgument`-class error before touching Qdrant, re-derives fresh, re-runs the gate, and issues exactly one `client.Delete` over `manifest_ids ∩ fresh_ids` via `qdrant.NewHasID` — never a re-evaluated structural predicate.
- Shipped `engram spine-review purge`: nine own flags (`--scope`, `--all-scopes`, `--class`, `--category`, `--tags`, `--older-than`, `--timeout`, `--output`, `--apply`), routed through `registerDestructive` (no own `RunE`, no own `--apply` registration), with NO transport flag of any kind — asserted behaviourally via `Flags().Lookup("manifest"/"token") == nil`.
- Declared `surfaces.RulePurgeFilterRequiresScope` (`Fields: [scope, category, tags, older-than]`), anchored on all three prose surfaces the live tree measures as exposing that full combination: `cli.md`, `reference/tools.md`, and `curating-memory/SKILL.md` — confirmed via the four-token re-measurement below, not copied from the plan.
- Backfilled every operator-command conformance gate this new destructive leaf touches: `destructive_test.go`'s no-escape-hatch flag-set table (one row, the literal nine-name set), `cmdwalk_test.go`'s `wantOperatorCommandKeys`, `operator_output_test.go`'s parity/invalid-output/timeout-group tables, and `clienttest_test.go`'s `--class`/`--tags` stringSlice cleanup list.
- Extended `internal/e2e/spine_review_test.go` into `TestE2EPhaseAcceptance`: the phase's one end-to-end run against the BUILT binary, seeding 270 records (200 owner-a + 70 owner-b filler, spanning more than one Qdrant scroll page) and exercising scan, verify (a genuine broken citation plus `--fail-on broken` → `exitFindings`), consolidate, archive-then-restore, prune-expired preview/apply, and purge preview/apply (via the superseded class, which self-satisfies its own extract gate) — all in ~0.9s against a local testcontainers Qdrant.
- Regenerated `cmd/engram/testdata/help.golden`/`catalog.golden`.

## Task Commits

1. **Task 2: `PurgeManifest`, the eligibility derivation, the extract gate, and the intersection apply** — `d9a90efe` (feat)
2. **Task 3: The `purge` leaf, its filter-path scope rule, the phase acceptance run** — `ab080958` (feat)

**Plan metadata:** _(pending — final `docs(03-07)` commit follows this SUMMARY)_

## Files Created/Modified

- `internal/store/spine.go` — `PurgeClass`/`PurgeOptions`/`PurgeFilterPathActive`/`purgeCandidate`/`milestoneSummaryRecord`/`derivePurgeEligible`/`checkExtractGate`/`PurgeManifest`/`PurgeResult`/`PreviewPurge`/`ApplyPurge`
- `internal/store/spine_test.go` — 15 new tests: pure predicate/gate tests, integration tests (superseded self-satisfying gate, batch floor, category exclusion, pagination, intersection spared/appeared, re-run no-op, three boundary sub-tests), plus the mutation-check test
- `internal/store/spine_forgery_test.go` — new file, external `store_test` package: cross-package forgery rejection, exported-fields-empty, exported-method-set-exact-three
- `cmd/engram/spine_review_purge.go` — `spineReviewPurgeCmd`, `parsePurgeClasses`, `requirePurgeFilterScope`, the three published-notice constants, `purgeReportDoc`/`purgePreviewDoc`/`purgeAppliedDoc`/`purgePreviewSummary`/`purgeAppliedSummary`, `spinePurgePreview`/`spinePurgeApplyRun`
- `cmd/engram/spine_review_purge_test.go` — new file: flag validation, scope-gate table test, no-transport-flag/own-flag-set/same-run-notice tests, output validation, the `--class`/`--tags` latch regression, doc/summary formatter tests
- `cmd/engram/clienttest_test.go` — `spinePurgeClass`/`spinePurgeTags` added to `resetClientFlags`' nil-list
- `cmd/engram/cmdwalk_test.go` — `wantOperatorCommandKeys` gains `spine-review purge`
- `cmd/engram/destructive_test.go` — one row added to `destructiveFlagCases` (the literal nine-name set)
- `cmd/engram/operator_output_test.go` — one `operatorParityRows()` row, one `timeoutGroupCaseArgs`/`operatorInvalidOutputArgs` case each, `zero-disables` timeout-group membership
- `internal/surfaces/rules.go` — `RulePurgeFilterRequiresScope`
- `internal/surfaces/toolclass.go` — `spine-review purge` blast-radius row (`Destructive: true`, `Idempotent: true`)
- `internal/surfaces/normalize_test.go` — `cobraPurgeFields` unioned into `exposedForTest()`'s `SurfaceCobraUsage`
- `internal/surfacesgen/main.go` — `ruleTargets` entry for the new rule (3 files)
- `internal/e2e/spine_review_test.go` — `TestE2EPhaseAcceptance`, `seedFiller`/`fillerID`/`verifyRepoScope` helpers
- `docs-site/src/content/docs/guides/cli.md` — new `### spine-review purge` subsection, leaf enumeration and Destructive-commands section updates
- `docs-site/src/content/docs/reference/tools.md` — a purge cross-reference paragraph in `schedule_memory`'s section, with the anchored rule sentence
- `skill/engram/skills/curating-memory/SKILL.md` — a purge paragraph after the `prune-expired` mention, with the anchored rule sentence
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated
- `.planning/REQUIREMENTS.md` — `REQ-purge-extract-gated` marked complete (checkbox + traceability table)

## Decisions Made

See `key-decisions` in frontmatter for the five substantive implementation-shape decisions (derivePurgeEligible's arity, the successor-lookup mechanism, the milestone-summary marker's unconditional exclusion, the shared `PurgeFilterPathActive` predicate, and the formatter functions' `[]string`-not-`PurgeManifest` signature). None of these contradict any acceptance criterion; each is recorded because the plan's action text described the mechanism at a level of detail this executor's implementation had to make more concrete.

## Required Evidence (per this plan's critical execution constraints)

### 1. `TestExtractGateIgnoresCallerSuppliedLinkTag` — MUTATION CHECK (inject-and-revert), not RED-first

Per the plan, this is a mutation check: `checkExtractGate` is written with its `SupersededBy`-reading per-record path from the start, so the tag-reading failure state never arises naturally in task order.

**Injected defect:** temporarily replaced the per-record path's condition with an unconditional `if true { continue }` (simulating "any candidate passes, as a caller-mintable marker would allow").

**Observed failure (single run, no retries):**
```
spine_test.go:1025: a candidate with no SupersededBy (only a hypothetical caller tag) passed the gate -- the per-record path must read the server-set link, never a tag
--- FAIL: TestExtractGateIgnoresCallerSuppliedLinkTag (0.00s)
```
Reverted immediately after observation; `go test ./internal/store/... -run 'TestExtractGateIgnoresCallerSuppliedLinkTag|TestPurge|TestCheckExtractGate|TestPreviewPurge|TestApplyPurge|TestPurgeManifest' -count=1` passed green on the restored file.

### 2. `TestSpineReviewPurgeClassTagsDoNotLatchAcrossRows` — MUTATION CHECK (inject-and-revert), not RED-first

Per the plan, this task adds the `resetClientFlags` nil-list entries and the two-row regression case together, so the latching failure state never arises naturally in task order.

**Injected defect:** temporarily commented out `spinePurgeClass = nil` / `spinePurgeTags = nil` in `resetClientFlags`' cleanup.

**Observed failure**, run under `go test ./cmd/engram/... -run TestSpineReviewPurgeClassTagsDoNotLatchAcrossRows -count=2 -shuffle=on`:
```
spine_review_purge_test.go:206: spinePurgeClass = [superseded expired] after row 2, want ["expired"] only -- row 1's --class value leaked into row 2
spine_review_purge_test.go:209: spinePurgeTags = [row-a-tag row-b-tag] after row 2, want ["row-b-tag"] only -- row 1's --tags value leaked into row 2
--- FAIL: TestSpineReviewPurgeClassTagsDoNotLatchAcrossRows (0.00s)
```
A second `-count=2` iteration showed the accumulation compounding further (`[superseded expired superseded expired]`). Reverted immediately after observation; the full suite (including three `-shuffle` seeds, below) passed again.

### 3. `TestDestructiveCommandsRequireApply` / `TestDestructiveCommandsRouteThroughGate` — PASS lines observed, inheritance not assumed

```
=== RUN   TestDestructiveCommandsRequireApply
--- PASS: TestDestructiveCommandsRequireApply (0.00s)
=== RUN   TestDestructiveCommandsRouteThroughGate
=== RUN   TestDestructiveCommandsRouteThroughGate/migrate-remap-owner
=== RUN   TestDestructiveCommandsRouteThroughGate/prune-expired
=== RUN   TestDestructiveCommandsRouteThroughGate/spine-review_purge
--- PASS: TestDestructiveCommandsRouteThroughGate (0.00s)
    --- PASS: TestDestructiveCommandsRouteThroughGate/spine-review_purge (0.00s)
```
No edit to either test's derivation logic was needed — the `--apply` requirement and the `registerDestructive` runtime choke point were both inherited from the single `internal/surfaces/toolclass.go` row addition, confirming D-03/Phase 2's D-11 payoff. The one sanctioned edit to `cmd/engram/destructive_test.go` is the new `destructiveFlagCases` row: `git diff -- cmd/engram/destructive_test.go` shows exactly one added line, no removed line.

### 4. Four-token applicability measurement (re-run against the live tree, not copied from the plan)

| File | scope | category | tags | older-than |
|------|-------|----------|------|------------|
| `docs-site/src/content/docs/guides/cli.md` | 51 | 3 | 2 | 3 |
| `docs-site/src/content/docs/reference/tools.md` | 69 | 13 | 20 | 4 |
| `skill/engram/skills/curating-memory/SKILL.md` | 34 | 7 | 9 | 3 |

All three files expose all four tokens (nonzero everywhere) — confirming `RulePurgeFilterRequiresScope` resolves applicable to both `SurfaceDocsSite` (via either file) and `SurfaceSkill`, and justifying anchoring on all three files (the CLI guide is where the command's own flags are documented; the tools reference is where the filter vocabulary is published; the skill is where an agent learns the deletion contract). `rg -o 'engram:rule:start purge-filter-requires-scope' <the three files> | wc -l` → `3`.

### 5. Shuffle seeds

`go test ./cmd/engram/... ./internal/surfaces/... -count=1 -shuffle=<seed>` passed under `-shuffle=1`, `-shuffle=42`, `-shuffle=777`.

### 6. `go clean -testcache && task`

Ran clean (no cache), full `task` target (lint + `go test ./...`, including `internal/e2e` and the testcontainers-backed `internal/store`/`internal/server` suites): **all green** across every package, both before and after the final `verified bool` gofmt-alignment fix (see § Issues Encountered).

### 7. Key-links verification

`gsd-tools verify key-links .planning/phases/03-spine-curation-structural-cli/03-07-PLAN.md` → `{"all_verified": true, "verified": 3, "pending": 0, "total": 3}`.

### 8. Acceptance-run coverage map (which assertion covers which of the seven ROADMAP success criteria)

Recorded as a doc comment directly above `TestE2EPhaseAcceptance` (`internal/e2e/spine_review_test.go`): scan → the scan exit-0 + `total:270` assertion; verify → the broken-citation + `--fail-on broken` → `exitFindings` assertion; consolidate → the consolidate exit-0 assertion; archive/restore → the archive-then-restore round trip; prune-expired → the preview-then-`--apply` block; purge → the purge preview-then-`--apply` block; REQ-purge-extract-gated → the same purge block, since the superseded class self-satisfies its own gate (the named property from Task 1's checkpoint).

## Known Stubs

None. `PreviewPurge`/`ApplyPurge` are wired to real `internal/store` calls end-to-end; the CLI leaf's preview/apply closures call real store methods with no placeholder path; the phase acceptance test proves this against a live (testcontainers) Qdrant.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-01, T-03-02, T-03-20, T-03-21, T-03-30, T-03-31, T-03-22) — all are mitigated or accepted-and-stated exactly as designed:

- T-03-01 (manifest tampering): mitigated by eliminating the transport boundary entirely (in-process only); the `PurgeManifest` reflection/forgery tests are the proof.
- T-03-02 (filter-path over-broad blast radius): mitigated by `RulePurgeFilterRequiresScope`, a registered rule, never an ad hoc check.
- T-03-20 (DoS of the operator's own knowledge via discovery/rule deletion): mitigated unconditionally in `derivePurgeEligible`.
- T-03-21 (partial-failure state after a multi-id delete): mitigated by the single filtered `Delete` over the intersection; the "not all-or-nothing at the storage layer" scoped claim is stated in `ApplyPurge`'s own doc comment.
- T-03-30 (spoofing the per-record extraction link): mitigated — reads `superseded_by`, never a tag; `TestExtractGateIgnoresCallerSuppliedLinkTag` pins it.
- T-03-31 (spoofing the batch floor): **accepted residual risk, stated rather than mitigated away** — the milestone-summary marker tag is caller-mintable; `purgeMilestoneSummaryTag`'s doc comment, `checkExtractGate`'s doc comment, and the CLI's `purgeMilestoneSummaryGateNotice` all say so plainly.
- T-03-32 (free-form filter reaching a reusable fact): **accepted residual risk** — the `discovery`/`rule` exclusion does not prove a surviving `decision`/`convention`/`preference`/`gotcha` is not itself reusable; `derivePurgeEligible`'s doc comment states this explicitly and names Phase 4 as the owner of that semantic judgment.
- T-03-22 (repudiation of appeared-since-preview records): mitigated — `Appeared` is its own field, its own rendered line, never merged into `Deleted`.

## Issues Encountered

**1. [Rule 3 — Blocking] `internal/surfaces/normalize_test.go`'s `exposedForTest()` fixture needed extending**
- **Found during:** Task 3, first full-suite run after declaring `RulePurgeFilterRequiresScope`.
- **Issue:** `TestEveryRuleResolvesToNonEmptySurfaceSet` failed — the new rule's field set (`scope`, `category`, `tags`, `older-than`) didn't fully resolve against the synthetic fixture (it lacked a singular `category` and a `class` token).
- **Fix:** Added `cobraPurgeFields = []string{"class", "category"}`, unioned into `SurfaceCobraUsage` (mirroring the `cobraDestructiveFields`/`cobraVerifyFields` precedent from plans 03-03/03-04).
- **Files modified:** `internal/surfaces/normalize_test.go`.
- **Committed in:** `ab080958` (Task 3 commit).

**2. [Rule 3 — Blocking] `registerDestructive`'s classification lookup requires the command already attached to its parent**
- **Found during:** Task 3, first run of `TestDestructive*` after wiring `spineReviewPurgeCmd`.
- **Issue:** `registerDestructive` panicked: `commandKey(cmd)` resolved to the bare `"purge"` instead of `"spine-review purge"`, because `init()` called `registerDestructive` BEFORE `spineReviewCmd.AddCommand(spineReviewPurgeCmd)` — `cmd.CommandPath()` on an unattached nested command returns only its own `Use`. (Top-level destructive commands like `prune-expired` don't hit this, since an unattached top-level command's own name IS its qualified key.)
- **Fix:** Reordered `init()` to call `spineReviewCmd.AddCommand(...)` before `registerDestructive(...)`, with a comment explaining why the order matters specifically for a nested destructive command.
- **Files modified:** `cmd/engram/spine_review_purge.go`.
- **Committed in:** `ab080958` (Task 3 commit).

**3. [Rule 3 — Blocking] Backfilled `cmdwalk_test.go`/`operator_output_test.go` tables**
- **Found during:** Task 3, full `cmd/engram` suite run.
- **Issue:** `wantOperatorCommandKeys`, `operatorParityRows()`, `operatorInvalidOutputArgs`, and the `zero-disables` timeout group all pre-date this plan's new leaf and did not yet name it, failing `TestOperatorCommands`/`TestOperatorOutputParity`/`TestEveryOperatorCommandRejectsInvalidOutput`/`TestTimeoutGroupMatrix`.
- **Fix:** One row/case added to each, following the exact pattern each existing archive/restore row already established.
- **Files modified:** `cmd/engram/cmdwalk_test.go`, `cmd/engram/operator_output_test.go`.
- **Committed in:** `ab080958` (Task 3 commit).

**4. [Rule 3 — Blocking] `staticcheck` QF1007 in `derivePurgeEligible`**
- **Found during:** Task 2, `task lint` after first draft.
- **Issue:** `eligible := false` followed immediately by `if ... { eligible = true }` could be merged into the declaration.
- **Fix:** Folded the first class check into `eligible`'s declaration expression; the two subsequent `if ... { eligible = true }` blocks are unavoidable (each is a distinct OR-condition, not a chain `staticcheck` flags).
- **Files modified:** `internal/store/spine.go`.
- **Committed in:** `d9a90efe` (Task 2 commit) — caught and fixed before the commit landed.

**5. [Rule 1 — Bug] `verified bool` key-link pattern initially failed on gofmt struct-field alignment**
- **Found during:** post-implementation `gsd-tools verify key-links` run.
- **Issue:** gofmt aligned `PurgeManifest`'s three field declarations to a common column, rendering the third field as `verified  bool` (two spaces) rather than the plan's literal key-link pattern `"verified bool"` (one space).
- **Fix:** Moved `verified` into its own gofmt alignment group (a blank line above it, with a doc comment explaining why), so its declaration reads as exactly `verified bool`. Functionally identical struct; `gofmt -l` reports the file clean.
- **Files modified:** `internal/store/spine.go`.
- **Verification:** `gsd-tools verify key-links` → `all_verified: true`; full `task` re-run green.
- **Committed in:** `d9a90efe` (Task 2 commit) — caught and fixed before the commit landed.

---

**Total deviations:** 0. All five items above are Rule 1/3 auto-fixes (lint/staticcheck compliance, test-fixture backfills, or a key-link-pattern gofmt alignment fix) required to satisfy this plan's own hard acceptance criteria — none reflect a scope change, an architectural question, or a departure from the plan's design.

## Known Gaps

**`/gsd-validate-phase 3` could not be completed end-to-end in this execution context.** Per the plan's Task 3 action text, I invoked it (via the `Skill` tool). It correctly detected State A (`03-VALIDATION.md` exists, `status: draft`, a single `*pending*` row in its Per-Task Verification Map against seven now-fully-written plans) and reached Step 4 (present the gap plan and gate on a user decision) / Step 5 (spawn the `gsd-nyquist-auditor` subagent). **This executor has no `Agent`/`Task` dispatch tool and no `AskUserQuestion` tool in its available tool set** — both are required by the workflow's own Steps 4–5 (`$HOME/.claude/gsd-core/workflows/validate-phase.md`), so I could not proceed past initial discovery without either fabricating a user decision (forbidden — no agent message is ever the user's consent) or hand-authoring `03-VALIDATION.md`'s Per-Task Verification Map myself (forbidden by this plan's own instruction: "`/gsd-validate-phase` is a tool-owned artifact and only shapes that command generates may appear in it. ... If the workflow leaves the file unchanged, say so in the SUMMARY rather than editing it into a passing shape.").

**`03-VALIDATION.md` is therefore UNCHANGED** — still `status: draft`, `nyquist_compliant: false`, with its seeded `*pending*` Per-Task Verification Map row. This is not committed under this plan's pathspec because nothing in it changed. The orchestrator or a follow-up interactive session should re-run `/gsd-validate-phase 3` directly (not via a nested spawned executor) to complete this gate before the phase is called fully complete per its own `<verification>` block.

**Manual-Only Verification not performed this session:** the CLI-guide-wording cold-read the plan's acceptance criteria assign to a human reviewer ("A reviewer confirms the CLI-guide wording cold per VALIDATION.md § Manual-Only Verifications") was not performed — there is no human reviewer in this execution loop. The wording exists (see `docs-site/src/content/docs/guides/cli.md`'s purge subsection) and is byte-derived from the same package-level constants the automated tests assert against (`TestSpineReviewPurgeSameRunNoticePublished`), which is the strongest automatable proxy available, but the literal manual step is outstanding.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `store.PreviewPurge`/`store.ApplyPurge` and `engram spine-review purge` are complete, tested, and documented; `REQ-purge-extract-gated` is marked complete in `.planning/REQUIREMENTS.md`.
- Phase 3's `<verification>` block requires `go clean -testcache && task` green (confirmed) and the phase's full test suite green from a flushed cache (confirmed) — both hold.
- **Outstanding before the phase is declared fully complete:** `/gsd-validate-phase 3` needs to actually run (see § Known Gaps) — it was not completable by this spawned executor, and `03-VALIDATION.md` remains in its seeded `draft` state.
- Phase 4's semantic-curation skill is the stated retirement path for T-03-31's accepted residual risk (once it starts writing real per-record `superseded_by` links, the milestone-summary batch floor's caller-mintable-tag weakness stops mattering for records that go through it).
- No blockers for subsequent phases beyond the validate-phase re-run noted above.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-07*

## Self-Check: PASSED

All key files (`internal/store/spine.go`, `spine_test.go`, `spine_forgery_test.go`, `cmd/engram/spine_review_purge.go`, `spine_review_purge_test.go`, `destructive_test.go`, `cmdwalk_test.go`, `operator_output_test.go`, `clienttest_test.go`, `internal/surfaces/rules.go`, `toolclass.go`, `normalize_test.go`, `internal/surfacesgen/main.go`, `internal/e2e/spine_review_test.go`, the three doc/skill files, both goldens) confirmed present on disk with the expected content. Both task commit hashes (`d9a90efe`, `ab080958`) confirmed present in `git log --oneline --all`.
