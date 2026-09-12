---
phase: 05-slash-command-delegation
plan: 03
subsystem: testing
tags: [setup, generation, drift, lint, delegation]
requires:
  - phase: 05-02
    provides: Real-Plan renderer and anchored setup command generation
provides:
  - Exact read-only setup command drift check
  - Unique setup-anchor rejection shared by check and writer
  - Check-only generator dispatch and lint dependency
  - Real argv mutation and scratch-git drift negative controls
affects: [slash-command-delegation, surfaces-generation]
tech-stack:
  added: []
  patterns:
    - Check and writer share Render with injectable Plan functions
    - Compare raw region bytes alongside ReadRegion to detect newline drift
key-files:
  created:
    - internal/surfacesgen/main_test.go
  modified:
    - internal/setupgen/setupgen.go
    - internal/setupgen/setupgen_test.go
    - internal/surfacesgen/main.go
    - Taskfile.yaml
key-decisions:
  - Preserve renderer trailing newline exactly, including its Markdown table separation
  - Reject duplicate setup anchors without changing the shared multi-region surfaces reader
  - Keep production regeneration and existing CI lane unchanged
  - Leave all commits and full repository gates to root as explicitly instructed
requirements-completed: [REQ-delegation-equivalence-derived]
coverage:
  - id: D1
    description: Exact read-only comparison rejects drift and malformed or duplicate anchors
    verification:
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestCheckReadOnly
        status: pass
    human_judgment: false
  - id: D2
    description: Check dispatch bypasses writers and main exits nonzero with actionable drift diagnostics
    verification:
      - kind: integration
        ref: internal/surfacesgen/main_test.go#TestCheckDispatch
        status: pass
      - kind: integration
        ref: internal/surfacesgen/main_test.go#TestCheckSubprocessExit
        status: pass
      - kind: integration
        ref: internal/surfacesgen/main_test.go#TestCheckDispatchWriterControl
        status: pass
    human_judgment: false
  - id: D3
    description: Actual argv mutation and checked-in artifact drift trigger both comparison mechanisms
    verification:
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestPlanMutationChangesRegion
        status: pass
      - kind: integration
        ref: internal/setupgen/setupgen_test.go#TestDriftChecks
        status: pass
    human_judgment: false
completed: 2026-09-12
status: complete
---

# Phase 05 Plan 03: Setup Drift Gates Summary

**Local lint now compares setup command bytes without writing; real-Plan mutation and committed scratch-artifact corruption prove both the read-only and regenerate/diff gates reject drift.**

## Accomplishments

- Added public `setupgen.Check(path)` using the same production `Render(setup.ClaudeCode.Plan)` as `Write`. Private check/write helpers accept an injected Plan for negative controls.
- Required exactly one canonical setup anchor pair before reading or writing. Missing, malformed, reversed, nested, duplicated, and same-line duplicate anchors cannot hide a stale second region. The shared `surfaces.ReadRegion` remains unchanged for consumers that intentionally support multiple regions.
- Compared the shared reader's body and the raw bytes between anchors. This preserves the renderer's trailing newline and rejects missing table separation or CRLF drift that the shared scanner otherwise normalizes. Errors name the target and `task surfaces:gen`, with anchor repair guidance.
- Added `--check-setup` dispatch before `run()` reaches any writer. Unknown or extra arguments fail closed. Subprocess tests call actual `main()` and verify success or exit 1, stderr guidance, unchanged target bytes, and an unchanged sentinel on the ordinary writer path. A separate ordinary-generation control proves that sentinel really is rewritten if dispatch reaches the writer.
- Added `lint:setup` to `lint` dependencies. It runs `go run ./internal/surfacesgen --check-setup`. The `surfaces:gen` task, its protobuf/golden sequence, and the existing CI regeneration/diff lane are unchanged.
- Added `TestPlanMutationChangesRegion`: copy a real OAuth-client add action and its Args before changing exactly one client-ID token. The full derived row changes, every unrelated row remains identical, and another real-Plan render remains equal to baseline.
- Added `TestDriftChecks`: committed temporary Git fixtures prove read-only rejection preserves all file bytes and clean Git status; mutation-driven generation yields diff exit 1; repeated generation is stable; authoritative restoration returns exact baseline bytes and clean diff. A separate committed-artifact corruption fixture proves generation detects stale checked-in output, followed by committing the authoritative repair and observing clean diff. Prefix and suffix bytes are checked throughout. Invalid anchors fail both check and writer without repair.

## Task commits and ownership

Root owns all staging and commits. The executor emitted `TASK READY FOR COMMIT` at both tested boundaries. Read-only history inspection confirmed:

1. `204e7815` — `test(05-03): specify read-only setup drift and anchor checks`
2. `074a078a` — `feat(05-03): add read-only setup command drift gate`
3. `d5105429` — `test(05-03): prove source and artifact drift detection`

The final writer-sentinel control in `internal/surfacesgen/main_test.go` was added and tested after root's third commit; root must include it in the remaining task commit. Suggested message: `test(05-03): prove check dispatch bypasses the active writer path`.

Summary commit is also root-owned: `docs(05-03): record setup drift gate execution`.

Only the five declared files and this summary were edited. STATE, ROADMAP, REQUIREMENTS, VALIDATION, other plans, and phase-wide tracking files were not modified. Requirement completion above describes the implemented plan scope, not a phase-wide or live-runtime verification verdict.

## Verification evidence

All Go commands used `GOCACHE=/tmp/engram-p05-gocache`. Generated fixture writes and Git repositories live in `t.TempDir()`. Fixture Git commands ignore global/system Git configuration, use an explicit local author identity, disable signing, and point hooks at a scratch path. No real runtime registration, real home configuration writes, credential reads, memory writes, network retries, or shared-index writes were performed.

| Check | Result | Evidence |
| --- | --- | --- |
| Task 1 RED | Expected compile failure: `undefined: Check` | `/tmp/05-03-task1-red.log` |
| Task 1 GREEN | Both generator packages passed, including exact bytes, bad anchors, read-only dispatch, and subprocess exit | `/tmp/05-03-task1-green.log` |
| Repeated tracer gate | Both generator packages passed again before Task 2 | `/tmp/05-03-tracer-gate.log` |
| `task lint:setup` | Passed against the existing generated setup command | `/tmp/05-03-lint-setup.log` |
| Task 2 controls | `TestPlanMutationChangesRegion` and `TestDriftChecks` passed; logs record actual git diff exit 1 controls | `/tmp/05-03-task2.log` |
| Final focused verification | All targeted tests passed, including writer-sentinel control and existing four-mode Cobra generated invocations | `/tmp/05-03-final-focused.log` |
| `git diff --check` | Passed after final code changes | Executor tool output |

Final focused command:

```sh
GOCACHE=/tmp/engram-p05-gocache go test \
  ./internal/setupgen ./internal/surfacesgen ./cmd/engram \
  -run '^(TestRender|TestWrite|TestCheck|TestDispatch|TestPlanMutationChangesRegion|TestDriftChecks|TestSetupGeneratedInvocations)' \
  -count=1 -v
```

The Task 2 selector intentionally finds its two named tests in setupgen; surfacesgen reports no matching tests for that selector. Its actual dispatch and subprocess tests ran in the Task 1 and final focused commands, rather than treating a no-tests package result as coverage.

## Deviations and pending root gates

- Explicit user override reserves commits, complete `task surfaces:gen`, full affected-package tests, and full `task` for root. These remain pending here. No git-index, network, or socket failure was retried and no caches were scanned.
- Task 1 followed observed RED then GREEN. Task 2 tests exercise behavior implemented in Task 1 and passed on their first executable run; the deliberate mutant and committed corruption supply their expected rejection controls without temporarily regressing production source.
- `BOOST.md` is absent from this worktree. The D-17–D-20 amendment and root's recorded 05-02 renderer newline correction were honored.

Root still needs to run the full affected-package suite and `task`, run complete `task surfaces:gen` after intended output is committed, and check:

```sh
git diff --exit-code -- skill/engram/commands/engram-setup.md \
  cmd/engram/testdata/help.golden cmd/engram/testdata/catalog.golden
```

Root owns writing that evidence into 05-VALIDATION and completing phase-wide tracking. No full-repository or live third-party runtime success is asserted by this executor.

## Self-Check: PASSED (implementation and focused verification)

All five declared files and this summary exist. Root's three recorded task commits were observed in current worktree history. The last writer-control test passed in final focused verification. Remaining commit and full-gate responsibilities are identified above; no placeholder implementation or skipped test was introduced.

Worktree: `/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-p05-03-5e01a88c`.
Branch: `worktree-agent-p05-03-5e01a88c`.
Expected base: `5e01a88ced1d0981caef6c9bccc051e9c32b6057`.

## Orchestrator verification completion

Root ran complete `task surfaces:gen` successfully, then full `task` passed after fixing an unused test-helper parameter reported by revive. Evidence: `/tmp/engram-05-03-quality.log`. The full suite includes the writer-control test and all earlier phase Go/Python tests. All sandbox-pending generation and quality gates above are resolved by root; no live runtime registrations occurred.
