---
phase: 03-curation-rpcs-mcp-tools
plan: 02
subsystem: api
tags: [connectrpc, mcp-tools, csrf, proto, buf, idempotency, supersede]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: "plan 03-01's option-a wire contract confirmation (SupersedeMemoryResponse shape, no buf.validate on new request messages) and the archiveBatch/archive.go precedent for a shared caller-lane dispatch"
provides:
  - "SupersedeMemory Connect RPC, CSRF-gated, taking the store_memory field set plus citations/supersedes/idempotency_key/validate_only (RPC-01, D-08, D-09, D-15)"
  - "The shared deps.supersede dispatch (real write vs validateSupersede dry run) both the Connect handler and the supersede_memory MCP tool call"
  - "validate_only on both lanes: names every resolved target or every offending target identically to a real call, writes nothing, never touches the idempotency ledger"
affects: [03-03-curation-rpcs-mcp-tools, 03-04-curation-rpcs-mcp-tools, 03-05-curation-rpcs-mcp-tools, 03-06-curation-rpcs-mcp-tools, 03-07-curation-rpcs-mcp-tools]

actuals:
  tokens: 34038
  tasks: 2
  commits: 2
  plan_head_before: 64d09a031ce500616b17cc4413cad9c280fe382e

tech-stack:
  added: []
  patterns:
    - "One shared dispatch (deps.supersede) both lanes call, branching on a caller-chosen control flag (ValidateOnly) that is never client-authored record content — extends the archiveBatch/archive.go precedent (plan 03-01) to a single-target-set write instead of a per-id batch"
    - "Dry-run-by-reusing-real-stage-functions: validateSupersede runs the identical staged preflight functions (supersedeArgChecks, resolveAndAuthorizeSupersedeTargets, validateSupersedeTargetState) the real call runs, never a parallel copy, so a preview cannot structurally drift from its commit"

key-files:
  created:
    - internal/server/supersedepreview.go
    - internal/server/supersedepreview_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/connectcsrf.go
    - internal/server/connectcsrf_test.go
    - internal/server/connectcsrf_lane_test.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/protoconv.go
    - internal/server/connectapi_write_parity_test.go

key-decisions:
  - "Shape follows 03-01 Task 1's approved option-a contract verbatim (SupersedeMemoryResponse {id=1, short_id=2, validated=3, supersedes=4, targets=5}; MCP validate_only result {validated, supersedes, targets}); no item-level changes, no new decision gate needed for this plan."
  - "supersedeArgChecks extracted from supersedeMemory (validateStoreArgs then validateCitations, unchanged order) so validateSupersede can run the IDENTICAL two checks for a dry run without duplicating them — the same discipline applied to the two later staged-preflight functions (resolveAndAuthorizeSupersedeTargets, validateSupersedeTargetState), which are called unmodified from both supersedeMemory and validateSupersede."
  - "Corrected a stale per-plan commit ledger (gsd-plan-head-before-03-02): the sentinel already existed on disk from an earlier milestone's own phase-03/plan-02 (a docs-only commit dated 2026-09-19) and pointed 100+ commits before this plan's actual start; corrected to the phase-3-planned HEAD (64d09a03) before computing actuals.commits — the same class of issue 03-01-SUMMARY.md and STATE.md's decision log record for plans 01-03/01-04/03-01."
  - "requirements-completed below is a verbatim copy of this plan's PLAN.md frontmatter requirements (RPC-01, RPC-05, RPC-06), per the SUMMARY template contract — it does not assert those IDs were marked complete in REQUIREMENTS.md this run; the shared-ID gate (RPC-01 also declared by 03-07; RPC-05/RPC-06 declared by every plan in this phase) determines that separately at the update_requirements step."

patterns-established:
  - "supersedeArgs.ValidateOnly is documented, at its declaration site, as a caller-chosen control flag that must NEVER be added to contentFingerprint/mergeFingerprint — it is not client-authored record content and never reaches a write."

requirements-completed: [RPC-01, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "SupersedeMemory lands on Connect behind CSRF with the full store_memory field set plus citations, supersedes, idempotency_key, and validate_only; delegates to the shared deps.supersede dispatch (RPC-01, D-08, D-09, D-15)"
    requirement: RPC-01
    verification:
      - kind: integration
        ref: "internal/server/supersedepreview_test.go#TestSupersedeMemoryConnectRoundTrip"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestCSRFCurationWritesRequireDoubleSubmit/SupersedeMemory"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestCSRFCurationWritesRequireDoubleSubmit/SupersedeMemory_validate_only"
        status: pass
    human_judgment: false
  - id: D2
    description: "validate_only runs the identical preflight a real call runs and writes nothing: no record, no superseded_by stamp, no idempotency ledger entry, no embed, no short-id mint — proven on both lanes (D-08)"
    requirement: RPC-01
    verification:
      - kind: integration
        ref: "internal/server/supersedepreview_test.go#TestSupersedeValidateOnlyWritesNothing"
        status: pass
      - kind: integration
        ref: "internal/server/supersedepreview_test.go#TestSupersedeValidateOnlyLeavesIdempotencyLedger"
        status: pass
    human_judgment: false
  - id: D3
    description: "A validate_only preview never disagrees with the commit that follows it — the same resolved supersedes list on a valid set, the same rejection text on an invalid set — because both run through the SAME staged preflight functions (D-18b)"
    requirement: RPC-01
    verification:
      - kind: integration
        ref: "internal/server/supersedepreview_test.go#TestSupersedeValidateOnlyMatchesCommit"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every offending target of one failure class (addressability, rule, already-superseded) is named identically across the Connect handler and the MCP-lane deps.supersede call, and across a real call and a validate_only call (SC3, D-18)"
    requirement: RPC-05
    verification:
      - kind: integration
        ref: "internal/server/supersedepreview_test.go#TestSupersedeMemoryConnectNamesEveryOffender"
        status: pass
    human_judgment: false
  - id: D5
    description: "MCP↔Connect lane parity: SupersedeMemory's success, cross-owner-rejection, and validate_only cases produce equivalent store traces and equivalent rejection envelopes on both lanes (SC2, D-20)"
    requirement: RPC-06
    verification:
      - kind: integration
        ref: "internal/server/connectapi_write_parity_test.go#TestWriteParity/SupersedeMemory"
        status: pass
      - kind: integration
        ref: "internal/server/supersedepreview_test.go#TestSupersedeValidateOnlyMCPTool"
        status: pass
    human_judgment: false

duration: 38min
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 2: Curation RPCs & MCP Tools — SupersedeMemory Summary

**SupersedeMemory lands on Connect behind CSRF with the full store_memory field set, and `validate_only` ships on both lanes through one shared dispatch (`deps.supersede`) so Phase 4's preview-before-commit dialog can never disagree with its commit.**

## Performance

- **Duration:** 38 min
- **Started:** 2026-09-27T06:55:00Z (approx)
- **Completed:** 2026-09-27T07:26:38Z
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 12 modified, 2 created

## Accomplishments

- `SupersedeMemory` published on Connect: additive `SupersedeMemoryRequest`/`SupersedeMemoryResponse` messages (option-a shape from 03-01's decision gate) and one RPC appended after `RestoreMemory`; CSRF-gated via `csrfWriteProcedures`, with `TestCSRFCurationWritesRequireDoubleSubmit` gaining `SupersedeMemory` and `SupersedeMemory_validate_only` subtests observed red before the allowlist entry existed.
- New `internal/server/supersedepreview.go`: `deps.supersede` is the ONE dispatch both the Connect handler and the `supersede_memory` MCP tool call, branching on `ValidateOnly` between the real write (`supersedeMemory`) and the dry run (`validateSupersede`). `validateSupersede` reuses the real call's own staged preflight functions (`supersedeArgChecks`, `resolveAndAuthorizeSupersedeTargets`, `validateSupersedeTargetState`) verbatim, never a parallel copy, and touches no embedder, no short-id minter, no idempotency ledger, and no `Store.Supersede` call.
- `supersedeArgChecks` extracted from `supersedeMemory` (the same two checks, same order) so both the real call and the dry run share one argument-checking function.
- `internal/server/protoconv.go` gained `supersedeMemoryRequestToArgs`/`supersedeOutcomeToResponse`; `internal/server/connectapi.go` gained the `SupersedeMemory` thin-adapter handler, delegating to `a.d.supersede` exactly like every other write RPC in the file.
- The `supersede_memory` MCP tool's closure now calls `d.supersede` and returns a `{validated, supersedes, targets}` structured result on a `validate_only` call (never the old `{id, short_id}` shape on that path); its Description gained the `validate_only=true` agent-guidance sentence.
- Every offender-naming class (addressability, rule, already-superseded) renders the byte-identical Connect code and message across all four call shapes — Connect and MCP-lane, real and validate_only — proven by `TestSupersedeMemoryConnectNamesEveryOffender` against a real Qdrant.
- A dry run's write-nothing guarantee, its agreement with the commit that follows, and its non-interference with the idempotency replay ledger are each proven on both lanes (`TestSupersedeValidateOnlyWritesNothing`, `TestSupersedeValidateOnlyMatchesCommit`, `TestSupersedeValidateOnlyLeavesIdempotencyLedger`), plus a real in-process MCP session drive of the tool closure itself (`TestSupersedeValidateOnlyMCPTool`).
- `connectapi_write_parity_test.go`'s `TestWriteParity` gained a `SupersedeMemory` row (`success`, `cross_owner_target_rejected`, `validate_only` — the last asserting the store trace carries no `Upsert`/`Supersede`/`MintShortID` call).

## Task Commits

Each task was committed atomically:

1. **Task 1: SupersedeMemory end to end on Connect behind CSRF, with validate_only through one shared dispatch on both lanes** — `101debfb` (feat)
2. **Task 2: validate_only agent contract and the D-18 proofs on both lanes, plus parity rows** — `5fe79ab4` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/server/supersedepreview.go` — `supersedeOutcome`, `deps.supersede` (the one dispatch both lanes call), `deps.validateSupersede`
- `internal/server/supersedepreview_test.go` — `TestSupersedeMemoryConnectRoundTrip`, `TestSupersedeMemoryConnectNamesEveryOffender`, `TestSupersedeValidateOnlyWritesNothing`, `TestSupersedeValidateOnlyMatchesCommit`, `TestSupersedeValidateOnlyLeavesIdempotencyLedger`, `TestSupersedeValidateOnlyMCPTool`, plus the `supersedeConnect`/`supersedeMCP`/`callSupersede`/`messageOf` test helpers
- `proto/engram/v1/engram.proto` — `SupersedeMemoryRequest`/`SupersedeMemoryResponse`, one RPC (additive; regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`)
- `internal/server/connectcsrf.go` / `connectcsrf_test.go` / `connectcsrf_lane_test.go` — the new write Procedure in the allowlist, the extended `csrfWriteCases`/`TestCSRFWriteProcedureAllowlist` (8→9), and the two `TestCSRFCurationWritesRequireDoubleSubmit` subtests
- `internal/server/tools.go` — `supersedeArgs.ValidateOnly`, `supersedeArgChecks` extraction, the rewired `supersede_memory` closure, the `validate_only=true` Description sentence
- `internal/server/connectapi.go` / `protoconv.go` — the `SupersedeMemory` Connect handler and `supersedeMemoryRequestToArgs`/`supersedeOutcomeToResponse`
- `internal/server/connectapi_write_parity_test.go` — the `SupersedeMemory` `TestWriteParity` row

## Decisions Made

See `key-decisions` in the frontmatter above (option-a shape reuse, the `supersedeArgChecks` extraction, the plan-ledger correction, and the `requirements-completed` frontmatter contract note).

## Deviations from Plan

None — plan executed exactly as written. (Two temporary, uncommitted D-27 mutations per task are documented below as non-vacuity evidence, not as deviations from the plan's instructions.)

## Non-Vacuity Evidence (D-27)

Each new/behavior-proving test was confirmed to go red against a temporary, uncommitted mutation of the guarded code, then the mutation was reverted (no harness, patch file, or meta-test committed):

1. **Task 1 — CSRF allowlist gate.** Temporarily removed `EngramServiceSupersedeMemoryProcedure` from `csrfWriteProcedures`. `TestCSRFCurationWritesRequireDoubleSubmit/SupersedeMemory` and `.../SupersedeMemory_validate_only` went red on every assertion: the no-cookie/no-header and cookie-only cases returned success or an unrelated `failed_precondition`/`invalid_argument` code instead of `PermissionDenied`, and the matching-token case succeeded on an already-superseded target from the unguarded prior call instead of failing CSRF first. Restored; both subtests pass green.
2. **Task 1 — the shared dispatch.** Temporarily made `validateSupersede` call the real `supersedeMemory` before its own checks. `TestSupersedeMemoryConnectRoundTrip` went red: the "preview" call actually wrote a real merge, so the subsequent staged-preflight run inside `validateSupersede` hit `store.ErrAlreadySuperseded` on the just-merged targets, failing with `failed_precondition: target is already superseded: ...` instead of returning a validated preview. Reverted; the test passes green.
3. **Task 2 — idempotency ledger poisoning.** Temporarily made `validateSupersede` `Upsert` a placeholder point at the deterministic idempotency point id (with the real `mergeFingerprint`) when `IdempotencyKey` is set — simulating what a recorded ledger entry would look like. `TestSupersedeValidateOnlyLeavesIdempotencyLedger` went red on both lanes: the subsequent "real" call matched the planted fingerprint and replayed the placeholder's id instead of ever calling `Store.Supersede`, so every target's `SupersededBy` stayed `nil` (four assertion failures, both `connect` and `mcp` subtests). Reverted; the test passes green.
4. **Task 2 — the state stage (Class 3/4 preflight).** Temporarily removed the `validateSupersedeTargetState` call from `validateSupersede`. `TestSupersedeMemoryConnectNamesEveryOffender`'s `rule` subtest went red: a validate_only call over two rule targets no longer rejected (returned `nil` error where `failed_precondition`/`errRuleImmutable` was expected), which the assertion loop correctly flagged as a code mismatch before a downstream nil-error dereference terminated the run — sufficient red evidence for both `TestSupersedeMemoryConnectNamesEveryOffender` and (by the same code path) `TestSupersedeValidateOnlyMatchesCommit`. Reverted; both tests pass green.

## TDD Gate Compliance (Task 2)

Task 2 (`type="auto" tdd="true"`) followed the plan's explicit single-commit instruction (one `test(...)` commit covering both the five new tests and the small production edit — the Description sentence) rather than a separate RED-phase/GREEN-phase commit pair, matching this phase's established convention of red evidence via temporary uncommitted mutation (D-27, see above) rather than a committed RED-phase commit.

- **RED (genuine, per the plan's own "record any first-run pass" instruction):** all five new tests (`TestSupersedeMemoryConnectNamesEveryOffender`, `TestSupersedeValidateOnlyWritesNothing`, `TestSupersedeValidateOnlyMatchesCommit`, `TestSupersedeValidateOnlyLeavesIdempotencyLedger`, `TestSupersedeValidateOnlyMCPTool`) passed on their first run. This is expected, not vacuous: Task 1 already implemented the full `deps.supersede`/`validateSupersede` dispatch and its staged-preflight reuse, so Task 2's tests exercise already-correct production code written one task earlier in this same plan. The genuine RED evidence for the invariants these tests protect is the four D-27 mutations above (items 2–4 target exactly the behaviors these five tests assert), which is what proves the tests are load-bearing rather than passing by construction.
- **GREEN:** all five tests pass; `golangci-lint run ./internal/server/...` and the full `internal/server` package (`go test ./internal/server/ -count=1`) are clean.

## Issues Encountered

- **Stale per-plan commit ledger.** `gsd-plan-head-before-03-02` already existed on disk pointing to a docs-only commit dated 2026-09-19 — a leftover sentinel from an earlier milestone's own phase-03/plan-02 — over 100 commits before this plan's actual start. Corrected the sentinel to `64d09a03` (this milestone's actual HEAD when this plan began, matching the phase's own prior-plan close) before computing `actuals.commits`. Same class of issue 03-01-SUMMARY.md and STATE.md's decision log record for plans 01-03, 01-04, and 03-01.
- **First D-27 mutation attempt for the ledger-poisoning check used a nil vector**, which the concrete Qdrant-backed store silently rejects on `Upsert` (dimension mismatch), so the first mutation attempt was a false negative (no observable change). Supplying a valid 3-dimension vector fixed the mutation to genuinely poison the ledger, confirmed red, then reverted — recorded here rather than silently discarded, per the "no meta-test, no harness" rule (`3p0zsqrhmb`) that keeps this kind of iteration out of the committed test suite.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 03-03..03-07 can proceed against the confirmed option-a wire contract with no further decision gate; `SupersedeMemory`'s shape (item 2 of 03-01's bundle) is now shipped alongside `ArchiveMemory`/`RestoreMemory` (item 1, plan 03-01).
- No blockers.

## Self-Check: PASSED
