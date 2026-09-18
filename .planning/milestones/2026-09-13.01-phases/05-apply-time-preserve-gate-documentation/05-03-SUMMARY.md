---
phase: 05-apply-time-preserve-gate-documentation
plan: 03
subsystem: docs
tags: [docs-site, docs-gate, install, plugin, man-pages, plugin-first, starlight]

# Dependency graph
requires:
  - phase: 05-apply-time-preserve-gate-documentation
    plan: 01
    provides: "the shipped apply-time preserve gate and OAuth re-login consequence this plan's guides reference by name (not by code dependency — this plan's content does not depend on 05-01's code, per its own objective)"
provides:
  - "docs-site/src/content/docs/guides/install.md: what the Homebrew cask installs (binary, bash/zsh/fish completions, one man page per command incl. `man engram-setup`), a truthful `Unreleased as of v0.16.1` notice, the archive-route no-hooks caveat, and a plugin-first pointer from the Next steps Agent Setup bullet"
  - "docs-site/src/content/docs/guides/plugin.md: the plugin-first install statement (`engram setup --apply` installs the plugin), the manual `claude plugin marketplace add`/`claude plugin install` commands kept as the by-hand route, an `Unreleased as of v0.16.1` notice, and a `preserved` remediation cross-link beside the standalone-fallback `claude mcp remove` block"
  - "cmd/engram/install_docs_test.go: installGuideRelPath, installGuideViolations (4 legs), TestInstallGuideDocumentsSetupV2, TestInstallGuideGateFiresOnInjectedViolation (5 subtests)"
  - "cmd/engram/plugin_docs_test.go: pluginGuideRelPath, pluginGuideViolations (4 legs), TestPluginGuideDocumentsPluginFirst, TestPluginGuideGateFiresOnInjectedViolation (5 subtests)"
affects: [05-04 (post-release closeout that replaces both unreleased notices with the observed release), 05-02 (agent-setup.md's own docs gate, unaffected by this plan's files)]

# Actuals (#2632)
actuals:
  tokens: 4955
  tasks: 2
  commits: 2
  plan_head_before: e9503712ffbe92dd1c21f6ec5310b99958affcfa

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "The docs-gate four-part shape (path constant -> pure violations func -> live-file test with skip-if-absent/fail-if-empty -> positive control with one clean case plus one case per violation class), replicated a third and fourth time from migrate_docs_test.go/agent_setup_docs_test.go, reusing the package-level `newline` var rather than redeclaring it"
    - "Version-agnostic unreleased-notice gating (`Unreleased as of v` substring match, not a pinned version string) so the post-release flip to an `available in` notice is a deliberate gate edit, not an accidental pass"

key-files:
  created:
    - cmd/engram/install_docs_test.go
    - cmd/engram/plugin_docs_test.go
  modified:
    - docs-site/src/content/docs/guides/install.md
    - docs-site/src/content/docs/guides/plugin.md

key-decisions:
  - "Split the cask-hooks sentence in install.md into three short paragraphs instead of one wrapped paragraph, because the docs-gate's leg 2 check requires `completions` and `man page` on the SAME literal source line (the test splits doc text on a raw newline) — Markdown's soft line-wrap would otherwise put the two words on adjacent-but-separate lines and fail the gate even though the rendered prose reads as one sentence"
  - "The Next steps Agent Setup bullet in install.md was written as a single unwrapped line rather than the file's usual ~80-column wrap, for the same same-line-substring reason (leg 3 needs `plugin` and `/guides/agent-setup/` together)"
  - "plugin_docs_test.go's `agent_setup_link_missing` positive-control fixture strips the `/guides/agent-setup/` substring from EVERY line (via strings.ReplaceAll), not just the deleted agent-setup-link line — the preserved-crosslink line independently names the same guide (via its results-section anchor), so removing only the dedicated link line left leg 1 accidentally still satisfied by leg 3's own cross-link"

requirements-completed: []  # REQ-docs-setup-v2 stays open per D-06/D-10 — see coverage/rationale below; the code-gated half for install.md/plugin.md is done, but the requirement itself is not checked off by this plan (sibling plans 05-02/05-04 also declare it and have not finished; post-release observation is still outstanding)

coverage:
  - id: D1
    description: "install.md truthfully describes what the Homebrew cask installs — binary, bash/zsh/fish completions, and one man page per command (`man engram-setup`, `man engram`) written into Homebrew's share/man/man1 — under an `Unreleased as of v0.16.1` notice, and points at plugin-first delivery via the agent-setup guide"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "cmd/engram/install_docs_test.go#TestInstallGuideDocumentsSetupV2"
        status: pass
      - kind: unit
        ref: "cmd/engram/install_docs_test.go#TestInstallGuideGateFiresOnInjectedViolation"
        status: pass
    human_judgment: false
  - id: D2
    description: "install.md's archive route states that an archive installation runs no hooks, so completions and man pages are not installed by hand, without documenting the hidden `engram man <dir>` verb"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "cmd/engram/install_docs_test.go#TestInstallGuideDocumentsSetupV2"
        status: pass
    human_judgment: true
    rationale: "The 'no hidden verb documented' property is proven by a zero-occurrence grep in the plan's acceptance criteria (rg -n -e 'engram man ' returns nothing), not by a dedicated go test assertion inside installGuideViolations — recorded as human_judgment so a reviewer confirms the grep result rather than trusting an implicit pass."
  - id: D3
    description: "plugin.md states that engram setup --apply installs the plugin plugin-first (engram's own marketplace/plugin, added/updated/left-alone as appropriate) on a Claude Code whose plugin CLI works, keeping the manual claude plugin marketplace/install commands as the by-hand route, under an Unreleased as of v0.16.1 notice"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "cmd/engram/plugin_docs_test.go#TestPluginGuideDocumentsPluginFirst"
        status: pass
      - kind: unit
        ref: "cmd/engram/plugin_docs_test.go#TestPluginGuideGateFiresOnInjectedViolation"
        status: pass
    human_judgment: false
  - id: D4
    description: "plugin.md cross-links a preserved setup result to the agent-setup guide's results section (#read-results-and-repeat-safely), placed directly after the standalone-fallback claude mcp remove engram --scope user block, so an operator who hits that block first discovers why setup declined before copying the destructive command"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "cmd/engram/plugin_docs_test.go#TestPluginGuideDocumentsPluginFirst"
        status: pass
    human_judgment: false

# Metrics
duration: ~25min
completed: 2026-09-16
status: complete
---

# Phase 5 Plan 3: install.md and plugin.md Documentation Summary

**`install.md` now states what the Homebrew cask installs (binary, completions, one man page per command including `man engram-setup`) and `plugin.md` now states that `engram setup --apply` installs the plugin plugin-first and cross-links a `preserved` result to the setup guide's remediation — both gated by new four-leg docs tests with positive controls, both under a truthful `Unreleased as of v0.16.1` notice.**

## Performance

- **Duration:** ~25 min (estimated — `PLAN_START_TIME` was not captured at the very first tool call this session; based on git commit timestamps and the scope of research reading before the first commit)
- **Started:** ~2026-09-16T21:43:00Z (estimated)
- **Completed:** 2026-09-16T22:07:41Z
- **Tasks:** 2
- **Files modified:** 4 (2 docs, 2 new Go test files)

## Accomplishments

- `install.md` gains a truthful `Unreleased as of v0.16.1` notice beside the existing `Setup requires v0.16.0 or later` aside; the Homebrew section now states the cask's hooks generate completions AND write one man page per command into `share/man/man1`, names `man engram-setup`/`man engram` as the resulting commands, and states `brew uninstall` removes them; the release-archive route states no hooks run there (no completions, no man pages by hand — `engram <command> --help` covers the same content); the Next steps Agent Setup bullet now names plugin-first delivery. The hidden `engram man <dir>` verb is never documented as a user command (`rg -n -e 'engram man '` returns nothing).
- `plugin.md` gains a plugin-first install statement above the manual `claude plugin marketplace add`/`claude plugin install` commands (which remain as the by-hand route), an `Unreleased as of v0.16.1` notice beside the existing delegation-availability aside, and a one-sentence `preserved` remediation cross-link to the agent-setup guide's results section, placed directly after the standalone-fallback `claude mcp remove engram --scope user` block.
- Two new docs-gate test files, `cmd/engram/install_docs_test.go` and `cmd/engram/plugin_docs_test.go`, replicate the exact four-part shape of `migrate_docs_test.go`/`agent_setup_docs_test.go` (relative-path constant, pure `<guide>Violations(doc string) []error`, a live-file test that skips on a trimmed checkout and fails on an empty file, and a positive control with a `clean` case plus one injected-violation case per leg) — a later edit that deletes any of the eight required sentences across both guides now fails `go test ./cmd/engram`.
- Both guides carry a version-agnostic `Unreleased as of v` gate leg (not pinned to `v0.16.1`), so 05-04's post-release flip to an "available in" notice is a deliberate gate edit rather than a silent pass.

## Task Commits

Each task was committed atomically:

1. **Task 1: `install.md` cask contents + plugin-first pointer + `install_docs_test.go`** - `4920d821` (docs)
2. **Task 2: `plugin.md` plugin-first install + `preserved` cross-link + `plugin_docs_test.go`** - `c67aa825` (docs)

**Plan metadata:** committed alongside this SUMMARY.

_Note: this plan carried `tdd="true"` on both tasks — see the RED evidence for each below._

**Task 1 RED evidence:** `go test ./cmd/engram -run '^(TestInstallGuideDocumentsSetupV2|TestInstallGuideGateFiresOnInjectedViolation)$' -count=1 -v` against the unedited live guide failed `TestInstallGuideDocumentsSetupV2` on all four legs (`man engram-setup` absent, no completions+man-page line, no plugin pointer, no unreleased notice); `TestInstallGuideGateFiresOnInjectedViolation`'s 5 subtests passed from the start (the positive control is fixture-only, never touches the live file). After the guide edits, both tests went GREEN.

**Task 2 RED evidence:** `go test ./cmd/engram -run '^(TestPluginGuideDocumentsPluginFirst|TestPluginGuideGateFiresOnInjectedViolation)$' -count=1 -v` against the unedited live guide failed `TestPluginGuideDocumentsPluginFirst` on legs 2-4 (plugin-first statement, `preserved` cross-link, unreleased notice) while leg 1 (agent-setup cross-link) already held — exactly as the plan's `<behavior>` block predicted, recorded honestly rather than silently treated as a pre-existing pass. `TestPluginGuideGateFiresOnInjectedViolation`'s positive control required one fixup during RED (see Deviations) before all 5 subtests passed. After the guide edits, both tests went GREEN.

## Files Created/Modified

- `docs-site/src/content/docs/guides/install.md` — unreleased notice, cask-contents sentence (completions + man pages), man-page usage sentence, archive-route no-hooks caveat, plugin-first Next-steps pointer
- `docs-site/src/content/docs/guides/plugin.md` — plugin-first install statement, unreleased notice, `preserved` remediation cross-link
- `cmd/engram/install_docs_test.go` — new file: `installGuideRelPath`, `installGuideViolations`, `TestInstallGuideDocumentsSetupV2`, `TestInstallGuideGateFiresOnInjectedViolation`
- `cmd/engram/plugin_docs_test.go` — new file: `pluginGuideRelPath`, `pluginGuideViolations`, `TestPluginGuideDocumentsPluginFirst`, `TestPluginGuideGateFiresOnInjectedViolation`

## Decisions Made

- **Broke the cask-hooks sentence into three short paragraphs** in `install.md` rather than keeping one Markdown-wrapped paragraph: the docs-gate's leg 2 (`completions` and `man page` on the same line) splits the document on a raw literal newline, so a soft line-wrap that puts the two words on adjacent source lines fails the gate even though the rendered HTML reads as one sentence. Same reasoning applied to the Next-steps Agent Setup bullet (leg 3: `plugin` and `/guides/agent-setup/` together).
- **`newline` is reused from `agent_setup_docs_test.go`, never redeclared** in either new file — both are `package main` in `cmd/engram`, so the existing package-level var is directly visible.
- **Followed `migrate_docs_test.go`'s simpler shape**, not `agent_setup_docs_test.go`'s more elaborate 7-leg version, per `05-PATTERNS.md`'s explicit recommendation for a fresh guide gate with a small (4-leg) violation set.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug I introduced] `plugin_docs_test.go`'s positive-control fixture for the `agent_setup_link_missing` case did not actually isolate leg 1**
- **Found during:** Task 2's RED-phase run of `TestPluginGuideGateFiresOnInjectedViolation`
- **Issue:** The clean fixture's `preservedCrosslinkLine` independently names `/guides/agent-setup/` (via its results-section anchor, required for leg 3), so removing only the dedicated `agentSetupLinkLine` left the substring still present elsewhere in the fixture — leg 1's check (`strings.Contains(line, "/guides/agent-setup/")`) still passed, and the subtest failed with `violations=[] (count 0), want expectViolation=true`.
- **Fix:** Changed the `agent_setup_link_missing` case to `strings.ReplaceAll` the constructed fixture, stripping every occurrence of `/guides/agent-setup/` rather than only omitting one line — isolates leg 1's check as intended (the case now trips both legs 1 and 3, which is fine since the assertion is boolean `len(violations) > 0`, not leg-specific).
- **Files modified:** `cmd/engram/plugin_docs_test.go`
- **Verification:** `go test ./cmd/engram -run '^TestPluginGuideGateFiresOnInjectedViolation$' -count=1 -v` — all 5 subtests pass, including `agent_setup_link_missing`.
- **Committed in:** `c67aa825` (Task 2 commit — caught and fixed before the commit, not a separate follow-up)

---

**Total deviations:** 1 auto-fixed (Rule 1 — a test-fixture bug caught during the plan's own RED-phase run, before any commit). **Impact:** none on shipped guide content; the fix only corrected the positive control's own test isolation.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `cmd/engram/install_docs_test.go` and `cmd/engram/plugin_docs_test.go` are stable four-leg gates, following the same shape `agent_setup_docs_test.go` (05-02's territory) already uses — no coordination needed between the two docs plans' files.
- Both guides' `Unreleased as of v0.16.1` notices are version-agnostic gate legs (`Unreleased as of v`), ready for 05-04's post-release flip to an observed-version "available in" notice without any gate-shape change.
- `REQ-docs-setup-v2` remains `[ ]` — this plan closes the code-gated half for `install.md` and `plugin.md` only. It is also declared by 05-02 (`agent-setup.md`'s own legs) and 05-04 (the post-release observation); the shared-ID gate (`requirements.ready-ids`) correctly held off marking it complete since those sibling plans have not produced a SUMMARY yet. Per D-06, the checkbox flips only after a `05-RELEASE-<ver>.md` observation is recorded.
- Full `task` gate (lint, all Go tests including `internal/store`'s live-Docker suite, license check, setup-drift check) is green on the final committed tree; `task license:check` and `go test ./internal/keylinks/ -count=1` both pass standalone as the plan's `<verification>` block requires.
- No blockers.

## Self-Check: PASSED

- Both created files confirmed present on disk: `cmd/engram/install_docs_test.go`, `cmd/engram/plugin_docs_test.go`.
- Both modified guides confirmed present and containing the expected content (`man engram-setup`, `Unreleased as of v0.16.1` in both files, the `preserved` cross-link in `plugin.md`).
- Commits `4920d821` and `c67aa825` confirmed present via `git log --oneline`.
- Plan-level `<verification>` re-run: `go test ./cmd/engram -run '^(TestInstallGuideDocumentsSetupV2|TestInstallGuideGateFiresOnInjectedViolation|TestPluginGuideDocumentsPluginFirst|TestPluginGuideGateFiresOnInjectedViolation)$' -count=1 -v` shows 0 `--- FAIL`, including both `clean` subtests.
- `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build && rumdl check docs-site/src/content/docs/guides/install.md docs-site/src/content/docs/guides/plugin.md && task && task license:check && go test ./internal/keylinks/ -count=1` all exit 0 on the final committed tree.
- `git diff --stat e9503712..HEAD` touches exactly the 4 files declared in `files_modified`: no scope creep.
- `git diff --exit-code -- go.mod go.sum docs-site/package.json docs-site/pnpm-lock.yaml` clean (no dependency changes).
- `rg -n -e 'engram man ' docs-site/src/content/docs/guides/install.md` prints nothing (hidden verb never documented).

---
*Phase: 05-apply-time-preserve-gate-documentation*
*Completed: 2026-09-16*
