---
phase: 05-slash-command-delegation
plan: 01
subsystem: cli
tags: [setup, cli, oauth, client-id]
requires:
  - phase: 03-runtime-registration
    provides: Authored runtime registration plans and fake execution environments
  - phase: 04-skills-distribution
    provides: Fake skills environment and setup registration/skills aggregation
provides:
  - Validated non-secret client-ID input from setup CLI through Claude Code and Codex argv
  - Regression coverage for exact opaque values, early rejection, and runtime capability boundaries
  - Updated live help and generated help/catalog fixtures
affects: [05-02, slash-command-delegation]
actuals:
  tokens: 10479
  tasks: 3
  commits: 5
tech-stack:
  added: []
  patterns:
    - Flag-only input with usageErrorf validation before runtime effects
    - Existing setup and skills fake environments for every simulated apply
key-files:
  created:
    - .planning/phases/05-slash-command-delegation/05-01-SUMMARY.md
  modified:
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/destructive_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/setup/runtime.go
    - internal/setup/claudecode.go
    - internal/setup/claudecode_test.go
    - internal/setup/codex.go
    - internal/setup/codex_test.go
    - internal/setup/plan_test.go
key-decisions:
  - Implement D-17 through D-20 without expanding unsupported runtime capabilities
  - Preserve client IDs byte-for-byte after checking trimmed emptiness
  - Keep credentials in runtime-owned environment prerequisites and retain nil subprocess stdin
patterns-established: []
requirements-completed: [REQ-engram-setup-delegates]
coverage:
  - id: D1
    description: CLI validation rejects missing or irrelevant client IDs before runtime or skills effects
    requirement: REQ-engram-setup-delegates
    verification:
      - kind: integration
        ref: cmd/engram/setup_test.go#TestSetupClientID
        status: pass
    human_judgment: false
  - id: D2
    description: Claude Code and Codex receive exact caller IDs while runtime capability boundaries remain unchanged
    verification:
      - kind: unit
        ref: internal/setup/claudecode_test.go#TestClaudeCodeClientID
        status: pass
      - kind: unit
        ref: internal/setup/codex_test.go#TestCodexClientID
        status: pass
      - kind: unit
        ref: internal/setup/plan_test.go#TestPlanAuthModes
        status: pass
    human_judgment: false
  - id: D3
    description: Live help and generated help/catalog expose the client-ID and credential contract
    verification:
      - kind: unit
        ref: cmd/engram/setup_test.go#TestSetupHelpClientIDContract
        status: pass
      - kind: unit
        ref: cmd/engram/golden_test.go#TestHelpGolden
        status: pass
      - kind: unit
        ref: cmd/engram/golden_test.go#TestCatalogGolden
        status: pass
    human_judgment: false
duration: 11min
completed: 2026-09-12
status: complete
---

# Phase 05 Plan 01: Setup Client-ID Input Summary

**`engram setup --auth oauth-client --client-id VALUE` validates the input before effects and forwards its exact bytes to Claude Code and Codex registration argv.**

## Performance and scope

- Three implementation tasks completed; eleven plan-owned files changed plus this summary.
- Measured execution began at the first test log, 2026-09-12T14:45:48Z; preparatory reading preceded that measurement.
- Summary prepared at 2026-09-12T14:57:56Z.
- Worktree: `/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-p05-01-259f22f9`.
- Branch: `worktree-agent-p05-01-259f22f9`; expected base: `259f22f994c4df896d70322ce9ab13de004453a1`.
- STATE.md, ROADMAP.md, REQUIREMENTS.md, and other phase artifacts were not modified. Phase-wide requirement completion remains the root orchestrator's responsibility; this plan supplies the delegation prerequisite, not the later slash-command implementation.

## Accomplishments

- Added flag-only `setupClientID` and `Options.ClientID`. OAuth-client rejects missing, empty, or Unicode-whitespace-only values; other modes reject even an explicitly empty client-ID flag. Nonempty values retain spaces and shell metacharacters exactly.
- Preserved Claude's tolerant remove/fatal add sequence, Codex's single add action, probes, skills targets, and unsupported OpenCode/generic OAuth-client combinations. Mixed default selection excludes generic and preserves partial-failure classification on fake apply.
- Help documents the non-secret ID, mode restriction, synthetic preview example, inherited `MCP_CLIENT_SECRET`, and lack of interactive stdin. Existing `ENGRAM_TOKEN` and generic-only token-file wording remain intact.

## Task commits

1. Task 1 RED: `ec1dd703` — `test(05-01): cover setup client ID validation and Claude argv`.
2. Task 1 GREEN: `083c5cb7` — `feat(05-01): validate and forward setup client IDs to Claude`.
3. Task 2 RED: `771bf03d` — `test(05-01): cover client IDs across runtime auth modes`.
4. Task 2 GREEN: `389c4678` — `feat(05-01): forward caller client IDs to Codex`.
5. Task 3: `5e3bf492` — `docs(05-01): publish setup client ID and credential requirements`.

Direct staging attempts in this executor returned the index-lock sandbox error below. The commits subsequently appeared on the same worktree branch and were verified from git history. No permission escalation or branch movement was attempted. The summary staging attempt returned the same sandbox error; at handoff this summary is untracked and left intact for the orchestrator to commit. The actuals count includes only the five verified commits.

## Verification evidence

Commands used `GOCACHE=/tmp/engram-p05-01-gocache` after the default Go cache was denied. Logs are temporary execution evidence, not committed artifacts.

| Check | Result | Evidence |
| --- | --- | --- |
| Task 1 RED | Expected failure: unknown client-ID flag and missing-ID acceptance | `/tmp/05-01-task1-red.log` |
| Task 1 GREEN and repeated tracer gate | Passed all rejection and exact Claude argv cases | `/tmp/05-01-task1-green.log`, `/tmp/05-01-tracer-gate.log` |
| Task 2 RED | Expected failure: Codex still authored `<id>` | `/tmp/05-01-task2-red.log` |
| Task 2 GREEN | Passed supported argv, unsupported row, and mixed runtime cases | `/tmp/05-01-task2-green.log` |
| `go test ./internal/setup -count=1` | Passed complete runtime package, including auth matrix | `/tmp/05-01-runtime-suite.log` |
| `go test ./cmd/engram -run '^TestSetup' -count=1 -v` | Passed all setup tests after golden generation | `/tmp/05-01-setup-suite-final.log` |
| Client-ID and live-help tests with `-count=2 -shuffle=on` | Passed; command reset seam prevents flag leakage | `/tmp/05-01-repeat.log` |
| Focused final package tests with `-json` | 55 RUN events, 55 test/subtest PASS events, two package passes, no failures; includes help/catalog without `-update` | `/tmp/05-01-final-focused.jsonl` |
| `go test ... -list` | Every named plan verification target resolves | `/tmp/05-01-test-list.log` |
| `go test ./cmd/engram -run '^TestDestructiveCommandsExactFlagSet$' -count=1 -v` | Passed complete flag inventory, including client-id | `/tmp/05-01-flags.log` |
| `task lint:go` | Passed, zero issues after preallocation fix | `/tmp/05-01-lint-go.log` |
| YAML, action, and Markdown lint from `task` | Passed | `/tmp/05-01-task.log` |
| `task license:check` | Passed: 397 valid, zero invalid, 1382 ignored | Tool output |
| `git diff --check` | Passed | Tool output |

Final focused command:

```sh
go test ./internal/setup ./cmd/engram   -run 'Test(ClaudeCodeClientID|CodexClientID|SetupClientID|ClaudeCodePlan|Plan.*OAuthClient|SetupUnsupportedAuthMode|SetupHelp|HelpGolden|CatalogGolden)'   -count=1 -json
```

The tests use synthetic IDs, injected runtime calls, and in-memory skills environments. No real Claude/Codex/OpenCode registration was executed. No live OAuth completion or third-party compatibility is claimed.

## Deviations from plan

1. **[Rule 3 — blocked generation step]** `task surfaces:gen` ran the surfaces generator but stopped during its unchanged `go tool buf generate` step because buf.build was unavailable. To finish the affected help/catalog artifacts, ran the exact final command already declared by that task: `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1`. No golden was hand-edited and no alternate generator or production task path was introduced. Reviewed diffs contain only the two intended golden changes. The full task still needs a successful orchestrator rerun.
2. **[Rule 2 — test isolation]** Converted the existing `internal/setup/plan_test.go` Plan calls from `OSEnvironment` to the established `fakeEnv()` while updating OAuth-client fixtures, so the owned matrix no longer consults a real home directory.
3. **[Rule 1 — directly related regression]** Fixed the new invalid-input table's preallocation lint finding during Task 3. Also clarified the adjacent Claude client-secret comment to match the planned noninteractive environment contract. These are within declared file ownership.
4. **Execution constraints:** Temporary Go/linter/UV cache paths avoided writes to user-home caches. TDD RED and GREEN commits are present despite direct executor git operations being sandbox-blocked; their hashes were verified rather than inferred from failed tool results.

## Outstanding verification limits and exact errors

- **Full affected-package suite:** `GOCACHE=/tmp/engram-p05-01-gocache go test ./internal/setup ./cmd/engram -count=1` passed the runtime package but could not complete the command package. Existing HTTP-server tests fail/panic at `listen tcp 127.0.0.1:0: bind: operation not permitted`. See `/tmp/05-01-packages.log`. These tests were not weakened or skipped in the required full-suite attempt.
- **Full repository gate:** `GOCACHE=/tmp/engram-p05-01-gocache GOLANGCI_LINT_CACHE=/tmp/engram-p05-01-lintcache UV_CACHE_DIR=/tmp/engram-p05-01-uvcache task` failed at lint. The introduced Go lint issue was fixed and its gate rerun successfully. Python lint remained blocked: `Failed to fetch: https://pypi.org/simple/ruff/` / `failed to lookup address information: nodename nor servname provided, or not known`. The default task did not reach its test stage. See `/tmp/05-01-task.log`.
- **Complete regeneration:** `GOCACHE=/tmp/engram-p05-01-gocache task surfaces:gen` failed: `Failure: the server hosted at that remote is unavailable. Are you sure "buf.build" is a valid remote address?` See `/tmp/05-01-surfaces-gen.log`. The affected goldens were generated and their read-only tests passed as documented above.
- **Direct git staging:** `git add cmd/engram/setup_test.go` (and later task-specific staging commands) returned `fatal: Unable to create '/Volumes/Code/github.com/seanb4t/engram/.git/worktrees/agent-p05-01-259f22f9/index.lock': Operation not permitted`. Changes were left intact for orchestrator commits, with no escalation retry.

Root orchestrator must rerun complete regeneration, the full affected-package suite, and `task` in an environment that permits their existing network and local test-server requirements. This summary reports implementation completion and focused verification, not a passed phase-wide gate.

## Self-Check: PASSED

All eleven declared changed files and this summary exist. All five task commits listed above resolve in this worktree's ancestry. No tracked files were deleted. Only the declared plan files and this summary changed relative to the captured base; no durable memory was written and no other plan was started.

## Orchestrator verification completion

After the executor sandbox failures, the orchestrator ran the complete prescribed
`task surfaces:gen` in this worktree successfully (exit 0), including buf generation and
the help/catalog regeneration. It then ran `task` successfully (exit 0): all lint checks,
33 Python tests, and the full Go suite including command HTTP and container-backed store
tests passed. Log: `/tmp/engram-05-01-quality.log`. These reruns resolve the environment
limits above; no live third-party registration or OAuth completion was tested.

The orchestrator performed each git commit because the executor sandbox could not write
the shared worktree index. Commits listed above are actual verified worktree commits.
