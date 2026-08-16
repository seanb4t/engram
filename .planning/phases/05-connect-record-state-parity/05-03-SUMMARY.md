---
phase: 05-connect-record-state-parity
plan: 03
subsystem: api
tags: [connect, protojson, go, testing]

# Dependency graph
requires:
  - phase: 05-01
    provides: "engramv1.Memory fields 23-30 with D-14 explicit presence; memoryToProto extended to populate all eight"
provides:
  - "TestBoundarySecondReadLaneIdentity — proves a sub-second scheduling bound written once through Connect ScheduleMemory comes back outward-widened and identical on the MCP and Connect read lanes, with a RED proof against a not_before json-tag rename"
  - "TestClientJSONSchemaVersionZeroVisible — proves engram list --output json renders schema_version:0 for an ASSIGNED-zero field and OMITS the key for an UNASSIGNED field (permanent negative fixture), plus number-not-string rendering and Timestamp-absent-not-null behavior"
affects: []

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 4468
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "json.Marshal(got) -> map[string]json.RawMessage key-presence-then-decode discipline for MCP-lane wire assertions, mirrored on the Connect side via .AsTime() and on the CLI side via renderJSON's decoded output — never a struct-field or substring comparison"
    - "Permanent negative fixture pairing (assigned-zero vs unassigned, identical harness) to prove a presence assertion is capable of failing, not merely satisfiable by an unconditional renderer"

key-files:
  created:
    - internal/server/connectapi_boundary_second_test.go
    - cmd/engram/client_schemaversion_json_test.go
  modified: []

key-decisions: []

patterns-established:
  - "A future MCP-lane wire assertion for a store.Memory field should marshal the returned value into map[string]json.RawMessage and assert key presence before decoding — never assert on the Go struct field alone, which stays green through a json:\"-\", omitempty, or tag-rename regression."

requirements-completed: [REQ-connect-record-state-parity, REQ-connect-parity-roundtrip-proof]

coverage:
  - id: D1
    description: "A not_before submitted with a sub-second component comes back floored to the containing whole second and a not_after comes back ceiled to it, identically on the MCP and Connect read lanes, from one write; a bound already on a whole second comes back unchanged; the MCP lane is read out of the record's serialized json form, proven RED against a not_before json-tag rename."
    requirement: "REQ-connect-parity-roundtrip-proof"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_boundary_second_test.go#TestBoundarySecondReadLaneIdentity"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram list --output json renders schema_version:0 for an ASSIGNED-zero Memory field (the state memoryToProto produces for a v0 record under D-14 §3) as a JSON number, and OMITS the key for an UNASSIGNED field — the permanent negative fixture proving the presence assertion can fail. Number-not-string rendering and Timestamp-absent-not-null behavior also pinned. Renderer half and failure shape only (D-03/SC1); the mapper half is plan 05-02's."
    requirement: "REQ-connect-record-state-parity"
    verification:
      - kind: unit
        ref: "cmd/engram/client_schemaversion_json_test.go#TestClientJSONSchemaVersionZeroVisible"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-16
status: complete
---

# Phase 05 Plan 03: Read-Lane Boundary-Second Identity & CLI schema_version Visibility Summary

**Two read-side proof tests — no production code changes: a Qdrant-backed sub-second scheduling-bound round trip proving MCP/Connect read-lane identity with a RED proof against a json-tag rename, and a CLI `renderJSON` test pinning `schema_version`'s presence-vs-absence contract with a permanent negative fixture.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-08-15T19:39:33Z (worktree base commit 886d0110)
- **Completed:** 2026-08-16T00:14:07Z
- **Tasks:** 2
- **Files modified:** 2 (both new test files; zero production files)

## Accomplishments

- `TestBoundarySecondReadLaneIdentity` (`internal/server/connectapi_boundary_second_test.go`): writes one record through the real Connect `ScheduleMemory` handler with a `T + 500ms` `not_before`/`not_after`, reads it back on both the MCP lane (via `d.getMemory`, marshalled through its json tags per the review-cycle-1 HIGH fix) and the Connect lane (via `api.GetMemory`'s `.AsTime()`), and asserts both come back outward-widened to the containing whole second and identical to each other. A second sub-test proves an already-whole-second bound is unchanged on both lanes. Expected values are computed via `time.Truncate`/`Add` arithmetic independent of the production `formatWindowBound`/`windowBoundFloor`/`windowBoundCeil` helpers, so the assertion cannot be a tautology.
- **RED proof performed and recorded** (see Acceptance Criteria Evidence): with `store.Memory.NotBefore`'s json tag temporarily renamed to `not_before_tmp_probe`, the test fails on the missing `not_before` key — a struct-field comparison (the cycle-1 shape) would have stayed green under this exact mutation. The tag was reverted and `git status --porcelain internal/store/store.go` confirmed empty.
- `TestClientJSONSchemaVersionZeroVisible` (`cmd/engram/client_schemaversion_json_test.go`): four sub-tests against `engram list --output json` through the real stub Connect handler. (1) `SchemaVersion: proto.Uint32(0)` renders `"schema_version":0`. (2) The permanent negative fixture — an otherwise-identical stub `Memory` (same `ShortId`, same `Scope`, same harness) leaving `SchemaVersion` nil — asserts the key is absent, proving sub-test (1)'s presence assertion is capable of failing. (3) A non-zero `SchemaVersion` renders as an unquoted JSON number, pinning `uint32`-over-`uint64` semantics. (4) The four new Timestamp fields (`not_before`, `not_after`, `archived_at`, `summary_egress_at`) are asserted absent, never present-and-null, when unset.
- Both tests were written under D-14: task 2's cycle-1..3 instruction ("do NOT assign a literal 0") was inverted per the `<d14_amendment>` — under D-14, `SchemaVersion`'s Go zero value is a nil `*uint32`, so the assigned-zero sub-test must explicitly assign `proto.Uint32(0)` to mirror what `memoryToProto` now produces for a v0 record.

## Task Commits

Each task was committed atomically:

1. **Task 1: Boundary-second read-lane identity (D-09)** - `581f20fe` (test)
2. **Task 2: Gate the operator-visible schema_version rendering, presence and absence (D-03, D-14)** - `08f23b29` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `internal/server/connectapi_boundary_second_test.go` — new: `TestBoundarySecondReadLaneIdentity`, `boundarySecondScope` (per-run unique scope helper), `mcpWireBounds` (json-marshal-then-decode MCP-lane helper)
- `cmd/engram/client_schemaversion_json_test.go` — new: `TestClientJSONSchemaVersionZeroVisible`, `decodeFirstMemory` (stub-server-then-decode helper)

## Decisions Made

None made in this plan. D-14's amendment to task 2's fixture shape (assign `proto.Uint32(0)` explicitly rather than leaving the Go zero value, per `<d14_amendment>` in the plan) was authored into the plan before execution — this plan implemented that restructure, it did not decide it.

## Deviations from Plan

None — plan executed exactly as written. No auto-fixes were needed; both tests passed on first write (after the deliberate RED-proof mutation/revert cycle for task 1, which is a mandated part of the plan's own action, not a deviation).

## Issues Encountered

None.

## Acceptance Criteria Evidence

**PLAN_BASE:** `886d0110942506d371bfc252daf316501111df50` (worktree base commit — nothing committed between plan start and task 1).

**Task 1 — no read-path rounding helper referenced (source, not tautology):**
```
$ rg -c 'formatWindowBound|windowBoundFloor|windowBoundCeil' internal/server/connectapi_boundary_second_test.go
(no matches, exit 1)
```

**Task 1 — MCP-lane json-marshal discipline:**
```
$ rg -c 'json.Marshal\(' internal/server/connectapi_boundary_second_test.go
1
$ rg -c 'map\[string\]json.RawMessage' internal/server/connectapi_boundary_second_test.go
2
```

**Task 1 — observed sub-second-bound sub-test (illustrative rerun; the committed test itself computes its own base from `time.Now()+48h`, so exact wall-clock values differ run to run — arithmetic is identical):**
```
input not_before: 2026-08-18T00:14:50.5Z
input not_after:  2026-08-19T00:14:50.5Z
Connect lane  not_before: 2026-08-18T00:14:50Z (floored)
Connect lane  not_after:  2026-08-19T00:14:51Z (ceiled)
MCP lane      not_before: 2026-08-18T00:14:50Z (floored)
MCP lane      not_after:  2026-08-19T00:14:51Z (ceiled)
```
Both lanes agree exactly with each other and with the outward-widened expected values, computed independently via `time.Truncate(time.Second)` / conditional `+1s` — not via the production `formatWindowBound` helper.

**Task 1 — RED PROOF (mandatory, performed and reverted):**
With `internal/store/store.go`'s `NotBefore` field's json tag temporarily changed from `` `json:"not_before,omitempty"` `` to `` `json:"not_before_tmp_probe,omitempty"` ``:
```
$ ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestBoundarySecondReadLaneIdentity$' -count=1 -v
=== RUN   TestBoundarySecondReadLaneIdentity/sub-second_bound_rounds_outward_and_both_lanes_agree
    connectapi_boundary_second_test.go:141: MCP get_memory wire is missing the not_before member: {"id":"ce4031fe-70ac-4ba3-af61-6d70b8524a2e", ... "not_before_tmp_probe":"2026-08-18T00:10:36Z","not_after":"2026-08-19T00:10:37Z", ...}
=== RUN   TestBoundarySecondReadLaneIdentity/exact-whole-second_bound_is_unchanged_on_both_lanes
    connectapi_boundary_second_test.go:229: MCP get_memory wire is missing the not_before member: {...}
--- FAIL: TestBoundarySecondReadLaneIdentity (0.09s)
    --- FAIL: TestBoundarySecondReadLaneIdentity/sub-second_bound_rounds_outward_and_both_lanes_agree (0.00s)
    --- FAIL: TestBoundarySecondReadLaneIdentity/exact-whole-second_bound_is_unchanged_on_both_lanes (0.00s)
FAIL
```
A struct-field comparison (`got.NotBefore` directly, the cycle-1 shape) would have stayed GREEN under this exact mutation — the stored value and the Go field are untouched by a json-tag rename; only the serialized wire shape changes. After reverting the tag: `git status --porcelain internal/store/store.go` returned empty, and the test passed again:
```
$ ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestBoundarySecondReadLaneIdentity$' -count=1 -v
--- PASS: TestBoundarySecondReadLaneIdentity (0.11s)
    --- PASS: TestBoundarySecondReadLaneIdentity/sub-second_bound_rounds_outward_and_both_lanes_agree (0.00s)
    --- PASS: TestBoundarySecondReadLaneIdentity/exact-whole-second_bound_is_unchanged_on_both_lanes (0.00s)
PASS
```

**Task 1 — full suites, no `--- SKIP` among new functions:**
```
$ ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1
ok  	github.com/seanb4t/engram/internal/server	7.466s
$ go test ./internal/store/ -count=1
ok  	github.com/seanb4t/engram/internal/store	23.252s
```

**Task 2 — no substring-on-stdout assertion, presence via decoded map:**
```
$ rg -c 'strings.Contains' cmd/engram/client_schemaversion_json_test.go
(no matches, exit 1)
$ rg -c 'proto\.Uint32\(' cmd/engram/client_schemaversion_json_test.go
3
```
(3 = two literal fixture assignments — `proto.Uint32(0)`, `proto.Uint32(7)` — plus one occurrence inside a doc comment discussing the first; the criterion's "at least 2" requires only the two fixture assignments.)

**Task 2 — `renderJSON` untouched:**
```
$ git status --porcelain cmd/engram/client_common.go
(empty)
```

**Task 2 — both rendered JSON documents verbatim** (reproduced via a throwaway `protojson.MarshalOptions{UseProtoNames:true, EmitDefaultValues:true, Multiline:false}` probe with the exact fixtures the test builds — not part of the committed diff):

Assigned-zero (`SchemaVersion: proto.Uint32(0)`):
```json
{"id":"", "content":"", "scope":"repo:x", "repo":"", "workspace":"", "worktree":"", "base_dir":"", "source":"", "category":"", "tags":[], "actor":"", "owner":"", "visibility":"", "summary":"", "summary_source":"", "score":0, "short_id":"AAAA111111", "access_count":"0", "kind":"", "citations":[], "supersedes":[], "schema_version":0}
```

Unassigned (`SchemaVersion` left nil):
```json
{"id":"", "content":"", "scope":"repo:x", "repo":"", "workspace":"", "worktree":"", "base_dir":"", "source":"", "category":"", "tags":[], "actor":"", "owner":"", "visibility":"", "summary":"", "summary_source":"", "score":0, "short_id":"AAAA111111", "access_count":"0", "kind":"", "citations":[], "supersedes":[]}
```
The `schema_version` key is present-and-`0` in the first document and entirely absent in the second — confirming `EmitDefaultValues` makes no difference to this `optional` field either way (D-14). **This task never invokes `memoryToProto`** — the stub builds the `Memory` by hand — so it proves the renderer's behavior and failure shape only, not that the mapper assigns the field for a real v0 record. That half is plan 05-02's zero-value sub-test and its RED PROOF 3.

**Task 2 — full suite and lint:**
```
$ go test ./cmd/engram/ -run '^TestClientJSONSchemaVersionZeroVisible$' -count=1 -v
--- PASS: TestClientJSONSchemaVersionZeroVisible (0.01s)
    --- PASS: .../assigned-zero_schema_version_renders_as_0 (0.00s)
    --- PASS: .../unassigned_schema_version_is_OMITTED_-_the_permanent_negative_fixture (0.00s)
    --- PASS: .../schema_version_renders_as_a_JSON_number,_not_a_string (0.00s)
    --- PASS: .../unset_scheduling_bounds_are_absent,_not_null (0.00s)
PASS
$ go test ./cmd/engram/ -count=1
ok  	github.com/seanb4t/engram/cmd/engram	2.326s
$ task lint:go
0 issues.
```

**Plan-level `<verification>` — no production file touched (in-task revert evidence):**
```
$ git status --porcelain internal/server/protoconv.go internal/store/store.go cmd/engram/client_common.go
(empty)
```

**Commit-range allowlist over PLAN_BASE (the assertion that actually proves no production file was touched, per `<commit_range_protocol>` — the bare `git diff --name-only` post-commit is unsatisfiable, per `<review_cycle_3_incorporation>`):**
```
$ git diff --name-only 886d0110942506d371bfc252daf316501111df50
cmd/engram/client_schemaversion_json_test.go
internal/server/connectapi_boundary_second_test.go
```
Non-empty (passes the mandatory non-empty check); both this plan's owned files are present; `rg -v '^(internal/server/connectapi_boundary_second_test\.go|cmd/engram/client_schemaversion_json_test\.go|\.planning/)'` over the list returned no matches (exit 1) — no file under `internal/store/`, no `internal/server/protoconv.go`, no `cmd/engram/client_common.go`, and no production file anywhere appears in this plan's whole change.

**Plan-level license check:**
```
$ task license:check
Totally checked 1517 files, valid: 336, invalid: 0, ignored: 1181, fixed: 0
```

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SC3's read-lane-identity property is now observed, not merely asserted: a Qdrant-backed round trip with a RED proof, no read-path rounding code added.
- D-03/SC1's CLI-visible `schema_version` presence contract is pinned at the renderer layer with a permanent negative fixture. The mapper-side half of the operator guarantee (`memoryToProto` assigning the field for a real v0 record) is gated by plan 05-02's zero-value sub-test — tracked as #499, a known accepted gap (closing it needs a real-server CLI harness, out of scope here).
- No production code was changed by this plan (D-09's constraint held throughout).
- No blockers.

## Self-Check: PASSED

- FOUND: `internal/server/connectapi_boundary_second_test.go`
- FOUND: `cmd/engram/client_schemaversion_json_test.go`
- FOUND: `.planning/phases/05-connect-record-state-parity/05-03-SUMMARY.md`
- FOUND commit `581f20fe` (Task 1)
- FOUND commit `08f23b29` (Task 2)

---
*Phase: 05-connect-record-state-parity*
*Completed: 2026-08-16*
