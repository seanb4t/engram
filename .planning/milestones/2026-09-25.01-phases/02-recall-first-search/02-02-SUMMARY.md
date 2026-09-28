---
phase: 02-recall-first-search
plan: 02
subsystem: cli
tags: [recall-gate, cli, docs, mcp]

# Dependency graph
requires:
  - phase: 02-01
    provides: "RecallGateHidden proto message + recall_gate_hidden fields on SearchMemoriesResponse/ListMemoriesResponse (Connect + MCP lanes)"
provides:
  - "renderRecallHiddenFooter (cmd/engram/client_common.go): shared CLI text-lane footer for recall_gate_hidden"
  - "engram search / engram list text output prints the recall-gate hidden-count footer whenever the server reports hidden records"
  - "CLAUDE.md, docs-site reference/tools.md, guides/cli.md, guides/upgrade.md, and both MCP tool descriptions each name recall_gate_hidden"
affects: [02-04, 02-08, 02-09]

# Actuals (#2632)
actuals:
  tokens: 5291
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A second, independently-gated CLI text footer (like renderCoverageFooter) lives inside its own helper so no call site can forget it; unlike the coverage footer, this one is NOT conditioned on --cross-spine, since a scope-confined call can hide records too"

key-files:
  created: []
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/client_search.go
    - cmd/engram/client_list.go
    - cmd/engram/client_search_test.go
    - cmd/engram/client_list_test.go
    - CLAUDE.md
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - internal/server/tools.go

key-decisions:
  - "Followed 02-01's option-a names verbatim (RecallGateHidden{Total,Archived,Superseded,Expired,Scheduled}, recall_gate_hidden field/key) — no new naming decision needed"

patterns-established: []

requirements-completed: [ENTRY-03]

coverage:
  - id: D1
    description: "engram search / engram list text output print recall_gate_hidden: N  archived: N  superseded: N  expired: N  scheduled: N whenever the response reports hidden records, on scope-confined AND cross-spine calls alike, and print nothing when the field is absent or all-zero"
    requirement: "ENTRY-03"
    verification:
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchRecallHiddenFooter"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchNoRecallHiddenFooterWhenAbsentOrZero"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_list_test.go#TestClientListRecallHiddenFooter"
        status: pass
    human_judgment: false
  - id: D2
    description: "JSON output lane is untouched (protojson already emits recall_gate_hidden under its proto name); no extra rendering code added"
    verification:
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchJSONOutputCarriesNoMigrationFooter (pre-existing, unaffected by this plan) plus manual acceptance-criteria grep proving renderRecallHiddenFooter is called exactly twice, once per text lane"
        status: pass
    human_judgment: false
  - id: D3
    description: "CLAUDE.md, reference/tools.md (search_memory and list_memory), guides/cli.md, and guides/upgrade.md each describe recall_gate_hidden's window, per-state overlap rule, and absence rule"
    requirement: "ENTRY-03"
    verification:
      - kind: other
        ref: "rg -o recall_gate_hidden across the four docs files, deduplicated by file — prints 4"
        status: pass
    human_judgment: false
  - id: D4
    description: "search_memory and list_memory MCP tool descriptions name recall_gate_hidden so an agent learns it from the tool listing, with no MCP input schema changes"
    verification:
      - kind: other
        ref: "rg -o 'Results also carry .recall_gate_hidden.' internal/server/tools.go — prints 2"
        status: pass
      - kind: integration
        ref: "go test ./internal/server/ -run 'Docs|Doc|Description|RegisterTools|Annotation' -count=1"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 2: Recall-Gate Hidden Count — CLI Footer and Docs Summary

**`engram search`/`engram list` text output now prints a `recall_gate_hidden` footer (not gated on `--cross-spine`), and CLAUDE.md, docs-site, and both MCP tool descriptions name what it means.**

## Performance

- **Duration:** 20 min
- **Started:** 2026-09-26T18:23:00Z
- **Completed:** 2026-09-26T18:43:04Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments

- `renderRecallHiddenFooter` (cmd/engram/client_common.go) renders `recall_gate_hidden: N  archived: N  superseded: N  expired: N  scheduled: N` — nil/no-op on a nil or all-zero field, unlike `renderCoverageFooter` it is never gated on `--cross-spine` since a scope-confined call can hide records too
- Wired into both `engram search` and `engram list` text lanes right after the existing coverage footer; JSON lanes untouched (protojson already emits the field)
- Three new tests (`TestClientSearchRecallHiddenFooter`, `TestClientSearchNoRecallHiddenFooterWhenAbsentOrZero`, `TestClientListRecallHiddenFooter`) prove the footer prints on both scope modes and stays byte-identical to the pre-phase baseline when the field is absent or zero
- CLAUDE.md's memory contract, docs-site `reference/tools.md` (search_memory and list_memory sections), `guides/cli.md` (footer example), and `guides/upgrade.md` (Unreleased entry #20, "Who should act: nobody") each describe `recall_gate_hidden`'s window, per-state overlap rule, and absence rule
- `internal/server/tools.go`'s `search_memory` and `list_memory` Descriptions each gained one sentence naming `recall_gate_hidden` — no MCP input schema changes (decision `fenpnam8ah` preserved)

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): CLI footer end to end** — `e80a8591` (feat)
2. **Task 2: Docs and MCP tool descriptions** — `67240050` (docs)

**Plan metadata:** committed separately (docs: complete plan)

## Files Created/Modified

- `cmd/engram/client_common.go` - `renderRecallHiddenFooter` beside `renderCoverageFooter`
- `cmd/engram/client_search.go` - wires the footer into the search text lane
- `cmd/engram/client_list.go` - wires the footer into the list text lane
- `cmd/engram/client_search_test.go` - `TestClientSearchRecallHiddenFooter`, `TestClientSearchNoRecallHiddenFooterWhenAbsentOrZero`
- `cmd/engram/client_list_test.go` - `TestClientListRecallHiddenFooter`
- `CLAUDE.md` - memory contract names `recall_gate_hidden`
- `docs-site/src/content/docs/reference/tools.md` - `recall_gate_hidden` paragraph on search_memory and list_memory
- `docs-site/src/content/docs/guides/cli.md` - footer example alongside the coverage footer
- `docs-site/src/content/docs/guides/upgrade.md` - Unreleased entry #20
- `internal/server/tools.go` - search_memory/list_memory Description sentences

## Decisions Made

- Used 02-01's shipped names verbatim (option-a: `RecallGateHidden{Total,Archived,Superseded,Expired,Scheduled}`, field/key `recall_gate_hidden`) — no new decision required for this plan.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- D-03 is now complete across every lane: Connect and MCP (plan 02-01), CLI text footer and JSON field, and every reader-facing contract (this plan).
- The console's results header work (plans 02-04/02-08/02-09) can read `recall_gate_hidden` directly off the Connect response; no further CLI or docs work needed for this field.
- No blockers.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*

## Self-Check: PASSED

- `cmd/engram/client_common.go`, `cmd/engram/client_search.go`, `cmd/engram/client_list.go`, `cmd/engram/client_search_test.go`, `cmd/engram/client_list_test.go` — all exist with the expected additions
- `CLAUDE.md`, `docs-site/src/content/docs/reference/tools.md`, `docs-site/src/content/docs/guides/cli.md`, `docs-site/src/content/docs/guides/upgrade.md`, `internal/server/tools.go` — all contain `recall_gate_hidden`
- Commits exist: `e80a8591` (Task 1), `67240050` (Task 2)
- Plan-level `<verification>` re-run: `go test ./cmd/engram/ ./internal/surfaces/ -count=1` — ok; `go test ./internal/server/ -run 'Docs|Doc' -count=1` — ok; `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -count=1` — ok; `task lint` — clean; `task fmt:check` — clean; `task license:check` — clean (2409 files, 0 invalid)
- All task-level `<acceptance_criteria>` re-verified passing (renderRecallHiddenFooter call count = 2, crossSpine occurrences inside the helper = 0, gofmt clean, recall_gate_hidden present in all 4 docs files, present ≥2 times in tools.md, "Results also carry `recall_gate_hidden`" present exactly twice in tools.go, task license:check exits 0)
