---
phase: 05-connect-record-state-parity
plan: 02
subsystem: testing
tags: [protobuf, connect, reflection, go, parity-testing]

# Dependency graph
requires:
  - phase: 05-01
    provides: "engramv1.Memory fields 23-30 with D-14 explicit presence; memoryToProto extended to populate all eight new fields"
provides:
  - "unmappedStoreFields — the single shared detector proving every json-visible store.Memory field maps onto engramv1.Memory by exact byte equality or the one declared rename"
  - "A permanent negative fixture and a near-miss fixture proving the detector can reject and never fuzzy-matches"
  - "autoFillMemory — a reflection auto-filled store.Memory with every json-visible field pairwise-distinct, never a hand-keyed literal"
  - "assertDecodeBackCoversAllFields — the decode-back comparator's own exhaustiveness gate"
  - "TestConnectMemoryFieldsPopulated — proves every proto field reports Has()==true, every value decodes back to its source, supersedes order survives, memoryToProto does not mutate its input, and D-14 §3's assign-always guarantee on schema_version/summary_model for a zero-value source"
affects: [05-03]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 6767
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Shared reflection detector (jsonschemaExposedFields idiom, surfaces_test.go) extended to a store-json-to-proto-field-name detector, driven by json:\"-\" as the sole exclusion mechanism plus a single explicit rename map"
    - "Reflection auto-fill fixture (D-06): a store.Memory populated entirely via reflect.Value setters keyed off each field's own name/position, never a hand-built composite literal, so every future field is covered by construction"
    - "Inline decode-back comparator with a self-asserted exhaustiveness gate (assertDecodeBackCoversAllFields), closing the cycle-1 gap where a comparator covering a subset of fields was invisible to descriptor-exhaustive detectors"

key-files:
  created:
    - internal/server/connectapi_parity_test.go
  modified: []

key-decisions:
  - "No decisions made in this plan — D-14 (presence model) and all four review-cycle-incorporated gate designs were fully specified in 05-02-PLAN.md prior to execution. This plan implemented the plan as written."

patterns-established:
  - "A future record-state field on store.Memory must appear, byte-for-byte or via storeToProtoFieldAlias, in engramv1.Memory's field-name set, or TestConnectMemoryParityDetector fails naming it — no manual audit required."
  - "A future explicit-presence (optional) scalar's assign-always requirement is provable only by a dedicated zero-value Has(fd)/pointer-level sub-test; Has(fd) on the auto-filled (non-zero) fixture cannot distinguish unconditional from conditional assignment."

requirements-completed: [REQ-connect-record-state-parity, REQ-connect-parity-roundtrip-proof]

coverage:
  - id: D1
    description: "unmappedStoreFields exists exactly once, is called by the real assertion, the permanent negative fixture, the near-miss fixture, and the determinism check; it rejects by exact byte equality only (no prefix/substring/case matching), returns sorted output, and is pinned by a whole-map-equality assertion on the one-entry alias map."
    requirement: "REQ-connect-record-state-parity"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_parity_test.go#TestConnectMemoryParityDetector"
        status: pass
    human_judgment: false
  - id: D2
    description: "A reflection auto-filled store.Memory (every json-visible field pairwise-distinct, including repeated and message-valued fields) maps to a proto message on which every field reports Has()==true, decodes back to its source under an exhaustiveness-gated comparator, and preserves supersedes order."
    requirement: "REQ-connect-parity-roundtrip-proof"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_parity_test.go#TestConnectMemoryFieldsPopulated"
        status: pass
    human_judgment: false
  - id: D3
    description: "For a zero-value store.Memory, memoryToProto still ASSIGNS schema_version and summary_model (Has(fd) true) while leaving superseded_by unassigned (Has(fd) false) — D-14 §3's assign-always guarantee, provable only via a dedicated sub-test since the auto-filled fixture's non-zero source cannot distinguish unconditional from conditional assignment."
    requirement: "REQ-connect-parity-roundtrip-proof"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_parity_test.go#TestConnectMemoryFieldsPopulated/zero-value_source:_timestamps_unset,_optional_scalars_still_assigned"
        status: pass
    human_judgment: false

duration: ~15min
completed: 2026-08-16
status: complete
---

# Phase 05 Plan 02: Connect Record-State Parity (Exhaustive Field-Mapping Proof) Summary

**Built the shared reflection detector, permanent negative/near-miss fixtures, reflection auto-fill population fixture, and inline decode-back comparator that together prove no `store.Memory` field can ever silently fall off the Connect wire — with all five anti-vacuity gates observed going RED against a deliberately introduced defect and reverted.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-08-16T00:04:37Z (worktree base commit, = PLAN_BASE)
- **Completed:** 2026-08-16T00:19:16Z
- **Tasks:** 2
- **Files modified:** 1 new test file

**PLAN_BASE:** `886d0110942506d371bfc252daf316501111df50` (per `<commit_range_protocol>`; recorded before any edit).

## Accomplishments

- `storeJSONVisibleFields` mirrors the established `jsonschemaExposedFields` idiom (surfaces_test.go) to derive the store's wire-visible json-tag name set from `reflect.VisibleFields`, never a hand-maintained list.
- `storeToProtoFieldAlias` records the sole store→proto name divergence (`worktree_path` → `worktree`) and is pinned by whole-map equality — width AND content in one assertion — so a second entry fails the suite, closing the cycle-1 HIGH finding that source-occurrence counts could not detect a widening.
- `unmappedStoreFields` — the single shared detector — resolves each store json name through the alias map (if present), then requires exact byte-equal membership in the real `engramv1.Memory` descriptor's field-name set. No case folding, prefix, or substring matching is applied.
- `negativeFixtureMemory` (permanent negative fixture) and `nearMissFixtureMemory` (prefix/substring/case-variant fixture) both route through the same detector, proving it can reject and never fuzzy-matches.
- `TestConnectMemoryParityDetector` — flatness, field-accounting, alias-map-width, real-mapping, negative-fixture, near-miss, and determinism sub-tests, all green with no skip.
- `autoFillMemory` — D-06's reflection auto-fill: walks every visible `store.Memory` field via `reflect.Value` setters keyed off the field's own name/position, producing a fully populated, pairwise-distinct fixture with no hand-built composite literal.
- `assertDecodeBackCoversAllFields` — the decode-back comparator's own exhaustiveness gate, closing the cycle-1 HIGH finding that a comparator covering 25/30 fields was invisible to both the name detector and the `Has(fd)` population loop.
- `TestConnectMemoryFieldsPopulated` — auto-fill coverage/distinctness guards, `Has(fd)` population proof, the exhaustiveness-gated decode-back comparator, supersedes-order proof, non-mutation proof, and the D-14 §3 zero-value assign-always proof (pointer/`Has(fd)`-level, never through the nil-safe `Get*()` accessors).

## Task Commits

Each task was committed atomically:

1. **Task 1: The shared name detector, its rename map, and the permanent negative fixture (D-05, D-07)** - `147b83da` (test)
2. **Task 2: Reflection auto-fill population fixture and inline decode-back comparison (D-06, D-08)** - `3edb4bce` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `internal/server/connectapi_parity_test.go` — new: the shared detector, rename map, permanent negative fixture, near-miss fixture, `TestConnectMemoryParityDetector`, the reflection auto-fill, the decode-back exhaustiveness gate, and `TestConnectMemoryFieldsPopulated`.

## Decisions Made

None — D-14 and all review-cycle gate designs were fully specified in `05-02-PLAN.md` before execution; this plan implemented, not decided.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed a literal occurrence of `shapeProtoMemories` from a doc comment**
- **Found during:** Task 2, `task lint:go`/acceptance-criteria pass
- **Issue:** The acceptance criterion `rg -c 'shapeProtoMemories' internal/server/connectapi_parity_test.go` requires zero matches (the population assertion must call `memoryToProto` directly, never the shaper). A doc comment on the "every proto field is populated" sub-test named `shapeProtoMemories` literally in prose, which the criterion's grep matches regardless of code-vs-comment context.
- **Fix:** Reworded the comment to describe the same fact ("the non-full recall shaper further down connectapi.go") without the literal symbol name.
- **Files modified:** `internal/server/connectapi_parity_test.go`
- **Verification:** `rg -c 'shapeProtoMemories' internal/server/connectapi_parity_test.go` exits 1 (no matches); full suite still green.
- **Committed in:** `3edb4bce` (part of Task 2 commit)

**2. [Rule 1 - Bug] `golangci-lint` govet/prealloc fixes**
- **Found during:** Task 2, `task lint:go`
- **Issue:** `govet`'s `inline` analyzer flagged two uses of the deprecated `reflect.Ptr` constant (should be `reflect.Pointer`); `prealloc` flagged the `compared` accumulator slice in the decode-back sub-test as growable-with-known-capacity.
- **Fix:** Replaced both `reflect.Ptr` occurrences with `reflect.Pointer`; changed `var compared []string` to `compared := make([]string, 0, 30)`.
- **Files modified:** `internal/server/connectapi_parity_test.go`
- **Verification:** `task lint:go` reports `0 issues`; full suite still green.
- **Committed in:** `3edb4bce` (part of Task 2 commit)

**3. [Rule 1 - Bug] Clarified `summary_model` decode-back error message**
- **Found during:** Task 2, while performing RED PROOF 2
- **Issue:** The original `t.Errorf("summary_model: got %v want %q", msg.SummaryModel, m.SummaryModel)` printed the raw pointer address (`0x...`) rather than the dereferenced string when the values differed, making the captured RED-proof output for the SUMMARY hard to read.
- **Fix:** Split into an explicit nil-check branch and a dereferenced-value branch so a mismatch prints both strings.
- **Files modified:** `internal/server/connectapi_parity_test.go`
- **Verification:** Re-ran RED PROOF 2; output now reads `summary_model: got "val-Summary" want "val-SummaryModel"`.
- **Committed in:** `3edb4bce` (part of Task 2 commit)

---

**Total deviations:** 3 auto-fixed (all Rule 1 — no scope, behavior, or API surface added beyond the plan's own specification).
**Impact on plan:** All three were necessary for the acceptance criteria / lint gate to pass and for the RED-proof evidence to be legible. No scope creep.

## Issues Encountered

None beyond the deviations above.

## RED Proof Evidence (mandatory per `<anti_vacuity_contract>`)

All five RED proofs were performed by the executor, observed failing, and reverted before the relevant task's commit. `git status --porcelain internal/server/connectapi.go internal/store/store.go` was empty at every task's end.

### Task 1, RED PROOF 1 — store-side gap

Temporarily added `TmpProbe string \`json:"tmp_probe_field"\`` to `store.Memory`.

```
$ go test ./internal/server/ -run '^TestConnectMemoryParityDetector$' -count=1
--- FAIL: TestConnectMemoryParityDetector (0.00s)
    --- FAIL: TestConnectMemoryParityDetector/every_json-visible_store_field_is_mapped (0.00s)
        connectapi_parity_test.go:171: unmapped store.Memory fields: [tmp_probe_field]
FAIL
```

Reverted; `go test ./internal/server/ -run '^TestConnectMemoryParityDetector$' -count=1` passed again.

### Task 1, RED PROOF 2 — alias-map widening

Temporarily added a second entry (`"tmp_alias_probe": "id"`) to `storeToProtoFieldAlias`.

```
$ go test ./internal/server/ -run '^TestConnectMemoryParityDetector$' -count=1 -v
    --- FAIL: TestConnectMemoryParityDetector/alias_map_is_exactly_one_entry (0.00s)
        connectapi_parity_test.go:165: storeToProtoFieldAlias = map[tmp_alias_probe:id worktree_path:worktree], want map[worktree_path:worktree]
    --- PASS: TestConnectMemoryParityDetector/every_json-visible_store_field_is_mapped (0.00s)
```

`every_json-visible_store_field_is_mapped` stayed GREEN under this same mutation — the exact asymmetry the whole-map-equality gate exists to close (without it the widening leaves no trace). Reverted; suite green again.

### Task 2, RED PROOF 1 — omission

Temporarily deleted the `SchemaVersion: proto.Uint32(uint32(m.SchemaVersion))` assignment from `memoryToProto` in `internal/server/connectapi.go`.

```
$ go test ./internal/server/ -run '^TestConnectMemoryFieldsPopulated$' -count=1
--- FAIL: TestConnectMemoryFieldsPopulated (0.00s)
    --- FAIL: TestConnectMemoryFieldsPopulated/every_proto_field_is_populated (0.00s)
        connectapi_parity_test.go:430: proto fields not populated: [schema_version]
    --- FAIL: TestConnectMemoryFieldsPopulated/values_decode_back_to_their_source (0.00s)
        connectapi_parity_test.go:599: schema_version: got <nil> want 32
    --- FAIL: TestConnectMemoryFieldsPopulated/zero-value_source:_timestamps_unset,_optional_scalars_still_assigned (0.00s)
        connectapi_parity_test.go:676: schema_version pointer: got nil, want non-nil (assigned zero)
FAIL
```

`TestConnectMemoryParityDetector` stayed GREEN under this same mutation — the proto field name still exists, only the assignment is gone, which is exactly the failure mode the task-1 name detector does NOT catch. Reverted; suite green again.

### Task 2, RED PROOF 2 — wrong-but-nonzero cross-wire

Temporarily changed the `SummaryModel` wrap's argument from `m.SummaryModel` to `m.Summary`.

```
$ go test ./internal/server/ -run '^TestConnectMemoryFieldsPopulated$' -count=1 -v
    --- FAIL: TestConnectMemoryFieldsPopulated/values_decode_back_to_their_source (0.00s)
        connectapi_parity_test.go:586: summary_model: got "val-Summary" want "val-SummaryModel"
    --- PASS: TestConnectMemoryFieldsPopulated/every_proto_field_is_populated (0.00s)
```

Confirmed separately: `TestConnectMemoryParityDetector` also stayed GREEN (`ok`). Both the name detector and the `Has(fd)` population assertion stayed green under this mutation — a populated, compiling, wrong mapping is invisible to both; only the decode-back comparator catches it. Reverted; suite green again.

### Task 2, RED PROOF 3 — conditional assignment (the D-14 §3 mutation)

Temporarily guarded `schema_version`'s assignment behind `if m.SchemaVersion != 0`, mirroring the neighbouring Timestamp guards (a local `*uint32` referenced from the composite literal in place of the direct wrap).

```
$ go test ./internal/server/ -run '^TestConnectMemoryFieldsPopulated$' -count=1 -v
    --- FAIL: TestConnectMemoryFieldsPopulated/zero-value_source:_timestamps_unset,_optional_scalars_still_assigned (0.00s)
        connectapi_parity_test.go:678: schema_version pointer: got nil, want non-nil (assigned zero)
    --- PASS: TestConnectMemoryFieldsPopulated/every_proto_field_is_populated (0.00s)
    --- PASS: TestConnectMemoryFieldsPopulated/values_decode_back_to_their_source (0.00s)
```

Confirmed separately: `TestConnectMemoryParityDetector` also stayed GREEN (`ok`). The name detector, the `every_proto_field_is_populated` `Has(fd)` loop, AND the decode-back comparator ALL stayed green under this mutation — the auto-filled source is non-zero, so a conditional assignment still fires for every other gate. Only the dedicated zero-value sub-test can see it. Reverted; suite green again.

## Which Gate Each Mutation Trips

| Mutation | Name detector (`TestConnectMemoryParityDetector`) | `Has(fd)` population (`every_proto_field_is_populated`) | Decode-back comparator (`values_decode_back...`) | Zero-value sub-test |
|---|---|---|---|---|
| Task 1: store-side gap (untracked field) | **FAIL** | n/a | n/a | n/a |
| Task 1: alias-map widening (2nd entry) | green (`every_json-visible...` sub-test) — **`alias_map_is_exactly_one_entry` FAILS** | n/a | n/a | n/a |
| Task 2: omission (deleted `SchemaVersion` wrap) | green | **FAIL** | **FAIL** | **FAIL** |
| Task 2: wrong-but-nonzero cross-wire (`SummaryModel` fed from `Summary`) | green | green | **FAIL** | (not exercised — mutation is on a different field) |
| Task 2: conditional assignment (D-14 §3) | green | green | green | **FAIL** |

The conditional-assignment mutation is the sharpest finding: under D-14's explicit presence, a future author copying the neighbouring Timestamp-guard idiom onto `schema_version` would pass every gate in this file except the one dedicated zero-value sub-test — and would silently drop the `schema_version` key from every rendered JSON document for a pre-migration (v0) record, which is exactly the guarantee D-14 §3 exists to make.

## Acceptance Criteria Evidence

**Task 1:**
```
$ rg -c '^func unmappedStoreFields\(' internal/server/
internal/server/connectapi_parity_test.go:1
$ rg -c 'unmappedStoreFields\(' internal/server/connectapi_parity_test.go
6
$ rg -c 'strings.Contains|strings.EqualFold|strings.HasPrefix|strings.ToLower' internal/server/connectapi_parity_test.go
(no matches, exit 1)
```

**Task 2:**
```
$ rg -c 'shapeProtoMemories' internal/server/connectapi_parity_test.go
(no matches, exit 1)
$ rg -c 'func autoFillMemory\(' internal/server/connectapi_parity_test.go
1
$ rg -c 'ProtoReflect\(\).Has\(' internal/server/connectapi_parity_test.go
4
$ rg -c '^func assertDecodeBackCoversAllFields\(' internal/server/connectapi_parity_test.go
1
$ rg -c 'assertDecodeBackCoversAllFields\(' internal/server/connectapi_parity_test.go
2
$ rg -U -c 'store\.Memory\{\s*[A-Z]' internal/server/connectapi_parity_test.go
(no matches, exit 1)
```

**Commit-range allowlist (post both tasks, PLAN_BASE-anchored):**
```
$ git diff --name-only 886d0110942506d371bfc252daf316501111df50
internal/server/connectapi_parity_test.go
```
Non-empty (a): OK. Contains the plan's own file (b): OK. `rg -v '^(internal/server/connectapi_parity_test\.go|\.planning/)'` over it: no matches, exit 1 (c) — no production file appears in this plan's whole change, committed or not.

**Plan-level `<verification>` commands, all run and green:**
- `go test ./internal/server/ -count=1` — `ok`, no skipped tests in the two new test functions.
- `go test ./internal/store/ -count=1` — `ok`.
- `go test ./cmd/engram/ -count=1` — `ok`.
- `task lint:go` — `0 issues` (one unrelated warning about a sibling parallel-executor worktree's file, not this plan's file).
- `task license:check` — 335 valid, 0 invalid.
- `git status --porcelain internal/server/connectapi.go internal/store/store.go` — empty at both task ends.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every `store.Memory` field is now provably present on the Connect wire, with a detector that fails the moment a future field is added without a corresponding proto field or explicit alias/exclusion.
- **Stated honestly, per this plan's own `<output>` instruction:** this plan's zero-value gate covers the MAPPER only (`memoryToProto` against a real `store.Memory{}`). Plan 05-03's CLI test renders a stub-built message and therefore characterizes the JSON renderer, not the mapper. No test in this phase spans the mapper→renderer seam — the operator-visible `schema_version` guarantee ("an operator asking a v0 record's version always gets a key") is the CONJUNCTION of this plan's gate and 05-03's, not a single proof. Tracked as **#499** (per the plan's own note): a real-server CLI harness in `cmd/engram` would close this, out of scope here.
- No new production symbol was added by this plan; `internal/server/connectapi.go` and `internal/store/store.go` were only touched transiently during the five RED proofs, and both are confirmed clean (`git status --porcelain`, commit-range allowlist) at every commit.
- No blockers for plan 05-03.

## Self-Check: PASSED

- FOUND: `internal/server/connectapi_parity_test.go`
- FOUND: `.planning/phases/05-connect-record-state-parity/05-02-SUMMARY.md`
- FOUND commit `147b83da` (Task 1)
- FOUND commit `3edb4bce` (Task 2)

---
*Phase: 05-connect-record-state-parity*
*Completed: 2026-08-16*
