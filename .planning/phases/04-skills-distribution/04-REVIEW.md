---
phase: 04-skills-distribution
reviewed: 2026-09-10T15:52:09Z
depth: deep
files_reviewed: 33
files_reviewed_list:
  - .licenserc.yaml
  - .rumdl.toml
  - Taskfile.yaml
  - cmd/engram/setup.go
  - cmd/engram/setup_test.go
  - cmd/engram/testdata/help.golden
  - go.mod
  - internal/setup/aggregate.go
  - internal/setup/aggregate_test.go
  - internal/setup/apply.go
  - internal/setup/claudecode.go
  - internal/setup/claudecode_test.go
  - internal/setup/codex.go
  - internal/setup/codex_test.go
  - internal/setup/generic.go
  - internal/setup/generic_test.go
  - internal/setup/opencode.go
  - internal/setup/opencode_test.go
  - internal/setup/plan.go
  - internal/skills/agentsmd.go
  - internal/skills/agentsmd_test.go
  - internal/skills/data/curating-memory/SKILL.md
  - internal/skills/data/curating-spine/SKILL.md
  - internal/skills/data/discovering/SKILL.md
  - internal/skills/data/migrating-from-beads/SKILL.md
  - internal/skills/data/promoting-memory/SKILL.md
  - internal/skills/drift_test.go
  - internal/skills/embed.go
  - internal/skills/environment.go
  - internal/skills/frontmatter.go
  - internal/skills/frontmatter_test.go
  - internal/skills/importgate_test.go
  - internal/skills/install.go
  - internal/skills/install_test.go
  - internal/skills/inventory.go
  - internal/skills/inventory_test.go
  - skill/engram/skills/curating-memory/SKILL.md
  - skill/engram/skills/curating-spine/SKILL.md
  - skill/engram/skills/discovering/SKILL.md
  - skill/engram/skills/migrating-from-beads/SKILL.md
  - skill/engram/skills/promoting-memory/SKILL.md
findings:
  critical: 0
  warning: 6
  info: 0
  total: 6
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-09-10T15:52:09Z
**Depth:** deep
**Files Reviewed:** 33 (of 40 changed; 5 `internal/skills/data/**/SKILL.md` files verified byte-identical to their `skill/engram/skills/**/SKILL.md` sources and reviewed once as the `skill/` copies, per scope instructions; `cmd/engram/testdata/help.golden` reviewed as a diff, not full content)
**Status:** issues_found

## Summary

Reviewed the full non-`.planning` diff for Phase 4 (Skills Distribution): the new `internal/skills`
package (embed, inventory, environment seam, native install, AGENTS.md scan/render/splice,
frontmatter parsing), `internal/setup`'s `SkillTarget`/`SkillFormat`/`SkillsOutcome`/
`AggregateOutcome` additions, each runtime's `Plan()` wiring, and `cmd/engram/setup.go`'s
composition and report-row changes.

**All ten locked invariants in the review brief were independently verified and hold:**
`exit.go`/`exit_test.go`/`catalog.golden` are byte-identical to `788d7127`; `internal/skills`
imports zero same-module packages and exactly one third-party package
(`go.yaml.in/yaml/v3`, matching the allowlist's single entry); `gopkg.in/yaml.v3` stays indirect
and unimported by engram code; the AGENTS.md splice hard-fails on any state other than
zero/one blocks, naming byte offsets, writing zero bytes (proven by both
`internal/skills/agentsmd_test.go` and `internal/skills/install_test.go`); the AGENTS.md write is
a single in-place `env.WriteFile` call with no `os.CreateTemp`/`os.Rename`/`EvalSymlinks`
anywhere in the package, and `TestAgentsMdPreservesSymlink` proves it writes through a real
symlink; no test in this diff reaches a real runtime binary or a real `$HOME` write path;
`//go:embed all:data` is used; `generic` always authors the explicit no-destination
`SkillFormatNone` and its own facet stays `would-write` in both preview and apply, with
`setupApplySkillsFacet` refusing to call `skills.Install` at all for that format; and no
new code path puts secret material into argv or a rendered row. The vendored
`internal/skills/data/**/SKILL.md` files are confirmed byte-for-byte identical to
`skill/engram/skills/**/SKILL.md` via direct diff.

The core write-path logic (`installFiles`, `installAgentsMDIndex`, `scanBlock`/`Splice`,
`AggregateOutcome`, `SkillsOutcome`) is correct on close reading, including the byte-offset
arithmetic in `spliceReplace` (verified safe against a start-marker-line-at-EOF edge case,
which cannot occur for a well-formed block) and the independent-failure/accumulation model
(D-07) across `installFiles` and `installAgentsMDIndex`. Test quality across the new
`internal/skills` and `internal/setup` test files is strong: exhaustive tables with
hand-authored expectations (`TestAggregateOutcomeExhaustive`, `TestSkillsOutcomeExhaustive`),
set-equality-before-byte-equality drift checks, and structural (never enumerated) inventory
tests.

No BLOCKER-level defect was found: nothing here causes incorrect behavior, a security
vulnerability, or data loss as shipped. The findings below are WARNING-level test-isolation,
test-coverage, and defense-in-depth gaps — several of which are inconsistent with the
discipline this same phase applies rigorously everywhere else (fake environments for every
new test, a structural per-runtime guard for `SkillFormat`), which is what makes their
absence elsewhere worth flagging rather than waving off.

## Warnings

### WR-01: Pre-existing claude-code/opencode `Plan()` tests now silently depend on the real `$HOME`

**File:** `internal/setup/claudecode_test.go:51` (`TestClaudeCodePlan`), `:88`
(`TestClaudeCodeBearerHeaderIsAnEnvVarReference`); `internal/setup/opencode_test.go:47,67`
(`TestOpenCodePlan`), `:176` (`TestOpenCodeBearerHeaderSyntax`)

**Issue:** Before this phase, `claudeCodeRuntime.Plan` and `openCodeRuntime.Plan` had the
signature `Plan(_ Environment, opts Options)` — the `Environment` argument was ignored
entirely, so calling them with `OSEnvironment` in a test was a safe no-op. This phase changes
both to `Plan(env Environment, opts Options)` and has them call `env.HomeDir()` (directly in
`claudecode.go:114`, and via `opencodeConfigRoot` in `opencode.go:158`) to author the new
`SkillTarget`. The pre-existing tests above were not updated and still pass `OSEnvironment`
directly — confirmed present at baseline `788d7127` via `git show 788d7127:internal/setup/opencode_test.go`.
These tests now transitively invoke the real `os.UserHomeDir()` as a side effect of testing
something unrelated (MCP registration argv shape, bearer header form), even though the value
is never asserted. In any environment where `$HOME` is unresolvable (minimal containers,
some sandboxes, a user with no home directory), these four tests would newly fail with an
unrelated "resolve home directory" error — a regression in test isolation this same phase
took care to prevent everywhere else it touched (e.g. `cmd/engram/setup_test.go`'s
`withFakeSetupEnv` was explicitly widened in 04-01 specifically to stop a pre-existing test
from reaching a real filesystem once the skills facet was wired in).

**Fix:** Give these four tests a fake `Environment` with a scripted `HomeDir` (mirroring
`fakeEnv()` already used in the phase's own new tests), e.g.:
```go
env := Environment{
	LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	Getenv:   func(string) string { return "" },
	HomeDir:  func() (string, error) { return "/home/fake", nil },
}
plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: tc.auth})
```

### WR-02: No unit test asserts claude-code's own `SkillTarget` value

**File:** `internal/setup/claudecode.go:113-121`; absent from `internal/setup/claudecode_test.go`

**Issue:** `codex.go` and `opencode.go` each received a dedicated test
(`TestCodexSkillTarget`, `TestOpenCodeSkillTarget`) asserting the exact `Format`/`Dir`/
`IndexFile` the runtime authors, across every auth mode. `claude-code`'s own destination
(`filepath.Join(home, ".claude", "skills")`, `SkillFormatNative`) has no equivalent —
`claudecode_test.go` was not modified by this phase at all. The only places `.claude/skills`
appears in any test are `cmd/engram/setup_test.go:1420` and `:1774`, both of which use the
literal as a fixture to script a fake write failure — they assume the path is correct rather
than independently verifying it against `claudeCodeRuntime.Plan()`'s output. A typo or
segment reordering in the authored `Dir` (e.g. `"skills", ".claude"`) would pass
`TestEveryRuntimeAuthorsAnExplicitSkillFormat` (which only checks `Format` is non-zero) and
every existing test, and would only surface as a live-machine discrepancy.

**Fix:** Add a `TestClaudeCodeSkillTarget` analogous to `TestCodexSkillTarget`/
`TestOpenCodeSkillTarget`, asserting `plan.Skills == SkillTarget{Format: SkillFormatNative,
Dir: filepath.Join(home, ".claude", "skills")}` across all four auth modes, plus a
`TestClaudeCodePlanFailsWhenHomeUnresolvable` mirroring the codex/opencode equivalents (the
`HomeDir` error path added at `claudecode.go:114-116` is currently untested).

### WR-03: No structural guard that a non-`none` `SkillTarget`'s destination is absolute

**File:** `internal/setup` (no such test exists); `internal/skills/install.go:96-124`
(`installFiles`), `:148-194` (`installAgentsMDIndex`)

**Issue:** `TestEveryRuntimeAuthorsAnExplicitSkillFormat` (`codex_test.go:99`) is a registry-wide
structural guard proving every runtime authors a real `SkillFormat` value, explicitly
designed to catch a future runtime shipping a silently-zero-valued target. No equivalent
guard exists for the `Dir`/`IndexFile` fields being absolute. Today every runtime happens to
derive its destination from `env.HomeDir()` correctly, and `opencode_test.go:139` independently
checks `filepath.IsAbs` for its own four XDG_CONFIG_HOME cases — but `claude-code` and `codex`
have no such assertion, there is no cross-runtime version of that check, and
`internal/skills.installFiles`/`installAgentsMDIndex` themselves perform no `filepath.IsAbs`
validation before `filepath.Join`-ing and writing. A relative `Dir` (an authoring bug in a
future runtime, or a refactor that drops the `env.HomeDir()` call) would silently install
skill files, or splice into an AGENTS.md-shaped file, relative to `engram`'s own current
working directory — precisely the class of hazard T-04-08 (`04-CONTEXT.md`) exists to close,
with nothing catching it before or during the write.

**Fix:** Add a registry-driven test (same shape as `TestEveryRuntimeAuthorsAnExplicitSkillFormat`)
asserting `filepath.IsAbs(plan.Skills.Dir)` for every non-`SkillFormatNone` target and
`filepath.IsAbs(plan.Skills.IndexFile)` for every `SkillFormatAgentsMD` target; consider also
having `internal/skills.Install` refuse (accumulate an error for) a non-absolute `Dir`/
`IndexFile` as a second, independent line of defense at the point where the actual write
happens.

### WR-04: `metadata.engram-summary`'s single-line/length bound is enforced only by a test, never at parse time

**File:** `internal/skills/frontmatter.go:80-118` (`ParseFrontmatter`);
`internal/skills/frontmatter_test.go:92-113` (`TestEverySkillCarriesIndexSummary`);
`internal/skills/agentsmd.go:171-184` (`RenderBlock`)

**Issue:** `MaxSummaryBytes` (`frontmatter.go:40`) and the "authored, single-line" contract
(D-14) are checked only by `TestEverySkillCarriesIndexSummary`, which runs against whatever
the CURRENTLY embedded inventory happens to contain. `ParseFrontmatter`/`walkSkills`
themselves impose no length bound and do not reject a `metadata.engram-summary` value
containing an embedded newline (a YAML block-scalar `summary: |` value would parse
successfully and pass straight through to `Skill.Summary`). If a future skill (or an edit to
an existing one) ships with such a value and, for any reason, the `go test ./internal/skills/...`
gate is not run before the binary is built (e.g. `task skills:vendor` run standalone,
per the Taskfile.yaml comment: "NOT a dependency of default/test/test:go"), `RenderBlock`
(`agentsmd.go:179`) would silently emit a malformed multi-line bullet into the spliced
AGENTS.md index block, and `Digest`/byte-count reporting would not flag it either.

**Fix:** Have `ParseFrontmatter` itself reject a summary containing `\n` or exceeding
`MaxSummaryBytes`, returning it as a parse error (consistent with how it already treats a
missing fence or invalid YAML) — this converts the invariant from "tested, if the test
happens to run" into "structurally impossible to embed."

### WR-05: A destination already resolved is dropped from the row on an inventory failure

**File:** `cmd/engram/setup.go:108-115` (`setupApplySkillsFacet`)

**Issue:** On `targetErr != nil` (an authoring bug) the function returns before populating
`row.SkillsDest`/`SkillsIndex`. On `invErr != nil` (`skills.Inventory()` failing — a broken
embed) it also returns early, even though `setupSkillsTarget` already succeeded and `target`
holds a valid, already-computed destination. In both cases the failed row's `Reason` names the
error, but an operator triaging a broken-build report loses the destination context that was
already available. Low impact in practice (an `Inventory()` failure means the shipped binary
itself is broken, not a per-machine condition), but it is an easy, free addition.

**Fix:** Populate `row.SkillsDest`/`row.SkillsIndex` from `target` before returning on the
`invErr` path (the `targetErr` path has no valid `target` to use, so that one is unavoidable).

### WR-06: `TestSetupReportCoversEveryRuntimeShape`'s digest/byte-count expectations are derived by calling the code under test

**File:** `cmd/engram/setup_test.go:1662-1663`, `:1734-1738`

**Issue:** `wantDigest := setupSkillsDigestSummary(inv)` and `wantBytes :=
strconv.Itoa(skills.TotalBytes(inv))` call the exact same production functions
(`setupSkillsDigestSummary`, `skills.TotalBytes`) that `setupApplySkillsFacet` calls to
populate `row.SkillsDigest`/`row.SkillsBytes`. The subsequent assertions
(`row.SkillsDigest != wantDigest`) therefore prove the composition threads the same
`skills.Inventory()` call through consistently across runtimes and lanes — a legitimate
plumbing check — but prove nothing about whether `setupSkillsDigestSummary` or
`skills.TotalBytes` compute the *correct* value; a bug shared between the test's setup and the
production call site would pass silently. This is the "test computing its own expectation by
calling the function under test" pattern the review brief calls out by name. Correctness of
`Digest`/`TotalBytes` themselves is independently covered by `internal/skills/inventory_test.go`
(`TestDigestIsStableAndTruncated`), which mitigates the risk, but the closing-gate test's own
comment (`setupOutcomeFoldTable`'s doc, `setup_test.go:1608-1611`) explicitly disclaims this
exact pattern for the outcome-fold table while the digest/byte-count assertions a few lines
away use it without comment.

**Fix:** Either compute `wantDigest`/`wantBytes` from a literal, hand-verified value (as
`setupOutcomeFoldTable` does for outcomes), or narrow the assertion's stated intent in a
comment to "the composition plumbs the same inventory through consistently," so a future
reader does not mistake it for independent verification of the digest/byte-count algorithms.

---

_Reviewed: 2026-09-10T15:52:09Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
