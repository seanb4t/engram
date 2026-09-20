---
phase: 06-cross-spine-partial-results
plan: 02
subsystem: cli
tags: [cli, coverage-footer, docs, D-05, D-01]

requires:
  - phase: 06-cross-spine-partial-results
    provides: "plan 06-01's scopes_unknown proto field (ListMemoriesResponse 7, SearchMemoriesResponse 4) and the scopeCoverage-returning searchedScopes — this plan surfaces that signal on the CLI and documents it"
provides:
  - "renderCoverageFooter's third form: `scopes_unknown: true` with NO count, checked BEFORE the truncated/count branches so it can never fall through to a misleading count-of-zero"
  - "TestClientListCoverageUnknownFooter and TestClientSearchCoverageUnknownFooter — full CLI round trips via stubEngramService + runClient"
  - "The coverage-unknown state documented on every published surface: reference/tools.md, guides/cli.md, the upgrade guide's Unreleased section, and CLAUDE.md"
affects: [03]

actuals:
  tasks: 3
  commits: 3
  plan_head_before: bafab3c48a2e6d0f0a0e0e2f9e3d5c1a0b7f4e21

key-files:
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/client_list.go
    - cmd/engram/client_search.go
    - cmd/engram/client_list_test.go
    - cmd/engram/client_search_test.go
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - CLAUDE.md

key-decisions:
  - "The unknown branch is checked FIRST in renderCoverageFooter and returns immediately, with the reasoning stated in the function's own doc comment: printing a count of zero would read as 'searched nothing', a different and false claim. This makes the D-03 absence-vs-empty property structural in the renderer rather than a convention."
  - "No stderr warning and no exit-code change (D-05's two explicitly rejected alternatives): the call genuinely succeeded and returned real data, and this repo reserves exit codes for actual failures."

requirements-completed: []

duration: interrupted-and-closed-by-orchestrator
completed: 2026-09-20
status: complete
---

# Phase 6 Plan 2: CLI Coverage Footer & Documentation Summary

**The coverage-unknown state now reaches a human: one new footer form on the single shared renderer, proven by a round trip through each verb, and documented on all four published surfaces.**

## Execution note: interrupted, closed out by the orchestrator

This plan's executor completed and committed all three tasks, then stalled twice waiting on a `task`
gate it had backgrounded itself, ending its turn without writing SUMMARY.md — the same pattern
plan 05-06 hit (durable record `9f0qav7xja`). The orchestrator stopped the agent and closed out.

**One thing that recovery caught, and it is the reason the pattern is worth recording:** the stalled
agent left a red-evidence patch APPLIED — `internal/store/store.go` was missing the
`grpc.WithChainUnaryInterceptor(classifyResponseTooLarge)` line, i.e. Phase 2's
`02-01-classifier-not-in-base-options.patch` mid apply→RED→revert cycle. Committing the tree as
found would have landed a deliberate defect in production code. It was restored from HEAD
(`git checkout -- internal/store/store.go`) and the build re-verified before anything was committed.
No file outside this plan's scope was committed.

What the orchestrator verified before closing:
- All three task commits present (`729a3d00`, `a9080711`, `9dc6a70d`).
- `go build ./...` clean; `go test ./cmd/engram/` green.
- Both new tests exist under the names the plan specified.
- The tree carries no left-applied patch.

## Accomplishments

- **`renderCoverageFooter` gained its third form** (`cmd/engram/client_common.go`): the signature
  takes `scopesUnknown bool`, and the unknown branch is checked **before** the truncated and count
  branches and returns immediately. Its doc comment states why: a count of zero would read as
  "searched nothing", which is a different and false claim. Both call sites
  (`client_list.go:94`, `client_search.go:82`) pass the new value.
- **Two CLI round trips** — `TestClientListCoverageUnknownFooter` and
  `TestClientSearchCoverageUnknownFooter` — following the established `stubEngramService` +
  `runClient` stdout-substring pattern rather than a new unit harness.
- **Four documentation surfaces** updated: `reference/tools.md` (the coverage documentation),
  `guides/cli.md` (which enumerates the footer forms), the upgrade guide's `## Unreleased` section
  (D-01 makes a previously-failing call succeed — a caller-visible behavior change, matching how
  Phases 2/3/4 of this milestone each added a section), and CLAUDE.md's memory-contract sentence.

## Task Commits

1. **Print the coverage-unknown footer form** — `729a3d00` (feat)
2. **Prove the search lane's coverage-unknown footer** — `a9080711` (test)
3. **Document the coverage-unknown state on every published surface** — `9dc6a70d` (docs)

## Deviations from Plan

- **Execution interrupted; SUMMARY and metadata written by the orchestrator** (see above). The
  plan's three tasks were executed and committed as written; only the close-out steps were
  performed by the orchestrator.

## Issues Encountered

- **A red-evidence patch was left applied by the stalled agent** (detail above). This is the second
  live confirmation that a stalled executor's tree must be checked for an applied patch BEFORE any
  recovery commit.
- `TestRedEvidencePatchesAreLive` continues to run long under concurrent machine load (wave 1 logged
  this as WINDOWS entry 13); it needs an extended `-timeout`, not a `Taskfile.yaml` change.

## Known Stubs

None.

## Next Phase Readiness

- The coverage-unknown signal is now surfaced on MCP, Connect and both CLI verbs, and documented.
- Plan 06-03 remains: register this phase's red-evidence patches, re-prove the earlier phases'
  patches over the two files this phase edited, and tick REQ-cross-spine-partial.

---
*Phase: 06-cross-spine-partial-results*
*Completed: 2026-09-20*
