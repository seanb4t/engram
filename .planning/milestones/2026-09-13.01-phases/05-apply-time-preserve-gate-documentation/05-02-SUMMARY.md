---
phase: 05-apply-time-preserve-gate-documentation
plan: 02
subsystem: cli-docs
tags: [cli-help, docs-gate, agent-setup, oauth, apply-gate, cobra, golden-files]

# Dependency graph
requires:
  - phase: 05-apply-time-preserve-gate-documentation
    plan: 01
    provides: "the shipped apply-time preserve gate (classifyProbe/renderClassification), Observation.RewriteConsequence/ManualRemediation, claudeCodeManualRemediation/claudeCodeOAuthReLoginNote, codexManualRemediation — this plan documents that exact shipped behavior, never a re-derivation"
provides:
  - "cmd/engram/setup.go: setupLongDescription's drift-comparison paragraph extended with the apply-gate/remediation/re-login sentences (D-01/D-04/D-05); help.golden regenerated to match; TestSetupHelpStatesApplyGate pins both"
  - "cmd/engram/setup_test.go: TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape gains a preview/apply mode loop proving the observed literal never crosses the process boundary under --apply in either output lane, and the apply-mode row's Reason carries the D-05 remediation command"
  - "docs-site/src/content/docs/guides/agent-setup.md: truthful Unreleased as of v0.16.1 notice; apply gate paragraph; corrected already-correct row (pre-write guarantee, replacing the stale 'does not guarantee that no write' claim); corrected preserved row (no-registration-command/untouched plus both runtimes' manual remediation); OAuth re-login consequence in the OAuth section; plugin-first delivery in Registration and curation skills; corrected repeat-safety paragraph"
  - "cmd/engram/agent_setup_docs_test.go: agentSetupGuideStaleClaimAnchors (zero-occurrence gate), alreadyCorrectRowPattern, legs 8-13 in agentSetupGuideDriftViolations, nine new positive-control cases in TestAgentSetupGuideDriftGateFiresOnInjectedViolation"
affects: [05-04 (post-release closeout replaces the Unreleased notice and records the 05-RELEASE observation that finally checks off REQ-docs-setup-v2)]

# Actuals (#2632)
actuals:
  tokens: 9493
  tasks: 2
  commits: 2
  plan_head_before: e966db949c0a2b5ceb049408b57ab0932c5d8812

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "The docs-gate stale-claim-anchor idiom (migrateGuideStaleClaimAnchors, migrate_docs_test.go) reused a second time in agent_setup_docs_test.go: a claim a code change made false is gated on ZERO remaining occurrences, never a conversion count, so a regression that re-introduces the stale sentence anywhere in the file is caught"
    - "A row-pattern regex (alreadyCorrectRowPattern) added alongside the existing preservedRowPattern/wouldWriteRowPattern siblings, same bracket-class-only construction, so a table row's own text (not merely the surrounding prose) carries the substrings a gate leg checks"
    - "withoutLine/withReplacedLine helpers over a single ordered []string of clean-fixture lines (agent_setup_docs_test.go) replaced the prior plan's flat strings.Join(...) call sites per case, so a 7-line clean fixture with 17 cases stays legible instead of hand-duplicating the join at every case"

key-files:
  created: []
  modified:
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/agent_setup_docs_test.go
    - docs-site/src/content/docs/guides/agent-setup.md

key-decisions:
  - "The apply-gate paragraph landed as a NEW paragraph appended to the existing drift-comparison paragraph in setupLongDescription, rather than rewriting the existing paragraph in place — keeps the (already help-golden-pinned) already-correct/would-write/preserved paragraph byte-stable and TestSetupHelpNamesDriftOutcomes green with no edits, while the new paragraph is independently pinned by TestSetupHelpStatesApplyGate"
  - "TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape's new apply-mode subtests script the plugin lane already-correct (claudeListCurrentJSON/claudeMarketplacePresentText via fakePluginRun, mirroring TestSetupApplyPreservedRuntimeSkipsRegistrationWrite's plugin-current fixture) rather than leaving plugin probes unscripted: an unscripted apply-mode plugin lane falls back to a first-run native skills WRITE on the fresh fake environment, which outranks Registration=preserved in aggregate.go's wrote > preserved precedence and masks the very property under test (discovered live: the first RED-then-GREEN pass without this fixture reported Outcome=wrote, not preserved)"
  - "The agent-setup.md apply-gate/preserved/already-correct/OAuth/plugin-first sentences that a gate leg checks are deliberately written as single, unwrapped-at-the-checked-substring source lines (never split by Markdown's normal ~80-column wrap at the exact word boundary a leg's strings.Contains check needs) — the same lesson 05-03's install.md/plugin.md hit for their own docs-gate legs, applied here before the RED run rather than discovered by it"
  - "opencode_claimed_compared's positive-control case now strips 'not compared' via strings.ReplaceAll across the WHOLE fixture, not just the dedicated opencodeLine: the corrected already-correct row (leg 9) independently states 'opencode is not compared' as part of its own pre-write guarantee, so omitting only the dedicated line left leg 6 accidentally still satisfied by the second, independent mention — the identical isolation defect 05-03's plugin_docs_test.go recorded for its own agent_setup_link_missing case, caught the same way (during RED, before the fix landed)"

patterns-established: []

requirements-completed: [REQ-apply-preserve-gate, REQ-apply-rewrite-consequence]  # REQ-docs-setup-v2 stays open per D-06 — see coverage/rationale below; this plan closes the code-gated half for agent-setup.md, but the requirement itself is not checked off here. It is also declared by 05-04 (the post-release observation), which has not produced a SUMMARY yet — the shared-ID gate (requirements.ready-ids) correctly holds it. REQ-apply-preserve-gate/REQ-apply-rewrite-consequence were shipped in code by 05-01 but this sibling plan ALSO declares them (the "only complete when an operator can learn it by reading" clause in this plan's own objective) — both declaring plans (05-01, 05-02) have now finished, so the shared-ID gate marks them complete as part of this plan's own state update.

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "engram setup --help (the Long text, pinned by help.golden) states that --apply makes the same comparison before writing, that an already-correct or preserved row runs no registration command (Claude Code's mcp remove included), that only a would-write row is written and then read back, that a preserved row's reason names the manual step in the runtime's own tool, and that a Claude Code rewrite of a registration carrying no Authorization header states the operator will need to log in again — Short and catalog.golden are byte-unchanged"
    requirement: "REQ-apply-rewrite-consequence"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpStatesApplyGate"
        status: pass
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestHelpGolden"
        status: pass
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestCatalogGolden"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram setup --apply --runtime claude-code against the observed x-litellm-api-key gateway shape leaks the literal to neither stdout nor stderr in either output lane and reports outcome: preserved in json, with the row's reason naming the D-05 manual-remediation command — the Phase 4 process-boundary redaction proof now covers the apply lane"
    requirement: "REQ-apply-preserve-gate"
    verification:
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape/apply"
        status: pass
    human_judgment: false
  - id: D3
    description: "docs-site/src/content/docs/guides/agent-setup.md documents the apply gate, the corrected already-correct/preserved row semantics, the preserved row's manual remediation for both runtimes, the OAuth re-login consequence, and plugin-first skill delivery per runtime, under an Unreleased as of v0.16.1 notice"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "cmd/engram/agent_setup_docs_test.go#TestAgentSetupGuideDocumentsDrift"
        status: pass
      - kind: unit
        ref: "cmd/engram/agent_setup_docs_test.go#TestAgentSetupGuideDriftGateFiresOnInjectedViolation"
        status: pass
    human_judgment: false
  - id: D4
    description: "agentSetupGuideDriftViolations gains one leg per new claim plus a zero-occurrence leg over the two stale anchors, with a positive control per leg (nine new cases, clean fixture extended) — the gate is a deterministic pure function over guide text, proven by TestAgentSetupGuideDriftGateFiresOnInjectedViolation's clean case reporting zero violations both before and after this plan's guide edits"
    requirement: "REQ-docs-setup-v2"
    verification:
      - kind: unit
        ref: "cmd/engram/agent_setup_docs_test.go#TestAgentSetupGuideDriftGateFiresOnInjectedViolation/clean"
        status: pass
    human_judgment: false

# Metrics
duration: ~26min
completed: 2026-09-16
status: complete
---

# Phase 5 Plan 2: Apply-Gate Help Text and Agent Setup Documentation Summary

**`engram setup --help` and `guides/agent-setup.md` now teach the apply-time preserve gate, the OAuth re-login consequence, and plugin-first delivery by reading — both gated by new/extended tests, with the process-boundary literal-leak proof extended to cover `--apply`.**

## Performance

- **Duration:** ~26 min (estimated — `PLAN_START_TIME` was not captured at the very first tool call this session; based on git commit timestamps against 05-03's final commit)
- **Started:** ~2026-09-16T22:10:00Z (estimated)
- **Completed:** 2026-09-16T22:31:22Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- `cmd/engram/setup.go`'s `setupLongDescription` gains a new paragraph in the drift-comparison section of `engram setup --help`: `--apply` makes the same comparison before writing, an already-correct or preserved row runs no registration command (Claude Code's tolerant `mcp remove` included), a preserved row's reason names the exact manual step in the runtime's own tool, and a Claude Code rewrite of a registration observed with no `Authorization` header states — in preview and apply alike — that the operator will need to log in again. `help.golden` regenerated via the sanctioned `-update -count=1` invocation; `catalog.golden` confirmed byte-unchanged.
- `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape` gains a `preview`/`apply` mode loop: the observed literal (`sk-DO-NOT-COMMIT-literal-test-abc123`) never crosses the process boundary under `--apply` in either the `json` or `text` output lane, and the apply-mode row's `Reason` carries the D-05 manual-remediation command — extending the Phase 4 redaction proof at the CLI process boundary to the apply lane 05-01 shipped.
- `docs-site/src/content/docs/guides/agent-setup.md` is brought current per D-07's reader-intent split: a truthful `Unreleased as of v0.16.1` notice; an apply-gate paragraph under "Preview, review, then apply"; a corrected `already-correct` results-table row stating the pre-write guarantee (replacing the stale "does not guarantee that no write commands ran" sentence); a corrected `preserved` row naming both runtimes' manual remediation (`claude mcp remove engram --scope user`; Codex's `[mcp_servers.engram]` table) while keeping the pre-existing whole-entry/never-merge/never-shown clauses; an OAuth re-login consequence paragraph in the OAuth section; a plugin-first rewrite of "Registration and curation skills" naming `seanb4t/engram`/`engram@engram`; and a corrected repeat-safety paragraph (removing the stale "but it may perform writes again" claim).
- `cmd/engram/agent_setup_docs_test.go`'s `agentSetupGuideDriftViolations` gains `agentSetupGuideStaleClaimAnchors` (a zero-occurrence gate over both stale sentences, the `migrateGuideStaleClaimAnchors` idiom), `alreadyCorrectRowPattern`, and legs 8-13 (stale anchors; the already-correct row's pre-write guarantee; the preserved row's no-registration-command/untouched claim; the preserved row's dual-runtime remediation; the OAuth re-login consequence; plugin-first delivery). `TestAgentSetupGuideDriftGateFiresOnInjectedViolation` gains nine new positive-control cases (17 total, `clean` included) proving the gate discriminates every new claim without false-positiving on the pre-existing seven legs.

## Task Commits

Each task was committed atomically:

1. **Task 1: `--help` states the apply gate, manual remediation, and re-login consequence; `help.golden` regenerated; the literal-leak proof runs `--apply`** - `bd924bfc` (feat)
2. **Task 2: `agent-setup.md` brought current under a truthful unreleased notice, with six new gate legs and their positive controls** - `d81f6928` (docs)

**Plan metadata:** committed alongside this SUMMARY.

_Note: this plan carried `tdd="true"` on both tasks._

**Task 1 RED evidence:** `go test ./cmd/engram -run '^TestSetupHelpStatesApplyGate$' -count=1 -v` against the unedited `setup.go`/`help.golden` failed on `"log in again"` — `setupCmd.Long` had none of the five new vocabulary items yet. `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape/apply/json` was GREEN on the very first run against 05-01's shipped tree once its supporting `pluginCurrentScript` fixture was in place (recorded honestly as pinning already-shipped behavior, not a faked RED, per the plan's own `<behavior>` instruction) — but getting there required one genuine debugging pass: the FIRST version of this subtest (no plugin-lane script) reported `Outcome = "wrote"`, not `"preserved"`, because a fresh fake environment's unscripted plugin probes read as unavailable, causing a first-run native skills WRITE that outranks `preserved` in `aggregate.go`'s `wrote > preserved` precedence. Adding the `pluginCurrentScript`/`withFakeSetupVersion` fixture (mirroring 05-01's `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite`) fixed it; documented in Deviations below since it was a genuine implementation correction, not merely narration.

**Task 2 RED evidence:** `go test ./cmd/engram -run '^(TestAgentSetupGuideDocumentsDrift|TestAgentSetupGuideDriftGateFiresOnInjectedViolation)$' -count=1 -v` against the unedited guide failed `TestAgentSetupGuideDocumentsDrift` with exactly seven errors: both stale-anchor occurrences (leg 8), the missing already-correct pre-write guarantee (leg 9), the preserved row missing `no registration command`/`untouched` (leg 10) and the dual-runtime remediation (leg 11), the missing OAuth line (leg 12), and the missing plugin-first line (leg 13) — while the pre-existing seven legs stayed green throughout, confirming no regression. `TestAgentSetupGuideDriftGateFiresOnInjectedViolation`'s positive control failed on its own `opencode_claimed_compared` case during this same RED pass (see Deviations) before all 17 cases passed. After the guide edits, both tests went GREEN.

## Files Created/Modified

- `cmd/engram/setup.go` — `setupLongDescription`'s drift-comparison paragraph extended with the apply-gate/remediation/re-login sentences; doc comment gains a "Phase 5 (Apply-Time Preserve Gate)" note citing D-01/D-04/D-05
- `cmd/engram/setup_test.go` — `TestSetupHelpStatesApplyGate` (new); `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape` gains a `preview`/`apply` mode loop with a `pluginCurrentScript` fixture
- `cmd/engram/testdata/help.golden` — regenerated `engram setup` section carrying the new paragraph
- `cmd/engram/agent_setup_docs_test.go` — `agentSetupGuideStaleClaimAnchors`, `alreadyCorrectRowPattern`, legs 8-13, nine new positive-control cases, updated doc comments
- `docs-site/src/content/docs/guides/agent-setup.md` — unreleased notice, apply-gate paragraph, corrected already-correct/preserved rows, OAuth re-login paragraph, plugin-first rewrite, corrected repeat-safety paragraph, `notes` named in the JSON field sentence

## Decisions Made

- The apply-gate help paragraph is a NEW paragraph appended after the existing drift-comparison paragraph, not an in-place rewrite — keeps `TestSetupHelpNamesDriftOutcomes` (04-04's pin) byte-stable and lets `TestSetupHelpStatesApplyGate` pin the addition independently.
- `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape`'s apply-mode subtests script the plugin lane already-correct (`fakePluginRun` + `claudeListCurrentJSON`/`claudeMarketplacePresentText`, matching 05-01's own fixture) so a first-run native skills write never masks the preserved registration facet in the aggregate `Outcome` — see Deviations.
- Every new guide sentence a docs-gate leg checks is written as a single physical source line at the checked substring (never wrapped mid-phrase) — applied proactively here, the same lesson 05-03 recorded reactively for its own two guides.
- `opencode_claimed_compared`'s positive-control case strips `"not compared"` across the WHOLE fixture (`strings.ReplaceAll`), not just the dedicated `opencodeLine`, because the corrected already-correct row independently states the same fact as part of its own pre-write guarantee.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug I introduced] `TestSetupJSONNeverLeaksProbeLiteral/claude-code-observed-shape`'s first apply-mode draft misclassified Outcome as `wrote`, not `preserved`**
- **Found during:** Task 1, writing the apply-mode subtest before the `<interfaces>`-specified fixture was in place
- **Issue:** The subtest's first draft reused the existing preview-mode harness unchanged (no plugin-lane script) for the new apply mode. Under `--apply`, an unscripted plugin probe reads as unavailable, causing a genuine first-run native skills WRITE on the fresh fake environment/store. `aggregate.go`'s documented `wrote > preserved` precedence then surfaces the row's aggregate `Outcome` as `wrote`, masking the preserved registration classification the subtest exists to prove — confirmed live via a temporary `t.Logf` dump of the row before removing it.
- **Fix:** Added a `pluginCurrentScript` (`claudeListCurrentJSON`/`claudeMarketplacePresentText` via `fakePluginRun`) plus `withFakeSetupVersion(t, "0.16.1")`, mirroring `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite`'s `plugin-current` fixture from 05-01 — keeps the plugin lane already-correct so no native skills write occurs, and the aggregate `Outcome` resolves to `preserved` as intended.
- **Files modified:** `cmd/engram/setup_test.go`
- **Verification:** `go test ./cmd/engram -run '^TestSetupJSONNeverLeaksProbeLiteral$' -count=1 -v` — all subtests, including `claude-code-observed-shape/apply/{json,text}`, pass.
- **Committed in:** `bd924bfc` (Task 1 commit — caught and fixed before the commit, not a separate follow-up)

**2. [Rule 1 - Bug I introduced] `agent_setup_docs_test.go`'s `opencode_claimed_compared` positive-control case did not isolate leg 6**
- **Found during:** Task 2's RED-phase run of `TestAgentSetupGuideDriftGateFiresOnInjectedViolation`, immediately after adding the corrected `already-correct` row line
- **Issue:** The corrected `alreadyCorrectRowLine` states "opencode is not compared" as part of its own pre-write guarantee (leg 9's own requirement), so omitting only the dedicated `opencodeLine` left leg 6's check (`strings.Contains(line, "opencode") && strings.Contains(line, "not compared")`) still satisfied by the second, independent mention — the case failed with `violations=[] (count 0), want expectViolation=true`. This is the identical isolation defect 05-03's `plugin_docs_test.go` recorded for its own `agent_setup_link_missing` case.
- **Fix:** Changed the case to `strings.ReplaceAll(cleanFixture, "not compared", "compared successfully")`, stripping every occurrence rather than omitting one line — isolates leg 6 as intended.
- **Files modified:** `cmd/engram/agent_setup_docs_test.go`
- **Verification:** `go test ./cmd/engram -run '^TestAgentSetupGuideDriftGateFiresOnInjectedViolation$' -count=1 -v` — all 17 subtests pass, including `opencode_claimed_compared`.
- **Committed in:** `d81f6928` (Task 2 commit — caught and fixed before the commit, not a separate follow-up)

**3. [Rule 3 - Blocking] `golangci-lint`'s staticcheck flagged `strings.Replace(..., -1)` as `QF1004`**
- **Found during:** the full `task` gate run at the end of Task 2
- **Issue:** `preserved_omits_untouched`'s case used `strings.Replace(preservedRowLine, "untouched", "left alone", -1)` (needed because "untouched" appears twice in the corrected row); `golangci-lint`'s staticcheck (`QF1004`) flags this idiom, preferring `strings.ReplaceAll`.
- **Fix:** Changed to `strings.ReplaceAll(preservedRowLine, "untouched", "left alone")` — identical behavior, lint-clean.
- **Files modified:** `cmd/engram/agent_setup_docs_test.go`
- **Verification:** `task` (full lint + test gate) passes clean.
- **Committed in:** `d81f6928` (Task 2 commit — caught and fixed before the commit, not a separate follow-up)

---

**Total deviations:** 3 auto-fixed (2 Rule 1 — bugs introduced by this plan's own test-fixture drafts, caught during each task's own RED phase before any commit; 1 Rule 3 — a lint-blocking idiom preference, fixed before the final commit). **Impact:** none on shipped guide or help-text content; all three were test-authoring corrections caught and fixed within the same task's commit, never a separate follow-up.

## Issues Encountered

None beyond the three auto-fixed deviations above, all caught and resolved before their respective task commits.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `engram setup --help` and `guides/agent-setup.md` are both correct-by-reading for the apply gate, the manual remediation, the OAuth re-login consequence, and plugin-first delivery — no forward references to an unshipped Phase 5 behavior remain in either.
- `REQ-apply-preserve-gate` and `REQ-apply-rewrite-consequence` are now closeable: both declaring plans (05-01, 05-02) have finished, so this plan's own `requirements.ready-ids`-gated `state.update_requirements` step marks them complete.
- `REQ-docs-setup-v2` remains open per D-06: this plan closes the `agent-setup.md` code-gated half (joining 05-03's `install.md`/`plugin.md` halves); the requirement itself stays unchecked until 05-04's `05-RELEASE-<ver>.md` post-release observation is recorded.
- Full `task` gate (lint including staticcheck, all Go tests including `internal/store`'s live-Docker suite, license check, setup-drift check), `task license:check`, `go test ./internal/keylinks/ -count=1`, `go run ./internal/surfacesgen --check-setup`, and `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` are all green on the final committed tree.
- No blockers. 05-04 (post-release closeout) can proceed once a qualifying release is cut.

## Self-Check: PASSED

- Both modified test files, the modified `setup.go`, the regenerated `help.golden`, and the modified guide are all confirmed present on disk with the expected content (`before writing`, `log in again`, `no registration command`, `runtime's own tool` in `setup.go`; `Unreleased as of v0.16.1`, `claude mcp remove engram --scope user`, `[mcp_servers.engram]`, `seanb4t/engram`/`engram@engram` in the guide).
- Commits `bd924bfc` and `d81f6928` confirmed present via `git log --oneline`.
- Plan-level `<verification>` block re-run: `go test ./cmd/engram -run '^(TestSetupHelpStatesApplyGate|TestHelpGolden|TestCatalogGolden|TestSetupJSONNeverLeaksProbeLiteral|TestAgentSetupGuideDocumentsDrift|TestAgentSetupGuideDriftGateFiresOnInjectedViolation)$' -count=1 -v` shows every named subtest passing, 0 `--- FAIL`.
- `task && task license:check && git diff --exit-code -- go.mod go.sum docs-site/package.json docs-site/pnpm-lock.yaml && go test ./internal/keylinks/ -count=1 && go run ./internal/surfacesgen --check-setup && pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` all exit 0 on the final committed tree.
- `git diff --stat e966db94..HEAD` touches exactly the 5 files declared in `files_modified`: no scope creep.
- `git status --porcelain -- skill/ internal/setupgen/ cmd/engram/testdata/catalog.golden release-please-config.json` is empty — no generated surface drifted as a side effect.
- `rg -q -F -e 'does not guarantee that no write' docs-site/src/content/docs/guides/agent-setup.md` and the `'but it may perform writes again'` equivalent both exit 1 (absent) — the two stale claims are gone.

---
*Phase: 05-apply-time-preserve-gate-documentation*
*Completed: 2026-09-16*
