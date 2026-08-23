---
phase: 02-record-schema-versioning-foundation
reviewed: 2026-08-13T23:46:45Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - internal/migrate/migrate.go
  - internal/migrate/migrate_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - internal/store/schemaversion_test.go
  - internal/store/schemaversion_stamp_gate_test.go
  - internal/store/schemaversion_stamp_test.go
  - internal/store/schemaversion_compat_test.go
  - internal/store/schemaversion_recallgate_test.go
  - internal/store/testdata/schemaversionstamp/good_pkg.go.txt
  - internal/store/testdata/schemaversionstamp/bad_pkg.go.txt
  - internal/store/testdata/schemaversionstamp/limits_pkg.go.txt
  - internal/server/schemaversion_wire_test.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-08-13T23:46:45Z
**Depth:** standard
**Files Reviewed:** 13 (plus `docs-site/src/content/docs/guides/upgrade.md`, prose-only, no findings)
**Status:** issues_found

## Summary

This phase adds a `schema_version` discriminator to `store.Memory`, threads it through
`payload()`/`fromPayload()` with a monotonic `max(migrate.CurrentVersion, existing)` stamp, and
backs the four ROADMAP success criteria with dedicated proof suites: a write-boundary AST gate
(`schemaversion_stamp_gate_test.go`), a behavioral stamping table (`schemaversion_stamp_test.go`),
a forward/backward decode-compatibility matrix (`schemaversion_compat_test.go`), a recursive
filter-key walker plus a live gRPC-interceptor capture proving `schema_version` never gates recall
(`schemaversion_recallgate_test.go`), and wire-visibility pins (`schemaversion_wire_test.go`).

Given the reviewer's mandate to specifically hunt for vacuous-gate shapes: I traced every gate's
derived set against the actual source by hand (`rg` cross-checks of `.Upsert(`,
`.SetPayload(`/`.DeletePayload(`/`.OverwritePayload(` call sites in `internal/store/*.go`
excluding `_test.go`), and every one matches the hand-maintained classification tables the tests
assert set-equality against — 4/4 full-write sites, 10/10 partial-write sites. `walkFilterKeys`
recurses through `Must`/`Should`/`MustNot` and every `Condition` oneof variant (verified against
the pinned go-client's 7 variants with a `t.Fatalf` default arm for an 8th), and its "nested Should
group" subtest specifically targets `categoryMatchCondition`'s wrapped-OR shape — the exact
top-level-only blind spot named in the review priorities. `TestSchemaVersionNeverGatesRecall`
captures the *actually-transmitted* `*qdrant.Filter` via a gRPC interceptor against a live Qdrant,
not a re-derivation of the filter-builder call graph, closing the "proves what it claims" gap this
project has previously shipped. I found no evidence of a vacuous gate, a weak `len > 0` assertion
standing in for set equality, or an escape hatch materially wider than what each test file's own
doc comment discloses.

Production-code correctness (`payload()`'s monotonic stamp, `fromPayload()`'s absent-safe decode,
`Store.Update`'s in-lock refresh of `cur.SchemaVersion`, `Store.Reindex`'s verbatim-copy exception)
matches the documented design decisions in the phase context and is exercised end-to-end against
real Qdrant. `go build`, `go vet`, and `task license:check` are clean for the reviewed set.

The one WARNING below is a real, if narrow, coverage gap the review priorities specifically asked
about (assertions "hard-coded to a literal where they must derive from `migrate.CurrentVersion`").
The two INFO items are pre-existing, disclosed limitations, noted for completeness rather than as
defects.

## Warnings

### WR-01: `TestPayloadRoundTripsSchemaVersion` has no case exercising `m.SchemaVersion < migrate.CurrentVersion`

**File:** `internal/store/store_test.go:2989-3005`
**Issue:** The three cases (`above`, `equal`, `zero`) exercise `SchemaVersion > CurrentVersion`,
`SchemaVersion == CurrentVersion`, and the zero-valued `Memory{}` (which is also `== CurrentVersion`
today). There is no case where a decoded record's `SchemaVersion` is strictly *below*
`migrate.CurrentVersion` and `payload()`'s `max()` must raise it. This is currently
unreachable-in-practice because `migrate.CurrentVersion == 0` and `migrate.Version` is unsigned in
spirit (no negative version is ever produced by a legitimate write path), so the gap has zero
practical exposure today. But the day `migrate.CurrentVersion` is raised above 0 (explicitly
anticipated by this very file's own doc comment: "keeps its meaning when a later phase raises the
constant"), the single most safety-critical branch of the monotonic stamp — an OLD record read by a
NEW binary getting bumped up to current — will have shipped with zero direct pure-function coverage
in this table. `schemaversion_compat_test.go`'s `TestSchemaVersionForwardBackwardCompat` does
partially cover this via its `postUpdateVersion` field once `migrate.CurrentVersion > 0` activates
the `older-explicit` row (store_test.go's own compat file, lines 68-74) — but that is an
integration-level proof requiring live Qdrant, not the pure-function unit proof this file's other
two cases already establish, and it is not derived from this file's stated case-count assertion
(`len(cases) != 3`), which will silently stay "3, correct" forever even after `CurrentVersion` is
raised and this exact case remains absent.
**Fix:** Add a fourth case that decodes a record whose stored version is below whatever
`migrate.CurrentVersion` will be, by exercising the "below" direction independent of the constant's
current value — e.g. seed the input via `fromPayload` on a raw payload with
`schema_version: int(migrate.CurrentVersion) - 1` when `migrate.CurrentVersion > 0`, or (to keep
coverage non-vacuous even while the constant is 0) assert directly on `payload()`'s `max()` helper
with an explicit below/above/equal table independent of `Memory`, so the "below" arm is proven at
the unit level regardless of the constant's current value:
```go
// below: a stored version lower than CurrentVersion must be raised, never left as-is.
{"below", Memory{ID: "...", Content: "c", Scope: "s", SchemaVersion: migrate.Version(-1)}, migrate.CurrentVersion},
```
(Using a synthetic negative sentinel today, since `CurrentVersion == 0` leaves no valid non-negative
"below" value to construct — swap to `migrate.CurrentVersion - 1` once a future phase raises the
constant, mirroring `schemaversion_compat_test.go`'s own `older-explicit` row pattern.)

## Info

### IN-01: `TestSchemaVersionForwardBackwardCompat`'s "older-than-binary" direction is dormant until a future phase raises `migrate.CurrentVersion`

**File:** `internal/store/schemaversion_compat_test.go:57-118`
**Issue:** This is disclosed accurately in the file's own doc comment ("The genuine older-than-binary
state at CurrentVersion == 0 is the key-absent legacy record — CurrentVersion - 1 is not
representable") and the `olderThanCovered` check does fail loudly if a future raise of the constant
isn't paired with wiring the `older-explicit` row's execution. Recorded here only so the reviewer's
mandate to "hunt for assertions hard-coded to a literal where they must derive from
`migrate.CurrentVersion`" is answered explicitly: this one is NOT hard-coded — every expectation in
this file is derived from the constant, and the row-activation logic is itself checked
(`len(rows) != expectedRowCount` fails the test if the derivation and the literal 2/3 disagree). No
action needed; not a defect.
**Fix:** None required — informational only.

### IN-02: `Store.Upsert`'s public replacement-by-id path can lower a stored `schema_version` when called with a stale `Memory`

**File:** `internal/store/store.go:353-352` (doc comment on `Memory.SchemaVersion`), confirmed at
`internal/store/store.go:793-816` (`Store.Upsert`)
**Issue:** This is explicitly disclosed in the `SchemaVersion` field's own doc comment ("Store.Upsert
is public replacement-by-id and does NOT read the stored record before writing, so a caller holding
a stale Memory CAN lower a stored version through it — that is outside the guarantee"). Confirmed
correct by inspection: `Store.Upsert` calls `payload(m)` directly with no `Get`-and-merge step,
unlike `Store.Update`'s in-lock re-read. This is a pre-existing, general property of `Upsert` being a
raw replace-by-id primitive (not specific to `schema_version` — a stale `Memory` can just as easily
regress `Tags`, `Summary`, etc.), and the phase context explicitly names this as a deliberate,
considered-and-rejected scope boundary ("A stored-version read or compare-and-swap on Upsert was
considered and rejected for this phase"). Recorded for completeness only, since it is exactly the
kind of narrowed-guarantee statement the review priorities asked to be checked for accuracy — it is
accurate.
**Fix:** None required — informational only; matches documented, deliberate scope.

---

_Reviewed: 2026-08-13T23:46:45Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
