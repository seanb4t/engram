---
phase: 15-additive-proto-stub-write-handlers
plan: 01
subsystem: api
tags: [protobuf, buf, connect-go, protovalidate, grpc, engram-proto]

# Dependency graph
requires: []
provides:
  - "EngramService proto extended with 11 RPCs total (5 existing read + 6 new additive write: StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory)"
  - "buf.lock pinning the buf.build/bufbuild/protovalidate BSR module commit"
  - "Regenerated gen/go/engram/v1 (message types, Visibility enum, engramv1connect client/handler methods) and gen/ts/engram/v1"
  - "protovalidate option-types Go package promoted to a direct module dependency"
affects: [15-02, 15-03, 15-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "buf.validate field/message annotations mirroring MCP tools.go arg-struct validation rules (string.in, min_len, max_bytes, repeated min/max_items, enum not_in, message-level CEL)"
    - "buf.gen.yaml managed-mode disable rule scoped to a specific BSR module (buf.build/bufbuild/protovalidate) to prevent go_package_prefix rewriting a dependency's published go_package"

key-files:
  created:
    - buf.lock
  modified:
    - buf.yaml
    - buf.gen.yaml
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - go.mod

key-decisions:
  - "UpdateMemoryRequest uses google.protobuf.FieldMask update_mask as the sole presence mechanism, guarded by a required field rule plus a message-level CEL allowlisting {content, shared, tags, summary} and rejecting empty paths — D-03 (absent/empty/unknown mask -> InvalidArgument) is enforced this phase, not deferred"
  - "ScheduleMemoryRequest is flattened (no nested StoreMemoryRequest), duplicating StoreMemoryRequest fields inline plus typed google.protobuf.Timestamp not_before/not_after"
  - "category on StoreMemoryRequest/ScheduleMemoryRequest constrained via buf.validate string.in [decision, preference, convention, gotcha], rejecting rule/discovery/garbage at the wire level"
  - "buf.gen.yaml gained a managed-mode disable rule scoped to buf.build/bufbuild/protovalidate (deviation, Rule 3) so the go_package_prefix override does not break the dependency's generated import path"

patterns-established:
  - "buf.validate annotations on new proto messages mirror the corresponding MCP tools.go arg struct's validation exactly, field-for-field"

requirements-completed: [REQ-connect-write-rpcs]

coverage:
  - id: D1
    description: "EngramService proto extended with six additive write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory) plus their messages and the Visibility enum; 11 RPCs total, additive-only vs main"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: other
        ref: "go tool buf lint"
        status: pass
      - kind: other
        ref: "go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'"
        status: pass
      - kind: other
        ref: "grep -REn 'idempotency_level' proto/ (expect no matches)"
        status: pass
    human_judgment: false
  - id: D2
    description: "buf.lock committed pinning the protovalidate BSR dependency; gen/go + gen/ts regenerated and drift-clean; go.mod promotes only the option-types package to direct (no new external Go module)"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: other
        ref: "go tool buf generate && git diff --exit-code -- gen/ (post-commit)"
        status: pass
      - kind: other
        ref: "go build ./..."
        status: pass
      - kind: other
        ref: "idempotency check: diff -r gen against a mktemp -d snapshot of a second buf generate run"
        status: pass
    human_judgment: false

duration: 7min
completed: 2026-07-11
status: complete
---

# Phase 15 Plan 01: Additive Proto Write RPCs Summary

**Extended engram.proto with six additive write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory), buf.validate wire-shape enforcement including a FieldMask allowlist CEL for UpdateMemory, and regenerated gen/go + gen/ts — 11 RPCs total, additive-only vs main.**

## Performance

- **Duration:** ~7 min
- **Completed:** 2026-07-11
- **Tasks:** 3
- **Files modified:** 8 (buf.yaml, buf.lock [new], buf.gen.yaml, proto/engram/v1/engram.proto, gen/go/engram/v1/engram.pb.go, gen/go/engram/v1/engramv1connect/engram.connect.go, gen/ts/engram/v1/engram_pb.ts, go.mod)

## Accomplishments
- Added `buf.build/bufbuild/protovalidate` as a BSR dependency in `buf.yaml`, generated and committed `buf.lock`
- Extended `EngramService` with 6 additive write RPCs and 13 new messages/enum (StoreMemoryRequest/Response, Citation, StoreDiscoveryRequest/Response, UpdateMemoryRequest/Response, DeleteMemoryRequest/Response, SetVisibilityRequest/Response, ScheduleMemoryRequest/Response, Visibility enum) — all buf.validate-annotated to mirror `internal/server/tools.go`'s MCP arg-struct rules
- `UpdateMemoryRequest.update_mask` enforces D-01/D-03: required FieldMask field rule + message-level CEL rejecting empty or unallowlisted mask paths, closing the review-flagged gap this phase (not deferred to Phase 17)
- `ScheduleMemoryRequest` flattened per D-05/D-06, with a message-level CEL mirroring `parseWindow`'s shape rules (at least one bound; not_after strictly after not_before)
- Regenerated `gen/go/engram/v1` and `gen/ts/engram/v1`; `go mod tidy` promoted the protovalidate option-types package to a direct dependency with zero new external Go modules
- Full plan-level verification green: `buf lint`, `buf breaking --against main` (additive-only), post-commit `gen/` drift check, `go build ./...`, and the `idempotency_level` grep-ban

## Task Commits

1. **Task 1: Add protovalidate BSR dependency and commit buf.lock** - `c09a054c` (chore)
2. **Task 2: Extend engram.proto with six additive write RPCs, messages, Visibility enum, and buf.validate annotations** - `a2cb8b91` (feat)
3. **Task 3: Regenerate gen/go + gen/ts and reconcile go.mod (option-types package -> direct)** - `5149d09d` (feat, includes the buf.gen.yaml deviation fix)

## Files Created/Modified
- `buf.yaml` - added `deps: [buf.build/bufbuild/protovalidate]`
- `buf.lock` (new) - pins the resolved protovalidate BSR module commit
- `buf.gen.yaml` - added a managed-mode `disable` rule scoped to `buf.build/bufbuild/protovalidate` (deviation, see below)
- `proto/engram/v1/engram.proto` - 6 write RPCs, 13 new messages/enum, buf.validate annotations, 2 new imports
- `gen/go/engram/v1/engram.pb.go` - regenerated message types + Visibility enum
- `gen/go/engram/v1/engramv1connect/engram.connect.go` - regenerated client/handler methods + Procedure constants for the 6 write RPCs
- `gen/ts/engram/v1/engram_pb.ts` - regenerated TS types
- `go.mod` - `buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go` promoted from indirect to direct

## Decisions Made
- Kept `UpdateMemoryRequest.shared` as `bool` (not a `Visibility` enum) per D-01's locked field set — rejected cross-AI review finding #10 as documented in the plan's `<review_notes>` (gated by the required, allowlisted `update_mask`, so `shared`'s effect is only applied when explicitly masked)
- Used buf.validate's message-literal syntax (`(buf.validate.field).string = {in: [...]}` / `.enum = {not_in: [...]}`) instead of the dotted-path form (`.string.in = [...]`), which `buf lint` rejects as "unexpected array expression in option setting value" for any repeated-value sub-field

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] buf.gen.yaml managed-mode override broke the protovalidate dependency's import path**
- **Found during:** Task 3 (regenerate gen/go + gen/ts, reconcile go.mod)
- **Issue:** `buf.gen.yaml`'s existing `managed.override` for `go_package_prefix` (unscoped) rewrote the go_package of `buf/validate/validate.proto` — sourced from the `buf.build/bufbuild/protovalidate` BSR dependency, not the local module — to a nonexistent local path `github.com/seanb4t/engram/gen/go/buf/validate`. `go mod tidy` failed with "no matching versions for query latest" trying to resolve that fabricated module path.
- **Fix:** Added a `managed.disable` rule scoped to `module: buf.build/bufbuild/protovalidate` for the `go_package_prefix` file option, per buf's documented fix for this exact class of issue (managed mode rewriting a BSR dependency's go_package). Re-ran `go tool buf generate`; the generated import corrected to `buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate`, matching the plan's expected go.mod outcome exactly.
- **Files modified:** `buf.gen.yaml` (not in the plan's `files_modified` list, but required to satisfy Task 3's acceptance criteria)
- **Verification:** `go mod tidy` succeeds; `go build ./...` passes; `go.mod` diff shows only the expected option-types promotion, no new external module; idempotency check (`diff -r` against a second `buf generate` run) passes
- **Committed in:** `5149d09d` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to satisfy Task 3's own acceptance criteria (go.mod promotion with zero new external modules, clean `go build`). No scope creep — the plan's `files_modified` list already covered `go.mod`; only the config file used to reach that state grew by one scoped rule.

## Issues Encountered
- `buf lint` initially rejected 5 buf.validate field options using the dotted-path array syntax (`.string.in = [...]`, `.enum.not_in = [...]`) with "unexpected array expression in option setting value" — fixed by switching to the message-literal form (`.string = {in: [...]}`, `.enum = {not_in: [...]}`) before the Task 2 commit, so no deviation entry was needed (caught pre-commit during the task's own verify step).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 02 (grep-gate + Taskfile/CI idempotency ban) and Plan 03 (connectvalidate.go interceptor + go.mod runtime-validator promotion) can proceed: the proto contract, buf.lock, and regenerated gen/ trees are in place and drift-clean.
- Plan 04's descriptor test can rely on `engramv1.File_engram_v1_engram_proto` exposing exactly 11 methods with `IDEMPOTENCY_UNKNOWN` on all of them.
- No blockers.

---
*Phase: 15-additive-proto-stub-write-handlers*
*Completed: 2026-07-11*

## Self-Check: PASSED

All created/modified files and task commit hashes verified present on disk and in git history.
