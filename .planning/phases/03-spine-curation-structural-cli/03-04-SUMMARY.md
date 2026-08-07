---
phase: 03-spine-curation-structural-cli
plan: 04
subsystem: cli
tags: [cobra, citations, exit-codes, conditional-rules, drift-detection]

# Dependency graph
requires:
  - phase: 03-spine-curation-structural-cli
    provides: "plan 03-01's cmdwalk.go (walkCommands/commandKey), spine.go's scrollAllPoints/Subject-less pattern; plan 03-02's operatorCommands()/operator_output.go; plan 03-03's registerDestructive/RuleDestructiveRequiresApply pattern for a registered conditional rule anchored via surfacesgen"
provides:
  - "cmd/engram/spine_review_verify.go: engram spine-review verify -- the excerpt-anchored four-tier (valid/moved/broken/unverifiable) citation drift classifier, its repo-identity gate, its RESOLVED path-safety gate, and its --fail-on opt-in exit gate"
  - "internal/store/spine.go: Store.EnumerateCitations -- Subject-less citation enumeration over the phase's one scrollAllPoints iterator"
  - "internal/surfaces/rules.go: RuleVerifyFailOnValues -- the registered conditional rule --fail-on's Usage composes"
  - "cmd/engram/client_common.go: exitFindings = 7, the published taxonomy addition for opt-in-flag findings, with catalog_test.go's TestCatalogExitCodesMatchMapper derivation extended via a named nonConnectProducedCodes allowlist"
affects: []

# Actuals (#2632)
actuals:
  tokens: 21781
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure four-tier citation classifier (verifyFileCitation) taking already-read content and existence, never *store.Store/context.Context/a path -- zero filesystem access in the classification logic itself"
    - "Per-Ref resolution cache (map[string]refResolution) so a citation Ref shared by many records is read at most once, regardless of classification fan-out"
    - "RESOLVED (filepath.EvalSymlinks) path containment, not lexical (absolute-plus-'..') -- the gap a lexical check leaves open against a symlink pointing outside the working tree"
    - "Exact-segment (never substring) scope-vs-derived-identity comparison for repo scoping, split on ':' and matched on a literal 'repo' segment"
    - "A named, explicitly-justified nonConnectProducedCodes allowlist unioned into an anti-drift gate's derived expected set, when a new taxonomy value has no producer the gate's own derivation function can discover on its own"

key-files:
  created:
    - cmd/engram/spine_review_verify.go
    - cmd/engram/spine_review_verify_test.go
  modified:
    - internal/store/spine.go
    - internal/store/spine_test.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/rules.go
    - internal/surfaces/rules_test.go
    - internal/surfaces/normalize_test.go
    - internal/surfacesgen/main.go
    - cmd/engram/client_common.go
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/reference/errors.md
    - docs-site/src/content/docs/guides/upgrade.md

key-decisions:
  - "citationVerdict is populated in two stages: verifyFileCitation (Task 1's pure classifier) sets only Tier/Reason/Ref; the leaf's verifyCitationRecord wrapper (Task 2) fills RecordID/ShortID/Index once it knows which owning record/citation-index produced the verdict. This keeps the Task 1 classifier's signature exactly as specified (no *store.Store, no context.Context, no path) while still producing report rows with full identity."
  - "The repo-identity gate is checked BEFORE the kind check and the path-safety gate, for every citation kind (not only file): a citation from a different repo is unverifiable with reason 'different repo' regardless of whether it is a file/commit/url/repo kind, since there is nothing to usefully check about a citation this working tree does not own."
  - "The per-Ref cache is keyed on the raw citation Ref string (not the resolved path), populated lazily only for citations that pass the repo-identity gate and are kind=file -- a Ref shared across records with different owning scopes is safe because the CLASSIFICATION (repo-identity check) happens per-citation even though the READ is deduplicated per-Ref."
  - "exitcode_baseline_test.go's three new spine-review verify rows cover the usage-error and unreachable-Qdrant paths (feasible without a live backend in this package), not the literal 'default exit 0 with findings' / '--fail-on broken exits 7' scenarios named in the plan's action text -- those two scenarios are unit-tested directly and more precisely via verifyFailOnErr's own pure-function tests (TestVerifyFailOnDefaultExitsZero, TestVerifyFailOnTripsExitFindings) in spine_review_verify_test.go, since cmd/engram carries no live-Qdrant test infrastructure (unlike internal/store's testcontainers-backed tests) through which a genuine end-to-end findings scenario could be driven via runClient."
  - "verifyReport/verifyEntry are hand-declared pure aggregation types (no json tags); verifyReportDoc/verifyEntryDoc are a SEPARATE hand-declared JSON-mode projection (verifyDoc builds one from the other) -- mirroring spineScanReportDoc's discipline so the Excerpt-exclusion guarantee (T-03-14) is enforced by the type itself, not by convention."

requirements-completed: [REQ-citation-drift-verify]

coverage:
  - id: D1
    description: "engram spine-review verify classifies every stored citation into valid/moved/broken/unverifiable, in the specified order, with the moved tier reported separately from broken and broken split by cause (file missing vs excerpt gone)"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyFileCitation"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestExcerptOffsetAt"
        status: pass
    human_judgment: false
  - id: D2
    description: "A drifted excerpt (lines inserted above the cited range, GitHub issue #355's shape) classifies moved, not broken; an excerpt starting at the locator but overrunning its end line classifies valid, not moved (start-anchored at-locator definition)"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyFileCitation/issue_355:_excerpt_drifted_after_lines_were_inserted_above_it"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyFileCitation/excerpt_starts_at_the_locator_but_overruns_its_end_line_--_valid,_not_moved"
        status: pass
    human_judgment: false
  - id: D3
    description: "verify never reads outside the working tree, even through a symlink whose target lies outside it; absolute and parent-traversal Refs are also rejected -- all as unverifiable, never a confident wrong verdict"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyRejectsAbsoluteRef"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyRejectsTraversalEscape"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyRejectsSymlinkEscape"
        status: pass
    human_judgment: false
  - id: D4
    description: "A citation whose owning record's scope names a different repo than the working tree classifies unverifiable with reason 'different repo', by an exact-segment (never substring) comparison, proven against this repo's own live scope shapes including the :ws: overlay form and SCP-style git remotes"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestSameRepoAsCWD"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestNormalizeGitRemote"
        status: pass
    human_judgment: false
  - id: D5
    description: "Citation enumeration is Subject-less and reuses the phase's single scrollAllPoints iterator; proven exhaustive across a forced batch-size-1 sweep and proven to include recall-hidden (superseded) records"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: integration
        ref: "internal/store/spine_test.go#TestEnumerateCitationsPaginatesEveryPage"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestEnumerateCitationsIncludesSuperseded"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestEnumerateCitationsExcludesUncited"
        status: pass
    human_judgment: false
  - id: D6
    description: "verify exits 0 by default even with findings; --fail-on (a registered conditional rule) turns a named tier's findings into the new exitFindings=7 code; an illegal --fail-on value exits 2; exitFindings is distinct from every other taxonomy constant"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyFailOnDefaultExitsZero"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyFailOnTripsExitFindings"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestValidateFailOnRejectsNonsense"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyFailOnExitFindingsIsDistinct"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogExitCodesMatchMapper"
        status: pass
    human_judgment: false
  - id: D7
    description: "The report never includes a citation's Excerpt text on any output path (text or JSON)"
    requirement: "REQ-citation-drift-verify"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_verify_test.go#TestVerifyReportNeverIncludesExcerpt"
        status: pass
    human_judgment: false

duration: ~3h (including a 1Password/op-ssh-sign commit-signing outage mid-run, unrelated to implementation)
completed: 2026-08-06
status: complete
---

# Phase 3 Plan 4: `engram spine-review verify` -- Excerpt-Anchored Citation Drift Classification Summary

**A pure four-tier (valid/moved/broken/unverifiable) citation classifier, a Subject-less citation enumeration over the phase's shared paginated iterator, a RESOLVED (not lexical) path-safety gate, an exact-segment repo-identity comparison, and the new `exitFindings=7` taxonomy code gated behind a registered `--fail-on` conditional rule.**

## Performance

- **Duration:** ~3h (includes a mid-run pause for a 1Password `op-ssh-sign` commit-signing outage on the host, unrelated to the implementation; Task 2's commit was made by the orchestrator using the exact command this executor supplied once signing recovered)
- **Completed:** 2026-08-06
- **Tasks:** 3
- **Files modified/created:** 20 (2 created, 18 modified)

## Accomplishments

- Shipped `verifyFileCitation`: a pure, zero-filesystem-access classifier sorting a stored citation into `valid`/`moved`/`broken`/`unverifiable`, tier order fixed (file-missing, no-excerpt, at-locator, same-file search, excerpt-gone) so a missing file can never report "excerpt gone". The at-locator check is start-anchored (an excerpt overrunning its cited range's end line is still `valid`), and the `moved` tier is a single in-file search only -- no whole-tree fallback, no fuzzy matching -- proven against GitHub issue #355's drift shape.
- Added `Store.EnumerateCitations` (Subject-less, built ONLY over plan 03-01's `scrollAllPoints`, never a second scroll call site), proven exhaustive with a forced batch-size-1 count-equality test and proven to include a recall-hidden (superseded) record.
- Built the `verify` leaf: repo-identity derivation from `git remote get-url origin` (with SCP-style-remote colon-to-slash normalisation), an exact-segment (never substring) scope comparison covering every live scope shape in this repo (`repo:`, `discovery:repo:`, `rule:repo:`, the `:ws:` overlay), and a RESOLVED path-safety gate (`filepath.EvalSymlinks`-based containment, not lexical) that rejects a symlink pointing outside the working tree even when the citing `Ref` itself contains no `..` segments.
- Registered `surfaces.RuleVerifyFailOnValues` and wired `--fail-on <broken|moved|unverifiable|any>` through it; `verify` exits `0` by default even with findings, `--fail-on` turns a named tier's findings into the new `exitFindings = 7` taxonomy code.
- Landed `exitFindings = 7` and `TestCatalogExitCodesMatchMapper`'s edited derivation (a named `nonConnectProducedCodes` allowlist) in the same commit, across `client_common.go`/`catalog.go`/`catalog_test.go`/`exitcode_baseline_test.go`/`reference/errors.md`/`guides/upgrade.md` -- the catalog cannot advertise a code the test's derivation doesn't know about.

## Task Commits

1. **Task 1: The pure four-tier citation classifier** -- `e0ab6adf` (feat)
2. **Task 2: The `verify` leaf -- citation enumeration, repo identity, and the report** -- `2e2ad1cb` (feat) -- committed by the orchestrator after a mid-run 1Password signing outage, using exactly the command this executor supplied; the implementation and its verification are this executor's own work (see § Issues Encountered)
3. **Task 3: `--fail-on` as a registered conditional rule, default exit 0, and the `exitFindings` taxonomy addition** -- `0d4dfca7` (feat)

**Plan metadata:** _(pending -- final `docs(03-04)` commit follows this SUMMARY)_

## Files Created/Modified

- `cmd/engram/spine_review_verify.go` -- `verifyFileCitation`, `excerptOffsetAt`, `repoIdentityFromCWD`/`normalizeGitRemote`, `sameRepoAsCWD`, `resolveContainedRef`/`deepestExistingAncestor`/`isContained`, `citationFileReader` (injectable), `runVerify`/`verifyReport`/`verifyEntry`, `verifySummary`/`verifyDoc`, `validateFailOn`/`verifyFailOnErr`, `spineReviewVerifyCmd`
- `cmd/engram/spine_review_verify_test.go` -- classifier table tests, repo-identity/scope-comparison tables, path-safety/symlink-escape/read-once tests, excerpt-leak and empty-array JSON tests, `--fail-on` gate tests
- `internal/store/spine.go` -- `CitationRecord`, `Store.EnumerateCitations`
- `internal/store/spine_test.go` -- empty-scope, excludes-uncited, includes-superseded, forced-batch-size-1 pagination tests
- `internal/surfaces/toolclass.go` -- `spine-review verify` blast-radius row
- `internal/surfaces/rules.go` -- `RuleVerifyFailOnValues`
- `internal/surfaces/rules_test.go` -- `TestRuleByIDVerifyFailOnValues`
- `internal/surfaces/normalize_test.go` -- `cobraVerifyFields` unioned into `exposedForTest()`'s `SurfaceCobraUsage`
- `internal/surfacesgen/main.go` -- `ruleTargets` entry for the new rule (cli.md only)
- `cmd/engram/client_common.go` -- `exitFindings = 7`
- `cmd/engram/catalog.go` -- `exitFindings` added to `buildCatalog`'s `ExitCodes` list
- `cmd/engram/catalog_test.go` -- `nonConnectProducedCodes` allowlist, `wantExitCodes` extended, `TestCatalogGoldenExitFindingsDescribesFailOn`
- `cmd/engram/cmdwalk_test.go` -- `wantOperatorCommandKeys` extended with `spine-review verify` (Rule 3, see below)
- `cmd/engram/exitcode_baseline_test.go` -- three new `spine-review verify` rows, `wantRows` bumped 34 -> 37
- `cmd/engram/operator_output_test.go` -- `timeoutGroups`/`timeoutGroupCaseArgs`/`operatorInvalidOutputArgs`/`operatorParityRows` extended for the new operator command (Rule 3, see below)
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` -- regenerated
- `docs-site/src/content/docs/guides/cli.md` -- new "spine-review verify" section with the anchored `--fail-on` rule sentence; exit-codes table extended to code 7; operator-tier/timeout tables and prose updated
- `docs-site/src/content/docs/reference/errors.md` -- new section documenting exit `7` as the one code with no hint-code or Connect-code counterpart
- `docs-site/src/content/docs/guides/upgrade.md` -- new numbered `## Unreleased` entry (#11) announcing `spine-review verify` and exit code 7

## Decisions Made

See `key-decisions` in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extended `internal/surfaces/normalize_test.go`'s `exposedForTest()` fixture**
- **Found during:** Task 3, first full `internal/surfaces` run after declaring `RuleVerifyFailOnValues`
- **Issue:** `TestEveryRuleResolvesToNonEmptySurfaceSet` would have failed -- the new rule's `Fields: ["fail-on"]` resolves to zero surfaces against the pre-existing synthetic exposed-fields fixture, which predates this rule.
- **Fix:** Added `cobraVerifyFields` (`fail-on`, `scope`, `all-scopes`) unioned into `SurfaceCobraUsage`, mirroring plan 03-03's identical `cobraDestructiveFields` fix.
- **Files modified:** `internal/surfaces/normalize_test.go`
- **Committed in:** `0d4dfca7` (Task 3 commit)

**2. [Rule 3 - Blocking] Extended `cmd/engram/cmdwalk_test.go`'s `wantOperatorCommandKeys`**
- **Found during:** Task 3's full-package `go test ./cmd/engram/...` run (a narrower, per-task-scoped test run during Task 2 did not catch this)
- **Issue:** `TestOperatorCommands` failed: `operatorCommands()` unexpectedly includes `"spine-review verify"` -- Task 2's leaf, once it gained a non-nil `RunE` and no `--server` flag, was automatically picked up by the existing structural predicate, but the hand-maintained expected-membership fixture (introduced in plan 03-02, predating this plan) had not been told about it.
- **Fix:** Added `"spine-review verify": true` to `wantOperatorCommandKeys`.
- **Files modified:** `cmd/engram/cmdwalk_test.go`
- **Committed in:** `0d4dfca7` (Task 3 commit, though the underlying cause traces to Task 2's registration)

**3. [Rule 3 - Blocking] Extended `cmd/engram/catalog_test.go`'s `wantExitCodes`**
- **Found during:** Task 3's full-package run, same pass as #2
- **Issue:** `TestCatalogListsEveryExitCode` failed (`len(exit_codes) = 8, want 7`) -- this pre-existing anti-drift gate derives its expected count from a hand-declared slice that had not been told about `exitFindings`, exactly the kind of drift its own doc comment warns against.
- **Fix:** Added `exitFindings` to `wantExitCodes`.
- **Files modified:** `cmd/engram/catalog_test.go`
- **Committed in:** `0d4dfca7` (Task 3 commit)

**4. [Rule 3 - Blocking] Extended `cmd/engram/operator_output_test.go`'s three gated tables (not in this plan's declared file list, same precedent as 03-02/03-03)**
- **Found during:** Task 2, immediately after wiring the leaf's `--timeout`/`--output` flags
- **Issue:** `TestTimeoutGroupMatrix`'s live-tree set-equality gate and `TestOperatorOutputParity`'s row-set gate (both keyed off `operatorCommands()`) would have failed without a corresponding row for the new command.
- **Fix:** Added `spine-review verify` to the `zero-disables` timeout group, `timeoutGroupCaseArgs`, `operatorInvalidOutputArgs`, and a full `operatorParityRows()` entry.
- **Files modified:** `cmd/engram/operator_output_test.go`
- **Committed in:** `2e2ad1cb` (Task 2 commit)

---

**Total deviations:** 4 auto-fixed, all Rule 3 (blocking pre-existing anti-drift gates this plan's own additions tripped)
**Impact on plan:** No scope creep -- every fix is a companion entry in a gate that already existed specifically to catch exactly this class of omission, in a file the plan did not (and could not, since the gates depend on symbols this plan introduces) declare in advance.

## Required Evidence (per this plan's critical execution constraints)

### 1. Repo-identity derivation actually implemented

`normalizeGitRemote` strips the scheme (`://`), strips user info (up to and including `@`), replaces the FIRST remaining colon with a slash (the SCP-style-remote step: `git@github.com:o/r.git` -> `github.com/o/r.git` after the first two steps -> `github.com/o/r.git` with the colon replaced -> trims `.git` -> `github.com/o/r`), then trims a trailing `.git` and trailing slash. `TestNormalizeGitRemote` proves `https://github.com/o/r.git`, `git@github.com:o/r.git`, `ssh://git@github.com/o/r`, and the bare already-normalised `github.com/o/r` all converge on `github.com/o/r`. `repoIdentityFromCWD` falls back to the working directory's base name when there is no origin remote or git is unavailable, per RESEARCH.md's Assumptions Log row A1 (fail SAFE: a mis-derivation produces `unverifiable`, never a confident wrong verdict).

The scope-comparison rule (`sameRepoAsCWD`) is exact-segment: split `scope` on `:`, find the FIRST segment exactly equal to `repo`, compare the NEXT segment to the derived identity by exact string equality, ignore everything after that (the `:ws:<workspace>` overlay suffix). `TestSameRepoAsCWD` asserts NOT-different-repo for `repo:github.com/seanb4t/engram`, `discovery:repo:github.com/seanb4t/engram`, `rule:repo:github.com/seanb4t/engram`, and the overlay form `repo:github.com/seanb4t/engram:ws:worktree-engram-mbnw`; and IS-different-repo for `myrepo:github.com/seanb4t/engram` (the substring false-positive this rule exists to prevent), `repo:github.com/other/thing`, and `project:engram` (no repo segment at all).

### 2. MUTATION CHECK -- `TestVerifyRejectsSymlinkEscape` (not RED-first)

Task 2 builds the RESOLVED (`filepath.EvalSymlinks`-based) containment gate directly, so the lexical-only failure state this test guards against never arises naturally in task order. Per the plan's own instruction, this is a MUTATION CHECK, not RED-first: `resolveContainedRef` was temporarily swapped for a lexical-only check (reject absolute paths and any `..` path segment; otherwise accept). Observed failure:

```
=== RUN   TestVerifyRejectsSymlinkEscape
    spine_review_verify_test.go:304: UnverifiableCount = 0, want 1 (symlink escape must be rejected)
--- FAIL: TestVerifyRejectsSymlinkEscape (0.00s)
```

The lexical check accepted the relative ref `link/passwd` (no `..` segments, not absolute) even though `link` is a symlink pointing outside the working tree -- exactly the gap the RESOLVED gate exists to close. `resolveContainedRef` was restored immediately after observing the failure; `diff` against the pre-mutation backup confirmed byte-identical restoration, and `TestVerifyRejectsSymlinkEscape` passed again afterward.

### 3. MUTATION CHECK -- `TestCatalogExitCodesMatchMapper`, both directions

Per the plan's explicit instruction, both directions of the edited derivation were observed failing before being trusted:

**Direction (a)** -- `exitFindings` advertised in the catalog, but `nonConnectProducedCodes` temporarily emptied:
```
=== RUN   TestCatalogExitCodesMatchMapper
    catalog_test.go:386: catalog exit codes = {0,1,2,3,4,5,6,7}, mapper-producible exit codes (incl. nonConnectProducedCodes) = {0,1,2,3,4,5,6}
--- FAIL: TestCatalogExitCodesMatchMapper (0.00s)
```

**Direction (b)** -- a bogus allowlist entry (`99`, a code no command produces) added alongside the real `exitFindings` entry:
```
=== RUN   TestCatalogExitCodesMatchMapper
    catalog_test.go:389: catalog exit codes = {0,1,2,3,4,5,6,7}, mapper-producible exit codes (incl. nonConnectProducedCodes) = {0,1,2,3,4,5,6,7,99}
--- FAIL: TestCatalogExitCodesMatchMapper (0.00s)
```

Both mutations were reverted immediately after observing their respective failures; `diff` against the pre-mutation backup confirmed byte-identical restoration of `catalog_test.go`, and the real test passed cleanly afterward.

### 4. Three `-shuffle=<seed>` runs

`go test ./cmd/engram/... -count=1 -shuffle={1,7,13}` all green, run after the full Task 3 implementation.

### 5. `go clean -testcache && task`

Ran after all three tasks landed: `task` (lint + `go test ./...`, including `internal/e2e` and `internal/store`, both testcontainers-backed) -- all green.

### 6. Key-links verification

`gsd-tools verify key-links .planning/phases/03-spine-curation-structural-cli/03-04-PLAN.md` -> `{"all_verified": true, "verified": 2, "pending": 0, "total": 2}`.

### 7. Grep gates

```
rg -v '^\s*//' cmd/engram/spine_review_verify.go | rg -o 'filepath\.Walk|filepath\.WalkDir|os\.ReadDir' | wc -l   -> 0
rg -n 'func verifyFileCitation' cmd/engram/spine_review_verify.go                                                  -> only (c store.Citation, fileContent string, fileExists bool)
rg -n 'func \(s \*Store\) EnumerateCitations' internal/store/spine.go                                              -> no Subject parameter
rg -v '^\s*//' internal/store/spine.go | rg -o 'ownerOrSharedCondition|ownerOnlyCondition' | wc -l                 -> 0
rg -v '^\s*//' internal/store/spine.go | rg -o 'client\.Scroll\(' | wc -l                                          -> 0
rg -v '^\s*//' internal/store/spine.go | rg -o 'ScrollAndOffset\(' | wc -l                                         -> 1
rg -c '^## engram spine-review verify$' cmd/engram/testdata/help.golden                                            -> 1
rg -o '"spine-review verify"' cmd/engram/testdata/catalog.golden | wc -l                                           -> 1
rg -c 'engram:rule:start verify-fail-on-accepted-values' docs-site/.../cli.md                                      -> 1
rg -o '"code": 7' cmd/engram/testdata/catalog.golden | wc -l                                                       -> 1
rg -o 'exitFindings' cmd/engram/client_common.go cmd/engram/catalog.go cmd/engram/catalog_test.go
   cmd/engram/spine_review_verify.go | wc -l                                                                       -> 11 (>= 4 required)
```

### 8. `go run ./internal/surfacesgen` idempotency

Ran twice; `md5` of the three touched docs-site files was identical across both runs -- no diff, confirming idempotence.

## Known Stubs

None. `verifyFileCitation`/`runVerify`/`EnumerateCitations` are wired to real `internal/store` calls and a real filesystem read (via the injectable `citationFileReader`, which defaults to `os.ReadFile`); no placeholder or hardcoded-empty rendering exists anywhere in this plan's diff.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-04, T-03-14, T-03-07, T-03-15, T-03-26) -- all five are mitigated as designed:

- T-03-04 (citation `Ref` -> local filesystem): the RESOLVED path-safety gate, proven by the symlink-escape mutation check above.
- T-03-14 (report rendering leaking stored content): `verifyReportDoc`/`verifyEntryDoc` are hand-declared structs with no `Excerpt` field at all; `TestVerifyReportNeverIncludesExcerpt` asserts a sentinel excerpt string never appears in either output mode.
- T-03-07 (coverage-claim spoofing): `EnumerateCitations` is Subject-less by signature; `TestEnumerateCitationsIncludesSuperseded` proves a recall-hidden record is still enumerated.
- T-03-15 (repudiation via exit-code confusion): `exitFindings` is distinct from every other constant (`TestVerifyFailOnExitFindingsIsDistinct`), and `TestCatalogExitCodesMatchMapper`'s derivation is extended rather than the catalog silently advertising an unaccounted-for code.
- T-03-26 (page-traversal spoofing): `internal/store/spine.go` carries exactly one `ScrollAndOffset` call site (grep-verified); `TestEnumerateCitationsPaginatesEveryPage` forces batch size 1 over 5 records and asserts a count equality.

## Issues Encountered

**A mid-run 1Password `op-ssh-sign` commit-signing outage.** After Task 1's commit succeeded, every subsequent `git commit` attempt for Task 2 failed with `error: 1Password: failed to fill whole buffer / fatal: failed to write commit object`. Diagnosis: `op whoami` reported "account is not signed in", and the 1Password desktop process had relaunched with `--just-updated --should-restart` (an auto-update), which plausibly cleared its in-memory signing-approval grant -- a host-environment authentication gate with no interactive surface available to satisfy it from this session. Per this executor's `<authentication_gates>` protocol, the run was halted and a `human-action` checkpoint was returned rather than bypassing signing (`--no-gpg-sign`/`-c commit.gpgsign=false`, both explicitly forbidden absent explicit user request) or leaving Task 2's fully-implemented, fully-tested, staged changes at risk of being lost to worktree cleanup. The orchestrator resolved the outage and committed Task 2 (`2e2ad1cb`) using exactly the `git commit --only ...` command this executor had supplied in the checkpoint; the implementation, its test coverage, and the mutation-check evidence recorded above are this executor's own work -- only the commit invocation was the orchestrator's.

Beyond that pause, the four Rule 3 auto-fixes recorded in § Deviations from Plan (two of which -- the `wantOperatorCommandKeys`/`wantExitCodes` gaps -- were caught only once a full, unfiltered `go test ./cmd/engram/...` run was performed rather than the narrower per-task test filters used during initial task-by-task verification) are the only other issues encountered. No architectural (Rule 4) questions arose.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- `internal/store/spine.go` now holds two Subject-less, `scrollAllPoints`-based sweeps (`ScanSpine`, `EnumerateCitations`); a later plan extending the spine-review tree with a third sweep (`consolidate`, `purge`) should route through the same iterator rather than introducing a fourth pattern.
- `internal/surfaces/rules.go` now holds two registered conditional rules anchored purely on `cli.md` with no proto/skill counterpart (`RuleDestructiveRequiresApply`, `RuleVerifyFailOnValues`) -- the pattern for "a CLI-only flag with no MCP/proto equivalent" is now established twice.
- `exitFindings = 7` is the first non-connect-produced code in the taxonomy; the `nonConnectProducedCodes` allowlist in `catalog_test.go` is the extension point for any future code with the same property.
- Requirement `REQ-citation-drift-verify` is satisfied; this executor did not run `requirements mark-complete` or touch `REQUIREMENTS.md`, `STATE.md`, or `ROADMAP.md` -- per the orchestrator's explicit instruction, those cross-plan files are the orchestrator's to update.
- No blockers for the next plan in this phase.

## Self-Check: PASSED

All key files (`cmd/engram/spine_review_verify.go`, `spine_review_verify_test.go`, `internal/store/spine.go`, `internal/surfaces/rules.go`, `cmd/engram/client_common.go`, `cmd/engram/catalog.go`, `docs-site/.../cli.md`, `docs-site/.../errors.md`, `docs-site/.../upgrade.md`) confirmed present on disk. All three task commit hashes (`e0ab6adf`, `2e2ad1cb`, `0d4dfca7`) confirmed present in `git log --oneline --all`.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-06*
