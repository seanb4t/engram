---
phase: 05-connect-record-state-parity
plan: 01
subsystem: api
tags: [protobuf, connect, buf, grpc-web, go, sveltekit]

# Dependency graph
requires: []
provides:
  - "engramv1.Memory fields 23-30 (superseded_by, supersedes, not_before, not_after, archived_at, schema_version, summary_model, summary_egress_at), with D-14's explicit presence on the three new scalars"
  - "memoryToProto extended in place to populate all eight new fields"
  - "TestConnectRecordStateOnGetMemoryHandler — a Qdrant-backed Connect GetMemory handler round trip proving the eight fields survive the real read path"
  - "store.Memory.SummaryEgressAt's doc comment corrected to state it is visible on both the MCP and Connect wires"
affects: [05-02, 05-03]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 14110
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Nil/zero Timestamp-to-proto discipline (existing lastAccessed guard shape) extended to NotBefore/NotAfter/ArchivedAt/SummaryEgressAt"
    - "D-14 assign-unconditionally: SchemaVersion/SummaryModel wrapped in proto.Uint32/proto.String with no nil guard, so a zero-valued record still carries the key on the wire"

key-files:
  created:
    - internal/server/connectapi_recordstate_handler_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/connectapi.go
    - internal/server/connectdescriptor_test.go
    - internal/store/store.go
    - internal/webauth/static/** (rebuilt SPA bundle, content-hash drift only)

key-decisions:
  - "D-14 (ratified 2026-08-15, pre-dating this plan's execution) governs the three new scalars' presence model — superseded_by/schema_version/summary_model are `optional` (explicit presence), generating *string/*uint32/*string in Go. This plan implemented, not decided, D-14."

patterns-established:
  - "A future field on store.Memory with explicit-presence needs on the Connect wire follows this plan's memoryToProto shape: wrap in proto.Uint32/proto.String/proto.Bool etc. UNCONDITIONALLY, never behind an `if` guard, because protojson omits an unset optional field regardless of EmitDefaultValues."

requirements-completed: [REQ-connect-record-state-parity]

coverage:
  - id: D1
    description: "engramv1.Memory carries fields 23-30 exactly matching D-04's table as amended by D-14 (three new scalars with `optional`, four Timestamps, one repeated); fields 1-22 untouched."
    requirement: "REQ-connect-record-state-parity"
    verification:
      - kind: unit
        ref: "proto/engram/v1/engram.proto field-number rg assertions (see Acceptance Criteria Evidence below) + TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs (internal/server/connectdescriptor_test.go)"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_recordstate_handler_test.go#TestConnectRecordStateOnGetMemoryHandler"
        status: pass
    human_judgment: false
  - id: D2
    description: "memoryToProto populates all eight new fields from a real store.Memory, proven via a Qdrant-backed Connect GetMemory handler round trip (not the mapper in isolation, not shapeProtoMemories)."
    requirement: "REQ-connect-record-state-parity"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_recordstate_handler_test.go#TestConnectRecordStateOnGetMemoryHandler"
        status: pass
    human_judgment: false
  - id: D3
    description: "store.Memory.SummaryEgressAt's doc comment no longer claims the field is store-only or absent from the Connect wire."
    verification:
      - kind: unit
        ref: "rg -c 'not on the Connect wire' internal/store/store.go (exit 1, no matches)"
        status: pass
    human_judgment: false

duration: ~13min
completed: 2026-08-15
status: complete
---

# Phase 05 Plan 01: Connect Record-State Parity (Wire Extension) Summary

**Added the eight D-04 record-state fields (23-30) to the Connect `Memory` message with D-14's explicit presence on the three new scalars, extended `memoryToProto` to populate all eight unconditionally, and proved the whole path with a Qdrant-backed `GetMemory` handler round trip.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-08-15T19:32:57-04:00 (worktree base commit)
- **Completed:** 2026-08-15T19:45:35-04:00
- **Tasks:** 2
- **Files modified:** 8 source/generated files + 1 new test file + rebuilt SPA bundle (`internal/webauth/static/`)

## Accomplishments

- `proto/engram/v1/engram.proto`'s `message Memory` gained fields 23-30 exactly per D-04's table as amended by D-14: `superseded_by` (23, `optional string`), `supersedes` (24, `repeated string`), `not_before`/`not_after`/`archived_at`/`summary_egress_at` (25/26/27/30, `google.protobuf.Timestamp`), `schema_version` (28, `optional uint32`), `summary_model` (29, `optional string`). Fields 1-22 untouched.
- Regenerated and committed all three generated trees (`gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts`) via `task proto:gen`, zero drift on re-run.
- `memoryToProto` (`internal/server/connectapi.go`) extended in place: three new Timestamp nil-guards (`NotBefore`/`NotAfter`/`ArchivedAt`) shaped exactly like the existing `lastAccessed` guard, a fourth `IsZero()` guard for the non-pointer `SummaryEgressAt`, a direct `*string`-to-`*string` copy for `SupersededBy`, and unconditional `proto.Uint32(uint32(m.SchemaVersion))` / `proto.String(m.SummaryModel)` wraps for the two D-14 scalars that must never be gated behind an `if`.
- New file `internal/server/connectapi_recordstate_handler_test.go` with `TestConnectRecordStateOnGetMemoryHandler`: seeds one record carrying all eight new states to distinct non-zero values directly through the real Qdrant-backed `*store.Store`, then calls `api.GetMemory` and asserts all eight arrive on the response message, including compile-time-gated `!= nil` pointer checks on the three D-14 fields.
- `internal/store/store.go`'s `SummaryEgressAt` doc comment corrected: no longer claims "Store-only; not on the Connect wire" (both clauses were false); now states the field carries a plain json tag visible on both the MCP and Connect wires.

## Task Commits

Each task was committed atomically:

1. **Task 1: One record's full state, end to end — proto to the Connect GetMemory handler** - `4b33cc6b` (feat)
2. **Task 2: Repair the SummaryEgressAt comment (D-04)** - `be42fab6` (docs)

**Deviation commit (Rule 1):** `33a6a8c5` (chore) — rebuilt the embedded web UI (`internal/webauth/static/`) after the regenerated Connect TS types produced content-hash drift; the plan's own `<verification>` section requires this drift be committed.

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `proto/engram/v1/engram.proto` — appended fields 23-30 to `message Memory`
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` — regenerated via `task proto:gen`
- `internal/server/connectapi.go` — `memoryToProto` extended with 8 new field mappings; added `google.golang.org/protobuf/proto` import
- `internal/server/connectapi_recordstate_handler_test.go` — new: `TestConnectRecordStateOnGetMemoryHandler`
- `internal/server/connectdescriptor_test.go` — pinned `Memory` field-shape table updated (field count 22→30, 8 new pins) — Rule 1 auto-fix, directly caused by this task's field addition
- `internal/store/store.go` — `SummaryEgressAt` comment repaired
- `internal/webauth/static/**` — rebuilt SPA bundle (content-hash filename drift only, no source change)

## Decisions Made

None made in this plan — D-14 (the presence-model decision) was ratified by Sean on 2026-08-15 prior to this plan's execution and is recorded in `05-CONTEXT.md`/`<ratified_decision>`. This plan implemented D-14, it did not decide it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs`'s pinned `Memory` field table**
- **Found during:** Task 1, first full `internal/server` suite run after adding fields 23-30
- **Issue:** `connectdescriptor_test.go`'s `assertFields(t, fd, "Memory", 22, ...)` hardcodes the pre-plan field count and field-23+ pins are absent; it failed with `Memory: field count = 30, want 22` — a direct, mechanical consequence of this task's own additive proto change, not a pre-existing unrelated failure.
- **Fix:** Bumped the expected count to 30 and added the eight new field pins (name/kind/cardinality/msgType) for 23-30, matching D-14's presence models. Noted in a comment that `optional` (explicit presence) does not change `protoreflect.Cardinality` for a singular field — it stays `Optional`, same as every other non-repeated field in the table — so no cardinality assertion needed correcting beyond the new rows.
- **Files modified:** `internal/server/connectdescriptor_test.go`
- **Verification:** `go test ./internal/server/ -run '^TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs$' -count=1 -v` passes; full `internal/server` suite passes.
- **Committed in:** `4b33cc6b` (part of Task 1 commit — same change that necessitated the fix)

**2. [Rule 3 - Blocking / plan-verification-mandated] Rebuilt embedded web UI**
- **Found during:** Plan-level `<verification>` step (`task ui:build` + `git status --porcelain ui/ internal/webauth/static/`)
- **Issue:** The plan's own verification section requires running `task ui:build` and committing any resulting drift, since `internal/webauth/static/` is a committed build artifact and the CI `ui` check runs outside the phase gate. Regenerating `ui/src/lib/gen/engram/v1/engram_pb.ts` (Task 1) changed the SPA bundle's content, producing new content-hashed chunk filenames.
- **Fix:** Ran `task ui:build`, inspected the resulting diff (renamed/re-hashed JS chunks plus `index.html` reference updates, no logic change), and committed it.
- **Files modified:** `internal/webauth/static/**`
- **Committed in:** `33a6a8c5`

### Discrepancy noted, not fixed (plan criterion inaccuracy, not a code defect)

The acceptance criterion `rg -c 'json:"-"' internal/store/store.go` returns `2` does not hold literally — the live count is `5` (2 actual struct tags on `EmbedderIdentity`/`IdempotencyFingerprint`, plus 3 pre-existing comment lines that quote the tag in prose, at lines 285, 298, and 332). This was verified to be pre-existing and unrelated to Task 2's edit: `git show <TASK2_BASE>:internal/store/store.go | rg -c 'json:"-"'` also returns `5`, and the task 2 diff touches none of those five lines. The underlying property the criterion intends to guard — both fields still carry their `json:"-"` exclusion, and this task introduces no new one — holds and was verified directly via the commit-range diff (see Acceptance Criteria Evidence below).

---

**Total deviations:** 2 auto-fixed (1 Rule 1 bug fix, 1 Rule 3/plan-mandated build-artifact commit); 1 pre-existing plan-criterion inaccuracy noted and worked around with an equivalent substantive check.
**Impact on plan:** Both fixes were necessary for the test suite to pass and for the plan's own verification section to be satisfied. No scope creep — no field, behavior, or API surface was added beyond D-04/D-14's eight fields.

## Issues Encountered

None beyond the deviations above.

## Acceptance Criteria Evidence

**TASK_BASE (task 1):** `059807ab2bdcc7fee5977ada2bb94c887b365ed4` (= PLAN_BASE, worktree base — nothing committed between plan start and task 1).
**TASK_BASE (task 2):** `4b33cc6b9b11d11f0ba1ef72649f619845857076` (task 1's commit).

**Field-number and D-14 presence-model regex assertions:**
```
$ rg -c 'superseded_by = 23|supersedes = 24|not_before = 25|not_after = 26|archived_at = 27|schema_version = 28|summary_model = 29|summary_egress_at = 30' proto/engram/v1/engram.proto
8
$ rg -c '^\s*optional (string superseded_by = 23|uint32 schema_version = 28|string summary_model = 29);' proto/engram/v1/engram.proto
3
```

**Proto commit-range criterion (locked-range 1-22 untouched, no new `deprecated`):** all three sub-assertions passed — the range diff (`git diff --unified=0 <TASK_BASE> -- proto/engram/v1/engram.proto`) was non-empty, `rg '^[+-].*= *([1-9]|1[0-9]|2[0-2]) *;'` matched nothing (exit 1), and `rg '^\+.*deprecated'` matched nothing (exit 1).

**No inverse mapper (D-08):** `rg -c '^func protoToMemory' internal/server/` — no matches, exit 1.

**Mapper commit-range "no new top-level function" criterion, plus the mandatory GATE-RED PROOF (`<TASK_BASE>`-anchored, working-tree form, per `<commit_range_protocol>`):**
- Clean state (before probe): `git diff --unified=0 <TASK_BASE> -- internal/server/connectapi.go | rg '^\+func[[:space:]]'` → no output, exit 1 (non-empty range diff confirmed separately).
- With a temporary `func tmpProbe() {}` appended to `internal/server/connectapi.go` (uncommitted): same command → printed `+func tmpProbe() {}`, exit 0.
- Probe deleted, re-run: → no output, exit 1 again.
- This confirms the base-to-working-tree spelling (not `<TASK_BASE>..HEAD`) correctly sees an uncommitted violator, exactly as `<review_cycle_3_incorporation>` requires.

**Mapper field-population and `out.SupersededBy` absence:**
```
$ rg -c 'SupersededBy|Supersedes:|NotBefore|NotAfter|ArchivedAt|SchemaVersion:|SummaryModel|SummaryEgressAt' internal/server/connectapi.go
19
$ rg -c 'SchemaVersion: *proto\.Uint32\(uint32\(m\.SchemaVersion\)\)' internal/server/connectapi.go
1
$ rg -c 'SummaryModel: *proto\.String\(m\.SummaryModel\)' internal/server/connectapi.go
1
$ rg -c 'out\.SupersededBy' internal/server/connectapi.go
(no matches, exit 1)
```

**Tracer test run (no skip):**
```
$ ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestConnectRecordStateOnGetMemoryHandler$' -count=1 -v
=== RUN   TestConnectRecordStateOnGetMemoryHandler
--- PASS: TestConnectRecordStateOnGetMemoryHandler (0.09s)
PASS
ok  	github.com/seanb4t/engram/internal/server	1.251s
```
No `--- SKIP` line. This test is a **Qdrant-backed Connect handler round trip** — it exercises the real store, the real `engramAPI.GetMemory` handler, and `memoryToProto`. It does NOT exercise Connect HTTP transport or protobuf binary serialization (never described as a "real RPC" or "wire round trip"). It seeds all eight new fields to distinct non-zero values and is therefore scoped to presence/value population only — it does NOT gate D-14 §3's assign-always-even-for-zero property; that gate and its own RED proof belong to plan 05-02.

**`buf breaking` (number/type evidence only, not mapper evidence):**
```
$ go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'
(exits 0, no output)
```

**`git status --porcelain gen/ ui/src/lib/gen/` after `task proto:gen`:** empty (no drift after regeneration and commit).

**Task 2 store.go comment-only commit-range criterion:**
```
$ git diff --unified=0 <TASK2_BASE> -- internal/store/store.go | rg '^[+-]' | rg -v '^(\+\+\+|---)'
-	// content was egressed to the summarizer model (auto path only). Store-only;
-	// not on the Connect wire. Zero if never egressed or the summary was
-	// client-authored/cleared.
+	// content was egressed to the summarizer model (auto path only). Plain
+	// json tag, visible on both the MCP and Connect wires. Zero if never
+	// egressed or the summary was client-authored/cleared.
```
Non-empty (passes the mandatory non-empty check), and `rg -v '^[+-][[:space:]]*//'` against this range returns no matches (exit 1) — every changed line is a comment line, confirming the comment-only edit.

**Plan-level `<verification>` commands, all run and green:**
- `task proto:lint` — clean.
- `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` — exits 0.
- `task proto:gen` then `git status --porcelain gen/ ui/src/lib/gen/` — empty.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1` — `ok`. (One `--- SKIP: TestSharedQdrantAddressHonored` observed under `-v`; this is a pre-existing, unrelated skip gated on `ENGRAM_QDRANT_TEST_ADDR`, not on Qdrant reachability — Qdrant itself was present via a booted testcontainer, and no test skipped for its absence.)
- `go test ./internal/store/ -count=1` — `ok`.
- `go test ./cmd/engram/ -count=1` — `ok`.
- `task lint` — all checks passed (markdown, go, actions, yaml, python).
- `task license:check` — 334 valid, 0 invalid.
- `task ui:build` — succeeded; resulting drift in `internal/webauth/static/` committed (see Deviations above).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The Connect `Memory` wire now carries all eight record-state fields; `memoryToProto` is the single funnel populating them for `GetMemory`, `ListMemories`, and `SearchMemories` alike (via `memoriesToProto`/`shapeProtoMemories`), so no second mapping call site was needed.
- Plan 05-02 (per this plan's `<output>` note) owns: the exhaustive parity/negative-fixture detector proving no `store.Memory` field is silently missing from the wire, and the zero-value assign-always gate (D-14 §3) with its own RED proof — this plan seeded all eight fields non-zero and does not claim that property.
- Plan 05-03 owns the `renderJSON`/CLI JSON-rendering assertions for `schema_version`'s zero-value visibility.
- No blockers.

---
*Phase: 05-connect-record-state-parity*
*Completed: 2026-08-15*
