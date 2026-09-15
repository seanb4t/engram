---
phase: 04-drift-detection-read-only
plan: 03
subsystem: docs
tags: [docs-gate, agent-setup, drift-detection, migrate_docs_test-shape, go]

# Dependency graph
requires:
  - phase: 04-drift-detection-read-only
    provides: "plan 04-01's OutcomePreserved, Facet enum, and Result.Facets/Result.Drift field names — the vocabulary this plan documents verbatim"
provides:
  - "docs-site/src/content/docs/guides/agent-setup.md: a `preserved` results-table row (Codex whole-entry/never-merge/never-shown clauses), a rewritten `would-write` row naming `facets`/`drift`, a rewritten `already-correct` row stating preview's real comparison, an explicit opencode-not-compared sentence, and a `registered`/`facets`/`drift` JSON-field sentence"
  - "cmd/engram/agent_setup_docs_test.go: a migrate_docs_test.go-shaped docs gate (agentSetupGuideDriftViolations, seven legs) with a positive control, pinning all of the above against silent deletion"
affects: [04-04 (--output json/text rendering of Facets/Drift; the same results-table row this plan documents), 04-05 (Claude Code's own DriftRuntime scanner)]

# Actuals (#2632)
actuals:
  tokens: 3395
  tasks: 2
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "migrate_docs_test.go-shaped docs gate: a pure *Violations(doc string) []error function, one test against the live file (os.IsNotExist skip, empty-file fail), one positive-control test proving the same function fires on injected fixture violations including a load-bearing clean case"
    - "No-backslash regex discipline in a new test file: bracket-class-only patterns ([|][ ]*, [.][.] etc.) and a raw-string newline literal in place of the \"\\n\" escape sequence, verified by rg -F -e '\\' returning zero matches"

key-files:
  created:
    - cmd/engram/agent_setup_docs_test.go
  modified:
    - docs-site/src/content/docs/guides/agent-setup.md

key-decisions:
  - "The registered/facets/drift JSON-field sentence in the Scripts-and-JSON section is written as one long unwrapped physical line (not hand-wrapped at ~80 chars like surrounding prose) because the docs gate's leg 7 checks for all three tokens on a single physical line, and docs-site/ is excluded from rumdl's MD013 line-length rule entirely (also globally disabled), so no lint cost"
  - "Reworded a doc comment from a quoted \"clean\" case reference to an unquoted the clean case reference — the quoted form doubled the acceptance criterion's case-name occurrence count (9 instead of the plan's expected 8) by also matching inside the explanatory comment, not just the struct literal"

requirements-completed: [REQ-drift-preserved-outcome, REQ-drift-facet-naming, REQ-drift-observed-registration]

coverage:
  - id: D1
    description: "guides/agent-setup.md documents the preserved outcome (with the Codex whole-entry, never-merge, and never-shown clauses), the would-write row's facet naming, the already-correct row's real preview comparison, the opencode not-compared statement, and the registered/facets/drift JSON field names -- pinned by a docs gate with a positive control"
    requirement: "REQ-drift-preserved-outcome"
    verification:
      - kind: unit
        ref: "cmd/engram/agent_setup_docs_test.go#TestAgentSetupGuideDocumentsDrift"
        status: pass
      - kind: unit
        ref: "cmd/engram/agent_setup_docs_test.go#TestAgentSetupGuideDriftGateFiresOnInjectedViolation"
        status: pass
      - kind: other
        ref: "rg checks in Task 1's acceptance criteria against the live guide (preserved row content, would-write row content, opencode statement, JSON field sentence, byte-identical not-present/wrote/failed rows)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Whole-package gate: lint, license check, dependency diff, the key-links gate, and the docs gate all exit 0 with a clean tree for the files this plan touched"
    requirement: "REQ-drift-facet-naming"
    verification:
      - kind: other
        ref: "task lint; task license:check; git diff --exit-code -- go.mod go.sum; go test ./internal/keylinks/ -count=1; go test ./cmd/engram -run '^TestAgentSetupGuide' -count=1"
        status: pass
    human_judgment: false

# Metrics
duration: ~15min
completed: 2026-09-15
status: complete
---

# Phase 4 Plan 3: Drift Detection Documentation Summary

**`guides/agent-setup.md` gains a `preserved` results row, a real facet-naming `would-write`/`already-correct` pair, and an explicit opencode-not-compared statement, all pinned by a new `migrate_docs_test.go`-shaped docs gate with a positive control.**

## Performance

- **Duration:** ~15 min (estimated — PLAN_START_TIME was not captured at the very first tool call this session; 04-01's SUMMARY completed at 2026-09-15T18:35:25Z immediately prior)
- **Started:** ~2026-09-15T18:38:00Z (estimated)
- **Completed:** 2026-09-15T18:48:45Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- `docs-site/src/content/docs/guides/agent-setup.md`'s results table gains a `preserved` row stating the existing registration carries something setup did not author and cannot reproduce, that setup leaves it untouched and never shows an observed header value, and that Claude Code and Codex both replace the whole entry on write (no partial merge) so a later `--apply` either overwrites the entry or leaves it untouched and never merges into it (REQ-drift-preserved-outcome).
- The preview paragraph now states, in one sentence naming `opencode` and `not compared` on the same line, that opencode's registration is not compared because its `mcp list` prints a table setup does not parse (D-10).
- The `would-write` row now names `facets` (`url`, `auth-mode`, `header-name`, `header-value-ref`) and `drift`, with a vendor-neutral `x-gateway-api-key: observed <redacted>, would write ${GATEWAY_KEY}` example; the `already-correct` row now states preview performs a real comparison of URL, auth mode, and header names/references read through the runtime's own CLI (REQ-drift-facet-naming).
- The Scripts-and-JSON section names all three new JSON fields (`registered`, `facets`, `drift`) beside the existing `headers` sentence.
- `cmd/engram/agent_setup_docs_test.go`: `agentSetupGuideDriftViolations(doc string) []error` checks seven legs (missing `preserved` row; missing whole-entry/never-merge/never-shown clauses; missing/incomplete `would-write` row; opencode claimed compared; missing JSON field sentence). `TestAgentSetupGuideDocumentsDrift` runs it against the live guide (skip-on-`os.IsNotExist`, fail-on-empty); `TestAgentSetupGuideDriftGateFiresOnInjectedViolation` is the positive control (8 subtests, including a load-bearing `clean` case that must NOT fire).
- The existing `not-present`/`wrote`/`failed` rows and the exit-code paragraph are confirmed byte-identical to the pre-phase commit (`ff5a6f94`).

## Task Commits

Each task was committed atomically:

1. **Task 1: Docs gate RED against the live guide, then the `preserved` row, the opencode statement, and the facets/drift/registered sentences** - `0bbae108` (docs)
2. **Task 2: Plan gate — full `task`, license check, key-links gate, and the docs gate in the whole-package run** - verification-only, no commit (no formatter rewrote either file; `git status --porcelain -- docs-site/ cmd/engram/` is clean)

**Plan metadata:** committed alongside this SUMMARY.

_Note: this is not a `type: tdd` plan; Task 1 carries `tdd="true"` at the task level and the plan's own `<action>` specifies a single commit covering both the RED test file and the GREEN doc edit (not a split test/feat/refactor triad). RED was captured by running `go test ./cmd/engram -run '^TestAgentSetupGuideDocumentsDrift$' -count=1 -v` against the unedited guide before any doc edit: it FAILED with legs 1 (`preserved` row missing), 5 (`would-write` row missing `facets`), 6 (opencode not stated), and 7 (JSON fields not named) all reported. GREEN was confirmed after the doc edit with the same command, and the positive-control test (`TestAgentSetupGuideDriftGateFiresOnInjectedViolation`, 8 subtests) passed unchanged throughout since it only exercises in-memory fixtures._

## Files Created/Modified

- `cmd/engram/agent_setup_docs_test.go` — NEW: `agentSetupGuideRelPath`, `newline` (a raw-literal-newline, avoiding the `\n` escape sequence per the no-backslash discipline), `preservedRowPattern`, `wouldWriteRowPattern`, `agentSetupGuideDriftViolations`, `TestAgentSetupGuideDocumentsDrift`, `TestAgentSetupGuideDriftGateFiresOnInjectedViolation`
- `docs-site/src/content/docs/guides/agent-setup.md` — preview paragraph gains the drift-comparison and opencode-not-compared sentences; results table gains the `preserved` row and rewritten `would-write`/`already-correct` rows; Scripts-and-JSON section names `registered`/`facets`/`drift`

## Decisions Made

- The registered/facets/drift JSON-field sentence is one long unwrapped physical line rather than hand-wrapped prose, because the docs gate's leg 7 requires all three tokens on the same physical line and `docs-site/` is entirely excluded from rumdl (MD013 is also globally disabled), so no lint cost was traded away.
- Reworded a test-file doc comment from a quoted `"clean"` reference to an unquoted `the clean case` reference so the acceptance criterion's case-name-occurrence grep counts exactly 8 (the 8 struct-literal case names), not 9 (which included the comment's own quoted echo of the same word).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed the Scripts-and-JSON sentence wrapping so the docs gate's leg 7 could see all three tokens on one line**
- **Found during:** Task 1 (first post-edit test run)
- **Issue:** The initial edit hand-wrapped the `registered`/`facets`/`drift` sentence across five physical lines (matching the surrounding prose's ~80-char wrap width), so `TestAgentSetupGuideDocumentsDrift`'s leg 7 (which requires all three tokens on one physical line, mirroring the plan's own gate-fixture design) still failed after the edit.
- **Fix:** Rewrote the sentence as a single unwrapped physical line; confirmed `docs-site/` is excluded from rumdl entirely (and MD013 is globally disabled) before doing so, so no lint regression was introduced.
- **Files modified:** `docs-site/src/content/docs/guides/agent-setup.md`
- **Verification:** `go test ./cmd/engram -run '^TestAgentSetupGuideDocumentsDrift$' -count=1 -v` passes; `task lint` still exits 0.
- **Committed in:** `0bbae108` (Task 1 commit)

**2. [Rule 1 - Bug] Fixed a self-inflicted acceptance-criterion miscount in the new test file's own comment**
- **Found during:** Task 1 (acceptance criteria verification loop)
- **Issue:** The plan's own acceptance criteria expect exactly 8 occurrences of the quoted case names in `agent_setup_docs_test.go`; the file as first written had 9, because the explanatory doc comment above `TestAgentSetupGuideDriftGateFiresOnInjectedViolation` also quoted `"clean"`.
- **Fix:** Reworded the comment to `the clean case` (unquoted), leaving all eight struct-literal case names as the only quoted occurrences.
- **Files modified:** `cmd/engram/agent_setup_docs_test.go`
- **Verification:** `rg -c -e '"(clean|row_deleted|...)"'  cmd/engram/agent_setup_docs_test.go` now prints `8`; both docs-gate tests still pass.
- **Committed in:** `0bbae108` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1, both self-corrections discovered by the plan's own acceptance-criteria gate before commit). **Impact:** cosmetic. No scope creep; neither fix touched the classification logic or the documented guarantees.

### Process incident (not a code deviation)

While diagnosing an unrelated `go vet` warning, a shell command intended to check whether that warning pre-dated this session was mistyped as `git stash -u` (destructive-git-prohibition violation) instead of a read-only check. This stashed the new (at that point untracked) `cmd/engram/agent_setup_docs_test.go` file along with the two pre-existing untracked `.planning/milestone.lock`/`.planning/state.json` files, creating `stash@{0}`. It was immediately detected via `git stash list` (a second, pre-existing stash — `stash@{1}`, belonging to a different worktree session — was present and left untouched throughout), confirmed via `git stash show -u --stat stash@{0}` to contain exactly the three expected untracked paths, and restored via `git stash pop stash@{0}` (never `git stash pop` bare, to avoid touching the unrelated `stash@{1}`). The restored file was verified byte-for-byte intact (180 lines, 2 test functions) before continuing. No commit, no file content, and no other worktree's state was affected. Recorded here per the destructive-git-prohibition rule's requirement to report rather than self-heal silently; in this case the self-heal (`git stash pop stash@{0}`, targeted, verified) was itself the correct and lowest-risk recovery, since `stash -u` (unlike `clean`) is losslessly reversible when the stash entry is popped before any further git operation touches the same paths.

## Issues Encountered

- The `internal/store` package's `TestDialTestClientFailsWhenRequiredAndUnavailable` fails in this sandbox for the same pre-existing, already-acknowledged reason recorded in `.planning/STATE.md`'s Deferred Items table (Docker/testcontainer unavailable — "rootless Docker not found"). `internal/store` has no import relationship to `internal/setup`, `cmd/engram`, or `docs-site/`, so `task`'s aggregate `test:go` step fails on this unrelated package while every package this plan actually touches passes. Task 2's individual gates (`task lint`, `task license:check`, `git diff --exit-code -- go.mod go.sum`, `go test ./internal/keylinks/ -count=1`, `go test ./cmd/engram -run '^TestAgentSetupGuide' -count=1`) were run and verified green individually per the plan's own `<verify>` command, which does not include the whole-repo `task test:go` step.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The results-table vocabulary (`preserved`, `facets`, `drift`, `registered`) plan 04-04 renders in `--output json`/text is now documented in `guides/agent-setup.md` and pinned by `agentSetupGuideDriftViolations` — 04-04 does not need to touch this plan's rows again except to keep them accurate.
- `requirements.ready-ids` reports 0/3 ready for the three requirements this plan advances (`REQ-drift-preserved-outcome`, `REQ-drift-facet-naming`, `REQ-drift-observed-registration`) — shared with sibling plans 04-01/04-04/04-05 in the same phase; correctly left unmarked in `REQUIREMENTS.md` here (shared-ID gate, #2388) and will flip to Complete once the last declaring sibling plan finishes.
- No blockers.

## Self-Check: PASSED

- `cmd/engram/agent_setup_docs_test.go` and `docs-site/src/content/docs/guides/agent-setup.md` confirmed present on disk with the expected content (`[ -f ]` plus `rg` content checks above).
- Commit `0bbae108` confirmed present via `git log --oneline --all`.
- Plan-level `<verification>` block re-run: `go test ./cmd/engram -run '^TestAgentSetupGuide' -count=1 -v` shows `--- PASS` for both tests, no `--- SKIP`; the `rg` checks in Task 1's acceptance criteria all hold against the live guide; the three untouched rows are byte-identical to `ff5a6f94`; `task lint`, `task license:check`, `git diff --exit-code -- go.mod go.sum`, `go test ./internal/keylinks/ -count=1` all exit 0.
- `git rev-list --count e3ac2478..HEAD` reports `1` (matches `commits: 1` above; `plan_head_before: e3ac2478`).

---
*Phase: 04-drift-detection-read-only*
*Completed: 2026-09-15*
