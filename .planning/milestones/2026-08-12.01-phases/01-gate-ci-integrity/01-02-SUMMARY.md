---
phase: 01-gate-ci-integrity
plan: 02
subsystem: testing
tags: [go, regexp, re2, conformance-gate, gsd-key-links, planning-artifacts]

# Dependency graph
requires: ["internal/keylinks (01-01)"]
provides:
  - "Every `pattern:` key-link field under `.planning/**` is in D-02's escape-free character-class form — zero backslashes remain."
  - "internal/keylinks/gate_test.go: TestNoEscapedPatternsRepoWide (repo-wide, archived milestones included) and TestActiveMilestoneKeyLinksSatisfiable (active milestone only) as recurring `go test ./...` gates, plus TestGateScopesAreDistinct pinning D-04's scope asymmetry."
affects: []

# Actuals (#2632)
actuals:
  tokens: 6732
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "run*(t) []string helper + Test* ranging with t.Error per line (internal/surfaces/conformance_test.go's shape), reused for both recurring key-link gates — never t.Fatal on first offender (D-07)"
    - "repoRoot resolved from the test file's own location (two levels up from the package dir) with a loud os.Stat failure if .planning is absent, so a mis-resolved root cannot silently scan an empty tree and report a false green"
    - "escape-free character-class form (`[.]`, `[(]`, `[)]`) for every key-link pattern; an embedded literal double-quote is represented by switching the YAML scalar's outer delimiter from double to single quotes rather than backslash-escaping the quote"

key-files:
  created:
    - internal/keylinks/gate_test.go
  modified:
    - .planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-01-PLAN.md
    - .planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-02-PLAN.md
    - .planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-03-PLAN.md
    - .planning/milestones/v0.12.x-phases/02-headless-cli-client/02-01-PLAN.md
    - .planning/milestones/v0.12.x-phases/02-headless-cli-client/02-02-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-01-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-02-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-03-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-04-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-05-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-06-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-07-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-08-PLAN.md
    - .planning/milestones/v0.13.x-phases/01-interface-enforceability/01-09-PLAN.md
    - .planning/milestones/v0.13.x-phases/02-interface-discoverability/02-01-PLAN.md
    - .planning/milestones/v0.13.x-phases/02-interface-discoverability/02-02-PLAN.md
    - .planning/milestones/v0.13.x-phases/02-interface-discoverability/02-03-PLAN.md
    - .planning/milestones/v0.13.x-phases/02-interface-discoverability/02-04-PLAN.md
    - .planning/milestones/v0.13.x-phases/02-interface-discoverability/02-05-PLAN.md
    - .planning/milestones/v0.13.x-phases/05-validation-debt-reconciliation/05-02-PLAN.md

key-decisions:
  - "The pre-rewrite scan found 39 offending lines across 20 files, not the 38 CONTEXT.md recorded from a coarse grep at planning time. The extra offender is `.planning/milestones/v0.13.x-phases/01-interface-enforceability/01-04-PLAN.md:49`'s `MarkFlagsMutuallyExclusive\\(\"scope\", \"cross-spine\"\\)` entry, which carries two escaped double-quotes IN ADDITION TO the two escaped parens a coarse grep for escaped metacharacters would have counted once. The discrepancy is recorded here per Task 1's instruction, not silently reconciled — the guard's own count (39) is authoritative since it comes from the same matcher the gate runs."
  - "Two entries needed the escaped-double-quote fix (drop the backslash, switch the YAML scalar's outer quote from double to single) rather than the mechanical char-class rewrite: `01-interface-enforceability/01-03-PLAN.md:51` (`koanf:\"client\"`, the case the plan's read_first section named in advance) and `01-interface-enforceability/01-04-PLAN.md:49` (`MarkFlagsMutuallyExclusive\\(\"scope\", \"cross-spine\"\\)`, found during the sweep — not named in advance, but the identical shape). Both verified via a re-run of ScanPlans after the edit, not merely by inspection."
  - "The Task 1 <verify>'s literal `rg -q 'pattern:.*[\\\\]' .planning --glob '*-PLAN.md'` self-matches this plan's OWN prose (three lines in 01-02-PLAN.md itself quote that exact rg invocation as acceptance-criteria text, two of them already carrying a `<!-- planner-discipline-allow -->` marker acknowledging the risk). This is a limitation of the literal shell command, not a real offender — internal/keylinks.ScanPlans (which parses only the must_haves.key_links frontmatter block, never body prose) is the authoritative check per Task 1's read_first instruction (\"the enumeration in this task is a call into this package, not a fresh grep\"), and it returns zero offenders. Excluding this plan's own file from the rg invocation (`--glob '!01-02-PLAN.md'`) also returns zero matches, confirming the true property holds; 01-02-PLAN.md was not itself part of the 20-file rewrite set."

requirements-completed: [REQ-keylink-pattern-matchable]

coverage:
  - id: D1
    description: "Every `pattern:` key-link field anywhere under `.planning/**` is escape-free character-class form; ScanPlans(repoRoot, [\".planning\"], ModeEscapingOnly) returns zero offenders"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: unit
        ref: "internal/keylinks/gate_test.go#TestNoEscapedPatternsRepoWide"
        status: pass
    human_judgment: false
  - id: D2
    description: "A reintroduced escaped pattern in an archived plan fails go test naming file and line; reverts cleanly"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: manual-red-proof
        ref: "SUMMARY Fail-First Observations, escaping gate"
        status: pass
    human_judgment: false
  - id: D3
    description: "A pattern in an active-milestone EXECUTED plan that matches neither its from nor its to file fails go test; the same shape in an archived plan does not (D-04's asymmetric scope)"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: manual-red-proof
        ref: "SUMMARY Fail-First Observations, satisfiability gate"
        status: pass
      - kind: unit
        ref: "internal/keylinks/gate_test.go#TestGateScopesAreDistinct"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every rewritten pattern preserves the symbol, file, and intent it pinned before the rewrite"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: manual-diff-audit
        ref: "git diff 6e74211c^ 6e74211c -- .planning/milestones — every changed line is confined to the pattern: scalar"
        status: pass
    human_judgment: false
  - id: D5
    description: "The escaping gate has no exclusion list, allowlist, or skip file — its scope is every -PLAN.md under .planning/"
    requirement: "REQ-keylink-pattern-matchable"
    verification:
      - kind: source-read
        ref: "internal/keylinks/gate_test.go — the only filtering is ScanPlans's -PLAN.md suffix check and the mode's root; no per-file conditional exists"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-13
status: complete
---

# Phase 01 Plan 02: Normalize Key-Link Patterns and Land the Recurring Gates Summary

**39 escaped key-link `pattern:` fields across 20 plan files rewritten to D-02's escape-free character-class form in one commit, then `internal/keylinks/gate_test.go` lands the two recurring `go test ./...` gates with D-04's asymmetric scopes — both proven red against a deliberately reintroduced defect before shipping.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-08-13
- **Tasks:** 2
- **Files modified:** 21 (1 created, 20 modified)

## Accomplishments
- Enumerated the authoritative offender set via a throwaway `go run` scratch program calling `keylinks.ScanPlans(repoRoot, []string{".planning"}, keylinks.ModeEscapingOnly)` directly — not a fresh grep — before touching anything.
- Rewrote all 39 offenders across 20 plan files (11 in v0.12.x-phases, 26 in v0.13.x-phases/01-interface-enforceability and 02-interface-discoverability, 2 in v0.13.x-phases/05-validation-debt-reconciliation) to the escape-free character-class form, in one commit with an explicit pathspec.
- Handled the two escaped-double-quote entries by dropping the backslash and switching the YAML scalar's outer delimiter from double to single quotes, so the embedded literal quote characters need no escaping in either YAML or the resulting regex, and the guard's naive quote-stripping parser still resolves the correct raw value.
- Landed `internal/keylinks/gate_test.go`: `TestNoEscapedPatternsRepoWide` (repo-wide, `ModeEscapingOnly`, archived milestones included — time-invariant), `TestActiveMilestoneKeyLinksSatisfiable` (`.planning/phases` only, `ModeSatisfiability` — HEAD-dependent, active milestone only per D-04), and `TestGateScopesAreDistinct` pinning the scope split as a test.
- Proved both gates red against a deliberately reintroduced defect, then reverted cleanly (zero diff remaining) before the final commit.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "enumerate, rewrite, re-scan clean" (tracer)** - `6e74211c` (fix)
2. **Task 2: Land the two recurring gates with D-04's asymmetric scopes** - `d318ed47` (feat)

**Plan metadata:** committed separately by the orchestrator after wave merge (worktree mode — this executor does not write STATE.md/ROADMAP.md).

## Files Created/Modified
- `internal/keylinks/gate_test.go` - `TestNoEscapedPatternsRepoWide`, `TestActiveMilestoneKeyLinksSatisfiable`, `TestGateScopesAreDistinct`, `gateRepoRoot`, `runEscapingGate`, `runSatisfiabilityGate`
- 20 `.planning/milestones/**/*-PLAN.md` files — each had 1-3 `pattern:` fields rewritten from escaped form to D-02's character-class form (see key-files above for the full list)

## Decisions Made
- The pre-rewrite scan found 39 offenders, not CONTEXT.md's recorded 38 — see frontmatter `key-decisions` for the reconciliation (the extra offender carries two escaped shapes a coarse grep would count once).
- Two entries (the koanf-tag and the `MarkFlagsMutuallyExclusive` flag-name patterns) needed the "drop the escape, switch YAML quoting style" fix rather than the mechanical char-class substitution — see frontmatter `key-decisions`.
- The Task 1 `<verify>` block's literal `rg` invocation self-matches this very plan file's own prose; treated as a known limitation of the literal shell command rather than a real offender, since `ScanPlans` (the authoritative matcher) returns zero — see frontmatter `key-decisions`.

## Deviations from Plan

None beyond the two documented decisions above (both fall under the plan's own "judgment required" callouts in Task 1's `<action>`, not unplanned deviations). No Rule 1-4 auto-fixes were needed.

## Pre-Rewrite Offender Report (verbatim)

Produced by a throwaway `go run ./cmd/tmpscanreport` scratch program (removed before commit) calling `keylinks.ScanPlans(repoRoot, []string{".planning"}, keylinks.ModeEscapingOnly)` directly:

```
total offenders: 39
file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-01-PLAN.md:73 shape=escaping pattern="withConnectLane\\(ctx" fix="withConnectLane[(]ctx"
file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-01-PLAN.md:77 shape=escaping pattern="laneFromConnectContext\\(ctx\\)" fix="laneFromConnectContext[(]ctx[)]"
file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-01-PLAN.md:81 shape=escaping pattern="auth\\.ExtractBearerCredential" fix="auth[.]ExtractBearerCredential"
file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-02-PLAN.md:42 shape=escaping pattern="laneFromConnectContext\\(ctx\\)" fix="laneFromConnectContext[(]ctx[)]"
file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-03-PLAN.md:52 shape=escaping pattern="server\\.NewConnectResolver\\(" fix="server[.]NewConnectResolver[(]"
file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-03-PLAN.md:56 shape=escaping pattern="cfg\\.Connect\\.Headless" fix="cfg[.]Connect[.]Headless"
file=.../v0.12.x-phases/02-headless-cli-client/02-01-PLAN.md:70 shape=escaping pattern="clientFromFlags\\(cmd\\)" fix="clientFromFlags[(]cmd[)]"
file=.../v0.12.x-phases/02-headless-cli-client/02-01-PLAN.md:74 shape=escaping pattern="errors\\.As\\(err, &ec\\)" fix="errors[.]As[(]err, &ec[)]"
file=.../v0.12.x-phases/02-headless-cli-client/02-01-PLAN.md:78 shape=escaping pattern="engramv1connect\\.NewEngramServiceClient" fix="engramv1connect[.]NewEngramServiceClient"
file=.../v0.12.x-phases/02-headless-cli-client/02-02-PLAN.md:53 shape=escaping pattern="clientFromFlags\\(cmd\\)" fix="clientFromFlags[(]cmd[)]"
file=.../v0.12.x-phases/02-headless-cli-client/02-02-PLAN.md:57 shape=escaping pattern="wrapRPCError\\(" fix="wrapRPCError[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-01-PLAN.md:41 shape=escaping pattern="exitCodeFromError\\(" fix="exitCodeFromError[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-01-PLAN.md:45 shape=escaping pattern="runClient\\(t" fix="runClient[(]t"
file=.../v0.13.x-phases/01-interface-enforceability/01-02-PLAN.md:49 shape=escaping pattern="ValidateFlagGroups\\(\\)" fix="ValidateFlagGroups[(][)]"
file=.../v0.13.x-phases/01-interface-enforceability/01-02-PLAN.md:53 shape=escaping pattern="usageErrorf\\(" fix="usageErrorf[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-03-PLAN.md:51 shape=escaping pattern="koanf:\"client\"" fix="koanf:\"client\""
file=.../v0.13.x-phases/01-interface-enforceability/01-03-PLAN.md:55 shape=escaping pattern="ValidateClient\\(" fix="ValidateClient[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-04-PLAN.md:49 shape=escaping pattern="MarkFlagsMutuallyExclusive\\(\"scope\", \"cross-spine\"\\)" fix="MarkFlagsMutuallyExclusive[(]\"scope\", \"cross-spine\"[)]"
file=.../v0.13.x-phases/01-interface-enforceability/01-04-PLAN.md:53 shape=escaping pattern="store\\.ValidateOwnerRemap\\(" fix="store[.]ValidateOwnerRemap[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-05-PLAN.md:47 shape=escaping pattern="store\\.Err(NotFound|InvalidArgument|AmbiguousShortID)" fix="store[.]Err(NotFound|InvalidArgument|AmbiguousShortID)"
file=.../v0.13.x-phases/01-interface-enforceability/01-05-PLAN.md:51 shape=escaping pattern="classifyOperatorErr\\(" fix="classifyOperatorErr[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-06-PLAN.md:40 shape=escaping pattern="usageErrorf\\(" fix="usageErrorf[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-06-PLAN.md:44 shape=escaping pattern="classifyOperatorErr\\(" fix="classifyOperatorErr[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-07-PLAN.md:44 shape=escaping pattern="config\\.Load\\(cmd\\.Flags\\(\\)\\)" fix="config[.]Load[(]cmd[.]Flags[(][)][)]"
file=.../v0.13.x-phases/01-interface-enforceability/01-07-PLAN.md:48 shape=escaping pattern="config\\.ValidateClient\\(" fix="config[.]ValidateClient[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-08-PLAN.md:43 shape=escaping pattern="context\\.WithTimeout\\(" fix="context[.]WithTimeout[(]"
file=.../v0.13.x-phases/01-interface-enforceability/01-09-PLAN.md:45 shape=escaping pattern="upgrade\\.md" fix="upgrade[.]md"
file=.../v0.13.x-phases/02-interface-discoverability/02-01-PLAN.md:67 shape=escaping pattern="surfaces\\.RuleByID" fix="surfaces[.]RuleByID"
file=.../v0.13.x-phases/02-interface-discoverability/02-01-PLAN.md:71 shape=escaping pattern="conditionalErrf\\(" fix="conditionalErrf[(]"
file=.../v0.13.x-phases/02-interface-discoverability/02-01-PLAN.md:75 shape=escaping pattern="surfaces\\.Rules\\(\\)" fix="surfaces[.]Rules[(][)]"
file=.../v0.13.x-phases/02-interface-discoverability/02-02-PLAN.md:54 shape=escaping pattern="mcp\\.NewInMemoryTransports" fix="mcp[.]NewInMemoryTransports"
file=.../v0.13.x-phases/02-interface-discoverability/02-02-PLAN.md:58 shape=escaping pattern="surfaces\\.Rules\\(\\)|Rules\\(\\)" fix="surfaces[.]Rules[(][)]|Rules[(][)]"
file=.../v0.13.x-phases/02-interface-discoverability/02-03-PLAN.md:60 shape=escaping pattern="conditionalErrf\\(" fix="conditionalErrf[(]"
file=.../v0.13.x-phases/02-interface-discoverability/02-03-PLAN.md:64 shape=escaping pattern="surfaces\\.RuleByID" fix="surfaces[.]RuleByID"
file=.../v0.13.x-phases/02-interface-discoverability/02-04-PLAN.md:48 shape=escaping pattern="surfaces\\.ClassForTool" fix="surfaces[.]ClassForTool"
file=.../v0.13.x-phases/02-interface-discoverability/02-04-PLAN.md:52 shape=escaping pattern="registeredTools\\(" fix="registeredTools[(]"
file=.../v0.13.x-phases/02-interface-discoverability/02-05-PLAN.md:56 shape=escaping pattern="surfaces\\.ClassForCommand" fix="surfaces[.]ClassForCommand"
file=.../v0.13.x-phases/05-validation-debt-reconciliation/05-02-PLAN.md:46 shape=escaping pattern="deps\\.searchMemory" fix="deps[.]searchMemory"
file=.../v0.13.x-phases/05-validation-debt-reconciliation/05-02-PLAN.md:50 shape=escaping pattern="store\\.EmbedText" fix="store[.]EmbedText"
```

Count relationship to CONTEXT.md: 39 observed vs. 38 recorded during discussion — see frontmatter `key-decisions` for the reconciliation. Both counts span the identical 20 files.

## Post-Rewrite Empty Result

```
$ go run ./cmd/tmpscanreport
total offenders: 0
```

(One intermediate run, before the second escaped-quote entry was found and fixed, still reported 1 offender at `01-interface-enforceability/01-04-PLAN.md:49` — the `MarkFlagsMutuallyExclusive` pattern's embedded quotes had not yet been addressed. Fixed by switching that entry's YAML outer quoting to single-quote form, matching the koanf-tag entry's treatment. Final re-run: zero.)

```
$ rg -c 'pattern:.*[\\]' .planning --glob '*-PLAN.md'
.planning/phases/01-gate-ci-integrity/01-02-PLAN.md:3
```

The one remaining rg match is this plan's own prose (the `<verify>`/acceptance-criteria text quoting the rg command itself), not a real `pattern:` key-link field — confirmed by excluding this plan's own file: `rg -c 'pattern:.*[\\]' .planning --glob '*-PLAN.md' --glob '!01-02-PLAN.md'` exits 1 (no matches).

## Fail-First Observations (verbatim, both directions)

**Task 2 — TestNoEscapedPatternsRepoWide, escaping gate:**

RED (reintroduced `pattern: "withConnectLane\\(ctx"` at `.planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-01-PLAN.md:73`, previously fixed by Task 1):
```
=== RUN   TestNoEscapedPatternsRepoWide
    gate_test.go:63: file=.../v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-01-PLAN.md:73 shape=escaping pattern="withConnectLane\\\\(ctx" fix="withConnectLane[(]ctx"
--- FAIL: TestNoEscapedPatternsRepoWide (0.01s)
FAIL
```

GREEN (reverted to `pattern: "withConnectLane[(]ctx"`):
```
=== RUN   TestNoEscapedPatternsRepoWide
--- PASS: TestNoEscapedPatternsRepoWide (0.01s)
```

**Task 2 — TestActiveMilestoneKeyLinksSatisfiable, satisfiability gate:**

RED (`.planning/phases/01-gate-ci-integrity/01-01-PLAN.md:53`'s `bad_key_links[.]md` pattern — an EXECUTED plan, sibling `01-01-SUMMARY.md` present — temporarily changed to `GATETEST_UNSATISFIABLE_SYMBOL_TEMP`, a symbol present in neither its `from` file, `internal/keylinks/keylinks_test.go`, nor its `to` file, `internal/keylinks/testdata/bad_key_links.md`):
```
=== RUN   TestActiveMilestoneKeyLinksSatisfiable
    gate_test.go:97: file=.../01-gate-ci-integrity/01-01-PLAN.md:53 shape=unsatisfiable pattern="GATETEST_UNSATISFIABLE_SYMBOL_TEMP" fix="pattern does not match .../internal/keylinks/keylinks_test.go or .../internal/keylinks/testdata/bad_key_links.md — update the pattern or the from/to paths"
--- FAIL: TestActiveMilestoneKeyLinksSatisfiable (0.00s)
FAIL
```

GREEN (reverted to `pattern: "bad_key_links[.]md"`):
```
=== RUN   TestActiveMilestoneKeyLinksSatisfiable
--- PASS: TestActiveMilestoneKeyLinksSatisfiable (0.00s)
```

Both reverts confirmed by `git diff --stat` on the two touched files: empty (no diff) before the final commit.

## Visited-File Count

`TestNoEscapedPatternsRepoWide`'s scan reaches archived milestones: it walked every `-PLAN.md` under `.planning` (the full tree, including `.planning/archive/`, `.planning/milestones/**`, and `.planning/phases/**`). The 39 offenders it found spanned exactly the 20 files rewritten in Task 1 (all under `.planning/milestones/`), which is at or beyond the 20-file floor Task 2's acceptance criteria named — the scan additionally visited every active-milestone plan under `.planning/phases/01-gate-ci-integrity/` (6 files) and found zero offenders there, since those were authored with escape-free patterns from the start.

## Issues Encountered
None beyond the two documented decisions above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/keylinks/gate_test.go`'s two Test functions now ride `go test ./...`, the gate that blocks CI merges — a future reintroduction of an escaped pattern anywhere under `.planning/**`, or an unsatisfiable pattern in an executed active-milestone plan, fails the build automatically.
- No exclusion list, allowlist, or skip file exists anywhere in `gate_test.go` — a source read confirms the only filtering is `ScanPlans`'s `-PLAN.md` suffix check and each gate's mode-selected root.
- Plan 01-03 (the one-time v0.13.x Phase 1-2 reassessment sweep) can now call the identical `ScanPlans`/`ModeSatisfiability` path this plan's recurring gate uses, with confidence that every archived pattern it resolves against HEAD is already in escape-free form — no further normalization needed before that sweep runs.
- No blockers.

---
*Phase: 01-gate-ci-integrity*
*Completed: 2026-08-13*
