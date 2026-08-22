---
phase: 02-record-schema-versioning-foundation
verified: 2026-08-13T23:55:08Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
deferred:
  - truth: "ROADMAP success criterion 5's explicit 'strictly older than migrate.CurrentVersion' fixture row (older-explicit) does not execute today — migrate.CurrentVersion is 0, so a version literally below it is not representable, and the only reachable older-than proxy is the key-absent legacy record (which decodes to the same v0 as CurrentVersion, not a value strictly below it)."
    addressed_in: "Phase 4"
    evidence: "ROADMAP Phase 4 goal: 'backfill-short-ids is registered as the v0->v1 step' — raises migrate.CurrentVersion to 1, at which point TestSchemaVersionForwardBackwardCompat's runtime-derived row count (2 while CurrentVersion==0, 3 once raised) self-activates the older-explicit row with no code change required in this phase's files."
---

# Phase 2: Record Schema Versioning Foundation Verification Report

**Phase Goal:** Every record carries a `schema_version` discriminator that is wire-visible,
absent-safe (no backfill needed), forward-compatible in both directions, and structurally incapable
of narrowing recall.

**Verified:** 2026-08-13T23:55:08Z
**Status:** passed
**Re-verification:** No — initial verification

## Verification Method

This report does not take SUMMARY.md or 02-REVIEW.md claims as evidence. Every truth below was
checked by reading the actual implementation and test source, and by executing the named tests
directly against a real Qdrant (local Docker, `qdrant/qdrant:v1.18.2`) with `-v -count=1` to observe
individual `--- PASS`/`--- FAIL` lines — never a package-level `ok` alone. For the two most
load-bearing gates (criterion 1's write-boundary AST scan, criterion 4's recall-filter negative), a
RED-proof patch was applied from `red-evidence/`, the test was re-run to observe the FAIL, and the
patch was reverted with `git apply -R` + `git diff --exit-code`, reproducing the phase's own claimed
prove-RED cycle independently rather than trusting the SUMMARY's narration of it.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every write path (store/schedule/supersede/update) stamps `schema_version`, proven 100%, not a sample | ✓ VERIFIED | `internal/store/schemaversion_stamp_gate_test.go`'s `TestEveryPointWriteRoutesThroughPayload`/`TestPartialWritePathsAreClassifiedNonStamping` derive the Upsert/SetPayload/DeletePayload/OverwritePayload call-site set from a non-recursive AST walk of `internal/store`'s real package directory (not a hand-listed fixture) and assert it **set-equal in both directions** against a hand-maintained classification table, with a zero-applicability guard (`filesScanned == 0` → `t.Fatal`) proven to fire on both a nonexistent dir and an empty-match dir. Ran live: `real_package` subtest logs "scanned 7 non-test .go files in internal/store" and passes. Behavioral half `TestEveryFullWriteMethodStampsSchemaVersion` (6 rows, executed-count-checked) proves `Upsert`/`Update`/`Supersede`/scheduled-Upsert all stamp against real Qdrant, and the D-02 partial-write negative (`SetVisibility` on a raw v0 record) proves the key stays absent. |
| 2 | A pre-migration record (no `schema_version` key) reads as v0 by absence, no backfill, fully recallable through every existing path, unchanged | ✓ VERIFIED | `fromPayload` (store.go:741-743): `if v, ok := p[schemaVersionKey]; ok { ... }` — absent key leaves `m.SchemaVersion` at its Go zero value, no panic, no default branch. `TestSchemaVersionForwardBackwardCompat`'s `absent` row raw-deletes the key from both an active and a windowed record and asserts recall membership across `Search`/`SearchReranked`/`SearchDiscovery`/`List`/`Get` (active) and `ListScheduled`/`Get` (windowed) — ran live, passed. |
| 3 | `schema_version` is a plain wire-visible `store.Memory` field (never `json:"-"`), observable on `full=true` recall and `get_memory` | ✓ VERIFIED | `store.go:353`: `SchemaVersion migrate.Version \`json:"schema_version"\`` — no `omitempty`, no `-`, confirmed by direct read and by `TestSchemaVersionOnRecallWire`'s `reflect.StructTag` assertion. `full path carries the field for a zero-versioned record` and `compact view omits the field` subtests both ran and passed, and `TestSchemaVersionOnGetMemoryWire` invokes the actual registered `get_memory` tool handler (not a helper) against a real Qdrant-backed store for both a normal and a legacy (key-absent) record — both passed live. |
| 4 | A negative "recall gate blast radius" test proves `schema_version` never appears in any Qdrant recall/authz filter condition built by `Search`/`SearchReranked`/`SearchDiscovery`/`List`/`ListScheduled` | ✓ VERIFIED | `walkFilter`/`walkCondition` in `schemaversion_recallgate_test.go` recurse through `Must`/`Should`/`MustNot` and all 7 `Condition` oneof variants with an exhaustive type switch (`t.Fatalf` default arm — no silent 8th-variant miss). The captured filter is the **actually-transmitted** `*qdrant.Filter`, obtained via a real `grpc.WithUnaryInterceptor` on a live client dial (not a re-derivation of the builder call graph). Ran live: `TestSchemaVersionNeverGatesRecall` — 15 subtests incl. `ListScopes`, all PASS, "total filters walked: 18; aggregate captured-method multiset: map[Count:4 Query:6 Scroll:8]". Independently reproduced the RED direction for the classification-coverage linkage guard by applying `red-evidence/02-03-red-5-linkage.patch` (removes the `ListScopes` invocation rows while `Store.ListScopes` stays in the entry-point seed set) — `TestRecallEmissionSetIsCompleteAndClassified/classification-coverage_linkage` FAILED as claimed, then reverted clean (`git diff --exit-code` empty). |
| 5 | A binary reads a record whose `schema_version` is NEWER than its own constant without rejecting/hiding/downgrading it, tested in both directions | ⚠️ VERIFIED WITH DISCLOSED SCOPE LIMIT | Newer-than direction: fully proven. `TestSchemaVersionForwardBackwardCompat`'s `newer` row raw-injects `migrate.CurrentVersion + 1` plus two unrecognized payload keys, decodes without error via `Store.Get`, is recalled through every applicable path, and — after `Store.Update` — keeps the raised version while losing the unknown keys (D-06, asserted not assumed). Ran live, passed. **Older-than direction is honestly dormant today**, not silently skipped: `migrate.CurrentVersion == 0` makes a version strictly below it unconstructible, so the only executed proxy is the key-absent legacy record, which decodes to v0 — equal to `CurrentVersion`, not strictly below it. The file discloses this itself (`schemaversion_compat_test.go` doc comment, code-review IN-01) and self-checks it: `expectedRowCount` is derived from `migrate.CurrentVersion` (2 today, 3 once raised) and `olderThanCovered` is a runtime-checked boolean, not a `t.Skip`, so the literal older-than fixture activates automatically the day Phase 4 raises the constant. Live run confirmed exactly 2 rows executed today (`absent`, `newer`), matching the derivation. See Deferred Items below. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified). Truth 5's older-than-literal
sub-case is not independently scorable today because the state it needs (`schema_version <
CurrentVersion`) does not exist in this codebase yet — this is a scope boundary intrinsic to Phase 2
running before Phase 4 raises the constant, not an implementation gap, and it is deferred per Step 9b
below rather than silently marked passed.

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | The literal strictly-older-than-`CurrentVersion` fixture row in `TestSchemaVersionForwardBackwardCompat` | Phase 4 | ROADMAP Phase 4 goal: "`backfill-short-ids` is registered as the v0→v1 step" raises `migrate.CurrentVersion` to 1; the row-count/`olderThanCovered` self-check in `schemaversion_compat_test.go` requires the `older-explicit` row to execute from that point on, with no code change needed in this phase's files. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/migrate/migrate.go` | `Version` named type + `CurrentVersion` constant | ✓ VERIFIED | Present, `CurrentVersion Version = 0`, documented rationale. |
| `internal/migrate/migrate_test.go` | Pins `CurrentVersion`'s value | ✓ VERIFIED | `TestCurrentVersionValue` — ran, passed. |
| `internal/store/store.go` | `Memory.SchemaVersion`, monotonic stamp, absent-safe decode, payload index | ✓ VERIFIED | Lines 353, 646, 741-743, 567-596 (index). |
| `internal/store/schemaversion_test.go` | End-to-end real-Qdrant carry-through proof | ✓ VERIFIED | Exists; `TestSchemaVersionEndToEnd`/`TestEnsureCollectionIndexesSchemaVersion`/`TestUpdateRefreshesSchemaVersionUnderLock` ran, passed. |
| `internal/store/schemaversion_stamp_gate_test.go` | Write-boundary AST gate | ✓ VERIFIED | 994 lines; ran, passed; RED-proof independently reproduced (direction 1). |
| `internal/store/testdata/schemaversionstamp/{good,bad,limits}_pkg.go.txt` | Gate fixtures | ✓ VERIFIED | All 3 present and exercised by the gate test. |
| `internal/store/schemaversion_stamp_test.go` | Behavioral per-write-method stamping proof | ✓ VERIFIED | 216 lines, 6-row table, ran, passed. |
| `internal/store/schemaversion_recallgate_test.go` | Recursive filter-key walker + live gRPC capture | ✓ VERIFIED | 1233 lines; ran, passed; RED-proof independently reproduced (linkage guard). |
| `internal/store/schemaversion_compat_test.go` | Forward/backward decode-compat matrix | ✓ VERIFIED | 541 lines; ran, passed; confirmed exactly 2/2 rows execute at `CurrentVersion==0`. |
| `internal/server/schemaversion_wire_test.go` | Wire-visibility + `get_memory` proof | ✓ VERIFIED | 336 lines; both `TestSchemaVersionOnRecallWire` and `TestSchemaVersionOnGetMemoryWire` ran, passed. |
| `docs-site/src/content/docs/guides/upgrade.md` | Operator-facing rollback-hazard note | ✓ VERIFIED | Section 12 documents the field and the lossy-rewrite rollback hazard. |
| `.planning/phases/.../red-evidence/*.patch` | Reproducible RED-proof patches | ✓ VERIFIED | 9 patches present; 2 sampled (02-02-red-1-bypass, 02-03-red-5-linkage) independently applied, observed FAIL, reverted clean. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/store/store.go` (`payload()`) | `internal/migrate/migrate.go` | `max(migrate.CurrentVersion, m.SchemaVersion)` | ✓ WIRED | store.go:646. |
| `internal/store/schemaversion_recallgate_test.go` | `internal/store/store.go` | `grpc.WithUnaryInterceptor` dial option capturing the transmitted filter | ✓ WIRED | Confirmed live capture (18 filters walked across 15 subtests). |
| `internal/server/schemaversion_wire_test.go` | `internal/server` recall shaping | `shapeRecall(full=true)` / `toRecallView` | ✓ WIRED | Both paths exercised and diverge as designed. |
| `internal/server/schemaversion_wire_test.go` | `internal/store/store.go` | `reflect.StructTag` lookup of `SchemaVersion`'s json tag | ✓ WIRED | Confirmed exact tag `schema_version`. |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Write-boundary AST gate (structural) | `go test ./internal/store/... -run 'TestEveryPointWriteRoutesThroughPayload$' -v -count=1` | 8/8 subtests PASS, "scanned 7 non-test .go files" | ✓ PASS |
| Partial-write classification gate | `-run 'TestPartialWritePathsAreClassifiedNonStamping$'` | PASS, "scanned 7 non-test .go files" | ✓ PASS |
| Cross-package `*qdrant.Client` guard | `-run 'TestQdrantClientIsHeldOnlyByStorePackage$'` | PASS, "scanned 97 non-test .go files across the module" | ✓ PASS |
| Pure-function monotonic stamp (incl. the WR-01-fixed "below" case) | `-run 'TestPayloadRoundTripsSchemaVersion$'` | 4/4 subtests PASS (above/equal/zero/below) | ✓ PASS |
| Behavioral full-write stamping table | `-run 'TestEveryFullWriteMethodStampsSchemaVersion$'` | 6/6 subtests PASS against real Qdrant | ✓ PASS |
| Forward/backward compat matrix | `-run 'TestSchemaVersionForwardBackwardCompat$'` | 2/2 subtests PASS, "executed compatibility rows: [absent newer] (expected 2, derived from migrate.CurrentVersion=0)" | ✓ PASS |
| Recall-gate negative (criterion 4) | `-run 'TestSchemaVersionNeverGatesRecall$'` | 15/15 subtests PASS, 18 filters walked live | ✓ PASS |
| Recall emission completeness/classification | `-run 'TestRecallEmissionSetIsCompleteAndClassified$'` | PASS baseline; FAILED under RED-proof injection; reverted clean | ✓ PASS |
| Wire visibility (full/compact/get_memory) | `-run 'TestSchemaVersionOnRecallWire\|TestSchemaVersionOnGetMemoryWire'` | 5/5 + 2/2 subtests PASS | ✓ PASS |
| Reindex non-advancement regression | `-run 'TestReindexRoundtrip$'` | PASS | ✓ PASS |
| RED-proof reproduction (bypass write) | `git apply red-evidence/02-02-red-1-bypass.patch && go test ... && git apply -R ...` | FAIL observed as claimed, then reverted clean (`git diff --exit-code`) | ✓ PASS |
| RED-proof reproduction (classification-coverage linkage) | `git apply red-evidence/02-03-red-5-linkage.patch && go test ... && git apply -R ...` | FAIL observed as claimed, then reverted clean | ✓ PASS |
| Full package regression | `go test ./internal/store/... ./internal/server/... ./internal/migrate/... -count=1` | `ok` all three packages (23.5s/12.0s/0.05s) | ✓ PASS |
| Build/vet | `go build ./...`, `go vet ./internal/{migrate,store,server}/...` | clean | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REQ-schema-version-stamped | 02-01, 02-02 | Every write path stamps current version; legacy reads as v0 | ✓ SATISFIED | Truths 1 & 2 above. |
| REQ-schema-version-never-gates-recall | 02-03 | `schema_version` never appears in a recall/authz filter | ✓ SATISFIED | Truth 4 above. |
| REQ-schema-version-wire-visible | 02-01, 02-05 | Wire-visible field, not `json:"-"` | ✓ SATISFIED | Truth 3 above. |
| REQ-schema-version-forward-compatible | 02-04 | Newer/older-than-binary compatibility, both directions | ✓ SATISFIED (older-than scope-limited — see deferred item) | Truth 5 above. |

**Discrepancy noted (documentation only, not a code gap):** `.planning/REQUIREMENTS.md` still shows
`REQ-schema-version-forward-compatible` as an unchecked `[ ]` box and "Pending" in its traceability
table row (lines 25, 96), even though `internal/store/schemaversion_compat_test.go` implements and
passes the proof, and `02-04-SUMMARY.md` records `requirements-completed:
[REQ-schema-version-forward-compatible]`. `git log -- .planning/REQUIREMENTS.md` shows the file's last
update was commit `1bd07bf6` ("complete recall-gate proof plan", i.e. through plan 02-03) — plans
02-04 and 02-05 never updated it. This is a tracking-artifact staleness issue, not evidence the
requirement itself is unmet (the code and tests independently confirm it is); recommend a follow-up
commit to check the box and update the status table before shipping the milestone.

### Anti-Patterns Found

None. Scanned every file this phase created/modified
(`internal/migrate/{migrate,migrate_test}.go`, `internal/store/store.go`,
`internal/store/store_test.go`, all five `internal/store/schemaversion_*.go` files,
`internal/server/schemaversion_wire_test.go`, `docs-site/.../upgrade.md`) for
`TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet implemented|coming soon` — zero matches.

### Human Verification Required

None. Every ROADMAP success criterion is either fully proven by an executed, independently-verified
test against real Qdrant, or (criterion 5's older-than-literal sub-case) honestly disclosed as
scope-dormant with a self-activating check, which is a documentation/scheduling matter rather than an
ambiguous behavior needing a human's judgment call.

### Gaps Summary

No blocking gaps. All five ROADMAP success criteria are met in the codebase today, verified by
reading the implementation, executing the named tests directly (not trusting SUMMARY/REVIEW
narration), and independently reproducing two of the phase's RED-proof cycles from their committed
patches. The one caveat (criterion 5's older-than-literal direction) is an intrinsic, disclosed,
self-checking scope boundary tied to `migrate.CurrentVersion` still being 0 in this phase — it is
recorded as a deferred item addressed by Phase 4, not scored as a failure. The one non-blocking
finding is `.planning/REQUIREMENTS.md`'s stale checkbox/status-table entry for
`REQ-schema-version-forward-compatible`, a documentation-sync issue with no bearing on the actual
implementation.

---

_Verified: 2026-08-13T23:55:08Z_
_Verifier: Claude (gsd-verifier)_
