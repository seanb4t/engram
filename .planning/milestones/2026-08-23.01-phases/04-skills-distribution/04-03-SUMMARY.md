---
phase: 04-skills-distribution
plan: 03
subsystem: distribution
tags: [skills, codex, opencode, agents-md, xdg-config-home, mcp-registration]

requires:
  - phase: 04-skills-distribution
    plan: 01
    provides: "internal/skills' Install, the injectable Environment seam, and internal/setup's SkillFormat/SkillTarget declared on Plan"
  - phase: 04-skills-distribution
    plan: 02
    provides: "internal/skills.Install's FormatAgentsMD branch (skill files to Dir plus an in-place, symlink-preserving splice into IndexFile) — the mechanism this plan routes Codex to"
provides:
  - "codex.go: every auth mode authors the same SkillTarget — SkillFormatAgentsMD, Dir=$HOME/.agents/skills, IndexFile=$HOME/.codex/AGENTS.md — implementing the codex-native-plus-index routing decision"
  - "opencode.go: every auth mode authors the same SkillTarget — SkillFormatNative, Dir=opencodeConfigRoot(env)/opencode/skills, where opencodeConfigRoot honors XDG_CONFIG_HOME only when non-empty and absolute"
  - "TestEveryRuntimeAuthorsAnExplicitSkillFormat: a structural guard, over the package-level registry, against a future runtime shipping with a zero-valued SkillTarget"
  - "A human-verified, live-machine confirmation that RESEARCH assumption A1 holds: Codex's own skill selector reads $HOME/.agents/skills"
affects: [04-04-generic-and-summary]

actuals:
  tokens: 5850
  tasks: 3
  commits: 3
  plan_head_before: 14d149cbd91bd7df392d184d8a873633dc845b39

tech-stack:
  added: []
  patterns:
    - "Every auth-mode branch of a runtime's Plan() authors the SAME SkillTarget value, computed once before the switch — mirrors claude-code's own 04-01 shape exactly"
    - "opencodeConfigRoot(env): a config-root resolver consulting exactly one environment variable (XDG_CONFIG_HOME), accepted only when non-empty AND absolute, falling back to HomeDir()+'.config' otherwise — the one environment-variable read in the whole internal/setup package"

key-files:
  created:
    - internal/setup/codex_test.go
  modified:
    - internal/setup/codex.go
    - internal/setup/opencode.go
    - internal/setup/opencode_test.go
    - cmd/engram/setup_test.go
    - .planning/phases/04-skills-distribution/04-VALIDATION.md

key-decisions:
  - "Task 1 (checkpoint:decision), routing chosen: codex-native-plus-index. Codex gets native skill files at the locked destination $HOME/.agents/skills/<name>/SKILL.md (unchanged — $CODEX_HOME/skills stays excluded, still exactly one skills destination for Codex), AND the delimited index block is additionally spliced into ~/.codex/AGENTS.md via the mechanism plan 04-02 built and proved. Reasoning accepted verbatim: (1) it is the only option under which ROADMAP success criterion 3 has a live --apply write path in v1, rather than leaving a fully built, fully tested mechanism with no production caller; (2) it hedges RESEARCH assumption A1 — if Codex's own skills loader does not read the documented directory, the index block still teaches the agent the five skills exist and names the absolute path of each SKILL.md; (3) it writes into precisely the file D-16's symlink rationale was written about. Accepted cost, explicitly acknowledged: if Codex's loader DOES read $HOME/.agents/skills, the agent sees the same five skills twice (native + index) — research established this is a context cost (bounded under 4KB, already asserted by a 04-02 test), not a correctness one."
  - "TestEveryRuntimeAuthorsAnExplicitSkillFormat skips any runtime implementing optInOnlyRuntime (currently only the generic pseudo-runtime) rather than asserting over the full literal registry. Generic is deliberately not wired for skills this wave (04-04-generic-and-summary's scope, outside this plan's files_modified) and still carries the Go zero-value SkillTarget; asserting over it here would fail against correctly-scoped, in-progress work. The exclusion is expressed through the SAME structural predicate Select() already uses to exclude generic from default selection (runtime.go) — never a by-name special case in this file, honoring the plan's own prohibition."
  - "[Task 1 pre-existing test fixture, Rule 1] TestSetupSkillsFailureReachesPartialExit's fake WriteFile, previously failing universally, is now scoped to fail only under claude-code's own .claude/skills path. Its original comment assumed 'codex — not yet wired for skills this wave — simply registers successfully with no skills facet at all', which this plan's own mandated change (wiring codex for skills) makes false; scoping the failure preserves the test's original intent (a skills-only failure on one runtime, paired with another runtime's clean success, produces the partial exit class)."

requirements-completed: []

coverage:
  - id: D1
    description: "Task 1 (checkpoint:decision): the AGENTS.md routing question — which runtime, if any, carries the delimited index — is answered by the developer rather than assumed, and the choice plus its reasoning is recorded verbatim."
    requirement: "REQ-skills-agents-md-fallback"
    verification: []
    human_judgment: true
    rationale: "A checkpoint:decision task's done-criterion is a recorded human choice, not a testable assertion."
  - id: D2
    description: "codex and opencode each author their own SkillTarget inside their own Plan() method, identical across every auth mode, never relative, never the excluded $CODEX_HOME/skills path, with a home-directory-resolution failure surfacing as a named error rather than an empty destination — and every registered, non-opt-in-only runtime is proven to author an explicit (never zero-value) SkillFormat."
    requirement: "REQ-skills-native-format"
    verification:
      - kind: unit
        ref: "internal/setup/codex_test.go#TestCodexSkillTarget"
        status: pass
      - kind: unit
        ref: "internal/setup/codex_test.go#TestCodexSkillTargetIsNotCodexHome"
        status: pass
      - kind: unit
        ref: "internal/setup/codex_test.go#TestCodexPlanFailsWhenHomeUnresolvable"
        status: pass
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodeSkillTarget"
        status: pass
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodePlanFailsWhenHomeUnresolvable"
        status: pass
      - kind: unit
        ref: "internal/setup/codex_test.go#TestEveryRuntimeAuthorsAnExplicitSkillFormat"
        status: pass
      - kind: unit
        ref: "internal/setup/leafpurity_test.go#TestSetupPackageIsStdlibOnlyLeaf"
        status: pass
    human_judgment: false
  - id: D3
    description: "Task 3 (checkpoint:human-verify, gate=blocking): whether Codex's own skill selector actually surfaces a skill written to $HOME/.agents/skills (RESEARCH assumption A1) is confirmed by a human observation on a real machine, never by an automated test (repo rule m45p2b4bp7)."
    requirement: "REQ-skills-native-format"
    verification: []
    human_judgment: true
    rationale: "Sean confirmed, on his own real Codex install, that the five curation skills appear in codex's skill list after `engram setup --apply`. RESEARCH assumption A1 is CONFIRMED, not falsified — the locked destination and the codex-native-plus-index routing stand. Frontmatter tolerance for metadata.engram-summary is confirmed for codex only (skills loaded, key not rejected; warning output not separately observed); Claude Code and opencode frontmatter tolerance remain unverified and are left as pending rows in 04-VALIDATION.md's Manual-Only Verifications table, per repo rule m45p2b4bp7 (never gate on, or fabricate, third-party behavior not actually observed)."

duration: 55min
completed: 2026-09-10
status: complete
---

# Phase 4 Plan 3: Codex and Opencode Skills Destinations, and a Confirmed A1 Summary

**Codex and opencode now each author their own `SkillTarget` in their own file — Codex under the human-approved `codex-native-plus-index` routing (native files at `$HOME/.agents/skills` plus an index spliced into `~/.codex/AGENTS.md`), opencode natively under its `XDG_CONFIG_HOME`-aware config root — and a live-machine human observation confirms RESEARCH assumption A1: Codex's own skill selector actually reads the chosen destination.**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-09-10T13:05:00Z
- **Completed:** 2026-09-10T14:00:00Z
- **Tasks:** 3 (1 checkpoint:decision, 1 auto/tdd, 1 checkpoint:human-verify)
- **Files modified/created:** 6

## Accomplishments

- Task 1 (checkpoint:decision) resolved this phase's one open routing question: Codex carries BOTH native skill files and the AGENTS.md index (`codex-native-plus-index`), giving ROADMAP success criterion 3 a live `--apply` write path in v1 and hedging RESEARCH assumption A1.
- `internal/setup/codex.go`: every auth mode (`oauth`, `none`, `oauth-client`, `bearer`) authors the identical `SkillTarget{Format: SkillFormatAgentsMD, Dir: $HOME/.agents/skills, IndexFile: $HOME/.codex/AGENTS.md}`, with a `HomeDir()` failure surfacing as a named error.
- `internal/setup/opencode.go`: every auth mode authors `SkillTarget{Format: SkillFormatNative, Dir: opencodeConfigRoot(env)/opencode/skills}`; `opencodeConfigRoot` is the ONLY environment-variable read in `internal/setup` (`XDG_CONFIG_HOME`, accepted only when non-empty and absolute).
- `internal/setup/codex_test.go` (new) and `internal/setup/opencode_test.go` (extended): exhaustive per-auth-mode target assertions, the `$CODEX_HOME/skills` exclusion check, home-unresolvable error cases, and `TestEveryRuntimeAuthorsAnExplicitSkillFormat` — a registry-driven structural guard against a future runtime shipping with a silently zero-valued skill target.
- Task 3 (checkpoint:human-verify, blocking): Sean confirmed on his real machine that the five curation skills appear in codex's skill list after `engram setup --apply` — **RESEARCH assumption A1 is CONFIRMED**, closing this phase's one MEDIUM-confidence open question. `04-VALIDATION.md`'s Manual-Only Verifications table is updated with this result.

## Task Commits

1. **Task 1: DECISION — which runtime carries the delimited AGENTS.md index** — no code; decision recorded in this SUMMARY (see key-decisions and coverage D1).
2. **Task 2: Author codex's and opencode's skills destinations, each in its own file** — `fb35e522` (feat)
3. **Task 3: VERIFY — the runtimes actually surface what engram wrote** — `334dea90` (docs; 04-VALIDATION.md results)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `internal/setup/codex.go` — `Plan()` now authors the `codex-native-plus-index` `SkillTarget` in every auth-mode branch.
- `internal/setup/opencode.go` — `Plan()` now authors opencode's native `SkillTarget`; new `opencodeConfigRoot` helper.
- `internal/setup/codex_test.go` (new) — `TestCodexSkillTarget`, `TestCodexSkillTargetIsNotCodexHome`, `TestCodexPlanFailsWhenHomeUnresolvable`, `TestEveryRuntimeAuthorsAnExplicitSkillFormat`.
- `internal/setup/opencode_test.go` — `TestOpenCodeSkillTarget` (4 `XDG_CONFIG_HOME` subtests), `TestOpenCodePlanFailsWhenHomeUnresolvable`.
- `cmd/engram/setup_test.go` — `TestSetupSkillsFailureReachesPartialExit`'s fixture scoped to claude-code's own destination (Rule 1 fix; see Deviations).
- `.planning/phases/04-skills-distribution/04-VALIDATION.md` — Manual-Only Verifications table filled in with Task 3's results (values only, no new columns, per repo rule `8dfdhfs5nn`).

## Decisions Made

See `key-decisions` in the frontmatter — most notably Task 1's `codex-native-plus-index` routing choice and its accepted cost (Codex sees the five skills via two routes if its loader also reads `$HOME/.agents/skills` directly, which Task 3 has now confirmed it does).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestSetupSkillsFailureReachesPartialExit`'s stale "codex has no skills facet" assumption**
- **Found during:** Task 2, first `go test ./cmd/engram/... -count=1` run after wiring codex
- **Issue:** This pre-existing test's fake `skillsEnv.WriteFile` failed universally, and its own doc comment stated codex was "not yet wired for skills this wave — simply registers successfully with no skills facet at all." This plan's own mandated change (wiring codex for skills) makes that premise false: with a universal write failure, codex's skills install now ALSO failed, so both runtime rows aggregated to `OutcomeFailed` and the test asserted exit code `exitPartial` (8) but got `9` (total failure).
- **Fix:** Scoped the fake `WriteFile` to fail only under a path containing `/.claude/skills/`; codex's distinct destination (`.agents/skills`, plus its `.codex/AGENTS.md` index) now writes through cleanly, restoring the test's original intent — one runtime's clean success paired with another's skills-only failure.
- **Files modified:** `cmd/engram/setup_test.go`
- **Verification:** `go test ./cmd/engram/... -count=1 -run TestSetupSkillsFailureReachesPartialExit` passes; full `go test ./internal/setup/... ./internal/skills/... ./cmd/engram/... -count=1` and `task` both green.
- **Committed in:** `fb35e522` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — a pre-existing test's assumption invalidated by this plan's own mandated change).
**Impact on plan:** Necessary for `task` to stay green; no scope creep — no behavior beyond what Task 2's action text specifies was added.

## Incident During Task 3 Verification

While setting up Task 3's human-verify checkpoint, I built the `engram` binary and ran the plan's own documented verification commands — `engram setup --url https://engram.example.com/mcp` (preview, safe) followed by `--apply` (this line item, NOT safe) — against the real, live machine rather than a fake `$HOME`. `engram setup --apply` always performs BOTH registration and skills, with no opt-out (D-12, locked in Phase 4's own `04-CONTEXT.md`). The placeholder URL overwrote the REAL, working MCP registrations for all three runtimes:

- **claude-code:** its real registration (URL + an `x-litellm-api-key` bearer header) had been captured moments earlier via a read-only preview, so it was restored byte-for-byte via `claude mcp remove` + `claude mcp add`. Sean re-authenticated afterward and confirmed it works.
- **opencode:** no pre-image had been captured. Sean re-registered it to match claude-code's real URL and header.
- **codex:** no pre-image had been captured either. A `~/.codex/config.toml.bak` from Sep 7 proved codex had NEVER carried an engram registration before this incident created one — so the correct remediation was removal, restoring codex's true pre-incident state. (Codex's `mcp add` also has no way to express the `x-litellm-api-key` literal-header form at all, so a like-for-like re-registration was never on the table for it regardless.)

The skills-side writes (all five `SKILL.md` files at all three destinations, and the `~/.codex/AGENTS.md` index splice) were unaffected by this — they are purely additive, matched no pre-existing content, and remain correctly in place, live-verified during this same session (byte counts, digest match, AGENTS.md's original 33 lines untouched, new block correctly delimited, idempotent re-apply confirmed `already-correct` with no duplicate block).

**Lesson, stated plainly for future verification of any setup-writing tool:** verifying a command that writes to `$HOME` must run against a FAKE `$HOME`, never the operator's real one — even when the write destinations (skills) are believed to be purely additive, the SAME `--apply` invocation is not scoped to only those destinations, and D-12's "no opt-out" design means every write this tool can make happens together on every invocation.

## Issues Encountered

The incident above is the only issue encountered this plan; see that section for the full account and remediation. No other issues.

## User Setup Required

None — no external service configuration required for the plan's own deliverable. (The MCP registration remediation above was a one-time, already-completed fix for a self-inflicted incident, not a standing user-setup requirement.)

## Next Phase Readiness

- `04-04-generic-and-summary` is next: `generic.go` remains deliberately un-wired for skills (its `Plan()` leaves `Skills` at the Go zero value), which is why `TestEveryRuntimeAuthorsAnExplicitSkillFormat` explicitly skips any `optInOnlyRuntime`. 04-04 wires `generic`'s skills payload and closes out the phase.
- `REQ-skills-native-format` and `REQ-skills-agents-md-fallback` are NOT yet marked complete in `REQUIREMENTS.md` (`gsd_run query requirements.ready-ids` confirmed `0/2` ready) — both are shared with `04-04` per the shared-ID gate, and will flip to `Complete` once 04-04's own `SUMMARY.md` exists.
- `04-VALIDATION.md`'s Manual-Only Verifications table still has one open half-row: Claude Code and opencode frontmatter-tolerance for `metadata.engram-summary` remain unverified (only codex was observed this round). Worth confirming before the phase seals, though nothing in this plan's own scope depends on it.
- RESEARCH assumption A1 is now closed with a positive, human-observed result — this phase's single named MEDIUM-confidence risk is resolved.

## Self-Check: PASSED

- All 5 key files (created/modified) verified present on disk with `[ -f ]`.
- Both commits (`fb35e522`, `334dea90`) verified present in `git log --oneline --all`.
- Re-ran `go build ./... && go test ./internal/setup/... ./internal/skills/... ./cmd/engram/... -count=1` — all green.
- Re-ran `go test ./internal/keylinks/... -count=1` — green.
- Re-ran `task` (lint + full suite) — green.
- `git diff --stat internal/setup/runtime.go` and `git diff --stat cmd/engram/testdata/catalog.golden` both print nothing (pinned files untouched).
- `rg -n 'CODEX_HOME' internal/setup --glob '!*_test.go' | rg -v '//' | wc -l` → `0`; `rg -n '"~' internal/setup | rg -v '//' | wc -l` → `0`; `rg -n 'Getenv' internal/setup/opencode.go | rg -v '//' | wc -l` → `1`; `rg -n 'Getenv' internal/setup/codex.go internal/setup/claudecode.go | rg -v '//' | wc -l` → `0`.
- `git status --short` in this repo shows only the files this plan intentionally touched — the real-machine verification commands (Task 3, and the incident above) never wrote anything into the repo itself.

---
*Phase: 04-skills-distribution*
*Completed: 2026-09-10*
