---
phase: 20-correctness-polish
plan: 01
subsystem: api
tags: [protobuf, connect-rpc, discovery, buf, jsonschema]

# Dependency graph
requires:
  - phase: 15-additive-proto-stub-write-handlers
    provides: Additive-only proto discipline (buf breaking gate) and the connectdescriptor_test.go per-field wire-shape pinning pattern this plan extends
  - phase: 19-console-write-ux
    provides: The re-vendor-console-gen-client pattern (task proto:gen -> gen/ts -> ui/src/lib/gen -> task ui:build)
provides:
  - Memory.kind (proto field 21) and Memory.citations (proto field 22, repeated Citation) additive extension
  - citationsToProto read-path mapper in internal/server/connectapi.go
  - Regenerated gen/go, gen/ts, ui/src/lib/gen, and rebuilt internal/webauth/static
  - Regression test pinning storeDiscoveryArgs.ID's short_id jsonschema wording
affects: [console-discovery-rendering (deferred, future phase per D-02)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Additive-only proto field append (Phase-15 discipline) applied to Memory fields 21/22
    - Store->proto conversion helper lives beside memoryToProto in connectapi.go (read path), never in protoconv.go (write path)

key-files:
  created: []
  modified:
    - proto/engram/v1/engram.proto
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go
    - internal/server/tools_test.go
    - internal/server/connectdescriptor_test.go
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/webauth/static/**

key-decisions:
  - "D-01 (locked pre-plan): additive Memory.kind=21/citations=22 reusing the existing Citation message, not a new dedicated Discovery message"
  - "D-02 (locked pre-plan): wire-fidelity only this phase; console rendering of kind/citations deferred"
  - "D-03/Pitfall-1 confirmed: summary was already mapped by memoryToProto pre-phase — zero changes to that line"
  - "#303 confirmed already-fixed (commit 92a6f610/PR #288) — verification-only, no tools.go diff, closed with regression test as guard"

patterns-established:
  - "citationsToProto(cs []store.Citation) []*engramv1.Citation — nil-for-empty read-path mapper, naming-symmetric with memoriesToProto"

requirements-completed: [REQ-discovery-proto-fidelity, REQ-discovery-shortid-schema]

coverage:
  - id: D1
    description: "SearchDiscoveries carries kind and citations over the Connect wire (Memory proto fields 21/22, additive)"
    requirement: "REQ-discovery-proto-fidelity"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestMemoryToProtoMapsKindAndCitations"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestMemoryToProtoZeroValueKindAndCitations"
        status: pass
      - kind: unit
        ref: "internal/server/connectdescriptor_test.go#TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs"
        status: pass
      - kind: other
        ref: "go tool buf lint && go tool buf breaking --against main && go tool buf generate && git diff --exit-code -- gen/"
        status: pass
    human_judgment: false
  - id: D2
    description: "storeDiscoveryArgs.ID jsonschema advertises short_id support, regression-pinned (#303 closed)"
    requirement: "REQ-discovery-shortid-schema"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreDiscoveryArgsIDSchemaAdvertisesShortID"
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-07-15
status: complete
---

# Phase 20 Plan 01: Discovery Proto Fidelity + Short-ID Schema Summary

**Additive Memory.kind/citations proto fields (21/22) wired through memoryToProto so SearchDiscoveries stops silently dropping discovery fidelity, plus a regression test closing the already-fixed #303 short_id jsonschema gap.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-15T23:38:00Z
- **Completed:** 2026-07-15T23:46:00Z
- **Tasks:** 3
- **Files modified:** 9 (+ regenerated gen/webauth-static trees)

## Accomplishments
- `Memory` proto message additively extended with `kind` (field 21) and `citations` (field 22, `repeated Citation`, reusing the existing `Citation` message) — `buf breaking` clean, no existing field touched
- `citationsToProto` mapper added beside `memoryToProto`/`memoriesToProto` in `internal/server/connectapi.go` (read path); `memoryToProto` now copies `Kind`/`Citations` through unconditionally (Go zero-value on every non-discovery write path already)
- Regenerated `gen/go`, `gen/ts`, re-vendored `ui/src/lib/gen`, and rebuilt `internal/webauth/static` via `task proto:gen` + `task ui:build` — no drift
- `storeDiscoveryArgs.ID`'s jsonschema `short_id` wording (already correct since commit `92a6f610`/PR #288) pinned by a new reflection-based regression test; GitHub #303 closed with that provenance cited
- `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (Phase-15 SC4 wire-shape pin) updated for the new field count/fields — caught by the full `go test ./...` gate, not by the plan's scoped test commands

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend Memory proto additively and regenerate** - `2aab7385` (feat)
2. **Task 2: Populate kind/citations in memoryToProto** - `f7e7e5bc` (test, RED) → `d6b9f2db` (feat, GREEN)
3. **Task 3: Pin storeDiscoveryArgs.ID short_id schema (#303)** - `5bc1d8b4` (test)
4. **Deviation fix (Rule 1):** `e836e463` (fix) — descriptor test field-count update, see below

**Plan metadata:** committed separately after this SUMMARY.

_Note: Task 2 followed the tdd="true" RED/GREEN cycle; no REFACTOR commit needed (code was already minimal/clean)._

## Files Created/Modified
- `proto/engram/v1/engram.proto` - Memory.kind=21, Memory.citations=22 (additive, ~4-line diff)
- `internal/server/connectapi.go` - new `citationsToProto` helper + extended `memoryToProto` return literal
- `internal/server/connectapi_test.go` - `TestMemoryToProtoMapsKindAndCitations`, `TestMemoryToProtoZeroValueKindAndCitations`
- `internal/server/tools_test.go` - `TestStoreDiscoveryArgsIDSchemaAdvertisesShortID` (+ `reflect` import)
- `internal/server/connectdescriptor_test.go` - Memory field-count/table pin updated 20→22, kind/citations rows added
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` - regenerated
- `internal/webauth/static/**` - rebuilt SPA (hashed asset renames only, no behavioral change per D-02)

## Decisions Made
- Followed all locked D-01/D-02/D-03 decisions from 20-CONTEXT.md exactly; no new architectural decisions required.
- `citationsToProto` placed in `connectapi.go` (read path), not `protoconv.go` (write path, which already holds the inverse `citationToArg`) — per 20-PATTERNS.md pattern assignment.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` for the new Memory field count**
- **Found during:** Post-Task-3 full-suite verification (`go test ./...`)
- **Issue:** The Phase-15 SC4 descriptor test pins `Memory`'s exact field count (was 20) and a per-field wire-shape table; adding fields 21/22 correctly failed this pre-existing regression guard (`field count = 22, want 20`) — this is the guard working as designed, not a bug in the new code.
- **Fix:** Updated the pinned count to 22 and added `kind`/`citations` rows to the wire-shape table (field number, name, kind, repeated, msgType) so a future accidental rename/retype of these new fields is caught too.
- **Files modified:** `internal/server/connectdescriptor_test.go`
- **Verification:** `go test ./internal/server/... -run TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs -v` passes; full `go test ./...` green afterward.
- **Committed in:** `e836e463`

---

**Total deviations:** 1 auto-fixed (Rule 1 — expected fallout from the additive proto change, not a planning gap)
**Impact on plan:** No scope creep — this is exactly the kind of drift the Phase-15 descriptor test exists to catch, and updating its expectations is part of correctly landing an additive field.

## Issues Encountered
- `task` (default lint+test target) halts at `lint:markdown` (`rumdl check .`) with 1476 pre-existing issues across `.planning/**` docs unrelated to this plan's files. This is the systemic `.rumdl.toml` `.planning`-exclude gap already tracked in STATE.md ("Systemic `.rumdl.toml` `.planning` exclude → Phase 21") — logged to `20-correctness-polish/deferred-items.md`, out of scope per the scope-boundary rule. Verified this plan's changes directly instead: `golangci-lint run ./...` (0 issues) and `go test ./...` (all packages `ok`).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan A (#307 proto fidelity + #303 short_id schema) is closed; `buf`, `ui-drift`, and Go test/lint CI gates are all clean at HEAD.
- Console rendering of `kind`/`citations` remains explicitly deferred (D-02) for a future console phase — the gen client now carries the types.
- Plans B (#304/#302 embed cleanups), C (#308 MintShortID cap), and D (#269 summarize CronJob) are unaffected by this plan and can proceed independently (D-10 file-disjoint split).

---
*Phase: 20-correctness-polish*
*Completed: 2026-07-15*

## Self-Check: PASSED

All claimed files (proto/engram/v1/engram.proto, internal/server/connectapi.go,
internal/server/connectapi_test.go, internal/server/tools_test.go,
internal/server/connectdescriptor_test.go, gen/go/engram/v1/engram.pb.go,
20-01-SUMMARY.md, deferred-items.md) and all claimed commit hashes (2aab7385,
f7e7e5bc, d6b9f2db, 5bc1d8b4, e836e463, cb96437a) verified present.
