---
phase: 04-skills-distribution
plan: 01
subsystem: distribution
tags: [go-embed, skills, setup-command, claude-code, testing-fstest]

requires:
  - phase: 03-runtime-registration
    provides: "internal/setup's Environment seam, Runtime interface, and the shared Preview/Apply executor D-08's byte-compare convergence pattern extends here"
  - phase: 02-setup-command-core
    provides: "the declarative Plan/Result/Outcome model and the leaf-purity gate D-05's declare/install split depends on"
provides:
  - "internal/skills package: //go:embed all:data, structural inventory walk, injectable filesystem seam, byte-compare-convergence native install"
  - "internal/skills/data/**: vendored, git-tracked, drift-gated copy of skill/engram/skills/"
  - "internal/setup.SkillFormat/SkillTarget declared on Plan, threaded through to Result"
  - "internal/setup.SkillsOutcome/AggregateOutcome — D-06's aggregation precedence, proven exhaustively over its whole input space"
  - "cmd/engram/setup.go's skills facet composition (setupSkillsTarget, setupApplySkillsFacet) and seven new report row fields"
  - "claude-code end-to-end: engram setup --apply --runtime claude-code installs skills natively, byte-identically, converging on re-run"
affects: [04-02-agents-md-fallback, 04-03-remaining-runtimes, 04-04-generic-and-summary]

actuals:
  tokens: 35688
  tasks: 3
  commits: 4
  plan_head_before: 788d7127e88d25882283b5d9b3fac33d1b0c14e8

tech-stack:
  added: []
  patterns:
    - "Vendor-then-//go:embed all: (mirrors internal/webauth/static.go verbatim)"
    - "Struct-of-func-fields injectable filesystem seam, never an interface (skills.Environment, mirrors internal/setup.Environment)"
    - "errors.Join failure accumulation across independent per-file writes (D-07, mirrors internal/migrate/registry.go's Validate)"
    - "testing/fstest.MapFS as the in-memory fs.FS driving the SAME unexported walk the binary uses"
    - "One D-05 composition point (cmd/engram is the only importer of both internal/setup and internal/skills)"

key-files:
  created:
    - internal/skills/embed.go
    - internal/skills/inventory.go
    - internal/skills/environment.go
    - internal/skills/install.go
    - internal/skills/drift_test.go
    - internal/skills/install_test.go
    - internal/skills/inventory_test.go
    - internal/skills/importgate_test.go
    - internal/skills/data/curating-memory/SKILL.md
    - internal/skills/data/curating-spine/SKILL.md
    - internal/skills/data/discovering/SKILL.md
    - internal/skills/data/migrating-from-beads/SKILL.md
    - internal/skills/data/promoting-memory/SKILL.md
    - internal/setup/aggregate.go
    - internal/setup/aggregate_test.go
  modified:
    - internal/setup/plan.go
    - internal/setup/apply.go
    - internal/setup/claudecode.go
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - Taskfile.yaml
    - .licenserc.yaml
    - .rumdl.toml

key-decisions:
  - "setupSkillsTarget returns (skills.Target, bool) rather than the plan's literal single-return signature, so a runtime not yet wired for skills this wave (codex/opencode/generic) is skipped entirely rather than reaching skills.Install with an empty, unauthored destination."
  - "cmd/engram/setup_test.go's withFakeSetupEnv now also fakes the package-level skillsEnv seam by default, protecting every pre-existing --apply test from a real filesystem write to $HOME/.claude/skills that Task 1's composition wiring would otherwise trigger."
  - "internal/skills/inventory.go's walk was factored into an fs.FS-driven unexported helper (walkSkills) in Task 1's own commit rather than deferred to Task 3, since writing it once in its final shape was lower-risk than writing an embed-specific version and refactoring it later in the same plan; Task 3 still lands its own dedicated refactor commit against the RED test it introduces, per the TDD cycle."

requirements-completed: []

coverage:
  - id: D1
    description: "A binary built from a clean checkout carries every byte of the five curation skills via a committed, drift-gated vendor copy — no Claude plugin required."
    requirement: "REQ-skills-embedded-in-binary"
    verification:
      - kind: unit
        ref: "internal/skills/drift_test.go#TestSkillsEmbedMatchesVendored"
        status: pass
      - kind: unit
        ref: "internal/skills/inventory_test.go#TestInventoryIsStructural"
        status: pass
      - kind: unit
        ref: "internal/skills/inventory_test.go#TestInventoryIncludesUnderscoreAndDotPrefixedContent"
        status: pass
      - kind: unit
        ref: "internal/skills/inventory_test.go#TestInventoryIsDeterministic"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram setup --apply --runtime claude-code installs every discovered skill natively, byte-identically, converging (already-correct) on re-run."
    requirement: "REQ-skills-native-format"
    verification:
      - kind: unit
        ref: "internal/skills/install_test.go#TestInstallNativeConverges"
        status: pass
      - kind: unit
        ref: "internal/skills/install_test.go#TestInstallOverwritesDifferingDestination"
        status: pass
      - kind: unit
        ref: "internal/skills/install_test.go#TestInstallAccumulatesFailuresAndAttemptsEverySkill"
        status: pass
    human_judgment: false
  - id: D3
    description: "A runtime row carries one aggregated Outcome (D-06 precedence: failed > wrote > already-correct > would-write > not-present), proven over its entire input space, and a skills-only failure reaches the partial exit class."
    verification:
      - kind: unit
        ref: "internal/setup/aggregate_test.go#TestAggregateOutcomeExhaustive"
        status: pass
      - kind: unit
        ref: "internal/setup/aggregate_test.go#TestAggregatePrecedenceIsAuthoredNotDerived"
        status: pass
      - kind: unit
        ref: "internal/setup/aggregate_test.go#TestSkillsOutcomeExhaustive"
        status: pass
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupSkillsFailureReachesPartialExit"
        status: pass
    human_judgment: false
  - id: D4
    description: "The dense text row never carries skill file content; the full content reaches the --output json lane only."
    verification:
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupPreviewSkillsJSON"
        status: pass
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupTextRowOmitsSkillContent"
        status: pass
    human_judgment: false
  - id: D5
    description: "internal/setup stays a stdlib-only leaf; internal/skills is proven never to import it (D-05's one-directional declare/install split)."
    verification:
      - kind: unit
        ref: "internal/setup/leafpurity_test.go#TestSetupPackageIsStdlibOnlyLeaf"
        status: pass
      - kind: unit
        ref: "internal/skills/importgate_test.go#TestSkillsPackageImportsAreGated"
        status: pass
    human_judgment: false

duration: 34min
completed: 2026-09-10
status: complete
---

# Phase 4 Plan 1: End-to-End Skills Install for Claude Code Summary

**`engram setup --apply --runtime claude-code` now installs the five curation skills into `~/.claude/skills/<name>/SKILL.md`, byte-identical to the plugin, converging on re-run — proven through a vendor-then-`//go:embed all:` pipeline with a byte-equality drift gate and an exhaustively-tested outcome-aggregation precedence.**

## Performance

- **Duration:** ~34 min
- **Started:** 2026-09-10T04:22:58Z
- **Completed:** 2026-09-10T04:57:05Z
- **Tasks:** 3
- **Files modified/created:** 23

## Accomplishments

- New `internal/skills` package: `//go:embed all:data` over a git-tracked vendored copy of `skill/engram/skills/`, a structural inventory walk (`Inventory`, `Digest`, `TotalBytes`, `ContentJSON`), an injectable filesystem seam, and a byte-compare-convergence native install (`Install`).
- `internal/skills/drift_test.go`'s `TestSkillsEmbedMatchesVendored` — set-equality then byte-equality between the embedded FS and the on-disk `skill/` tree, live-verified to fail on a single corrupted byte and pass again once restored.
- `internal/setup`: `SkillFormat`/`SkillTarget` declared on `Plan`, threaded onto `Result` (json-excluded carrier), and `claude-code`'s `Plan()` now authors its skills destination from `env.HomeDir()` for every auth mode.
- `internal/setup/aggregate.go`: `SkillsOutcome`/`AggregateOutcome` implementing D-06's precedence (`failed > wrote > already-correct > would-write > not-present`), proven exhaustively over all 36 ordered pairs plus dedicated precedence and empty-inventory-is-failure tests.
- `cmd/engram/setup.go`: the one D-05 composition point (`setupSkillsTarget`), the skills facet (`setupApplySkillsFacet`), and seven new omitempty report row fields — `skills_content` is populated only for a non-text output lane.
- `internal/skills/inventory.go`'s walk factored into an `fs.FS`-driven `walkSkills`, proven structural (not an enumeration) against an in-memory `testing/fstest.MapFS`, and `internal/skills/importgate_test.go` narrows the same-module import ban with an (empty, this wave) named third-party allowlist.

## Task Commits

Each task was committed atomically (Task 2 and Task 3 carried the `tdd="true"` attribute — see TDD Gate Compliance below):

1. **Task 1: End-to-end skills install for claude-code — vendor, embed, inventory, write, report** — `24194900` (feat)
2. **Task 2: Outcome aggregation proven exhaustively, and the report shape D-03 requires** — `48d52049` (test)
3. **Task 3: The structural-inventory guarantee and the narrowed import gate** — `bd8e8a5d` (test, RED) + `4b41db68` (refactor, GREEN)

## Files Created/Modified

- `internal/skills/embed.go` — `//go:embed all:data`, the webauth-precedent rationale comment.
- `internal/skills/inventory.go` — `Inventory`/`walkSkills`/`Digest`/`TotalBytes`/`ContentJSON`.
- `internal/skills/environment.go` — `Environment` seam (`ReadFile`/`WriteFile`/`MkdirAll`), `OSEnvironment`.
- `internal/skills/install.go` — `Format`/`Target`/`Report`/`Install` (native implemented; agents-md deliberately not wired yet — see Known Stubs).
- `internal/skills/data/**` — vendored, git-tracked copy of `skill/engram/skills/`.
- `internal/setup/plan.go` — `SkillFormat`, `SkillTarget`, `Plan.Skills`, `Result.Skills` (json-excluded).
- `internal/setup/apply.go` — threads `plan.Skills` onto `Result` on every path reaching a plan.
- `internal/setup/aggregate.go` — `SkillsOutcome`, `AggregateOutcome`.
- `internal/setup/claudecode.go` — authors `SkillTarget` from `env.HomeDir()` in all four auth-mode branches.
- `cmd/engram/setup.go` — `skillsEnv`, `setupSkillsTarget`, `setupApplySkillsFacet`, seven new row fields.
- `cmd/engram/setup_test.go` — three new Task 2 tests, plus the `skillsEnv` test-isolation fix in `withFakeSetupEnv`.
- `Taskfile.yaml` / `.licenserc.yaml` / `.rumdl.toml` — `skills:vendor` target and the vendored-tree carve-outs.

## Decisions Made

- **`setupSkillsTarget` returns `(skills.Target, bool)`, not the plan's literal single-return signature.** Only `claude-code` authors a `SkillTarget` this wave; `codex`/`opencode`/`generic` leave `Plan.Skills` at its Go zero value (`Format: ""`), which is not one of the three explicit `SkillFormat` values. Roughly a dozen pre-existing `cmd/engram/setup_test.go` tests exercise `claude-code`, `codex`, and `opencode` together under both preview and `--apply`. A single-return `setupSkillsTarget` with no `default` case (as literally specified) would have produced `skills.Target{}`'s zero value for those un-wired runtimes and either called `skills.Install` against an empty destination or required a `default: panic(...)`, either of which breaks those tests. Returning `ok=false` lets the caller skip the skills facet entirely for an un-wired runtime, leaving its row exactly as it was before this plan.
- **`withFakeSetupEnv` now also fakes `skillsEnv` by default.** Task 1's composition wiring means `setupApplySkillsFacet` runs for every present, skills-wired runtime reached through `setupPreview`/`setupApplyRun` — including every pre-existing `--apply` test exercising `claude-code`. Without this, those tests would have called `skills.Install` against the REAL `skills.OSEnvironment`, attempting to create `/home/fake/.claude/skills/...` on the actual host filesystem (this was caught live: `mkdir /home/fake: operation not supported`, since `fakeSetupEnv`'s `HomeDir` returns the non-existent path `/home/fake`).
- **`internal/skills/inventory.go`'s `fs.FS`-driven `walkSkills` was written in its final shape in Task 1's own commit**, rather than as an embed-specific walk later refactored in Task 3. Task 3 still lands its own dedicated commit pair (RED test referencing `walkSkills`, confirmed failing to compile via `go vet`; GREEN refactor extracting it) — see TDD Gate Compliance.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `setupSkillsTarget` widened to return `(skills.Target, bool)`**
- **Found during:** Task 1 (composition wiring in `cmd/engram/setup.go`)
- **Issue:** The plan's literal signature `setupSkillsTarget(setup.SkillTarget) skills.Target`, called for every present runtime "whose `Result.Skills.Format` is not the no-destination value," has no way to distinguish "explicitly authored as `none`" from "never authored" (the Go zero value `""`) — both are `!= SkillFormatNone`. Only `claude-code`'s `Plan()` sets `Skills` this wave.
- **Fix:** `setupSkillsTarget` returns `(skills.Target, bool)`; `ok` is `false` for any format outside the three explicit values (including the zero value), and the composition point (`setupApplySkillsFacet`) returns the registration outcome unchanged when `ok` is `false` — no skills facet at all for an un-wired runtime.
- **Files modified:** `cmd/engram/setup.go`
- **Verification:** `go test ./cmd/engram/... -count=1` — full package green, including every pre-existing `codex`/`opencode`/`generic` test.
- **Committed in:** `24194900` (Task 1 commit)

**2. [Rule 3 - Blocking] `withFakeSetupEnv` widened to fake `skillsEnv`**
- **Found during:** Task 1 (running `go test ./cmd/engram/...` for the first time after composition wiring)
- **Issue:** `TestSetupApplyRunsRealRegistrationSucceeds` and `TestSetupPartialExitIsLiveProducible` failed live: `skills.Install` ran against the real `skills.OSEnvironment`, attempting `mkdir /home/fake/.claude/skills/curating-memory` on the actual filesystem and failing with `operation not supported`. Every pre-existing `--apply` test exercising `claude-code` was at risk of the same class of failure (or, worse, a real write, had `/home/fake` been creatable).
- **Fix:** `withFakeSetupEnv` (`cmd/engram/setup_test.go`) now also overrides the package-level `skillsEnv` seam with a fresh in-memory `fakeSkillsEnv()` (backed by a map), restored via `t.Cleanup` alongside `setupEnv`. A test needing a specific skills outcome (`TestSetupSkillsFailureReachesPartialExit`) overrides `skillsEnv` again, after calling the helper.
- **Files modified:** `cmd/engram/setup_test.go` (not in this plan's `files_modified` frontmatter list — added as a necessary test-isolation fix)
- **Verification:** `go test ./cmd/engram/... -count=1` — full package green; re-ran with `-v` to confirm no test attempts a real filesystem write.
- **Committed in:** `24194900` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking issues discovered while making the plan's own tracer `<verify>` block pass).
**Impact on plan:** Both fixes were necessary for the plan's own stated invariant ("no test touches a real home directory," `install_test.go`'s own action text) to actually hold across the WHOLE package, not just the new files. No scope creep — no new behavior was added beyond what D-05/D-06/D-08 already require.

## TDD Gate Compliance

Task 2 and Task 3 both carried `tdd="true"`. `workflow.tdd_mode` is `false` in this project's `.planning/config.json`, so the strict plan-level gate (`gsd_run check tdd-red-evidence`) was not invoked; the RED→GREEN cycle was still followed where it could be, and the situation where it could not is disclosed here rather than silently glossed over.

- **Task 2 — no clean RED phase; disclosed, not hidden.** `internal/setup/aggregate.go`'s `SkillsOutcome`/`AggregateOutcome` were fully specified, in complete behavioral detail, in Task 1's own action text (04-01-PLAN.md), and had to be implemented in Task 1's commit to make that task's own tracer `<verify>` block pass. There was no intermediate "stub" state for Task 2's tests to target as RED. Every test in `internal/setup/aggregate_test.go` and the three new `cmd/engram/setup_test.go` tests passed on their FIRST run (see commit `48d52049`'s message). This is a single `test(04-01): ...` commit with no paired `feat(04-01): ...` — the implementation commit is Task 1's `24194900`, one commit earlier in the same plan.
- **Task 3 — clean RED→GREEN.** `TestInventoryIsStructural` referenced an unexported `walkSkills(fsys fs.FS)` that did not yet exist. RED evidence: `go vet ./internal/skills/...` exited 1 with `internal/skills/inventory_test.go:25:21: undefined: walkSkills` — the target test's own package failed to build on the very symbol the test needed, which is the idiomatic Go-TDD shape for a "not yet extracted" refactor (a runtime assertion failure is not obtainable when the function under test doesn't exist as a symbol at all). Committed as `bd8e8a5d` (`test(04-01): add failing test for structural inventory walk`). GREEN: `internal/skills/inventory.go` refactored to extract `walkSkills`, confirmed via `go build ./...` succeeding and all four new tests plus `TestSkillsPackageImportsAreGated` passing. Committed as `4b41db68` (`refactor(04-01): ...` — no observable behavior change to `Inventory()`'s own output, hence `refactor` rather than `feat`, per the commit-type table).
- No REFACTOR-phase test regression occurred in either task.

## Issues Encountered

None beyond the two deviations documented above, both resolved within the same task's commit.

## Known Stubs

- **`internal/skills.Install`'s `FormatAgentsMD` case returns `Report{Err: errors.New("skills: agents-md install is not wired yet")}`.** This is an EXPLICIT, plan-specified stub — Task 1's own action text states "the agents-md case returns a Report whose `Err` states it is not wired yet" — not a gap discovered during implementation. No runtime authors `SkillFormatAgentsMD` yet (`claude-code` is the only wired runtime this wave, and it uses `SkillFormatNative`), so this branch is currently unreachable in production. Resolved by `04-02-PLAN.md`, which implements the AGENTS.md delimited-block splice (D-13/D-15/D-16).
- File: `internal/skills/install.go`, the `case FormatAgentsMD:` branch.

## Threat Flags

None. Every write path in this plan matches the phase's own registered threat model (`04-01-PLAN.md`'s `<threat_model>`): T-04-08 (absolute, `HomeDir`-derived destinations only — `rg -n '~|os\.Getenv' internal/setup/claudecode.go` finds neither a tilde literal nor an environment-variable read), T-04-04 (skill content reaches the `--output json` lane only, proven by `TestSetupTextRowOmitsSkillContent`), and T-04-03/T-04-SC (no staging-and-rename write path, zero new third-party dependencies — `internal/skills/importgate_test.go`'s allowlist is empty).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The tracer slice is proven end-to-end for `claude-code`: vendor → embed → structural inventory → declarative `SkillTarget` → injectable install → aggregated outcome → dense/full report rows. Every seam Waves 2-4 need (`internal/skills.Environment`, `internal/setup.SkillTarget`/`SkillFormat`, `setup.AggregateOutcome`, `cmd/engram`'s `setupSkillsTarget`) is in place and tested.
- `codex`, `opencode`, and `generic` deliberately carry NO `SkillTarget` yet — their `Plan()` functions are untouched, exactly as this plan's `files_modified` frontmatter scoped it. `cmd/engram/setup.go`'s `setupApplySkillsFacet` already degrades correctly for an un-wired runtime (no facet, row unchanged), so `04-02`/`04-03`/`04-04` can wire each remaining runtime independently without revisiting this plan's composition point.
- `04-02-PLAN.md` is next: the AGENTS.md delimited-block splice and `SkillFormatAgentsMD`'s real implementation (closing this plan's one disclosed stub), plus the YAML frontmatter parser (`go.yaml.in/yaml/v3`) that widens `internal/skills`' now-empty third-party allowlist by exactly one entry.
- `REQ-skills-embedded-in-binary` and `REQ-skills-native-format` are NOT yet marked complete in `REQUIREMENTS.md` — both are shared with `04-02`/`04-03`/`04-04` per the shared-ID gate (`gsd_run query requirements.ready-ids` confirmed `0/2` ready), and will flip to `Complete` once every plan declaring them has its own `SUMMARY.md`.

## Self-Check: PASSED

- All 12 key files (created in Tasks 1-3) verified present on disk with `[ -f ]`.
- All 5 commits (`24194900`, `48d52049`, `bd8e8a5d`, `4b41db68`, `0dccc1d0`) verified present in `git log --oneline --all`.
- Re-ran `go build ./... && go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` — all green.
- Re-ran the drift-gate live falsification (corrupt one byte under `internal/skills/data`, confirm `TestSkillsEmbedMatchesVendored` fails naming the exact path, restore, confirm it passes again) — behaves as documented.
- `git diff --stat internal/setup/exit.go internal/setup/exit_test.go` and `cmd/engram/testdata/catalog.golden` both print nothing (pinned files untouched).

---
*Phase: 04-skills-distribution*
*Completed: 2026-09-10*
