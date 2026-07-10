---
phase: 12-per-memory-usage-signals
plan: 03
subsystem: api
tags: [protobuf, buf, connect-go, codegen]

requires:
  - phase: 12-01
    provides: internal/store.Memory struct AccessCount/LastAccessedAt fields (payload round-trip)
  - phase: 12-02
    provides: async usage-signal queue wiring the store's IncrementAccess

provides:
  - "proto Memory.access_count = 19 (uint64), additive"
  - "proto Memory.last_accessed_at = 20 (google.protobuf.Timestamp), additive"
  - "regenerated committed gen/go/engram/v1/engram.pb.go with AccessCount/LastAccessedAt accessors"
  - "regenerated committed gen/ts/engram/v1/engram_pb.ts with accessCount/lastAccessedAt fields"

affects: [12-06]

tech-stack:
  added: []
  patterns: ["additive proto field append (never renumber 1-18), task proto:gen + git add gen/ as the only mutation path for generated code"]

key-files:
  created: []
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts

key-decisions:
  - "last_accessed_at uses google.protobuf.Timestamp (matching created_at=14's type), not a string, per PLAN.md's must_haves and PATTERNS.md's type analog"

patterns-established:
  - "Isolate proto+codegen churn in its own plan/commit ahead of the handler code that references the new generated fields, so the drift-check gate is exercised in isolation"

requirements-completed: [REQ-usage-signals]

coverage:
  - id: D1
    description: "Memory proto message gains additive access_count (uint64, field 19) and last_accessed_at (Timestamp, field 20) fields"
    requirement: "REQ-usage-signals"
    verification:
      - kind: other
        ref: "task proto:lint"
        status: pass
      - kind: other
        ref: "grep AccessCount/LastAccessedAt gen/go/engram/v1/engram.pb.go"
        status: pass
    human_judgment: false
  - id: D2
    description: "task proto:gen regenerates gen/go and gen/ts; regenerated tree committed drift-free (CI parity)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: other
        ref: "git diff --exit-code -- gen/ (post-stage)"
        status: pass
      - kind: other
        ref: "go build ./..."
        status: pass
    human_judgment: false

duration: 6min
completed: 2026-07-10
status: complete
---

# Phase 12 Plan 03: Proto Codegen Ripple Summary

**Additive `Memory.access_count`/`Memory.last_accessed_at` wire fields (19/20) added to engram.proto and the committed gen/go + gen/ts trees regenerated drift-free via `task proto:gen`.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-07-10T17:12:05Z
- **Completed:** 2026-07-10T17:18:00Z
- **Tasks:** 1
- **Files modified:** 3

## Accomplishments
- Appended two additive fields to the `Memory` proto message after `short_id = 18`: `uint64 access_count = 19` and `google.protobuf.Timestamp last_accessed_at = 20`, each with a one-line doc comment matching the surrounding style
- Ran `task proto:lint` (clean) then `task proto:gen`, regenerating `gen/go/engram/v1/engram.pb.go` (new `AccessCount`/`LastAccessedAt` fields + getters) and `gen/ts/engram/v1/engram_pb.ts` (new `accessCount`/`lastAccessedAt` fields)
- Staged the full regenerated `gen/` tree and reproduced CI's drift-check locally (`git diff --exit-code -- gen/` exits clean post-stage); `go build ./...` green

## Task Commits

Each task was committed atomically:

1. **Task 1: Add proto fields 19/20 and regenerate gen/ (BLOCKING)** - `b5ff0c7` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified
- `proto/engram/v1/engram.proto` - Memory message gains additive fields 19/20
- `gen/go/engram/v1/engram.pb.go` - regenerated: AccessCount/LastAccessedAt struct fields + getters
- `gen/ts/engram/v1/engram_pb.ts` - regenerated: accessCount/lastAccessedAt TS fields

## Decisions Made
- `last_accessed_at` typed as `google.protobuf.Timestamp` (not a string), matching `created_at = 14`'s existing type and PLAN.md's explicit must_haves — this is the authoritative type per the plan contract and 12-PATTERNS.md's stated type analog.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 06's `memoryToProto` mapping in `connectapi.go` can now reference `engramv1.Memory.AccessCount`/`LastAccessedAt` on the generated struct — this plan intentionally did NOT touch any Go handler code (per plan scope).
- `gen/` tree is committed drift-free; CI's `buf` drift-check job should pass unchanged.

---
*Phase: 12-per-memory-usage-signals*
*Completed: 2026-07-10*

## Self-Check: PASSED

- FOUND: proto/engram/v1/engram.proto
- FOUND: gen/go/engram/v1/engram.pb.go
- FOUND: gen/ts/engram/v1/engram_pb.ts
- FOUND: .planning/phases/12-per-memory-usage-signals/12-03-SUMMARY.md
- FOUND: b5ff0c7 (Task 1 commit)
