---
phase: 04-skills-distribution
verified: 2026-09-12T20:25:07Z
status: passed
score: 53/53 must-haves verified (46 carried-forward truths + 7 gap-closure truths for B01/#559)
covered_files:
  - .planning/2026-08-23.01-INTEGRATION.md
  - .planning/2026-08-23.01-MILESTONE-AUDIT.md
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
  - .planning/phases/04-skills-distribution/04-01-PLAN.md
  - .planning/phases/04-skills-distribution/04-01-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-02-PLAN.md
  - .planning/phases/04-skills-distribution/04-02-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-03-PLAN.md
  - .planning/phases/04-skills-distribution/04-03-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-04-PLAN.md
  - .planning/phases/04-skills-distribution/04-04-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-05-PLAN.md
  - .planning/phases/04-skills-distribution/04-05-SUMMARY.md
  - .planning/phases/04-skills-distribution/04-CONTEXT.md
  - .planning/phases/04-skills-distribution/04-REVIEW.md
  - .planning/phases/04-skills-distribution/04-VALIDATION.md
  - cmd/engram/setup.go
  - cmd/engram/setup_delegation_test.go
  - cmd/engram/setup_test.go
  - cmd/engram/testdata/help.golden
  - go.mod
  - internal/setup/aggregate.go
  - internal/setup/apply.go
  - internal/setup/claudecode.go
  - internal/setup/claudecode_test.go
  - internal/setup/codex.go
  - internal/setup/codex_test.go
  - internal/setup/generic.go
  - internal/setup/opencode.go
  - internal/setup/plan.go
  - internal/setup/plan_test.go
  - internal/setup/runtime.go
  - internal/skills/agentsmd.go
  - internal/skills/drift_test.go
  - internal/skills/embed.go
  - internal/skills/environment.go
  - internal/skills/frontmatter.go
  - internal/skills/importgate_test.go
  - internal/skills/install.go
  - internal/skills/install_test.go
  - internal/skills/inventory.go
covered_digest: "v1:sha256:7d35d101cd76789664350332e14cf73271015d0cf4b34b76aeb1c1fcd1255116"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: "46/46 truths carried forward; gap B01 (REQ-skills-agents-md-fallback) blocking"
  gaps_closed:
    - "B01: an existing AGENTS.md index that fails to read for any reason other than confirmed nonexistence (permission denied, transient I/O error, arbitrary error) is now preserved byte-for-byte with zero writes; only errors.Is(readErr, fs.ErrNotExist) triggers the create case."
  gaps_remaining: []
  regressions: []
human_verification_evidence:
  - test: "Start Claude Code and opencode with the five curation skills installed via `engram setup --apply --runtime claude-code` and `--runtime opencode` (against a scratch/fake $HOME, never the operator's real one), then check each runtime's own skill list and startup/log output."
    expected: "All five skills appear in the runtime's skill list, and no warning is emitted about the unrecognized `metadata.engram-summary` frontmatter key."
    why_human: "Repo rule m45p2b4bp7 forbids an automated test asserting third-party runtime behavior."
    status: satisfied
    observed: "2026-09-10 (Sean), carried forward unchanged from the prior verification: all three runtimes tolerate the key; five skills listed on codex and opencode; Claude Code showed the five loose skills alongside the plugin's `engram:*` entries. Non-rejection (the property REQ-skills-native-format depends on) is established on all three runtimes; the narrower 'emits no warning' half of the expectation remains unobserved and is not claimed. This re-verification performed no new live runtime observation — B01's fix and regression tests do not touch this surface."
---

# Phase 4: Skills Distribution Verification Report

**Phase Goal:** The five curation skills reach every configured runtime — installed in that
runtime's native skill or rules format where one exists, and appended into a delimited,
re-detectable AGENTS.md block where none does — from a brew-installed binary that carries the
skill content itself, so a Claude-plugin-free install still teaches an agent how to curate. The
embedded content is sourced from the same files the plugin ships, so the two cannot drift apart.

**Verified:** 2026-09-12T20:25:07Z
**Status:** passed
**Re-verification:** Yes — after gap-closure plan 04-05 (`gap_closure: true`) fixed audit blocker
B01 / GitHub issue #559.

## Re-verification Summary

The prior `04-VERIFICATION.md` (2026-09-12T15:41:47Z) recorded `status: gaps_found` after the
milestone audit (`2026-08-23.01-MILESTONE-AUDIT.md`) reproduced B01: `internal/skills.Install`'s
`FormatAgentsMD` branch treated **every** AGENTS.md index read error — not just confirmed
nonexistence — as "no index exists," spliced a fresh document containing only the managed block,
and overwrote the operator's file with `Report.Err == nil` (a false success). This violated
REQ-skills-agents-md-fallback's "content outside the block is left byte-for-byte untouched"
contract on the read-error path.

Plan `04-05` (gap closure) fixed this in commits `95daee01` (fix), `bae018f2` (doc-comment
wording follow-up), and `c773d80e` (CLI-boundary regression test). This re-verification
independently re-ran every relevant test (not merely trusted SUMMARY.md's claims) and read the
actual diff.

**Verdict: B01 is genuinely closed in code and tests. Status: passed.**

### Independent code inspection

`internal/skills/install.go`'s `installAgentsMDIndex` (read directly, lines 160-171):

```go
existing, readErr := env.ReadFile(target.IndexFile)
if readErr != nil {
	if !errors.Is(readErr, fs.ErrNotExist) {
		errs = append(errs, fmt.Errorf("skills: read index %s: %w", target.IndexFile, readErr))
		return wrote, alreadyCorrect, errs
	}
	existing = nil
}
```

This is exactly the fix the plan specified: only `errors.Is(readErr, fs.ErrNotExist)` (bare or
wrapped) is the create case; any other read error returns *before* `RenderBlock`, `Splice`,
`MkdirAll`, or `WriteFile` are ever reached — zero writes, error wrapped and named.

`git diff efcfb0ad6..c773d80e -- internal/skills/install.go` (the pre-B01 baseline through the
gap-closure commits) shows the change is scoped to exactly this predicate plus the doc comment;
`installFiles` (the engram-owned skill-file writer, D-08's unconditional-overwrite posture) has
zero body changes in the same diff — confirmed by inspecting the diff hunks directly, not by
trusting the SUMMARY's claim.

### Independent test execution (not SUMMARY-trusted)

| Command | Result |
|---|---|
| `go test ./internal/skills/... -count=1 -run '^TestInstallPreservesIndexOnReadError$' -v` | `--- PASS` all 4 subtests (permission_denied, transient_error, bare_nonexistence, wrapped_nonexistence); `ok` |
| `go test ./cmd/engram/... -count=1 -run '^TestSetupIndexReadFailureReachesPartialExit$' -v` | `--- PASS`; `ok` |
| `go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` | all `ok`, no regressions |
| `go test ./internal/skills/... -count=1 -v` (full package) | all 16 tests `--- PASS`, including `TestInstallAgentsMdConverges`, `TestAgentsMdPreservesSymlink`, `TestInstallNativeConverges`, `TestInstallOverwritesDifferingDestination`, `TestInstallAccumulatesFailuresAndAttemptsEverySkill`, `TestAgentsMdSplice`, `TestSkillsEmbedMatchesVendored` |
| `go test ./cmd/engram/... -count=1 -run 'TestSetup' -v` | `TestSetupSkillsFailureReachesPartialExit` and `TestSetupReportCoversEveryRuntimeShape` both `--- PASS` (pre-existing analogs unregressed) |
| `rg -n 'os\.UserHomeDir\|exec\.Command\(\|os\.Getenv\("HOME"\)' cmd/engram/setup_test.go internal/skills/install_test.go` | prints nothing (exit 1) — no test reaches the real `$HOME` or a real subprocess |
| `gofmt -l internal/skills/ cmd/engram/` | empty (clean) |
| `go vet ./internal/skills/... ./cmd/engram/...` | one pre-existing, unrelated finding in `cmd/engram/operator_view_test.go` (duplicate JSON struct tag in a test fixture, untouched by this plan's `files_modified`) — not a regression from this gap closure |
| `task license:check` | `Totally checked 1811 files, valid: 401, invalid: 0` |
| `head -1` on the three touched Go files | `// SPDX-License-Identifier: Apache-2.0` present in all three |
| `git status --porcelain internal/skills/data` | empty — vendored skill tree untouched |

### Read the new CLI-boundary test in full (not summary-trusted)

`cmd/engram/setup_test.go:1486-1560+` (`TestSetupIndexReadFailureReachesPartialExit`) was read in
full. It builds a fake `skillsEnv` that returns `os.ErrPermission` for exactly
`/home/fake/.codex/AGENTS.md` (seeded with operator guidance) and `os.ErrNotExist` for unknown
paths, never fails a write, and asserts: `exitCodeFromError(err) == exitPartial`; the codex row's
`Outcome`/`Skills` are both `failed`, `Registration` is not `failed`, `SkillsIndex` equals the
index path, and `Reason` contains the index path; the claude-code row is not `failed`; and (in the
remainder of the function, confirmed present via `rg`) the write log excludes the index path and
includes at least one write under `/home/fake/.agents/skills/`, with the seeded index bytes
unchanged. This matches must_haves truth #4 verbatim.

## Goal Achievement

### Success Criteria (ROADMAP contract)

| # | Success Criterion | Status | Evidence |
|---|---|---|---|
| 1 | A brew-installed engram binary with no Claude plugin present can still produce the full content of the five curation skills, sourced from the same files the plugin ships. | ✓ VERIFIED | Unchanged by this gap closure. `internal/skills/embed.go`'s `//go:embed all:data`; `TestSkillsEmbedMatchesVendored` passes; `diff -rq internal/skills/data skill/engram/skills` exits 0 (re-confirmed this session via full package test run). |
| 2 | For a runtime with a native skill or rules format, `engram setup --apply` installs the skills in that native format. | ✓ VERIFIED | Unchanged; `installFiles`'s `FormatNative` path is byte-identical pre/post gap-closure (confirmed via `git diff`); `TestInstallNativeConverges`, `TestInstallOverwritesDifferingDestination` pass. |
| 3 | For a runtime with no native skill format, `engram setup --apply` writes the guidance into AGENTS.md inside a delimited, re-detectable block; re-running replaces that block rather than appending a second copy, and content outside the block is left byte-for-byte untouched. | ✓ VERIFIED (B01 closed) | Previously verified for the create/replace/malformed/symlink cases (`TestAgentsMdSplice`, `TestInstallAgentsMdConverges`, `TestAgentsMdPreservesSymlink`) but FAILED on the unreadable-index case (B01). Now also verified for that case: `TestInstallPreservesIndexOnReadError` (permission-denied and transient-error subtests) independently re-run and PASS; the fix in `installAgentsMDIndex` is read directly in the code and matches. "Byte-for-byte untouched" now holds across every read-result class: nonexistent (create), well-formed (replace), malformed (hard-fail, zero bytes), symlinked (write-through), and now unreadable (preserve, zero writes, reported failure). |

### Requirements Coverage

| Requirement | Status | Evidence |
|---|---|---|
| REQ-skills-embedded-in-binary | ✓ SATISFIED | Unchanged. `internal/skills/embed.go` + `drift_test.go`. |
| REQ-skills-native-format | ✓ SATISFIED | `claudecode.go`, `opencode.go`, `codex.go` each author an explicit `SkillTarget`; `installFiles` unchanged; plus 04-05's truth that the native skill-file pass is independent of, and unaffected by, an index read failure — proven at both the package boundary (`len(Wrote) == 3`) and the CLI boundary (write under `/home/fake/.agents/skills/`). |
| REQ-skills-agents-md-fallback | ✓ SATISFIED (B01 closed) | `agentsmd.go` scan/splice (unchanged) + `install.go`'s corrected read-error classification (`errors.Is(readErr, fs.ErrNotExist)`). Independently re-run `TestInstallPreservesIndexOnReadError` and `TestSetupIndexReadFailureReachesPartialExit` both green. |

**Note on planning-artifact staleness (non-blocking):** `.planning/REQUIREMENTS.md` still shows
`REQ-skills-agents-md-fallback` unchecked (`- [ ]`) with the 2026-09-12 audit-gap note citing
#559, and `.planning/ROADMAP.md` still marks "Phase 4: Skills Distribution" unchecked with the
same note. Both predate plan 04-05's fix and have not been updated since. This is a documentation-
sync gap, not a code gap — B01 is closed in the actual codebase and tests, verified independently
above. Per the planning-artifacts convention (fill in values in shapes the tool already writes,
never invent structure), the next step that touches these files should flip the checkbox and
remove/resolve the audit-gap annotation now that #559 is closed; this verifier does not do so
itself since editing ROADMAP.md/REQUIREMENTS.md checkbox state is outside the verifier's role and
belongs to the workflow step that owns milestone bookkeeping (e.g. `/gsd-audit-milestone` or a
close-out step). Flagged here so it is not silently dropped.

No orphaned requirements: `.planning/REQUIREMENTS.md`'s "Skills Distribution" section maps exactly
the three requirements declared across the five plans' frontmatter (04-01 through 04-05).

### Gap-Closure Truths (Plan 04-05, B01 / #559) — newly verified this pass

| # | Truth (from 04-05-PLAN.md must_haves.truths) | Status | Evidence |
|---|---|---|---|
| 1 | Non-nonexistence `ReadFile` error on the index → zero `WriteFile` calls for the index path, index absent from `Wrote`/`AlreadyCorrect`, existing bytes preserved, `Report.Err` non-nil satisfying `errors.Is` and naming the path. | ✓ VERIFIED | Code read directly (lines 165-169 above); `TestInstallPreservesIndexOnReadError/permission_denied` and `/transient_error` independently re-run, PASS. |
| 2 | `fs.ErrNotExist` (bare or wrapped) → unchanged create path: `Splice(nil, block)`, index in `Wrote`, `Report.Err` nil. | ✓ VERIFIED | `TestInstallPreservesIndexOnReadError/bare_nonexistence` and `/wrapped_nonexistence` independently re-run, PASS. |
| 3 | Index read failure never aborts/undoes the independent native skill-file pass (D-07). | ✓ VERIFIED | Test asserts `len(report.Wrote) == 3` (all three skill files) on failure rows; independently re-run and confirmed passing as part of the same test. |
| 4 | CLI propagation: codex row `failed`/`failed` (registration/skills split), claude-code row unaffected, exit code `exitPartial`, write log excludes index, includes skills-dir writes, seeded bytes unchanged. | ✓ VERIFIED | `TestSetupIndexReadFailureReachesPartialExit` read in full and independently re-run; PASS. |
| 5 | `installFiles`'s own posture unchanged — unconditional overwrite on any skill-file read failure (D-08). | ✓ VERIFIED | `git diff` of the fix commit shows zero body changes inside `installFiles`; `TestInstallNativeConverges`, `TestInstallOverwritesDifferingDestination`, `TestInstallAccumulatesFailuresAndAttemptsEverySkill` independently re-run, all PASS. |
| 6 | Every pre-existing AGENTS.md proof (4 `TestInstallAgentsMdConverges` subtests, `TestAgentsMdPreservesSymlink`) passes unmodified. | ✓ VERIFIED | Independently re-run as part of the full `internal/skills` package suite; all PASS. |
| 7 | `Environment.ReadFile` seam doc comment and `installAgentsMDIndex` doc comment state the engram-owned-vs-operator-owned distinction instead of claiming uniform handling. | ✓ VERIFIED | Read directly: `environment.go` lines 23-37 name both `installFiles` and `installAgentsMDIndex` and the `fs.ErrNotExist` rule; `rg -n 'ANY read error' internal/skills/install.go internal/skills/environment.go` prints nothing (the stale claim, corrected in follow-up commit `bae018f2`, is gone). |

**Score (this pass):** 7/7 gap-closure truths verified, all with independently-executed test
evidence (not SUMMARY-trusted) or direct code inspection.

### Historical Verification Evidence (carried forward, plans 04-01 through 04-04)

The 46 observable truths verified in the initial phase pass (2026-09-12T15:41:47Z, prior to the
milestone audit) are preserved below unchanged. Plan 04-05 touched only `internal/skills/install.go`
(one predicate + doc comment), `internal/skills/environment.go` (doc comment), and two test files;
none of the 46 truths' supporting artifacts were touched outside the scope already accounted for
above, so a full re-derivation was not repeated — instead the affected surfaces (`installFiles`,
`installAgentsMDIndex`'s other branches, `TestInstallAgentsMdConverges`, `TestAgentsMdPreservesSymlink`)
were independently re-run this pass (see command table above) and confirmed passing, closing the
regression-risk gap that a narrow fix always carries.

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | A clean-checkout binary carries every byte of `skill/engram/skills/**` with no plugin present, via a committed vendored copy read by `//go:embed all:data`. | ✓ VERIFIED | `internal/skills/embed.go`; `diff -rq` confirms byte-identity. |
| 2 | The drift gate is set-equality-then-byte-equality, never containment, and runs inside `go test ./...`. | ✓ VERIFIED | `internal/skills/drift_test.go`. |
| 3 | The skill inventory is whatever the embedded FS structurally walks — no hardcoded count or name list. | ✓ VERIFIED | `internal/skills/inventory.go`'s `walkSkills`. |
| 4 | `engram setup --apply --runtime claude-code` writes each skill to `$HOME/.claude/skills/<name>/SKILL.md`, byte-identical, no provenance header. | ✓ VERIFIED | `claudecode.go:113-121`. |
| 5 | A second consecutive apply reports `already-correct`, decided by byte-compare. | ✓ VERIFIED | `TestInstallNativeConverges`, re-run this pass. |
| 6 | A differing destination file is overwritten unconditionally and reported `wrote`. | ✓ VERIFIED | `TestInstallOverwritesDifferingDestination`, re-run this pass. |
| 7 | Every skill is attempted even after an earlier failure; failures accumulate via `errors.Join`. | ✓ VERIFIED | `TestInstallAccumulatesFailuresAndAttemptsEverySkill`, re-run this pass. |
| 8 | A runtime row carries exactly one aggregated `Outcome`, precedence `failed > wrote > already-correct > would-write > not-present`, mixed state resolves UP. | ✓ VERIFIED | `internal/setup/aggregate.go`. |
| 9 | `internal/setup/exit.go` is byte-identical to pre-phase state. | ✓ VERIFIED | Confirmed still byte-identical this pass (`git diff efcfb0ad..HEAD -- internal/setup/exit.go` empty). |
| 10 | `internal/setup` imports nothing outside stdlib and nothing from this module. | ✓ VERIFIED | `internal/setup/leafpurity_test.go`. |
| 11 | `internal/skills` imports no package from this module, and exactly one allowlisted third-party package. | ✓ VERIFIED | `internal/skills/importgate_test.go`, re-run this pass (`TestSkillsPackageImportsAreGated` PASS). |
| 12 | Every skills destination is absolute, derived from `Environment.HomeDir()`. | ✓ VERIFIED | `plan_test.go`. |
| 13 | The text lane never carries skill content; `--output json` carries full bytes. | ✓ VERIFIED | `TestSetupPreviewSkillsJSON`. |
| 14 | A skills-only failure alongside another runtime's success produces the partial exit class. | ✓ VERIFIED | `TestSetupSkillsFailureReachesPartialExit`, re-run this pass. |
| 15 | Every shipped SKILL.md carries one authored, single-line `metadata.engram-summary` index entry. | ✓ VERIFIED | `TestEverySkillCarriesIndexSummary`, re-run this pass. |
| 16 | A newline or oversized summary is rejected at PARSE time. | ✓ VERIFIED | `frontmatter.go`; `TestParseFrontmatter`, re-run this pass. |
| 17 | `go.yaml.in/yaml/v3` is direct; `gopkg.in/yaml.v3` stays indirect. | ✓ VERIFIED | `go.mod`. |
| 18 | The AGENTS.md block marker pair is fixed and parameterless. | ✓ VERIFIED | `agentsmd.go`. |
| 19 | Zero blocks → append; one well-formed block → replace interior only; 2+/unmatched → hard-fail naming byte offsets, zero bytes written. | ✓ VERIFIED | `TestAgentsMdSplice`, re-run this pass. |
| 20 | The AGENTS.md write is in-place, single write, writes through a symlink. | ✓ VERIFIED | `TestAgentsMdPreservesSymlink`, re-run this pass. |
| 21 | A malformed AGENTS.md fails the index splice but does not prevent skill files from being written (D-07). | ✓ VERIFIED | `installAgentsMDIndex`'s early-return path. |
| 22 | Codex's destination is `$HOME/.agents/skills`; opencode's honors `XDG_CONFIG_HOME`. | ✓ VERIFIED | `codex.go`, `opencode.go`. |
| 23 | The `Runtime` interface stays at exactly three methods. | ✓ VERIFIED | `internal/setup/runtime.go`. |
| 24 | `generic` authors no-destination format, stays `would-write` in both lanes. | ✓ VERIFIED | `generic.go`; `TestGenericSkillsOutcomeIsWouldWriteInBothLanes`. |
| 25 | `setupCmd`'s short description and flag set byte-identical. | ✓ VERIFIED | `git diff --stat` confined to `help.golden`. |
| 26 | A single invocation across all four runtimes renders one row each. | ✓ VERIFIED | `TestSetupReportCoversEveryRuntimeShape`, re-run this pass. |
| 27 | No test invokes a real runtime binary or writes to a real home directory (except the one scoped symlink test). | ✓ VERIFIED | Confirmed again this pass via negative grep across the two new/modified test files. |
| 28 | Claude Code, Codex, and opencode tolerate `metadata.engram-summary` without dropping the skill. | ✓ VERIFIED (human, non-rejection) | Sean's 2026-09-10 observation, carried forward (see `human_verification_evidence`). |

(Rows 29-46 are the remaining plan-04-01..04-04 must-haves already exhaustively verified in the
prior report; their supporting artifacts were not touched by 04-05 and were not re-derived here.
None intersect the B01 fix's changed lines.)

**Score:** 46/46 historical truths remain valid (spot-re-checked this pass, no regressions found)
+ 7/7 new gap-closure truths = 53/53.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/skills/install.go` | `installAgentsMDIndex` classifies read errors two ways | ✓ VERIFIED | Read in full; matches plan exactly. |
| `internal/skills/environment.go` | `ReadFile` doc comment states the two-consumer distinction | ✓ VERIFIED | Read in full; no stale "ANY read error" claim about `installAgentsMDIndex`. |
| `internal/skills/install_test.go` | `TestInstallPreservesIndexOnReadError` (4-subtest table) | ✓ VERIFIED | Present, independently re-run, all 4 subtests pass. |
| `cmd/engram/setup_test.go` | `TestSetupIndexReadFailureReachesPartialExit` | ✓ VERIFIED | Present, read in full, independently re-run, passes. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/skills/install.go` (`installAgentsMDIndex`) | `internal/skills/agentsmd.go` (`Splice`) | Confirmed nonexistence hands `Splice` a nil document; any other read error returns before `RenderBlock`/`Splice` | ✓ WIRED | Confirmed by direct code read — the early return is textually before `RenderBlock(`. |
| `cmd/engram/setup.go` (`setupApplySkillsFacet`) | `internal/skills/install.go` (`Install`'s `Report.Err`) | `Report.Err` becomes `installErr`, feeds `SkillsOutcome(failed=true)`, joined into `row.Reason` | ✓ WIRED | `TestSetupIndexReadFailureReachesPartialExit` proves this end-to-end: codex row `Reason` contains the index path. |
| `internal/setup/aggregate.go` (`AggregateOutcome`) | `internal/setup/exit.go` (`Classify`) | `SkillsOutcome`'s `OutcomeFailed` folds into the row's one `Outcome`; `Classify` turns one failed + one non-failed row into `ExitPartial` | ✓ WIRED | Same test confirms `exitCodeFromError(err) == exitPartial`. |

### Data-Flow Trace (Level 4)

Unchanged from the prior report — the B01 fix touches only the error-classification branch, not
any data-producing path. `row.SkillsContent`/`SkillsDigest`/`SkillsBytes`, the spliced AGENTS.md
block, and installed `SKILL.md` bytes all still trace to `skills.Inventory()` over the embedded
FS; no hardcoded/static fallback introduced.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Fix predicate present exactly once | `rg -n 'errors\.Is\(readErr, fs\.ErrNotExist\)' internal/skills/install.go` | 1 line | ✓ PASS |
| Wrapped error message present | `rg -n 'skills: read index %s: %w' internal/skills/install.go` | 1 line | ✓ PASS |
| Stale doc claim removed | `rg -n 'ANY read error' internal/skills/install.go internal/skills/environment.go` | 0 lines | ✓ PASS |
| No staging/renaming/locking introduced | `rg -n 'os\.CreateTemp\|os\.Rename\|EvalSymlinks\|flock' internal/skills/install.go` | 1 pre-existing line (D-16 rationale comment, unrelated to this fix — confirmed present in the pre-plan baseline too) | ✓ PASS |
| Regression gate: gap-closure unit test | `go test ./internal/skills/... -count=1 -run '^TestInstallPreservesIndexOnReadError$' -v` | 4/4 subtests PASS | ✓ PASS |
| Regression gate: CLI propagation test | `go test ./cmd/engram/... -count=1 -run '^TestSetupIndexReadFailureReachesPartialExit$' -v` | PASS | ✓ PASS |
| Full touched-package suite | `go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` | all `ok` | ✓ PASS |
| Format/vet clean | `gofmt -l internal/skills/ cmd/engram/` | empty | ✓ PASS |
| License headers | `task license:check` | 0 invalid | ✓ PASS |
| No real-`$HOME` reach in new/modified tests | `rg -n 'os\.UserHomeDir\|exec\.Command\(\|os\.Getenv\("HOME"\)' cmd/engram/setup_test.go internal/skills/install_test.go` | 0 lines | ✓ PASS |
| Vendored skill tree untouched | `git status --porcelain internal/skills/data` | empty | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` convention in this repo and no probe declared by any Phase 4
plan/SUMMARY — skipped, consistent with the prior report.

### Anti-Patterns Found

No `TBD`/`FIXME`/`XXX`/`HACK`/`TODO` markers in any file touched by plan 04-05. `04-REVIEW.md`
(status: `clean`, 2026-09-12) independently re-reviewed the same three production/test files and
found 0 critical/warning findings, 1 info-level note — consistent with this verification's own
read of the code.

### Human Verification Required

None pending. Existing evidence from 2026-09-10 (Sean) is preserved unchanged in
`human_verification_evidence` — the B01 fix and its regression tests do not touch runtime-loading
behavior, so no new live observation is needed or was performed.

## Gaps Summary

B01 (GitHub issue #559) is closed: `internal/skills.Install`'s `FormatAgentsMD` branch now
distinguishes confirmed index nonexistence from every other read error, preserving an unreadable
operator AGENTS.md byte-for-byte with zero writes and a wrapped, path-naming error, while the
independent native skill-file pass still completes. This was verified independently in this pass
via direct code reading and independently-executed tests (not SUMMARY.md-trusted): both new
regression tests (`TestInstallPreservesIndexOnReadError`, `TestSetupIndexReadFailureReachesPartialExit`)
pass, `installFiles` and every other D-07/D-08/D-15/D-16 proof remain unmodified and green, and no
test reaches a real `$HOME`.

**Non-blocking note:** `.planning/REQUIREMENTS.md` and `.planning/ROADMAP.md` still carry the
pre-fix "Audit gap (2026-09-12)" / `[ ]` annotations for `REQ-skills-agents-md-fallback` and
"Phase 4: Skills Distribution" respectively, referencing #559. These are stale planning-artifact
bookkeeping, not a code gap — the underlying requirement is satisfied in the codebase as verified
above. Whatever workflow step owns milestone/requirement checkbox bookkeeping should update these
two files to reflect the closure; this verifier does not edit them itself.

Status: **passed**.

---

_Verified: 2026-09-12T20:25:07Z_
_Verifier: Claude (gsd-verifier); historical evidence retained, B01 gap independently re-verified closed_

## Tracking-only freshness check — 2026-09-12

After this report's verdict, `phase.complete` and the orchestrator updated only
tracking bookkeeping in the covered inputs: `ROADMAP.md` (Phase 4 checkbox,
plan count, progress-table row → Complete 3/3) and `REQUIREMENTS.md`
(`REQ-skills-agents-md-fallback` checkbox and traceability row → Complete; the
audit-gap annotation now records closure by 04-05). No covered source, test,
skill or plan/summary file changed. Reviewed that scoped diff and refreshed the
covered digest; the verified behavioral score (53/53) is unchanged.
