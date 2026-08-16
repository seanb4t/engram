---
phase: 05-connect-record-state-parity
reviewed: 2026-08-15T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - proto/engram/v1/engram.proto
  - internal/server/connectapi.go
  - internal/store/store.go
  - internal/server/connectapi_recordstate_handler_test.go
  - internal/server/connectapi_parity_test.go
  - internal/server/connectapi_boundary_second_test.go
  - internal/server/connectdescriptor_test.go
  - cmd/engram/client_schemaversion_json_test.go
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-08-15T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Phase 05 adds `engramv1.Memory` fields 23-30 and the hand-written `memoryToProto`
mapping for them (`internal/server/connectapi.go`). The actual code diff for this
phase is small and surgical: `proto/engram/v1/engram.proto` (field declarations),
`internal/server/connectapi.go` (the mapping block, +18 lines net of comments), and
a doc-comment-only change in `internal/store/store.go` (no logic change — confirmed
via `git diff 059807ab..HEAD -- internal/store/store.go`). Everything else in scope
is new test code.

**D-14 conformance (review priority 1):** verified field-by-field against `memoryToProto`
(`internal/server/connectapi.go:49-106`). Every one of the eight fields follows its
mandated rule exactly, not a uniform style:
- `superseded_by` (23, `optional string`): direct pointer copy (`SupersededBy: m.SupersededBy`) — nil source stays unset. Correct.
- `supersedes` (24, `repeated`): direct slice assign, no presence concept. Correct.
- `not_before`/`not_after`/`archived_at`/`summary_egress_at` (25-27, 30, `Timestamp`): each guarded by a nil-pointer (or `IsZero()` for the non-pointer `SummaryEgressAt`) check before `timestamppb.New(...)`, so an unset source never emits a year-1 Timestamp. Correct.
- `schema_version`/`summary_model` (28-29, `optional uint32`/`optional string`, non-pointer sources): assigned unconditionally via `proto.Uint32(...)`/`proto.String(...)`, with an inline comment explaining why (protojson omits an unset optional even under `EmitDefaultValues`). Correct.

**Vacuous-gate sweep (review priority 2):** none of the anti-patterns named in the
review brief (working-tree git diff assertions, `rg -v '^\s*//'` comment-stripping
gates, unreachable branches, `len(x) > 0` in place of set-equality, zero-by-construction
fixtures, `-run` patterns matching nothing) were found in any of the three new test
files. Specifically for `connectapi_parity_test.go`'s reflection detector: the
permanent negative fixture (`negativeFixtureMemory`) and near-miss fixture
(`nearMissFixtureMemory`) genuinely exercise the detector's ability to reject, and the
"every proto field is populated" `Has(fd)` sub-test is paired with a dedicated
"zero-value source" sub-test that is the only place in the suite that would actually
go red if `schema_version`/`summary_model` reverted to a conditional assignment (the
population sub-test alone cannot see that regression, since `Has()` is true for any
assignment including an assigned zero — the file's own comment at
`connectapi_parity_test.go:639-645` states this explicitly and the code backs it up).
The decode-back comparator's coverage is independently pinned
(`assertDecodeBackCoversAllFields`) against all 30 json-visible `store.Memory` fields,
counted and cross-checked by hand against `store.go`'s struct tags — the comparator
list is complete, not partial-and-silently-passing.

**Existing-invariant / consumer sweep (review priority 3):** no other hand-written Go
production code constructs or reflects over `engramv1.Memory` outside
`connectapi.go` (confirmed by search across `internal/` and `cmd/`) — no consumer
assumed the old, smaller field set. `renderJSON` (CLI) is a generic `protojson`
marshal with no hardcoded field list, so it required no update.

**Read-gate safety (review priority 4):** none of the eight new fields —
`schema_version` above all — appear in any recall-gate or authz-filter code path
(`effectiveSearchScope`, `deps.listMemory`, `deps.searchMemory`, etc.); they are
read-and-return-only in this phase, and none of the six write RPCs (`StoreMemoryRequest`,
`ScheduleMemoryRequest`, `UpdateMemoryRequest`, ...) expose them as client-writable
fields, so there is no authz gap either.

No Critical or Warning findings. One Info-level observation below.

## Info

### IN-01: `uint32(m.SchemaVersion)` has no bounds/sign check

**File:** `internal/server/connectapi.go:102`
**Issue:** `m.SchemaVersion` is `migrate.Version`, an `int`-backed type
(`internal/migrate/migrate.go:20`). `proto.Uint32(uint32(m.SchemaVersion))` performs an
unchecked `int`→`uint32` conversion. In every code path that populates
`store.Memory.SchemaVersion` today the value is non-negative and store-controlled
(`migrate.CurrentVersion`, or `0` for legacy/never-set records decoded from a missing
Qdrant payload key), so this is not reachable in practice. If a corrupted or
maliciously-crafted Qdrant payload ever decoded to a negative integer via
`migrate.Version(v.GetIntegerValue())` (`internal/store/store.go:742`), the conversion
would silently wrap to a large positive `uint32` (e.g. `-1` → `4294967295`) rather than
erroring, producing a nonsensical but non-crashing `schema_version` on the wire.
**Fix:** Not urgent given the field's internal-only provenance; if hardening is
desired, clamp or reject negative values at the `store.go` decode site
(`migrate.Version(v.GetIntegerValue())`) rather than at the proto-mapping boundary, so
the invariant "SchemaVersion is never negative" is enforced once, at the point data
enters the type, instead of defensively re-checked at every consumer.

---

_Reviewed: 2026-08-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
