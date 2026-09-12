---
phase: 05-slash-command-delegation
plan: 02
subsystem: cli
tags: [setup, generation, slash-command, delegation, cobra]
requires:
  - phase: 05-01
    provides: Validated client-ID input and four-mode runtime Plans
provides:
  - Plan-derived four-mode registration and delegation command tables
  - Preview, show, confirm, apply routing with Claude-only fallback
  - Actual Cobra conformance and captured fake execution argv checks
  - Deterministic anchored writer using synthetic environment inputs
affects: [05-03, slash-command-delegation]
tech-stack:
  added: []
  patterns:
    - Importable renderer consumes runtime Plans through PlanFunc
    - Shared synthetic Options and delegation argv checked against actual Cobra
key-files:
  created:
    - internal/setupgen/setupgen.go
    - internal/setupgen/setupgen_test.go
    - cmd/engram/setup_delegation_test.go
  modified:
    - internal/surfacesgen/main.go
    - skill/engram/commands/engram-setup.md
key-decisions:
  - Preserve all four auth modes and native environment credential references per D-17 through D-20
  - Select exactly one Claude add action structurally; render with Action.Command
  - Retain default detected runtimes and unsupported report rows
  - Leave complete generation, full gates, and git commits to root as explicitly instructed
patterns-established:
  - setupgen.Cases returns independent synthetic options and full preview argv
requirements-completed: [REQ-engram-setup-delegates, REQ-engram-setup-prose-fallback, REQ-delegation-equivalence-derived]
coverage:
  - id: D1
    description: Four fallback rows and preview invocations derive from shared inputs and real Claude Plans
    verification:
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestRenderRealPlans
        status: pass
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestRenderSelectsActionAndQuotes
        status: pass
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestRenderRejectsInvalidPlans
        status: pass
    human_judgment: false
  - id: D2
    description: Anchored generation preserves authored fixture bytes and is deterministic
    verification:
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestWriteNoneTracer
        status: pass
      - kind: unit
        ref: internal/setupgen/setupgen_test.go#TestWriteRejectsInvalidAnchors
        status: pass
    human_judgment: false
  - id: D3
    description: Generated preview and fake apply inputs conform to actual Cobra and runtime-authored argv
    verification:
      - kind: integration
        ref: cmd/engram/setup_delegation_test.go#TestSetupGeneratedInvocations
        status: pass
    human_judgment: false
  - id: D4
    description: Authored delegation and Claude-only fallback instructions guide confirmation and failure handling
    verification: []
    human_judgment: true
    rationale: Source review checks authored instructions; automated artifact tests do not prove an agent follows prose or completes third-party OAuth.
duration: 9min
completed: 2026-09-12
status: complete
---

# Phase 05 Plan 02: Generated Setup Delegation Summary

**The slash command now offers four-mode delegation and Claude-only fallback through command tables derived from real setup Plans, with generated inputs exercised against actual Cobra.**

## Performance and scope

- Two tasks implemented; five declared files changed plus this summary.
- Work occurred approximately 2026-09-12T15:01Z–15:10Z; first recorded implementation timestamp was 15:03:46Z after preparatory reading.
- Worktree: `/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-p05-02-f5f279b1`.
- Branch: `worktree-agent-p05-02-f5f279b1`; expected base: `f5f279b165d10e3aeeebdb43c9693d6a46443a69`.
- STATE, ROADMAP, REQUIREMENTS, and all other phase artifacts remain untouched. No next plan or phase was started. Requirement IDs above reflect this plan's frontmatter, not an independent phase-wide verification verdict.

## Accomplishments

- Added `setupgen.PlanFunc`, `Case`, `Cases`, `Render`, and `Write`, plus exported `RegionID` and canonical `Path`. Cases return fresh options and argv for oauth, oauth-client, bearer, and none. Only oauth-client carries a synthetic non-secret client ID.
- Rendering supplies a synthetic HomeDir and rejects unexpected Getenv, LookPath, and Run use even if the injected Plan ignores their returned error. Plan failure, missing add, ambiguous add, or a nil Plan function returns no partial body. The add action is selected by the `claude mcp add` argv prefix, independent of action position. Both tables use `Action.Command` quoting; bearer retains the literal environment reference.
- Wired the existing surfacesgen entry point to the real Plan renderer after its other outputs. The generated region uses the existing surfaces anchor API. Temporary-fixture tests prove none-mode rendering, preservation outside the region, repeat identity, and fail-closed malformed/missing anchors.
- Reworked authored command prose around the generated region: gather URL and auth inputs, detect with `command -v engram`, preview across default runtimes, show every row, explain limitations, obtain explicit confirmation, then append apply to the same invocation. Nonzero results stop without automatic retry or fallback. The binary-absent branch retains Claude registration, OAuth `/mcp` completion, removal/reconfiguration, and one nonblocking brew pointer.
- Added actual Cobra tests for each generated mode in preview and fake apply. They compare report commands and the entire captured probe/action sequence to real Plans, assert client-ID wiring and literal bearer references independently, preserve default detected runtime rows and unsupported OpenCode OAuth-client behavior, check preview has no mutation effects, and reject an unknown generated flag before runtime detection.

## Task commits

The executor emitted `TASK READY FOR COMMIT` at each tested task boundary:

1. Task 1 — `feat(05-02): generate four-mode setup delegation and fallback`
   - `internal/setupgen/setupgen.go`
   - `internal/setupgen/setupgen_test.go`
   - `internal/surfacesgen/main.go`
   - `skill/engram/commands/engram-setup.md`
2. Task 2 — `test(05-02): verify generated setup inputs against Cobra and runtime plans`
   - `cmd/engram/setup_delegation_test.go`

Final focused verification also tightened Task 1's action-position negative fixture and Task 2's shared-input equality assertions; use the final working-tree versions for these commits.

At the final read-only git check, HEAD remained the captured base and these task changes were uncommitted. No commit hash is claimed. Root owns staging and commits because the verified sandbox cannot write the shared git index. Suggested summary commit: `docs(05-02): record generated setup delegation execution`.

## Verification evidence

All Go commands used `GOCACHE=/tmp/engram-p05-gocache`. Runtime execution is injected, skills writes are in-memory, and file generation fixtures live under temporary directories. No real runtime registration, home configuration writes, credential access, or memory writes were performed.

| Check | Result | Evidence |
| --- | --- | --- |
| Initial none tracer RED | Expected compile failure before Write and RegionID existed | `/tmp/05-02-tracer-red.log` |
| None tracer GREEN | Passed real Plan to temporary anchored file, preserved surrounding bytes, identical repeated write | `/tmp/05-02-tracer-green.log` |
| Full generator tests and repeated tracer gate | Passed four modes, quoting, structural selection, invalid Plan/environment rejection, and invalid anchors | `/tmp/05-02-task1.log`, `/tmp/05-02-tracer-gate.log` |
| Existing surfacesgen binary in temporary mirrored target fixtures | Two runs passed and produced identical command bytes; only the owned generated command was copied back | Executor tool output |
| Actual Cobra generated invocation test | Passed four modes, each with preview/apply, plus unknown-flag negative control | `/tmp/05-02-task2.log` |
| `go test ./internal/setupgen ./internal/surfacesgen ./internal/setup -count=1` | Passed setupgen and entire setup package; surfacesgen compiled and has no test files | `/tmp/05-02-focused-packages.log` |
| Cobra conformance with `-count=2 -shuffle=on` | Passed repeated command state isolation | `/tmp/05-02-cli-repeat.log` |
| Final focused verification after tightened assertions | Passed both packages, including all new renderer/writer and generated-invocation tests | `/tmp/05-02-final-focused.log` |
| `git diff --check` | Passed | Executor tool output |

Final focused command:

```sh
GOCACHE=/tmp/engram-p05-gocache go test ./internal/setupgen ./cmd/engram \
  -run '^(TestRender|TestWrite|TestSetupGeneratedInvocations)' -count=1 -v
```

Source review separately confirmed the preview/show/confirm/apply order, nonzero-stop rule, explicit environment credential prerequisites, placeholder replacement, and fallback scope. This is not behavioral verification of a Claude agent obeying the prose, nor live OAuth or third-party runtime compatibility evidence.

## Deviations from plan

- **Explicit execution-environment override:** Root requested no git index attempts, full socket-dependent command suite, full task gate, or complete `task surfaces:gen` inside this sandbox. Those steps remain pending with root. Instead, built the existing surfacesgen entry point and ran it twice in temporary mirrored target fixtures, copying only the owned generated command back. No alternate production generation entry point or task was added.
- **Test sequence:** Task 1 followed RED then GREEN. Task 2 adds conformance tests for behavior already implemented by 05-01 and Task 1; after fixing a test-only compile typo, its first executable run passed. Its explicit unknown-flag negative control demonstrates actual Cobra rejection; no artificial production regression was introduced to manufacture a RED implementation cycle.
- **Read context:** `BOOST.md` is absent in this worktree. There is no `.codegraph` directory, so CodeGraph lookup was not applicable. The approved D-17–D-20 amendment and 05-01 summary supersede the older research's client-ID limitation.

No architectural deviation, dependency addition, or out-of-scope code change was needed.

## Pending root gates

Root must run the complete prescribed `task surfaces:gen`, inspect/commit generated output, rerun generation and check command-file identity, and run the full affected-package suite and `task`. These are pending, not failed or passed by this executor. The environment restrictions were provided as already verified; no repeat git, network, socket, or sandbox workaround probes were attempted.

The generator build succeeded but Go printed a nonfatal module-cache metadata write denial under `/Users/sean/go/pkg/mod/cache/download/...`: `operation not permitted`. The generated binary and temporary-fixture runs succeeded. This was not a runtime/home configuration write and did not require a workaround.

## Self-Check: PASSED (implementation and focused verification)

All five declared code/artifact files and this summary exist. Working-tree changes are restricted to those files. No commits are asserted, no full repository gate is asserted, and no live setup effects were used. Root should update gate evidence and commit this summary after completing its assigned verification.

## Orchestrator verification completion

Root ran the complete `task surfaces:gen` and `task` successfully. The initial Markdown gate found missing table separation before the closing anchor; retaining the renderer trailing newline fixed it. The repeated full generation and quality gate then passed (lint, Python and Go suites). Evidence: `/tmp/engram-05-02-quality.log`. Task commits: 05ba6443, 0611ccaa. Sandbox-pending gates above are resolved by this root verification; third-party live behavior remains unclaimed.
