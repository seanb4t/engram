---
phase: 03-curation-rpcs-mcp-tools
plan: 01
subsystem: api
tags: [connectrpc, mcp-tools, csrf, proto, buf, archive-restore]

requires:
  - phase: 01-store-prerequisites
    provides: "authz.ActionArchive; owner-gated ArchiveAs/RestoreAs on internal/store.Store"
provides:
  - "ArchiveMemory/RestoreMemory Connect RPCs, CSRF-gated (D-06, D-07, D-15)"
  - "archive_memory/restore_memory MCP tools, owner-gated on both lanes (D-16)"
  - "The shared archiveBatch core (deps.archiveMemory/restoreMemory) both lanes call"
  - "The phase's discretionary wire contract (option-a) confirmed for plans 03-02..03-06"
affects: [03-02-curation-rpcs-mcp-tools, 03-03-curation-rpcs-mcp-tools, 03-04-curation-rpcs-mcp-tools, 03-05-curation-rpcs-mcp-tools, 03-06-curation-rpcs-mcp-tools, 03-07-curation-rpcs-mcp-tools]

actuals:
  tokens: 43107
  tasks: 3
  commits: 2
  plan_head_before: 2ab4e15e90467fb39e8786938aacf4ce94c051fa

tech-stack:
  added: []
  patterns:
    - "Batch-outcome core (archiveBatch): loop over a single-id owner-gated store verb, synthesizing a not_found row for a resolution failure rather than calling the verb — no store-level batch primitive"
    - "Thin Connect adapter delegating to the same deps.* core the MCP tool calls (SC2), extended to the two new write RPCs"

key-files:
  created:
    - internal/server/archive.go
    - internal/server/archive_test.go
    - .planning/phases/03-curation-rpcs-mcp-tools/deferred-items.md
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/connectdescriptor_test.go
    - internal/server/connectcsrf.go
    - internal/server/connectcsrf_test.go
    - internal/server/connectcsrf_lane_test.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/connectapi.go
    - internal/server/protoconv.go
    - internal/server/tools.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/toolclass_test.go
    - internal/server/registertools_test.go
    - internal/e2e/boot_test.go
    - internal/server/connectapi_write_parity_test.go
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "Task 1 wire-contract decision: option-a (user-selected 2026-09-27, no item-level changes). Verbatim bundle: (1) Archive/Restore — enum ArchiveOutcome {UNSPECIFIED, ARCHIVED, ALREADY_ARCHIVED, RESTORED, NOT_ARCHIVED, NOT_FOUND}; message ArchiveResult {requested=1, id=2, outcome=3}; MCP words archived/already_archived/restored/not_archived/not_found; id empty on not_found; one result per supplied id in input order (a duplicate reported per occurrence); at most 1000 ids, each at most 256 bytes. (2) SupersedeMemoryResponse {id=1, short_id=2, validated=3, supersedes=4, targets=5}; MCP validate_only result {validated, supersedes, targets}. (3) ListRules empty scopes: one cross-scope read of every readable rule, oldest-first, up to 1000 total; coverage reuses searched_scopes/scopes_truncated/scopes_unknown filtered to rule:* scopes. (4) ListScheduled: Connect page_token/next_page_token, MCP cursor/next_cursor; no full knob (always full). (5) enum EdgeType {UNSPECIFIED, SUPERSESSION, CITATION, TAG, VECTOR}; enum SupersessionDirection {UNSPECIFIED, SUCCESSOR, PREDECESSOR}; oneof evidence {VectorEvidence vector, TagEvidence tag, CitationEvidence citation, SupersessionEvidence supersession}. (6) No buf.validate rules on any new request message — the one shared core validates both lanes identically. This plan (03-01) implements item (1) only; plans 03-02..03-06 implement items (2)-(5) as approved."
  - "Corrected a stale per-plan commit ledger (gsd-plan-head-before-03-01): the sentinel already existed on disk from an earlier milestone's own phase-03/plan-01 and pointed 159 commits before this plan's actual start; corrected to the phase-3-planned HEAD (2ab4e15e, matching STATE.md's own state_head) before computing actuals.commits. Same class of issue as the precedent recorded for 01-03/01-04 in STATE.md's decision log."
  - "archiveBatch reports one outcome row per caller-supplied token, never merged or deduplicated across repeats within one call (D-06's discretion item), matching Task 1's option-a bundle and pinned by TestArchiveMemoryBatchOutcomes' repeated-id case."
  - "Logged a pre-existing, out-of-scope internal/keylinks finding (03-04-PLAN.md:65's key_links escaping shape, authored before this plan's execution) to deferred-items.md and WINDOWS.md (id 18) rather than fixing a sibling plan file — scope boundary (Rule 1-3 do not apply to files this plan does not touch)."

patterns-established:
  - "archiveBatch: validate shape once (validateArchiveIDs), then loop ids in order, resolving each via ResolvePointID and calling the owner-gated store verb; a resolution failure synthesizes its own not_found row rather than ever calling the verb, and a not_found row's id is ALWAYS empty (DEC-xa6) — this is the template plans 03-02..03-06 should NOT need to repeat since no other RPC in this phase needs a new batch primitive."

requirements-completed: [RPC-02, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "ArchiveMemory/RestoreMemory Connect RPCs: additive proto (enum, ArchiveResult, two request/response pairs, two RPCs appended after ScheduleMemory), CSRF-gated via csrfWriteProcedures, thin adapters delegating to the shared archiveBatch core"
    requirement: RPC-02
    verification:
      - kind: integration
        ref: "internal/server/archive_test.go#TestArchiveMemoryConnectRoundTrip"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestCSRFCurationWritesRequireDoubleSubmit"
        status: pass
    human_judgment: false
  - id: D2
    description: "archive_memory/restore_memory MCP tools, owner-gated identically on both lanes (a non-owner, an anonymous caller, and a cross-bucket caller all get not_found; a re-read shows no mutation)"
    requirement: RPC-02
    verification:
      - kind: integration
        ref: "internal/server/archive_test.go#TestArchiveMemoryOwnerGate"
        status: pass
    human_judgment: false
  - id: D3
    description: "Per-id batch outcomes in caller order (mixed owned/already/shared-not-owned/nonexistent/short_id/ambiguous-short_id/repeated-id) and malformed-batch rejection (empty, over-cap, blank, over-length) with byte-identical field=/hint= envelopes across both lanes"
    requirement: RPC-02
    verification:
      - kind: integration
        ref: "internal/server/archive_test.go#TestArchiveMemoryBatchOutcomes"
        status: pass
      - kind: unit
        ref: "internal/server/archive_test.go#TestArchiveMemoryRejectsMalformedBatch"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_write_parity_test.go#TestWriteParity/ArchiveMemory"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_write_parity_test.go#TestWriteParity/RestoreMemory"
        status: pass
    human_judgment: false
  - id: D4
    description: "Blast-radius classification (surfaces.Class rows) and self-describe/registration inventories bumped from 15 to 17 tools by construction"
    requirement: RPC-06
    verification:
      - kind: unit
        ref: "internal/surfaces/toolclass_test.go#TestOperationsCoverEveryTool"
        status: pass
      - kind: integration
        ref: "internal/server/registertools_test.go#TestRegisterToolsEnumerable"
        status: pass
    human_judgment: false
  - id: D5
    description: "Task 1 decision gate: the phase's discretionary wire contract confirmed as option-a with no item-level changes, unblocking plans 03-02..03-06"
    human_judgment: true
    rationale: "A decision-gate outcome recorded from the user's checkpoint response, not a behavior a test asserts."

duration: 2h19m
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 1: Curation RPCs & MCP Tools — ArchiveMemory/RestoreMemory Summary

**ArchiveMemory/RestoreMemory land on Connect (CSRF-gated) and as MCP tools, sharing one batch-outcome core over the Phase 1 owner-gated ArchiveAs/RestoreAs — plus the phase's wire-contract decision gate confirmed for the remaining six plans.**

## Performance

- **Duration:** 2h19m
- **Started:** 2026-09-27T04:32:09Z
- **Completed:** 2026-09-27T06:51:05Z
- **Tasks:** 3 (1 decision gate, 1 tracer, 1 auto/tdd)
- **Files modified:** 19 modified, 3 created

## Accomplishments

- Task 1's decision gate resolved: option-a, the phase's full seven-RPC wire contract, confirmed with no item-level changes — recorded verbatim above so plans 03-02..03-06 implement it unchanged.
- `ArchiveMemory`/`RestoreMemory` published on Connect: additive `ArchiveOutcome` enum, `ArchiveResult` message (reused across both response messages), and two RPCs appended after `ScheduleMemory`; CSRF-gated via `csrfWriteProcedures` with the D-15 first-test discipline (`TestCSRFCurationWritesRequireDoubleSubmit`, observed red before the CSRF map entries existed, restored to green).
- The shared `archiveBatch` core (`internal/server/archive.go`): validates ids once, then loops calling `ArchiveAs`/`RestoreAs` per resolved id, synthesizing a `not_found` row (id always empty, DEC-xa6) for any resolution failure — the one genuinely new piece of logic this phase needed, matching the plan's Pattern 2 exactly.
- `archive_memory`/`restore_memory` MCP tools registered over the same core; a per-id `not_found` never becomes a tool error. Blast-radius rows and tool inventories bumped 15→17 by construction (`toolclass.go`, `toolclass_test.go`, `registertools_test.go`, `boot_test.go`); `task surfaces:gen` regenerated `docs-site/reference/tools.md`'s tool-blast-radius table.
- Owner gate proven through both lanes plus the direct `deps.*` call (`TestArchiveMemoryOwnerGate`): non-owner, anonymous, and cross-bucket callers all get `not_found` with no mutation; positive controls confirm the true owner (and the anonymous caller, for the anonymous-bucket record) succeed.
- Per-id batch ordering and outcome vocabulary proven on both lanes with fresh fixtures (`TestArchiveMemoryBatchOutcomes`) and malformed-batch rejection proven with zero store calls plus cross-lane envelope-text parity (`TestArchiveMemoryRejectsMalformedBatch`, `assertEnvelopeParity`).
- `connectdescriptor_test.go` trimmed per D-26: the RPC-count assertion and the per-RPC name/type map are gone, leaving only the IDEMPOTENCY_UNKNOWN guard; the `source_delegates_to_named_deps_methods` AST subtest was deliberately left unextended (D-22).

## Task Commits

Each task was committed atomically:

1. **Task 1: DECISION — the phase's discretionary wire contract (one-way)** — no commit (decision recorded in this SUMMARY per the plan's resume-signal instruction; pre-resolved by the user before dispatch).
2. **Task 2: ArchiveMemory and RestoreMemory end to end on Connect** — `c202ba7c` (feat)
3. **Task 3: archive_memory / restore_memory MCP tools, blast-radius rows, owner-gate proofs** — `fdcba249` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/server/archive.go` — the shared batch core: `archiveArgs`, `archiveResult`, the five outcome-word constants, `maxArchiveIDBytes`, `validateArchiveIDs`, `archiveBatch`, `archiveMemory`, `restoreMemory`, `archiveSummaryText`
- `internal/server/archive_test.go` — `TestArchiveMemoryConnectRoundTrip`, `TestArchiveMemoryOwnerGate`, `TestArchiveMemoryBatchOutcomes`, `TestArchiveMemoryRejectsMalformedBatch`
- `proto/engram/v1/engram.proto` — `ArchiveOutcome`, `ArchiveResult`, `ArchiveMemoryRequest/Response`, `RestoreMemoryRequest/Response`, two RPCs (additive; regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`)
- `internal/server/connectcsrf.go` / `connectcsrf_test.go` / `connectcsrf_lane_test.go` — the two new write Procedures in the allowlist, the extended `csrfWriteCases`/`TestCSRFWriteProcedureAllowlist`, and `TestCSRFCurationWritesRequireDoubleSubmit`
- `internal/server/store_iface.go` / `fakestore_test.go` — `memStore.ArchiveAs/RestoreAs`, `spyStore`'s matching fakes
- `internal/server/connectapi.go` / `protoconv.go` — the two Connect handlers and `archiveOutcomeToProto`/`archiveResultsToProto`
- `internal/server/tools.go` — the `archive_memory`/`restore_memory` tool registrations
- `internal/surfaces/toolclass.go` / `toolclass_test.go`, `internal/server/registertools_test.go`, `internal/e2e/boot_test.go` — two new `Operation` rows, inventories bumped 15→17
- `internal/server/connectapi_write_parity_test.go` — `assertEnvelopeParity`, `ArchiveMemory`/`RestoreMemory` `TestWriteParity` subtests
- `docs-site/src/content/docs/reference/tools.md` — regenerated tool-blast-radius table (via `task surfaces:gen`)
- `.planning/phases/03-curation-rpcs-mcp-tools/deferred-items.md` — new, logs the pre-existing out-of-scope `internal/keylinks` finding below

## Decisions Made

See `key-decisions` in the frontmatter above (Task 1's option-a confirmation, the plan-ledger correction, the per-occurrence duplicate-id reporting, and the deferred sibling-plan finding).

## Deviations from Plan

None — plan executed exactly as written. (Two temporary, uncommitted D-27 mutations and one test-fixture bug are documented below as non-vacuity evidence and TDD-cycle detail respectively, not as deviations from the plan's instructions.)

## Non-Vacuity Evidence (D-27)

Each new/behavior-proving test was confirmed to go red against a temporary, uncommitted mutation of the guarded code, then the mutation was reverted (no harness, patch file, or meta-test committed):

1. **Task 2 — CSRF allowlist gate.** Temporarily removed `EngramServiceArchiveMemoryProcedure` from `csrfWriteProcedures`. `TestCSRFCurationWritesRequireDoubleSubmit/ArchiveMemory` went red on all four assertions: the no-cookie and cookie-only cases both got `<nil>` (success) instead of `PermissionDenied`; the empty-ids-no-token case got `invalid_argument: field=ids hint=required` instead of `PermissionDenied` (proving CSRF had stopped running before validation); the first-call outcome check failed because the second call's repeat had already archived the record on the first (unguarded) attempt. Restored; `TestCSRFCurationWritesRequireDoubleSubmit` and `TestCSRFWriteProcedureAllowlist` pass green.
2. **Task 3 — owner gate (D-16).** Temporarily added a subject-less `Archive(ctx, id)` method to `memStore` (plus a matching `spyStore` fake with no owner check) and routed `deps.archiveMemory` through it instead of `ArchiveAs`. `TestArchiveMemoryOwnerGate` went red on 7 assertions: the non-owner, anonymous, and cross-bucket calls that should have reported `not_found` instead reported `archived`/`already_archived`, and the "no mutation" re-read found `RA.ArchivedAt` set. Reverted all three files (`archive.go`, `store_iface.go`, `fakestore_test.go`) to their exact prior content (confirmed via `git diff --stat` showing zero residual change beyond the plan's own legitimate edits); `TestArchiveMemoryOwnerGate` passes green.
3. **Task 3 — DEC-xa6 empty-id guarantee.** Temporarily changed `archiveBatch`'s not-found branch to echo the resolved canonical id instead of leaving it empty. `TestArchiveMemoryBatchOutcomes` went red on both the `Connect` and `MCP` subtests (the shared-not-owned and ambiguous-short-id rows now carried a non-empty `ID`). Reverted; the test passes green.

## TDD Gate Compliance (Task 3)

Task 3 (`type="auto" tdd="true"`) followed the plan's explicit single-commit instruction rather than a separate `test(...)`/`feat(...)` commit pair — the plan's own Action text specifies one `feat(server): ...` commit with a pathspec that includes `archive_test.go` alongside the production files, matching this phase's established convention of red evidence via temporary uncommitted mutation (D-27) rather than a committed RED-phase commit. The RED/GREEN cycle was still followed in substance:

- **RED (genuine):** `TestArchiveMemoryOwnerGate` passed on its first run (no implementation bug found for that behavior — recorded per the plan's own "if any passes on first run" instruction). `TestArchiveMemoryBatchOutcomes` failed on its first run — not from a missing implementation, but from a test-authoring bug: the `R1` fixture record was seeded without a `ShortID`, and the batch's last entry (`f.r1.ShortID`) was empty, tripping `validateArchiveIDs`' blank-entry rejection. Fixed the test fixture (never the production code) by minting `r1Short`. `TestArchiveMemoryRejectsMalformedBatch` passed on its first run.
- **GREEN:** All four tests pass after the fixture fix, with production code unchanged from its Task-2-inherited-plus-Task-3-registration state.
- **Evidence of load-bearing tests:** the three D-27 mutations above (items 2-3) prove the two hardest-to-get-right invariants (owner gate, DEC-xa6 empty-id) are actually enforced by these tests, not merely passing vacuously.

## Issues Encountered

- **Stale per-plan commit ledger.** `gsd-plan-head-before-03-01` already existed on disk pointing to a commit 159 commits before this plan's actual start (a leftover sentinel from an earlier milestone's own phase-03/plan-01, per the guard's own written contract: "the ledger persists on disk... per-plan filename, so sequential plans cannot contaminate each other" — except a *cross-milestone* filename collision was not something that guard anticipated). Corrected the sentinel to `2ab4e15e` (this milestone's actual phase-3-planned HEAD, matching STATE.md's `state_head`) before computing `actuals.commits`. Same class of issue STATE.md's decision log records for plans 01-03 and 01-04.
- **Pre-existing `internal/keylinks` failure, out of scope.** `go test ./internal/keylinks/ -count=1` fails on `03-04-PLAN.md:65`'s key_links escaping shape — that plan file was committed at `b1b0057e`, before this plan's execution began, and is not in this plan's `files_modified` list. Logged to `deferred-items.md` and `WINDOWS.md` (id 18) rather than fixed (scope boundary).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 03-02..03-06 can proceed against the confirmed option-a wire contract with no further decision gate.
- `archive_memory`/`restore_memory`'s agent-facing guidance (curating-memory skill, CLAUDE.md §Memory contract, `reference/memory-record.md`) is explicitly deferred to plan 03-07 per this plan's own objective text ("D-01's agent guidance ships in plan 03-07") — not a gap in this plan.
- No blockers.

## Self-Check: PASSED
